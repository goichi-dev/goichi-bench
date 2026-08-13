package apps

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func init() { register("gin", buildGin) }

func buildGin() *App {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	r.GET("/user/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, IDResult{ID: c.Param("id")})
	})

	r.GET("/user/:id/posts/:pid/comments/:cid", func(c *gin.Context) {
		c.JSON(http.StatusOK, ParamsResult{
			User:    c.Param("id"),
			Post:    c.Param("pid"),
			Comment: c.Param("cid"),
		})
	})

	r.GET("/json", func(c *gin.Context) {
		c.JSON(http.StatusOK, SampleUser)
	})

	r.POST("/echo", func(c *gin.Context) {
		var req EchoRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		c.JSON(http.StatusOK, req)
	})

	for _, p := range fillerPaths(colonParam) {
		r.GET(p, func(c *gin.Context) {
			c.JSON(http.StatusOK, IDResult{ID: c.Param("id")})
		})
	}

	return &App{
		Name:    "gin",
		Kind:    KindNetHTTP,
		Stack:   "net/http",
		NetHTTP: r,
		Listen:  netHTTPListen(r),
	}
}

func netHTTPListen(h http.Handler) func(addr string) error {
	return func(addr string) error {
		srv := &http.Server{Addr: addr, Handler: h}
		return srv.ListenAndServe()
	}
}
