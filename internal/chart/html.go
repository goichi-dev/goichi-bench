package chart

import (
	"fmt"
	"sort"
	"strings"

	"github.com/goichi-dev/goichi-bench/internal/result"
)

// HTML renders the full report page: every figure, the table behind each
// figure, the environment the numbers came from, and the caveats that decide
// whether they mean anything.
func HTML(run *result.Run, figures []Figure) string {
	var b strings.Builder

	b.WriteString("<title>goichi REST benchmark</title>\n")
	b.WriteString(pageStyle)

	b.WriteString(`<main>`)
	fmt.Fprintf(&b, `<h1>goichi REST benchmark</h1>`)
	fmt.Fprintf(&b, `<p class="lead">goichi measured against gin, fiber, chi, echo and atreugo on identical REST routes. Run on %s, %s/%s, Go %s.</p>`,
		esc(run.RunAt.Format("2 January 2006 15:04")), esc(run.Env.OS), esc(run.Env.Arch), esc(run.Env.GoVersion))

	writeGate(&b, run)

	b.WriteString(`<section class="note"><h2>How to read this</h2><ul>
<li><strong>Every framework answers the same requests with the same bytes.</strong> A correctness gate compares each response against a fixed expected body before any timing is recorded, so no framework can look fast by doing less.</li>
<li><strong>Each framework runs in its own process</strong>, one at a time, with the load generator pinned to the other half of the cores.</li>
<li><strong>Server CPU per request is the number to trust when throughput ties.</strong> A loopback interface saturates long before a modern router does; CPU time per request keeps measuring the framework after that point.</li>
<li><strong>Each framework is used the way its own documentation shows</strong> — its own router, its own JSON writer, its own request binder, and whatever middleware it installs by default.</li>
</ul></section>`)

	for _, fig := range figures {
		fmt.Fprintf(&b, `<section><h2>%s</h2><p class="sub">%s</p>`, esc(fig.Title), esc(fig.Subtitle))
		fmt.Fprintf(&b, `<figure>%s</figure>`, fig.SVG)
		writeTable(&b, fig.Table)
		b.WriteString(`</section>`)
	}

	writeEnv(&b, run)

	b.WriteString(`<section class="note"><h2>What these numbers do not say</h2><ul>
<li><strong>Throughput here is a floor, not a ceiling.</strong> Client and server share one machine over loopback; on this host the network path saturates near the frameworks' own limit, which compresses the differences between them.</li>
<li><strong>The in-process figures compare a framework with its own past</strong> first and other frameworks second: a fasthttp stack and a net/http stack are driven through different request objects, and that difference is part of the measurement.</li>
<li><strong>Defaults are part of the framework.</strong> Where a framework installs middleware of its own accord, that cost is included — which is what a user actually pays.</li>
<li><strong>One machine, one operating system.</strong> Numbers from a different host are not comparable to these; re-run the suite there.</li>
</ul></section>`)

	b.WriteString(`</main>`)
	return b.String()
}

func writeGate(b *strings.Builder, run *result.Run) {
	var failed []result.Gate
	for _, g := range run.Gate {
		if !g.OK {
			failed = append(failed, g)
		}
	}
	if len(failed) == 0 {
		fmt.Fprintf(b, `<p class="gate ok">Correctness gate passed: %d framework/scenario responses matched the expected body exactly.</p>`, len(run.Gate))
		return
	}
	fmt.Fprintf(b, `<p class="gate bad">Correctness gate FAILED for %d of %d responses — the affected numbers below are not comparable.</p><ul class="gate-list">`, len(failed), len(run.Gate))
	for _, g := range failed {
		fmt.Fprintf(b, `<li>%s / %s: status %d, got <code>%s</code>, want <code>%s</code></li>`,
			esc(g.Framework), esc(g.Scenario), g.Status, esc(g.Got), esc(g.Want))
	}
	b.WriteString(`</ul>`)
}

func writeTable(b *strings.Builder, t Table) {
	b.WriteString(`<details><summary>Table view</summary><div class="scroll"><table><thead><tr>`)
	for _, c := range t.Columns {
		fmt.Fprintf(b, `<th>%s</th>`, esc(c))
	}
	b.WriteString(`</tr></thead><tbody>`)
	for _, row := range t.Rows {
		b.WriteString(`<tr>`)
		for i, cell := range row {
			tag := "td"
			if i == 0 {
				tag = "th"
			}
			fmt.Fprintf(b, `<%s>%s</%s>`, tag, esc(cell), tag)
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table></div></details>`)
}

func writeEnv(b *strings.Builder, run *result.Run) {
	b.WriteString(`<section><h2>Environment</h2><div class="scroll"><table><tbody>`)
	row := func(k, v string) { fmt.Fprintf(b, `<tr><th>%s</th><td>%s</td></tr>`, esc(k), esc(v)) }
	row("Go", run.Env.GoVersion)
	row("OS / arch", run.Env.OS+"/"+run.Env.Arch)
	if run.Env.CPUModel != "" {
		row("CPU", run.Env.CPUModel)
	}
	row("Logical CPUs", fmt.Sprintf("%d (server %d, load generator %d)", run.Env.NumCPU, run.Env.ServerCPUs, run.Env.ClientCPUs))
	row("Load", fmt.Sprintf("%d connections, %.0fs warmup then %.0fs measured, per scenario",
		run.Load.Connections, run.Load.WarmupSec, run.Load.DurationSec))

	names := make([]string, 0, len(run.Env.Versions))
	for name := range run.Env.Versions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		row(name, run.Env.Versions[name])
	}
	b.WriteString(`</tbody></table></div></section>`)
}

const pageStyle = `<style>
:root {
  color-scheme: light;
  --page: #f9f9f7;
  --surface: #fcfcfb;
  --ink: #0b0b0b;
  --ink-2: #52514e;
  --ink-3: #898781;
  --rule: #e1e0d9;
  --accent: #2a78d6;
  --good: #006300;
  --bad: #d03b3b;
}
@media (prefers-color-scheme: dark) {
  :root:not([data-theme="light"]) {
    color-scheme: dark;
    --page: #0d0d0d;
    --surface: #1a1a19;
    --ink: #ffffff;
    --ink-2: #c3c2b7;
    --ink-3: #898781;
    --rule: #2c2c2a;
    --accent: #3987e5;
    --good: #0ca30c;
    --bad: #e66767;
  }
}
:root[data-theme="dark"] {
  color-scheme: dark;
  --page: #0d0d0d;
  --surface: #1a1a19;
  --ink: #ffffff;
  --ink-2: #c3c2b7;
  --ink-3: #898781;
  --rule: #2c2c2a;
  --accent: #3987e5;
  --good: #0ca30c;
  --bad: #e66767;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  background: var(--page);
  color: var(--ink);
  font: 400 15px/1.6 system-ui, -apple-system, "Segoe UI", sans-serif;
}
main { max-width: 1100px; margin: 0 auto; padding: 40px 20px 72px; }
h1 { font-size: 28px; line-height: 1.25; margin: 0 0 8px; }
h2 { font-size: 18px; margin: 0 0 4px; }
p { margin: 0 0 12px; }
.lead { color: var(--ink-2); margin-bottom: 24px; }
.sub { color: var(--ink-2); font-size: 13.5px; max-width: 72ch; }
section { margin: 32px 0; padding-top: 24px; border-top: 1px solid var(--rule); }
section.note ul { margin: 8px 0 0; padding-left: 20px; color: var(--ink-2); }
section.note li { margin-bottom: 6px; }
figure { margin: 16px 0 8px; overflow-x: auto; }
figure svg { max-width: 100%; height: auto; display: block; }
.gate { font-size: 13.5px; padding: 10px 14px; border-radius: 6px; border: 1px solid var(--rule); background: var(--surface); }
.gate.ok { color: var(--good); }
.gate.bad { color: var(--bad); }
.gate-list { color: var(--bad); font-size: 13px; }
details { margin-top: 8px; }
summary { cursor: pointer; color: var(--ink-2); font-size: 13.5px; }
.scroll { overflow-x: auto; }
table { border-collapse: collapse; width: 100%; margin-top: 12px; font-size: 13px; }
th, td { text-align: left; padding: 6px 12px 6px 0; border-bottom: 1px solid var(--rule); white-space: nowrap; }
td { font-variant-numeric: tabular-nums; color: var(--ink-2); }
thead th { color: var(--ink-3); font-weight: 600; }
tbody th { font-weight: 500; color: var(--ink); }
code { font-family: ui-monospace, "Cascadia Code", Consolas, monospace; font-size: 12px; }
</style>
`
