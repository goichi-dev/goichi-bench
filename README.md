# goichi-bench

REST benchmark for [goichi](https://github.com/goichi-dev/goichi), measured
against **gin**, **fiber**, **chi**, **echo** and **atreugo** on identical
routes — and the regression gate that says whether a change to goichi cost
anything.

This repository exists because a framework that calls itself high-performance
owes the reader numbers, and because the numbers have to be reproducible by
somebody who does not trust the author.

> Only the REST/HTTP surface is measured. goichi's WebSocket, GraphQL, gRPC and
> MQTT protocols are out of scope here.

## What is measured

Six scenarios, issued identically to every framework:

| scenario | request | what it exercises |
|---|---|---|
| `static` | `GET /ping` | dispatch with no parameters, plain-text body |
| `param` | `GET /user/:id` | one captured parameter, JSON body |
| `params3` | `GET /user/:id/posts/:pid/comments/:cid` | deep tree walk, three captures |
| `json` | `GET /json` | serialising a fixed six-field struct |
| `post` | `POST /echo` | binding a JSON request body, then writing it back |
| `many` | `GET /api/v1/segment42/:id/detail` | matching inside a 150-route table |

Two phases produce two kinds of number:

- **HTTP phase** — a real server on loopback TCP, driven by a fasthttp load
  generator: throughput, latency percentiles, and **server CPU time per
  request**.
- **In-process phase** — the framework's own handler called directly, with no
  sockets: **nanoseconds and heap allocations per request**.

## Results

**Not published yet.** The suite runs and its correctness gate passes, but no
measurement taken so far is fit to publish.

Every run made during development was made on a thermally constrained laptop
with a hybrid P-core/E-core mobile CPU, and the HTTP numbers from it turned out
to be measuring the machine rather than the frameworks: see the timer period
note under [How the comparison is kept honest](#how-the-comparison-is-kept-honest).
With that fixed the spread falls from a median of ±61% to under ±5%, which is
the difference between numbers that rank six frameworks and numbers that rank
nothing.

The published figures will come from a quiet Linux host. Until they do, this
repository is the method and the tooling, not the answer.

Charts and the HTML report are generated into `docs/` by `cmd/report`, and the
raw measurements land in `results/`, one JSON file per run.

## Running it

```bash
go run ./cmd/goichi-bench -out results/latest.json   # measure (~13 min)
go run ./cmd/report -in results/latest.json -out docs # render charts + report
```

Useful flags:

| flag | default | meaning |
|---|---|---|
| `-frameworks` | all | comma-separated subset, e.g. `goichi,fiber` |
| `-phase` | `all` | `http` or `micro` to run one phase |
| `-connections` | `64` | concurrent connections in the HTTP phase |
| `-duration` | `5s` | measured window per scenario |
| `-repeat` | `3` | measurements per scenario; the median is kept |
| `-benchtime` | `1s` | measured window per in-process benchmark |

The run exits non-zero if any framework fails the correctness gate.

## The regression gate

Record a baseline, change goichi, measure again, and let the exit code decide:

```bash
go run ./cmd/goichi-bench -out results/baseline.json
# ... change goichi ...
go run ./cmd/goichi-bench -out results/latest.json
go run ./cmd/compare -base results/baseline.json -new results/latest.json
```

Allocation and byte counts are deterministic, so they are allowed to move **0%**
— one extra allocation per request is a regression and is reported as one.
Timings are noisy, so they get a tolerance (`-tolerance`, default 15%; tail
latency gets double). `compare` exits non-zero when anything regresses.

## Benchmarking unreleased goichi

`go.mod` pins the published goichi release, so a fresh clone reproduces the
published numbers. To measure a working tree instead:

```bash
go mod edit -replace github.com/goichi-dev/goichi=../goichi
go run ./cmd/goichi-bench -frameworks goichi -out results/goichi-dev.json
go mod edit -dropreplace github.com/goichi-dev/goichi
```

## How the comparison is kept honest

**Every framework returns the same bytes.** Before any timing, each server is
asked every scenario once and its response is compared against a fixed expected
body. A framework that answers something shorter, or with a different status,
fails the gate and the run fails with it — being fast by doing less is not
available.

**Each framework is used the way its own documentation shows.** Its own router
syntax, its own JSON writer, its own request binder, and whatever middleware it
installs by default. Where a framework does more work per request because that
is what it ships, the cost is counted: that is what its users actually pay.

**One framework per process, one at a time.** No two frameworks share a heap, a
garbage collector or a scheduler. The load generator is pinned to half the
machine's cores and the server to the other half, so neither starves the other.

**The median of repeated measurements**, with the spread between fastest and
slowest repeat recorded next to it. A difference smaller than that spread is
not a difference.

**Server CPU time per request is measured, not inferred.** The orchestrator
reads the server process's own CPU counters around the measured window
(`GetProcessTimes` on Windows, `/proc/<pid>/stat` on Linux). This matters
because a loopback interface saturates before a modern Go router does: once
every framework is bounded by the same network path they report almost the same
throughput, while CPU per request keeps separating them.

**Latency is measured with a clock that can see it.** On Windows `time.Now()`
advances in steps of about half a millisecond — longer than the request being
timed — so percentiles taken from it collapse into "0 ms" and "1 ms". The load
generator uses `QueryPerformanceCounter` there instead.

**The system timer period is pinned for the length of the run.** Windows wakes a
waiting thread on a timer whose period defaults to 15.625 ms, and the Go runtime
raises it to 1 ms only while it has timers to service, dropping it again when
the process goes briefly idle — so which period is in force during a measurement
is decided by unrelated activity on the machine. A measurement that straddles a
drop sees requests wait for the next tick: throughput falls by a factor of
three, tail latency lands exactly on 15.6 ms, and CPU time per request does not
move at all, because the work per request never changed and only the waiting
grew. Unpinned, one framework measured anywhere between 40k and 180k req/s
within a single run, a gap far wider than any real difference between the
frameworks. The orchestrator now holds the finest period from start to finish
and records it in the result file, so a run made without it can be recognised
later.

## What these numbers do not say

- **Throughput here is a floor, not a ceiling.** Client and server share one
  machine; on a saturated loopback the frameworks bunch together.
- **The in-process phase compares a framework with its own past first.** Across
  stacks it also compares two different request objects: a fasthttp
  `RequestCtx` is not a `net/http` `Request`, and that difference is inside the
  measurement.
- **atreugo has no in-process figures.** It keeps its `fasthttp.RequestHandler`
  unexported, so it can only be driven through a real listener. It takes part
  in the HTTP phase and is reported as `n/a` in the in-process one.
- **One machine, one operating system.** Numbers measured elsewhere are not
  comparable to these. Re-run the suite on the host you care about.

## Layout

```
cmd/goichi-bench   orchestrator: runs every framework, writes the result JSON
cmd/report         result JSON -> SVG charts + HTML report
cmd/compare        two result JSONs -> regression verdict
cmd/worker         serves, or measures, exactly one framework (spawned)
internal/apps      one REST application per framework, identical routes
internal/loadgen   HTTP load generator and latency histogram
internal/micro     in-process measurement
internal/procstat  another process's CPU time
internal/clock     a monotonic clock fine enough to time one request
internal/chart     SVG charts and the HTML report
```

## License

MIT — see [LICENSE](LICENSE).
