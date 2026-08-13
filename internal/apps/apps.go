// Package apps builds one REST application per framework. Every builder
// registers the same routes and returns the same bodies, so the only thing that
// differs between them is the framework doing the work.
package apps

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/valyala/fasthttp"
)

// Kind describes how an application can be driven in-process.
type Kind int

const (
	// KindNetHTTP exposes an http.Handler.
	KindNetHTTP Kind = iota
	// KindFastHTTP exposes a fasthttp.RequestHandler.
	KindFastHTTP
	// KindOpaque only serves over a real listener; it cannot be driven
	// in-process, so it takes part in the load phase only.
	KindOpaque
)

// App is one framework's REST application, ready to be measured.
type App struct {
	Name     string
	Kind     Kind
	Stack    string
	NetHTTP  http.Handler
	FastHTTP fasthttp.RequestHandler
	Listen   func(addr string) error
	Shutdown func() error
}

// Builder constructs an application. Listen is only wired up when serve is
// true, so the in-process phase never opens a socket.
type Builder func() *App

var builders = map[string]Builder{}

func register(name string, b Builder) { builders[name] = b }

// Names returns every registered framework name, in a stable order.
func Names() []string {
	out := make([]string, 0, len(builders))
	for name := range builders {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Build returns the application for one framework.
func Build(name string) (*App, error) {
	b, ok := builders[name]
	if !ok {
		return nil, fmt.Errorf("unknown framework %q (have %v)", name, Names())
	}
	return b(), nil
}

// User is the object returned by the JSON scenarios.
type User struct {
	ID      int      `json:"id"`
	Name    string   `json:"name"`
	Email   string   `json:"email"`
	Active  bool     `json:"active"`
	Balance float64  `json:"balance"`
	Tags    []string `json:"tags"`
}

// SampleUser is the fixed payload written by the json scenario.
var SampleUser = User{
	ID:      1,
	Name:    "Somchai Jaidee",
	Email:   "somchai@example.com",
	Active:  true,
	Balance: 1234.56,
	Tags:    []string{"staff", "verified", "th"},
}

// EchoRequest is the body accepted by POST /echo.
type EchoRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

// EchoBody is the request body the load generator posts to /echo.
const EchoBody = `{"name":"Somchai Jaidee","email":"somchai@example.com","age":37}`

// ParamsResult is returned by the multi-parameter route.
type ParamsResult struct {
	User    string `json:"user"`
	Post    string `json:"post"`
	Comment string `json:"comment"`
}

// FillerRoutes is the number of extra routes registered on every framework so
// the "many routes" scenario measures matching in a realistically sized table
// rather than in a table of five.
const FillerRoutes = 150

// fillerPath returns the i-th filler route in a framework's own syntax.
// colon is ":" for frameworks using :name and "{" / "}" style is handled by
// wrapping in the caller.
func fillerPaths(param func(string) string) []string {
	out := make([]string, 0, FillerRoutes)
	for i := 0; i < FillerRoutes/3; i++ {
		out = append(out,
			fmt.Sprintf("/api/v1/segment%d/%s", i, param("id")),
			fmt.Sprintf("/api/v1/segment%d/%s/detail", i, param("id")),
			fmt.Sprintf("/api/v1/static%d/leaf", i),
		)
	}
	return out
}

// colonParam renders a parameter in the :name style used by goichi, gin, echo,
// fiber and atreugo.
func colonParam(name string) string { return ":" + name }

// braceParam renders a parameter in the {name} style used by chi.
func braceParam(name string) string { return "{" + name + "}" }
