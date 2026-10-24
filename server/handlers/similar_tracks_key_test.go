package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/vector"

	"github.com/gin-gonic/gin"
)

type leakyTestEmbedder struct {
	apiKey string
}

func (e leakyTestEmbedder) Embed(string) ([]float32, error) {
	return nil, &embedErrWithKey{key: e.apiKey}
}

type stubEmbedIndexer struct {
	emb vector.Embedder
}

func (s stubEmbedIndexer) EnsureIndex(context.Context, int) error { return nil }
func (s stubEmbedIndexer) Get(context.Context, string) (*vector.VenueTrackDoc, error) {
	return nil, nil
}
func (s stubEmbedIndexer) Upsert(context.Context, string, vector.VenueTrackDoc) error { return nil }
func (s stubEmbedIndexer) DeleteByVenueID(context.Context, uint) (int, error)        { return 0, nil }
func (s stubEmbedIndexer) SearchSimilar(context.Context, uint, []float32, int) ([]vector.SearchHit, error) {
	return nil, nil
}
func (s stubEmbedIndexer) SearchByText(_ context.Context, _ uint, q string, embedder vector.Embedder, _ int) ([]vector.SearchHit, error) {
	emb := embedder
	if s.emb != nil {
		emb = s.emb
	}
	_, err := emb.Embed(q)
	return nil, err
}

type embedErrWithKey struct{ key string }

func (e *embedErrWithKey) Error() string {
	return "embed HTTP 404: https://generativelanguage.googleapis.com/v1beta/models/x:embedContent?key=" + e.key
}

func TestSimilarVenueTracks_neverLeaksAPIKeyInResponseOrLogs(t *testing.T) {
	const secret = "AIzaSyLeakedKeyForTestOnly1234567890" // test-only fake (35 chars after AIza)
	leaky := leakyTestEmbedder{apiKey: secret}
	vector.SetDefaultService(vector.Service{
		Enabled:    true,
		Indexer:    stubEmbedIndexer{emb: leaky},
		Embedder:   leaky,
		IndexReady: false,
	})

	var logBuf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logBuf)
	t.Cleanup(func() { log.SetOutput(prev) })

	gin.SetMode(gin.TestMode)
	cfg := &config.Config{VibeSenseVectorEnabled: true}
	r := gin.New()
	r.GET("/api/venues/:id/tracks/similar", SimilarVenueTracks(cfg))
	req := httptest.NewRequest(http.MethodGet, "/api/venues/1/tracks/similar?q=lofi", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, secret) || strings.Contains(body, "?key=") {
		t.Fatalf("response leaked key: %s", body)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["degraded"] != vector.DegradedSemanticSearch {
		t.Fatalf("unexpected degraded: %v", parsed["degraded"])
	}
	if strings.Contains(logBuf.String(), secret) {
		t.Fatalf("logs leaked key: %s", logBuf.String())
	}
}
