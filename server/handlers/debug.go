package handlers

import (
	"io"
	"log"
	"net/http"

	"juke-spotify-poc/server/spotify"

	"github.com/gin-gonic/gin"
)

// DebugPlaylistAccess tests which Spotify endpoints work for a playlist (helps diagnose 403)
func DebugPlaylistAccess(svc *spotify.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		playlistID := c.Param("id")
		if playlistID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "playlist id required"})
			return
		}

		result := gin.H{
			"playlist_id": playlistID,
			"playlist_metadata": nil,
			"playlist_tracks": nil,
		}

		// Test 1: GET /playlists/{id} (metadata only)
		resp1, err := svc.Get("/playlists/" + playlistID)
		if err != nil {
			result["playlist_metadata"] = gin.H{"error": err.Error()}
		} else {
			body1, _ := io.ReadAll(resp1.Body)
			resp1.Body.Close()
			if resp1.StatusCode != http.StatusOK {
				result["playlist_metadata"] = gin.H{"status": resp1.StatusCode, "body": string(body1)}
				log.Printf("debug: GET /playlists/%s -> %d %s", playlistID, resp1.StatusCode, string(body1))
			} else {
				result["playlist_metadata"] = gin.H{"ok": true}
			}
		}

		// Test 2: GET /playlists/{id}/items (tracks endpoint is deprecated, use items)
		resp2, err := svc.Get("/playlists/" + playlistID + "/items?limit=1")
		if err != nil {
			result["playlist_tracks"] = gin.H{"error": err.Error()}
		} else {
			body2, _ := io.ReadAll(resp2.Body)
			resp2.Body.Close()
			if resp2.StatusCode != http.StatusOK {
				result["playlist_tracks"] = gin.H{"status": resp2.StatusCode, "body": string(body2)}
				log.Printf("debug: GET /playlists/%s/tracks -> %d %s", playlistID, resp2.StatusCode, string(body2))
			} else {
				result["playlist_tracks"] = gin.H{"ok": true}
			}
		}

		c.JSON(http.StatusOK, result)
	}
}
