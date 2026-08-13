// Command goichi-bench measures goichi against gin, fiber, chi, echo and
// atreugo on the same set of REST scenarios, and writes one JSON result file.
//
//	go run ./cmd/goichi-bench -out results/latest.json
//
// Every framework is measured in its own process, one at a time.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/goichi-dev/goichi-bench/internal/apps"
	"github.com/goichi-dev/goichi-bench/internal/loadgen"
	"github.com/goichi-dev/goichi-bench/internal/procstat"
	"github.com/goichi-dev/goichi-bench/internal/result"
	"github.com/goichi-dev/goichi-bench/internal/timerres"
)

func main() {
	var (
		list        = flag.String("frameworks", strings.Join(apps.Names(), ","), "comma-separated frameworks to measure")
		phase       = flag.String("phase", "all", "all | micro | http")
		connections = flag.Int("connections", 64, "concurrent connections in the HTTP phase")
		duration    = flag.Duration("duration", 5*time.Second, "measured window per scenario")
		warmup      = flag.Duration("warmup", 2*time.Second, "discarded window before each measurement")
		benchtime   = flag.Duration("benchtime", time.Second, "measured window per in-process benchmark")
		repeat      = flag.Int("repeat", 3, "how many times each HTTP scenario is measured; the median is kept")
		portBase    = flag.Int("port-base", 18080, "first TCP port used by the servers")
		out         = flag.String("out", "results/latest.json", "where to write the result JSON")
	)
	flag.Parse()

	// Held for the whole run, before anything is measured: the timer period is
	// system-wide, and a run that loses it part-way measures the machine's idle
	// behaviour rather than the framework's.
	defer timerres.Pin()()

	frameworks := splitList(*list)
	if len(frameworks) == 0 {
		exit(fmt.Errorf("no frameworks selected"))
	}
	for _, name := range frameworks {
		if _, err := apps.Build(name); err != nil {
			exit(err)
		}
	}

	serverCPUs := runtime.NumCPU() / 2
	if serverCPUs < 1 {
		serverCPUs = 1
	}
	clientCPUs := runtime.NumCPU() - serverCPUs
	if clientCPUs < 1 {
		clientCPUs = 1
	}
	// The load generator and the server under test share this machine. Pinning
	// each to half the cores keeps one from starving the other, which is what
	// makes repeated runs comparable at all.
	runtime.GOMAXPROCS(clientCPUs)

	worker, cleanup, err := buildWorker()
	if err != nil {
		exit(err)
	}
	defer cleanup()

	run := &result.Run{
		Schema: result.Schema,
		RunAt:  time.Now(),
		Env: result.Env{
			GoVersion:  runtime.Version(),
			OS:         runtime.GOOS,
			Arch:       runtime.GOARCH,
			NumCPU:     runtime.NumCPU(),
			CPUModel:   cpuModel(),
			Versions:   frameworkVersions(),
			ServerCPUs: serverCPUs,
			ClientCPUs: clientCPUs,

			TimerPeriodNs: timerPeriodNs(),
		},
		Load: result.LoadCfg{
			Connections: *connections,
			WarmupSec:   warmup.Seconds(),
			DurationSec: duration.Seconds(),
			Host:        "127.0.0.1",
		},
	}

	doHTTP := *phase == "all" || *phase == "http"
	doMicro := *phase == "all" || *phase == "micro"

	for i, name := range frameworks {
		fmt.Printf("\n=== %s ===\n", name)

		if doHTTP {
			addr := "127.0.0.1:" + strconv.Itoa(*portBase+i)
			gates, loads, err := httpPhase(worker, name, addr, serverCPUs, *repeat, loadgen.Config{
				Host:        addr,
				Connections: *connections,
				Warmup:      *warmup,
				Duration:    *duration,
			})
			if err != nil {
				exit(fmt.Errorf("%s: %w", name, err))
			}
			run.Gate = append(run.Gate, gates...)
			run.HTTP = append(run.HTTP, loads...)
		}

		if doMicro {
			ms, err := microPhase(worker, name, serverCPUs, *benchtime)
			if err != nil {
				exit(fmt.Errorf("%s: %w", name, err))
			}
			run.Micro = append(run.Micro, ms...)
			for _, m := range ms {
				if !m.Supported {
					fmt.Printf("  micro %-8s skipped (%s)\n", m.Scenario, m.Note)
					continue
				}
				fmt.Printf("  micro %-8s %8.0f ns/op  %4d allocs/op  %6d B/op\n",
					m.Scenario, m.NsPerOp, m.AllocsPerOp, m.BytesPerOp)
			}
		}
	}

	if err := result.Save(*out, run); err != nil {
		exit(err)
	}
	fmt.Printf("\nwrote %s\n", *out)

	if failed := gateFailures(run); len(failed) > 0 {
		fmt.Fprintf(os.Stderr, "\ncorrectness gate FAILED for %d framework/scenario pairs:\n", len(failed))
		for _, g := range failed {
			fmt.Fprintf(os.Stderr, "  %-8s %-8s status=%d got=%q want=%q\n",
				g.Framework, g.Scenario, g.Status, g.Got, g.Want)
		}
		os.Exit(1)
	}
}

// httpPhase starts one framework in a child process, checks that it answers
// every scenario correctly, then loads it.
func httpPhase(worker, name, addr string, cpus, repeat int, base loadgen.Config) ([]result.Gate, []result.HTTP, error) {
	cmd := exec.Command(worker, "-framework", name, "-phase", "serve", "-addr", addr)
	cmd.Env = append(os.Environ(), "GOMAXPROCS="+strconv.Itoa(cpus))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	if err := waitReady(addr, 15*time.Second); err != nil {
		return nil, nil, err
	}

	gates := make([]result.Gate, 0, len(apps.Scenarios))
	for _, sc := range apps.Scenarios {
		gates = append(gates, checkScenario(addr, name, sc))
	}

	serverPID := cmd.Process.Pid
	clientPID := os.Getpid()

	client := loadgen.NewClient(addr, base.Connections)

	loads := make([]result.HTTP, 0, len(apps.Scenarios))
	for _, sc := range apps.Scenarios {
		reps := make([]result.HTTP, 0, repeat)
		for i := 0; i < repeat; i++ {
			reps = append(reps, measureOnce(client, base, sc, name, serverPID, clientPID))
		}
		rec := median(reps)
		rec.Repeats = repeat
		rec.SpreadPct = spreadPct(reps)

		fmt.Printf("  http  %-8s %10.0f req/s ±%.1f%%  p50 %6.3fms  p99 %6.3fms  srv %5.1fµs/req (%.1f cores)  err %d\n",
			sc.Key, rec.RPS, rec.SpreadPct, rec.P50ms, rec.P99ms, rec.ServerCPUus, rec.ServerCores,
			rec.Errors+rec.NonOK)
		loads = append(loads, rec)
	}
	return gates, loads, nil
}

// measureOnce runs one load window and records what the server spent on it.
func measureOnce(client *fasthttp.HostClient, base loadgen.Config, sc apps.Scenario, name string, serverPID, clientPID int) result.HTTP {
	cfg := base
	cfg.Method, cfg.Path, cfg.Body = sc.Method, sc.Path, sc.Body

	var serverBefore, clientBefore procstat.CPU
	var serverAfter, clientAfter procstat.CPU
	var haveCPU bool
	cfg.BeforeMeasure = func() {
		serverBefore, haveCPU = procstat.Read(serverPID)
		clientBefore, _ = procstat.Read(clientPID)
	}
	cfg.AfterMeasure = func() {
		serverAfter, _ = procstat.Read(serverPID)
		clientAfter, _ = procstat.Read(clientPID)
	}

	r := loadgen.Run(client, cfg)

	var serverCPUus, clientCPUus, serverCores, clientCores float64
	if haveCPU && r.Requests > 0 {
		serverCPU := serverAfter.Total() - serverBefore.Total()
		clientCPU := clientAfter.Total() - clientBefore.Total()
		serverCPUus = float64(serverCPU.Microseconds()) / float64(r.Requests)
		clientCPUus = float64(clientCPU.Microseconds()) / float64(r.Requests)
		serverCores = serverCPU.Seconds() / r.Elapsed.Seconds()
		clientCores = clientCPU.Seconds() / r.Elapsed.Seconds()
	}

	return result.HTTP{
		Framework:   name,
		Scenario:    sc.Key,
		RPS:         r.RPS,
		Requests:    r.Requests,
		Errors:      r.Errors,
		NonOK:       r.NonOK,
		P50ms:       ms(r.P50),
		P90ms:       ms(r.P90),
		P99ms:       ms(r.P99),
		MaxMs:       ms(r.Max),
		BytesIn:     r.BytesIn,
		ServerCPUus: serverCPUus,
		ServerCores: serverCores,
		ClientCPUus: clientCPUus,
		ClientCores: clientCores,
	}
}

// median returns the repeat with the middle throughput, keeping that run's own
// latency and CPU figures rather than averaging numbers from different runs.
func median(reps []result.HTTP) result.HTTP {
	sorted := slices.Clone(reps)
	slices.SortFunc(sorted, func(a, b result.HTTP) int {
		switch {
		case a.RPS < b.RPS:
			return -1
		case a.RPS > b.RPS:
			return 1
		default:
			return 0
		}
	})
	return sorted[len(sorted)/2]
}

// spreadPct is how far apart the fastest and slowest repeats were, relative to
// the median: the size of this host's own noise.
func spreadPct(reps []result.HTTP) float64 {
	if len(reps) < 2 {
		return 0
	}
	lo, hi := reps[0].RPS, reps[0].RPS
	for _, r := range reps[1:] {
		lo = min(lo, r.RPS)
		hi = max(hi, r.RPS)
	}
	mid := median(reps).RPS
	if mid == 0 {
		return 0
	}
	return (hi - lo) / mid * 100
}

func microPhase(worker, name string, cpus int, benchtime time.Duration) ([]result.Micro, error) {
	cmd := exec.Command(worker, "-framework", name, "-phase", "micro", "-benchtime", benchtime.String())
	cmd.Env = append(os.Environ(), "GOMAXPROCS="+strconv.Itoa(cpus))
	cmd.Stderr = os.Stderr
	stdout, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var ms []result.Micro
	if err := json.Unmarshal(stdout, &ms); err != nil {
		return nil, fmt.Errorf("decoding worker output: %w", err)
	}
	return ms, nil
}

// checkScenario issues one request and compares it with what every framework
// is required to return. Timing a framework that answers something else would
// be meaningless, so this runs before the load.
func checkScenario(addr, name string, sc apps.Scenario) result.Gate {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI("http://" + addr + sc.Path)
	req.Header.SetMethod(sc.Method)
	if sc.Body != "" {
		req.SetBodyString(sc.Body)
		req.Header.SetContentType("application/json")
	}

	g := result.Gate{Framework: name, Scenario: sc.Key, Want: apps.ExpectedBody(sc.Key)}
	if err := fasthttp.Do(req, resp); err != nil {
		g.Got = err.Error()
		return g
	}
	g.Status = resp.StatusCode()
	got := strings.TrimSpace(string(resp.Body()))
	g.OK = g.Status == fasthttp.StatusOK && got == g.Want
	if !g.OK {
		g.Got = got
	}
	return g
}

func waitReady(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, _, err := fasthttp.Get(nil, "http://"+addr+"/ping")
		if err == nil && status == fasthttp.StatusOK {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("server at %s did not become ready", addr)
}

// buildWorker compiles the worker once, into a temporary directory that is
// removed when the run ends.
func buildWorker() (path string, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "goichi-bench-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup = func() { _ = os.RemoveAll(dir) }

	path = filepath.Join(dir, "worker")
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", path, "./cmd/worker")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("building worker: %w", err)
	}
	return path, cleanup, nil
}

// frameworkVersions reads the module versions this binary was built with, so
// the report always names the exact versions that were measured.
func frameworkVersions() map[string]string {
	want := map[string]string{
		"github.com/goichi-dev/goichi":   "goichi",
		"github.com/gin-gonic/gin":       "gin",
		"github.com/gofiber/fiber/v3":    "fiber",
		"github.com/go-chi/chi/v5":       "chi",
		"github.com/labstack/echo/v4":    "echo",
		"github.com/savsgio/atreugo/v11": "atreugo",
	}
	out := map[string]string{}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return out
	}
	for _, dep := range info.Deps {
		if name, ok := want[dep.Path]; ok {
			out[name] = dep.Version
		}
	}
	return out
}

func timerPeriodNs() int64 {
	if p, ok := timerres.Current(); ok {
		return int64(p)
	}
	return 0
}

func cpuModel() string {
	if v := os.Getenv("PROCESSOR_IDENTIFIER"); v != "" {
		return v
	}
	if b, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "model name") {
				if _, v, ok := strings.Cut(line, ":"); ok {
					return strings.TrimSpace(v)
				}
			}
		}
	}
	return ""
}

func gateFailures(run *result.Run) []result.Gate {
	var out []result.Gate
	for _, g := range run.Gate {
		if !g.OK {
			out = append(out, g)
		}
	}
	return out
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func exit(err error) {
	fmt.Fprintln(os.Stderr, "goichi-bench:", err)
	os.Exit(1)
}
