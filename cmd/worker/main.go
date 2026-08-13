// Command worker runs exactly one framework, in its own process. The
// orchestrator spawns it once per framework so no two frameworks ever share a
// heap, a GC cycle or a scheduler.
//
//	worker -framework gin -phase serve -addr 127.0.0.1:18080
//	worker -framework gin -phase micro         # JSON on stdout
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/goichi-dev/goichi-bench/internal/apps"
	"github.com/goichi-dev/goichi-bench/internal/micro"
	"github.com/goichi-dev/goichi-bench/internal/result"
)

func main() {
	framework := flag.String("framework", "", "framework to run")
	phase := flag.String("phase", "serve", "serve | micro")
	addr := flag.String("addr", "127.0.0.1:18080", "listen address for the serve phase")
	benchtime := flag.Duration("benchtime", time.Second, "measurement window per micro benchmark")
	flag.Parse()

	app, err := apps.Build(*framework)
	if err != nil {
		fail(err)
	}

	switch *phase {
	case "serve":
		if app.Listen == nil {
			fail(fmt.Errorf("%s cannot listen", app.Name))
		}
		if err := app.Listen(*addr); err != nil {
			fail(err)
		}
	case "micro":
		micro.SetBenchTime(*benchtime)
		out := make([]result.Micro, 0, len(apps.Scenarios))
		for _, sc := range apps.Scenarios {
			// Rebuild between scenarios so a scenario never inherits the
			// warmed-up state of the previous one.
			a, err := apps.Build(*framework)
			if err != nil {
				fail(err)
			}
			out = append(out, micro.Run(a, sc))
		}
		if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
			fail(err)
		}
	default:
		fail(fmt.Errorf("unknown phase %q", *phase))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "worker:", err)
	os.Exit(1)
}
