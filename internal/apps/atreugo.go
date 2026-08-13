package apps

import (
	"encoding/json"
	"io"
	"log"

	"github.com/savsgio/atreugo/v11"
)

func init() { register("atreugo", buildAtreugo) }

// buildAtreugo returns an opaque application: atreugo keeps its
// fasthttp.RequestHandler unexported, so it can only be driven through a real
// listener. It takes part in the load phase and is reported as n/a in the
// in-process phase.
func buildAtreugo() *App {
	return &App{
		Name:  "atreugo",
		Kind:  KindOpaque,
		Stack: "fasthttp",
		Listen: func(addr string) error {
			return newAtreugo(addr).ListenAndServe()
		},
	}
}

func newAtreugo(addr string) *atreugo.Atreugo {
	s := atreugo.New(atreugo.Config{
		Addr:   addr,
		Logger: log.New(io.Discard, "", 0),
	})

	s.GET("/ping", func(ctx *atreugo.RequestCtx) error {
		return ctx.TextResponse("pong")
	})

	s.GET("/user/{id}", func(ctx *atreugo.RequestCtx) error {
		return ctx.JSONResponse(IDResult{ID: atreugoParam(ctx, "id")})
	})

	s.GET("/user/{id}/posts/{pid}/comments/{cid}", func(ctx *atreugo.RequestCtx) error {
		return ctx.JSONResponse(ParamsResult{
			User:    atreugoParam(ctx, "id"),
			Post:    atreugoParam(ctx, "pid"),
			Comment: atreugoParam(ctx, "cid"),
		})
	})

	s.GET("/json", func(ctx *atreugo.RequestCtx) error {
		return ctx.JSONResponse(SampleUser)
	})

	s.POST("/echo", func(ctx *atreugo.RequestCtx) error {
		var req EchoRequest
		if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
			return ctx.JSONResponse(map[string]string{"error": "bad request"}, 400)
		}
		return ctx.JSONResponse(req)
	})

	for _, p := range fillerPaths(braceParam) {
		s.GET(p, func(ctx *atreugo.RequestCtx) error {
			return ctx.JSONResponse(IDResult{ID: atreugoParam(ctx, "id")})
		})
	}

	return s
}

func atreugoParam(ctx *atreugo.RequestCtx, key string) string {
	v, _ := ctx.UserValue(key).(string)
	return v
}
