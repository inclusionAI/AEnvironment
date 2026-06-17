package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed web/dist
var consoleAssets embed.FS

func registerConsoleRoutes(r *gin.Engine) {
	r.RedirectTrailingSlash = false

	dist, err := fs.Sub(consoleAssets, "web/dist")
	if err != nil {
		log.Fatalf("failed to load console assets: %v", err)
	}
	indexHTML, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		log.Fatalf("failed to load console index: %v", err)
	}

	fileServer := http.FileServer(http.FS(dist))
	serveIndex := func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	}

	r.GET("/console", serveIndex)
	r.GET("/console/*path", func(c *gin.Context) {
		requestPath := strings.TrimPrefix(c.Param("path"), "/")
		if requestPath == "" {
			serveIndex(c)
			return
		}

		if _, err := fs.Stat(dist, requestPath); err == nil {
			c.Request.URL.Path = "/" + requestPath
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		if strings.HasPrefix(requestPath, "assets/") && filepath.Ext(requestPath) != "" {
			c.Status(http.StatusNotFound)
			return
		}
		serveIndex(c)
	})
}
