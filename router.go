package main

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func SetupRouter(h *Handler) *gin.Engine {
	if strings.TrimSpace(os.Getenv("GO_ENV")) == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	if strings.TrimSpace(os.Getenv("GO_ENV")) == "prod" {
		r.SetTrustedProxies([]string{"127.0.0.1", "::1"})
	} else {
		r.SetTrustedProxies(nil)
	}

	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "./static")

	r.GET("/", h.IndexView)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	r.GET("/documents", h.ListDocuments)
	r.POST("/upload", h.UploadDocument)
	r.POST("/search", h.Search)
	r.POST("/query", h.Query)

	return r
}
