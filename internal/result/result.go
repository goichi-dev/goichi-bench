// Package result defines the on-disk shape of a benchmark run. The charts and
// the report are generated from this file and nothing else, so a run can be
// re-rendered without re-measuring.
package result

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Schema is the version of the result document. Bump it when a field changes
// meaning, so old result files are not silently mixed with new ones.
const Schema = "goichi-bench/v1"

// Run is one complete benchmark run.
type Run struct {
	Schema string    `json:"schema"`
	RunAt  time.Time `json:"run_at"`
	Env    Env       `json:"env"`
	Load   LoadCfg   `json:"load_config"`
	Gate   []Gate    `json:"correctness_gate"`
	Micro  []Micro   `json:"micro"`
	HTTP   []HTTP    `json:"http"`
}

// Env records the machine and the exact framework versions measured.
type Env struct {
	GoVersion  string            `json:"go_version"`
	OS         string            `json:"os"`
	Arch       string            `json:"arch"`
	NumCPU     int               `json:"num_cpu"`
	CPUModel   string            `json:"cpu_model,omitempty"`
	Versions   map[string]string `json:"framework_versions"`
	ServerCPUs int               `json:"server_gomaxprocs"`
	ClientCPUs int               `json:"client_gomaxprocs"`

	// TimerPeriodNs is the system timer period in force during the run, where
	// the platform has one. It is recorded because a coarse period inflates
	// waiting time without changing CPU per request, which is the difference
	// between a run that measures frameworks and one that measures the timer.
	TimerPeriodNs int64 `json:"timer_period_ns,omitempty"`
}

// LoadCfg records how the HTTP phase was driven.
type LoadCfg struct {
	Connections int     `json:"connections"`
	WarmupSec   float64 `json:"warmup_sec"`
	DurationSec float64 `json:"duration_sec"`
	Host        string  `json:"host"`
}

// Gate is the result of the correctness check for one framework and scenario.
// A framework that fails the gate is excluded from the charts.
type Gate struct {
	Framework string `json:"framework"`
	Scenario  string `json:"scenario"`
	Status    int    `json:"status"`
	OK        bool   `json:"ok"`
	Got       string `json:"got,omitempty"`
	Want      string `json:"want,omitempty"`
}

// Micro is one in-process routing measurement.
type Micro struct {
	Framework   string  `json:"framework"`
	Scenario    string  `json:"scenario"`
	Supported   bool    `json:"supported"`
	Note        string  `json:"note,omitempty"`
	NsPerOp     float64 `json:"ns_per_op"`
	AllocsPerOp int64   `json:"allocs_per_op"`
	BytesPerOp  int64   `json:"bytes_per_op"`
	Iterations  int     `json:"iterations"`
}

// HTTP is one end-to-end load measurement.
type HTTP struct {
	Framework string  `json:"framework"`
	Scenario  string  `json:"scenario"`
	RPS       float64 `json:"rps"`
	Requests  int64   `json:"requests"`
	Errors    int64   `json:"errors"`
	NonOK     int64   `json:"non_ok"`
	P50ms     float64 `json:"p50_ms"`
	P90ms     float64 `json:"p90_ms"`
	P99ms     float64 `json:"p99_ms"`
	MaxMs     float64 `json:"max_ms"`
	BytesIn   int64   `json:"bytes_in"`

	// ServerCPUus is the server's own CPU time per request. Unlike RPS it does
	// not stop discriminating once the loopback interface saturates, so it is
	// the primary cross-framework number on a saturated host.
	ServerCPUus float64 `json:"server_cpu_us_per_req"`
	// ServerCores is how many cores the server kept busy on average.
	ServerCores float64 `json:"server_cores_busy"`
	// ClientCPUus is the load generator's own cost per request, recorded so a
	// run where the client was the bottleneck can be recognised as one.
	ClientCPUus float64 `json:"client_cpu_us_per_req"`
	// ClientCores is how many cores the load generator kept busy on average.
	ClientCores float64 `json:"client_cores_busy"`

	// Repeats is how many times this scenario was measured; the record holds
	// the median run.
	Repeats int `json:"repeats"`
	// SpreadPct is the gap between the fastest and slowest repeat, as a
	// percentage of the median. It says how much of a difference between two
	// frameworks is just noise on this host.
	SpreadPct float64 `json:"spread_pct"`
}

// Save writes the run to path, creating parent directories as needed.
func Save(path string, run *Run) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// Load reads a run written by Save.
func Load(path string) (*Run, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var run Run
	if err := json.Unmarshal(b, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// MicroFor returns the in-process measurement for one framework and scenario.
func (r *Run) MicroFor(framework, scenario string) (Micro, bool) {
	for _, m := range r.Micro {
		if m.Framework == framework && m.Scenario == scenario {
			return m, true
		}
	}
	return Micro{}, false
}

// HTTPFor returns the load measurement for one framework and scenario.
func (r *Run) HTTPFor(framework, scenario string) (HTTP, bool) {
	for _, h := range r.HTTP {
		if h.Framework == framework && h.Scenario == scenario {
			return h, true
		}
	}
	return HTTP{}, false
}
