package chart

import (
	"fmt"
	"sort"
	"strings"

	"github.com/goichi-dev/goichi-bench/internal/apps"
	"github.com/goichi-dev/goichi-bench/internal/result"
)

// Subject is the framework the report is written about; it carries the accent
// colour in every chart.
const Subject = "goichi"

// Figure is one rendered chart plus the table that carries the same numbers.
type Figure struct {
	Name     string // file name without extension
	Title    string
	Subtitle string
	SVG      string
	Table    Table
}

// Table is the accessible twin of a figure.
type Table struct {
	Columns []string
	Rows    [][]string
}

// Figures renders every chart for a run.
func Figures(run *result.Run) []Figure {
	frameworks := frameworkList(run)

	return []Figure{
		httpFigure(run, frameworks, "throughput", "HTTP throughput",
			"Requests per second over loopback TCP, higher is better.",
			func(h result.HTTP) float64 { return h.RPS },
			func(v float64) string { return comma(int64(v)) },
			true),
		httpFigure(run, frameworks, "server-cpu", "Server CPU per request",
			"Microseconds of server CPU time per request, lower is better. Unlike throughput this keeps discriminating when the network path saturates.",
			func(h result.HTTP) float64 { return h.ServerCPUus },
			func(v float64) string { return fmt.Sprintf("%.1f µs", v) },
			false),
		httpFigure(run, frameworks, "latency-p99", "Tail latency (p99)",
			"99th percentile round-trip latency in milliseconds, lower is better.",
			func(h result.HTTP) float64 { return h.P99ms },
			func(v float64) string { return fmt.Sprintf("%.2f ms", v) },
			false),
		microFigure(run, frameworks, "micro-ns", "In-process dispatch time",
			"Nanoseconds per request handled in-process, lower is better. Compares a framework against its own history; across stacks it also compares two different request objects.",
			func(m result.Micro) float64 { return m.NsPerOp },
			func(v float64) string { return comma(int64(v)) + " ns" }),
		microFigure(run, frameworks, "micro-allocs", "Allocations per request",
			"Heap allocations per request handled in-process, lower is better. The number that predicts how a service behaves under sustained load.",
			func(m result.Micro) float64 { return float64(m.AllocsPerOp) },
			func(v float64) string { return fmt.Sprintf("%.0f", v) }),
	}
}

func httpFigure(run *result.Run, frameworks []string, name, title, subtitle string,
	get func(result.HTTP) float64, format func(float64) string, higherBetter bool) Figure {

	panels := make([]Panel, 0, len(apps.Scenarios))
	table := Table{Columns: append([]string{"scenario"}, frameworks...)}

	for _, sc := range apps.Scenarios {
		bars := make([]Bar, 0, len(frameworks))
		row := []string{sc.Key}
		for _, fw := range frameworks {
			h, ok := run.HTTPFor(fw, sc.Key)
			v := get(h)
			bars = append(bars, Bar{
				Label:     fw,
				Value:     v,
				Display:   displayOr(ok, v, format),
				Missing:   !ok,
				Highlight: fw == Subject,
			})
			row = append(row, displayOr(ok, v, format))
		}
		sortBars(bars, higherBetter)
		panels = append(panels, Panel{Title: sc.Title, Note: sc.Method + " " + sc.Path})
		panels[len(panels)-1].Bars = bars
		table.Rows = append(table.Rows, row)
	}

	return Figure{
		Name:     name,
		Title:    title,
		Subtitle: subtitle,
		SVG:      Render(Options{Title: title, Subtitle: subtitle, Columns: 3}, panels),
		Table:    table,
	}
}

func microFigure(run *result.Run, frameworks []string, name, title, subtitle string,
	get func(result.Micro) float64, format func(float64) string) Figure {

	panels := make([]Panel, 0, len(apps.Scenarios))
	table := Table{Columns: append([]string{"scenario"}, frameworks...)}

	for _, sc := range apps.Scenarios {
		bars := make([]Bar, 0, len(frameworks))
		row := []string{sc.Key}
		for _, fw := range frameworks {
			m, ok := run.MicroFor(fw, sc.Key)
			supported := ok && m.Supported
			v := get(m)
			display := "n/a"
			if supported {
				display = format(v)
			}
			bars = append(bars, Bar{
				Label:     fw,
				Value:     v,
				Display:   display,
				Missing:   !supported,
				Highlight: fw == Subject,
			})
			row = append(row, display)
		}
		sortBars(bars, false)
		panels = append(panels, Panel{Title: sc.Title, Note: sc.Method + " " + sc.Path, Bars: bars})
		table.Rows = append(table.Rows, row)
	}

	return Figure{
		Name:     name,
		Title:    title,
		Subtitle: subtitle,
		SVG:      Render(Options{Title: title, Subtitle: subtitle, Columns: 3}, panels),
		Table:    table,
	}
}

// sortBars ranks the panel best-first, with unmeasurable frameworks last.
func sortBars(bars []Bar, higherBetter bool) {
	sort.SliceStable(bars, func(i, j int) bool {
		if bars[i].Missing != bars[j].Missing {
			return !bars[i].Missing
		}
		if higherBetter {
			return bars[i].Value > bars[j].Value
		}
		return bars[i].Value < bars[j].Value
	})
}

func displayOr(ok bool, v float64, format func(float64) string) string {
	if !ok {
		return "n/a"
	}
	return format(v)
}

// frameworkList returns the frameworks present in a run, subject first.
func frameworkList(run *result.Run) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	for _, h := range run.HTTP {
		add(h.Framework)
	}
	for _, m := range run.Micro {
		add(m.Framework)
	}
	sort.Strings(out)

	// Keep the subject first so it reads as the reference in every table.
	for i, name := range out {
		if name == Subject {
			copy(out[1:i+1], out[:i])
			out[0] = Subject
			break
		}
	}
	return out
}

func comma(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
