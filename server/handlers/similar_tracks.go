package handlers

import (
	"errors"
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
		if !cfg.VibeSenseVectorEnabled {
			c.JSON(http.StatusNotFound, gin.H{"error": "vector search disabled", "enabled": false})
			return
		}
		svc := vector.Default()
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
		queryText := q
		if queryText == "" {
			queryText = trackID
		}
		if queryText == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "q or track_id required"})
			return
		}
		hits, err := svc.Indexer.SearchByText(c.Request.Context(), uint(id64), queryText, svc.Embedder, limit)
		resp := gin.H{
			"tracks":      []gin.H{},
			"venue_id":    id64,
			"enabled":     true,
			"index_ready": svc.IndexReady,
		}
		if err != nil {
			apiKey := ""
			if ge, ok := svc.Embedder.(vector.GeminiEmbedder); ok {
				apiKey = ge.APIKey
			}
			vector.LogEmbedError("similar tracks embed/search", err, apiKey)
			if errors.Is(err, vector.ErrEmbedUnavailable) || strings.Contains(strings.ToLower(err.Error()), "gemini_api_key") {
				resp["degraded"] = "embeddings unavailable (set GEMINI_API_KEY for semantic search)"
			} else {
				resp["degraded"] = vector.DegradedSemanticSearch
			}
			c.JSON(http.StatusOK, resp)
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
		resp["tracks"] = out
		c.JSON(http.StatusOK, resp)
	}
}
