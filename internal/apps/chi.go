package apps

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func init() { register("chi", buildChi) }

func buildChi() *App {
	r := chi.NewRouter()

	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("pong"))
	})

	r.Get("/user/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, IDResult{ID: chi.URLParam(r, "id")})
	})

	r.Get("/user/{id}/posts/{pid}/comments/{cid}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, ParamsResult{
			User:    chi.URLParam(r, "id"),
			Post:    chi.URLParam(r, "pid"),
			Comment: chi.URLParam(r, "cid"),
		})
	})

	r.Get("/json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, SampleUser)
	})

	r.Post("/echo", func(w http.ResponseWriter, r *http.Request) {
		var req EchoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(w, req)
	})

	for _, p := range fillerPaths(braceParam) {
		r.Get(p, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, IDResult{ID: chi.URLParam(r, "id")})
		})
	}

	return &App{
		Name:    "chi",
		Kind:    KindNetHTTP,
		Stack:   "net/http",
		NetHTTP: r,
		Listen:  netHTTPListen(r),
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
