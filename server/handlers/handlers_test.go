package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"juke-spotify-poc/server/config"

	"github.com/gin-gonic/gin"
)

func TestHealth(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", body["status"])
	}
}

func TestPlaceholder(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/placeholder", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if _, ok := body["message"]; !ok {
		t.Errorf("expected 'message' in response, got %v", body)
	}
}

func TestVibeSenseHealth_notConfigured(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/health/vibesense", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["gemini_configured"] != false || body["vibesense_ready"] != false {
		t.Errorf("expected gemini not configured, got %v", body)
	}
}

func TestVibeSenseHealth_configured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	cfg := &config.Config{
		GeminiAPIKey: "test-key",
		GeminiModel:  "gemini-2.5-flash",
	}
	r.GET("/api/health/vibesense", VibeSenseHealth(cfg))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health/vibesense", nil)
	r.ServeHTTP(rec, req)

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["gemini_configured"] != true || body["vibesense_ready"] != true {
		t.Errorf("expected configured true, got %v", body)
	}
	if body["gemini_model"] != "gemini-2.5-flash" {
		t.Errorf("gemini_model: %v", body["gemini_model"])
	}
}
