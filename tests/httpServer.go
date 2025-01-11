//go:build ignore
// +build ignore

package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	s := gin.Default()
	s.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"msg": "passed"})
	})
	s.RunTLS(":5002", "manager.crt", "manager.key")
}
