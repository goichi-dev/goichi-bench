// Package loadgen drives a running server over real HTTP and records
// throughput and latency percentiles.
package loadgen

import (
	"sync"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/goichi-dev/goichi-bench/internal/clock"
)

// Config describes one load run against one endpoint.
type Config struct {
	Host        string
	Connections int
	Warmup      time.Duration
	Duration    time.Duration
	Method      string
	Path        string
	Body        string

	// BeforeMeasure and AfterMeasure bracket the measured window only, so a
	// caller can sample counters without the warmup polluting them.
	BeforeMeasure func()
	AfterMeasure  func()
}

// Result is what one load run measured.
type Result struct {
	Requests int64
	Errors   int64
	NonOK    int64
	BytesIn  int64
	Elapsed  time.Duration
	RPS      float64
	P50      time.Duration
	P90      time.Duration
	P99      time.Duration
	Max      time.Duration
}

// NewClient returns a client for one host. Build it once per server and reuse
// it for every scenario and repeat: a fresh pool per measurement re-opens every
// connection, and the sockets left in TIME_WAIT behind it are enough to stall a
// later measurement on the same host.
func NewClient(host string, connections int) *fasthttp.HostClient {
	return &fasthttp.HostClient{
		Addr:                          host,
		MaxConns:                      connections * 2,
		MaxIdleConnDuration:           10 * time.Minute,
		MaxConnDuration:               0,
		DisableHeaderNamesNormalizing: true,
		ReadTimeout:                   10 * time.Second,
		WriteTimeout:                  10 * time.Second,
	}
}

// Run issues requests from Connections goroutines for Warmup (discarded) then
// Duration (recorded), and returns the aggregate.
func Run(client *fasthttp.HostClient, cfg Config) Result {
	uri := "http://" + cfg.Host + cfg.Path

	if cfg.Warmup > 0 {
		runPhase(client, cfg, uri, cfg.Warmup)
	}

	if cfg.BeforeMeasure != nil {
		cfg.BeforeMeasure()
	}
	start := time.Now()
	hists := runPhase(client, cfg, uri, cfg.Duration)
	elapsed := time.Since(start)
	if cfg.AfterMeasure != nil {
		cfg.AfterMeasure()
	}

	total := newHistogram()
	var res Result
	for _, h := range hists {
		total.merge(h)
		res.Requests += h.requests
		res.Errors += h.errors
		res.NonOK += h.nonOK
		res.BytesIn += h.bytesIn
	}

	res.Elapsed = elapsed
	if elapsed > 0 {
		res.RPS = float64(res.Requests) / elapsed.Seconds()
	}
	res.P50 = total.quantile(0.50)
	res.P90 = total.quantile(0.90)
	res.P99 = total.quantile(0.99)
	res.Max = total.max
	return res
}

func runPhase(client *fasthttp.HostClient, cfg Config, uri string, d time.Duration) []*histogram {
	deadline := time.Now().Add(d)
	hists := make([]*histogram, cfg.Connections)

	var wg sync.WaitGroup
	for i := 0; i < cfg.Connections; i++ {
		h := newHistogram()
		hists[i] = h
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(client, cfg, uri, deadline, h)
		}()
	}
	wg.Wait()
	return hists
}

func worker(client *fasthttp.HostClient, cfg Config, uri string, deadline time.Time, h *histogram) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(uri)
	req.Header.SetMethod(cfg.Method)
	if cfg.Body != "" {
		req.SetBodyString(cfg.Body)
		req.Header.SetContentType("application/json")
	}

	// Check the wall clock every few requests instead of every one; the
	// per-request timing uses the high-resolution clock.
	const clockEvery = 16
	for n := 0; ; n++ {
		if n%clockEvery == 0 && time.Now().After(deadline) {
			return
		}

		resp.Reset()
		t0 := clock.Now()
		err := client.Do(req, resp)
		lat := clock.Since(t0)

		h.requests++
		if err != nil {
			h.errors++
			continue
		}
		if resp.StatusCode() != fasthttp.StatusOK {
			h.nonOK++
		}
		h.bytesIn += int64(len(resp.Body()))
		h.record(lat)
	}
}
