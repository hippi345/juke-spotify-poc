package handlers

import (
	"net/http"

	"juke-spotify-poc/server/spotify"

	"github.com/gin-gonic/gin"
)

// PlayerNowPlaying returns the currently playing track from Spotify
func PlayerNowPlaying(svc *spotify.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		cp, err := svc.GetCurrentlyPlaying()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		if cp == nil {
			c.JSON(http.StatusOK, gin.H{"playing": false, "item": nil})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"playing":     cp.IsPlaying,
			"progress_ms": cp.ProgressMs,
			"item":        cp.Item,
		})
	}
}
