package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/vector"

	"github.com/gin-gonic/gin"
)

func TestSimilarVenueTracksDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{VibeSenseVectorEnabled: false}
	r := gin.New()
	r.GET("/api/venues/:id/tracks/similar", SimilarVenueTracks(cfg))
	req := httptest.NewRequest(http.MethodGet, "/api/venues/1/tracks/similar?q=lofi", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when disabled, got %d", w.Code)
	}
}

func TestSimilarVenueTracksEnabledNoKeyGraceful(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_ = vector.Init(true, "", "", "gemini")
	cfg := &config.Config{VibeSenseVectorEnabled: true}
	r := gin.New()
	r.GET("/api/venues/:id/tracks/similar", SimilarVenueTracks(cfg))
	req := httptest.NewRequest(http.MethodGet, "/api/venues/1/tracks/similar?q=lofi", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 graceful degrade, got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["enabled"] != true {
		t.Fatalf("expected enabled true: %v", body)
	}
}
