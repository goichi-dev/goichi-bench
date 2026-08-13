// Command compare diffs two result files and fails when a framework got worse.
//
//	go run ./cmd/compare -base results/baseline.json -new results/latest.json
//
// This is the regression gate: record a baseline before a change, re-run the
// benchmark after it, and let the exit code say whether the change cost
// anything.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/goichi-dev/goichi-bench/internal/apps"
	"github.com/goichi-dev/goichi-bench/internal/result"
)

// metric is one comparable number, with the tolerance that separates noise
// from a real regression on this metric.
type metric struct {
	name         string
	unit         string
	higherBetter bool
	// tolerance is the fraction a value may move in the wrong direction before
	// it counts as a regression.
	tolerance float64
	micro     func(result.Micro) float64
	http      func(result.HTTP) float64
}

func metrics(timingTolerance float64) []metric {
	return []metric{
		{name: "allocs/op", unit: "", tolerance: 0, micro: func(m result.Micro) float64 { return float64(m.AllocsPerOp) }},
		{name: "bytes/op", unit: "B", tolerance: 0, micro: func(m result.Micro) float64 { return float64(m.BytesPerOp) }},
		{name: "ns/op", unit: "ns", tolerance: timingTolerance, micro: func(m result.Micro) float64 { return m.NsPerOp }},
		{name: "rps", unit: "req/s", higherBetter: true, tolerance: timingTolerance, http: func(h result.HTTP) float64 { return h.RPS }},
		{name: "cpu/req", unit: "µs", tolerance: timingTolerance, http: func(h result.HTTP) float64 { return h.ServerCPUus }},
		{name: "p99", unit: "ms", tolerance: timingTolerance * 2, http: func(h result.HTTP) float64 { return h.P99ms }},
	}
}

func main() {
	base := flag.String("base", "results/baseline.json", "baseline result file")
	newer := flag.String("new", "results/latest.json", "result file to check against the baseline")
	framework := flag.String("framework", "goichi", "framework to compare")
	tolerance := flag.Float64("tolerance", 0.15, "fraction a timing metric may worsen before it is a regression")
	flag.Parse()

	a, err := result.Load(*base)
	if err != nil {
		exit(err)
	}
	b, err := result.Load(*newer)
	if err != nil {
		exit(err)
	}

	fmt.Printf("comparing %s: %s -> %s\n", *framework, *base, *newer)
	fmt.Printf("allocation counts must not grow at all; timings may move %.0f%% before they count\n\n",
		*tolerance*100)
	fmt.Printf("%-9s %-9s %14s %14s %10s\n", "scenario", "metric", "base", "new", "change")
	fmt.Println(strings.Repeat("-", 60))

	var regressions []string
	for _, sc := range apps.Scenarios {
		for _, m := range metrics(*tolerance) {
			before, ok1 := value(a, *framework, sc.Key, m)
			after, ok2 := value(b, *framework, sc.Key, m)
			if !ok1 || !ok2 || before == 0 {
				continue
			}

			change := (after - before) / before
			worse := change > m.tolerance
			if m.higherBetter {
				worse = -change > m.tolerance
			}

			mark := " "
			if worse {
				mark = "!"
				regressions = append(regressions,
					fmt.Sprintf("%s/%s: %s %s -> %s (%+.1f%%)",
						sc.Key, m.name, m.name, num(before), num(after), change*100))
			}
			fmt.Printf("%-9s %-9s %14s %14s %+9.1f%% %s\n",
				sc.Key, m.name, num(before)+m.unit, num(after)+m.unit, change*100, mark)
		}
	}

	if len(regressions) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d regression(s):\n", len(regressions))
		for _, r := range regressions {
			fmt.Fprintln(os.Stderr, "  "+r)
		}
		os.Exit(1)
	}
	fmt.Println("\nno regressions")
}

func value(run *result.Run, framework, scenario string, m metric) (float64, bool) {
	if m.micro != nil {
		mi, ok := run.MicroFor(framework, scenario)
		if !ok || !mi.Supported {
			return 0, false
		}
		return m.micro(mi), true
	}
	h, ok := run.HTTPFor(framework, scenario)
	if !ok {
		return 0, false
	}
	return m.http(h), true
}

func num(v float64) string {
	switch {
	case v >= 1000:
		return fmt.Sprintf("%.0f", v)
	case v >= 10:
		return fmt.Sprintf("%.1f", v)
	default:
		return fmt.Sprintf("%.2f", v)
	}
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, "compare:", err)
	os.Exit(1)
}
