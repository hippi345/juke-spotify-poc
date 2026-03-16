package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Health returns a simple health check response
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "juke-spotify-poc-api",
	})
}

// Placeholder returns a placeholder API response
func Placeholder(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "API is ready for implementation",
	})
}
