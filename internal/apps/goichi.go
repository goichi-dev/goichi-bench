package apps

import (
	"github.com/goichi-dev/goichi"
)

func init() { register("goichi", buildGoichi) }

func buildGoichi() *App {
	show := false
	app := goichi.New(goichi.Config{
		Server: goichi.ServerConfig{AppName: "bench", ShowStartup: &show},
	})

	app.GET("/ping", func(c *goichi.Context) error {
		c.RequestCtx.SetContentType("text/plain; charset=utf-8")
		c.RequestCtx.SetBodyString("pong")
		return nil
	})

	app.GET("/user/:id", func(c *goichi.Context) error {
		return c.JSON(IDResult{ID: c.Param("id")})
	})

	app.GET("/user/:id/posts/:pid/comments/:cid", func(c *goichi.Context) error {
		return c.JSON(ParamsResult{
			User:    c.Param("id"),
			Post:    c.Param("pid"),
			Comment: c.Param("cid"),
		})
	})

	app.GET("/json", func(c *goichi.Context) error {
		return c.JSON(SampleUser)
	})

	app.POST("/echo", func(c *goichi.Context) error {
		var req EchoRequest
		if err := c.Bind(&req); err != nil {
			return err
		}
		return c.JSON(req)
	})

	for _, p := range fillerPaths(colonParam) {
		app.GET(p, func(c *goichi.Context) error {
			return c.JSON(IDResult{ID: c.Param("id")})
		})
	}

	return &App{
		Name:     "goichi",
		Kind:     KindFastHTTP,
		Stack:    "fasthttp",
		FastHTTP: app.Router.HandleRequest,
		Listen:   app.Listen,
	}
}
