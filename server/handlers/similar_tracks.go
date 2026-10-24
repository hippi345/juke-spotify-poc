package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/vector"

	"github.com/gin-gonic/gin"
)

// SimilarVenueTracks semantic search over venue_tracks (flag-gated).
func SimilarVenueTracks(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := vector.Default()
		if !svc.Enabled {
			c.JSON(http.StatusNotFound, gin.H{"error": "vector search disabled"})
			return
		}
		id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || id64 == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid venue id"})
			return
		}
		q := strings.TrimSpace(c.Query("q"))
		trackID := strings.TrimSpace(c.Query("track_id"))
		limit := 20
		if l := c.Query("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 50 {
				limit = n
			}
		}
		var hits []vector.SearchHit
		if trackID != "" {
			hits, err = svc.Indexer.SearchByText(c.Request.Context(), uint(id64), trackID, svc.Embedder, limit)
		} else if q != "" {
			hits, err = svc.Indexer.SearchByText(c.Request.Context(), uint(id64), q, svc.Embedder, limit)
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "q or track_id required"})
			return
		}
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		out := make([]gin.H, 0, len(hits))
		for _, h := range hits {
			out = append(out, gin.H{
				"spotify_id": h.SpotifyID,
				"title":      h.Title,
				"artist":     h.Artist,
				"score":      h.Score,
			})
		}
		c.JSON(http.StatusOK, gin.H{"tracks": out, "venue_id": id64})
	}
}
