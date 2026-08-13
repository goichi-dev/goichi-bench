package apps

import (
	"github.com/gofiber/fiber/v3"
)

func init() { register("fiber", buildFiber) }

func buildFiber() *App {
	app := fiber.New()

	app.Get("/ping", func(c fiber.Ctx) error {
		return c.SendString("pong")
	})

	app.Get("/user/:id", func(c fiber.Ctx) error {
		return c.JSON(IDResult{ID: c.Params("id")})
	})

	app.Get("/user/:id/posts/:pid/comments/:cid", func(c fiber.Ctx) error {
		return c.JSON(ParamsResult{
			User:    c.Params("id"),
			Post:    c.Params("pid"),
			Comment: c.Params("cid"),
		})
	})

	app.Get("/json", func(c fiber.Ctx) error {
		return c.JSON(SampleUser)
	})

	app.Post("/echo", func(c fiber.Ctx) error {
		var req EchoRequest
		if err := c.Bind().Body(&req); err != nil {
			return fiber.ErrBadRequest
		}
		return c.JSON(req)
	})

	for _, p := range fillerPaths(colonParam) {
		app.Get(p, func(c fiber.Ctx) error {
			return c.JSON(IDResult{ID: c.Params("id")})
		})
	}

	return &App{
		Name:     "fiber",
		Kind:     KindFastHTTP,
		Stack:    "fasthttp",
		FastHTTP: app.Handler(),
		Listen: func(addr string) error {
			return app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true})
		},
		Shutdown: app.Shutdown,
	}
}
