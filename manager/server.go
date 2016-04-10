package manager

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type worker struct {
	// worker name
	name string
	// url to connect to worker
	url string
	// session token
	token string
}

func makeHTTPServer(debug bool) *gin.Engine {
	if !debug {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"msg": "pong"})
	})
	// List jobs, status page
	r.GET("/jobs", func(c *gin.Context) {})
	// worker online
	r.POST("/workers/:name", func(c *gin.Context) {})
	// post job list
	r.POST("/workers/:name/jobs", func(c *gin.Context) {})
	// post job status
	r.POST("/workers/:name/jobs/:job", func(c *gin.Context) {})

	return r
}
