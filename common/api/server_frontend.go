//go:build with_api

package common

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed frontend/build/*
var frontendFiles embed.FS

// SetupFrontend configures the router to serve the frontend static files
func SetupFrontend(r *gin.Engine) {
	frontendFS, err := fs.Sub(frontendFiles, "frontend/build")
	if err != nil {
		panic(err)
	}

	// Handle all routes
	r.Use(func(c *gin.Context) {
		// Skip if it's an API route
		if strings.HasPrefix(c.Request.URL.Path, "/api/") ||
			strings.HasPrefix(c.Request.URL.Path, "/swagger/") {
			c.Next()
			return
		}

		// Try to serve static files
		path := c.Request.URL.Path
		if path == "/" {
			path = "index.html"
		}

		content, err := fs.ReadFile(frontendFS, strings.TrimPrefix(path, "/"))
		if err == nil {
			// Set content type based on file extension
			switch {
			case strings.HasSuffix(path, ".html"):
				c.Header("Content-Type", "text/html")
			case strings.HasSuffix(path, ".js"):
				c.Header("Content-Type", "application/javascript")
			case strings.HasSuffix(path, ".css"):
				c.Header("Content-Type", "text/css")
			case strings.HasSuffix(path, ".json"):
				c.Header("Content-Type", "application/json")
			}
			c.Data(http.StatusOK, c.GetHeader("Content-Type"), content)
			c.Abort()
			return
		}

		// If file not found, serve index.html for SPA routing
		content, err = fs.ReadFile(frontendFS, "index.html")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			c.Abort()
			return
		}

		c.Header("Content-Type", "text/html")
		c.Data(http.StatusOK, "text/html", content)
		c.Abort()
	})
}
