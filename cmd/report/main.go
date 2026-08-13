// Command report turns a result file into charts and an HTML report.
//
//	go run ./cmd/report -in results/latest.json -out docs
//
// Rendering is separate from measuring on purpose: a report can be rebuilt
// from a stored result without running the benchmark again.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goichi-dev/goichi-bench/internal/chart"
	"github.com/goichi-dev/goichi-bench/internal/result"
)

func main() {
	in := flag.String("in", "results/latest.json", "result file to render")
	outDir := flag.String("out", "docs", "directory to write the charts and report into")
	flag.Parse()

	run, err := result.Load(*in)
	if err != nil {
		exit(err)
	}
	if run.Schema != result.Schema {
		exit(fmt.Errorf("result file is schema %q, this build reads %q", run.Schema, result.Schema))
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		exit(err)
	}

	figures := chart.Figures(run)
	for _, fig := range figures {
		path := filepath.Join(*outDir, fig.Name+".svg")
		if err := os.WriteFile(path, []byte(fig.SVG), 0o644); err != nil {
			exit(err)
		}
		fmt.Println("wrote", path)
	}

	path := filepath.Join(*outDir, "index.html")
	if err := os.WriteFile(path, []byte(chart.HTML(run, figures)), 0o644); err != nil {
		exit(err)
	}
	fmt.Println("wrote", path)
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, "report:", err)
	os.Exit(1)
}
