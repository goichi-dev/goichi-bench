// Package micro measures a framework in-process: no sockets, no kernel, just
// the framework's own request path. The numbers are mainly useful as a
// regression signal for one framework over time — comparing a fasthttp stack
// against a net/http stack here compares two different request objects, not
// two routers.
package micro

import (
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/goichi-dev/goichi-bench/internal/apps"
	"github.com/goichi-dev/goichi-bench/internal/result"
)

// SetBenchTime asks the testing package for a longer measurement window than
// its one-second default. It is best effort: if the testing flags are not
// registered the default stands.
func SetBenchTime(d time.Duration) {
	testing.Init()
	_ = flag.CommandLine.Set("test.benchtime", d.String())
}

// Run measures one scenario against one application.
func Run(app *apps.App, sc apps.Scenario) result.Micro {
	out := result.Micro{Framework: app.Name, Scenario: sc.Key}

	switch app.Kind {
	case apps.KindNetHTTP:
		out.Supported = true
		fill(&out, benchNetHTTP(app.NetHTTP, sc))
	case apps.KindFastHTTP:
		out.Supported = true
		fill(&out, benchFastHTTP(app.FastHTTP, sc))
	default:
		out.Note = "no in-process handler exposed by the framework"
	}
	return out
}

func fill(out *result.Micro, res testing.BenchmarkResult) {
	out.NsPerOp = float64(res.NsPerOp())
	out.AllocsPerOp = res.AllocsPerOp()
	out.BytesPerOp = res.AllocedBytesPerOp()
	out.Iterations = res.N
}

func benchNetHTTP(h http.Handler, sc apps.Scenario) testing.BenchmarkResult {
	req := httptest.NewRequest(sc.Method, sc.Path, nil)
	if sc.Body != "" {
		req.Header.Set("Content-Type", "application/json")
		req.ContentLength = int64(len(sc.Body))
	}
	w := newSink()

	return testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if sc.Body != "" {
				req.Body = io.NopCloser(strings.NewReader(sc.Body))
			}
			w.reset()
			h.ServeHTTP(w, req)
		}
	})
}

func benchFastHTTP(h fasthttp.RequestHandler, sc apps.Scenario) testing.BenchmarkResult {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(sc.Method)
	ctx.Request.SetRequestURI(sc.Path)
	if sc.Body != "" {
		ctx.Request.Header.SetContentType("application/json")
		ctx.Request.SetBodyString(sc.Body)
	}

	return testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ctx.Response.Reset()
			h(ctx)
		}
	})
}

// sink is an http.ResponseWriter that keeps the response body in a reused
// buffer. It discards nothing: the handler still pays for serialising the body,
// which is what the fasthttp side pays for too.
type sink struct {
	header http.Header
	body   []byte
}

func newSink() *sink {
	return &sink{header: make(http.Header, 8), body: make([]byte, 0, 1024)}
}

func (s *sink) Header() http.Header { return s.header }

func (s *sink) Write(p []byte) (int, error) {
	s.body = append(s.body, p...)
	return len(p), nil
}

func (s *sink) WriteHeader(int) {}

func (s *sink) reset() {
	s.body = s.body[:0]
	for k := range s.header {
		delete(s.header, k)
	}
}
