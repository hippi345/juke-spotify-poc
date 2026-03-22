package handlers

import (
	"net/http"

	"juke-spotify-poc/server/config"

	"github.com/gin-gonic/gin"
)

// Root identifies this API when you open http://127.0.0.1:<PORT>/ in a browser (avoids guessing wrong process).
func Root(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service": "juke-spotify-poc-api",
		"endpoints": []string{
			"GET /health",
			"GET /health/vibesense",
			"GET /api/health/vibesense",
		},
	})
}

// NotFound is a JSON 404 so unknown paths are obvious (wrong URL vs wrong server on this port).
func NotFound(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{
		"error":   "not_found",
		"path":    c.Request.URL.Path,
		"service": "juke-spotify-poc-api",
		"hint":    "Use GET / for a list of paths. If everything 404s, something else may be bound to this port.",
	})
}

// Health returns a simple health check response
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "juke-spotify-poc-api",
	})
}

// VibeSenseHealth reports whether Gemini is configured for VibeSense (no secrets exposed).
func VibeSenseHealth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg == nil {
			c.JSON(http.StatusOK, gin.H{
				"gemini_configured": false,
				"vibesense_ready":   false,
				"gemini_model":      "",
			})
			return
		}
		ready := cfg.GeminiAPIKey != ""
		c.JSON(http.StatusOK, gin.H{
			"gemini_configured": ready,
			"vibesense_ready":   ready,
			"gemini_model":      cfg.GeminiModel,
		})
	}
}

// Placeholder returns a placeholder API response
func Placeholder(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "API is ready for implementation",
	})
}
