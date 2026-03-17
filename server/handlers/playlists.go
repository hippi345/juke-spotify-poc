package handlers

import (
	"log"
	"net/http"

	"juke-spotify-poc/server/spotify"

	"github.com/gin-gonic/gin"
)

// PlaylistsList returns the user's playlists for the admin picker
func PlaylistsList(svc *spotify.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		items, err := svc.GetPlaylists()
		if err != nil {
			log.Printf("playlists: %v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"playlists": items})
	}
}
