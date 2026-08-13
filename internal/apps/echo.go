package apps

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func init() { register("echo", buildEcho) }

func buildEcho() *App {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})

	e.GET("/user/:id", func(c echo.Context) error {
		return c.JSON(http.StatusOK, IDResult{ID: c.Param("id")})
	})

	e.GET("/user/:id/posts/:pid/comments/:cid", func(c echo.Context) error {
		return c.JSON(http.StatusOK, ParamsResult{
			User:    c.Param("id"),
			Post:    c.Param("pid"),
			Comment: c.Param("cid"),
		})
	})

	e.GET("/json", func(c echo.Context) error {
		return c.JSON(http.StatusOK, SampleUser)
	})

	e.POST("/echo", func(c echo.Context) error {
		var req EchoRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest)
		}
		return c.JSON(http.StatusOK, req)
	})

	for _, p := range fillerPaths(colonParam) {
		e.GET(p, func(c echo.Context) error {
			return c.JSON(http.StatusOK, IDResult{ID: c.Param("id")})
		})
	}

	return &App{
		Name:    "echo",
		Kind:    KindNetHTTP,
		Stack:   "net/http",
		NetHTTP: e,
		Listen:  netHTTPListen(e),
	}
}
