package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"
)

func TestSpotifyLogin_NoCredentials(t *testing.T) {
	r := setupTestRouterNoSpotifyCreds(t)

	req := httptest.NewRequest(http.MethodGet, "/api/spotify/login", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if _, ok := body["error"]; !ok {
		t.Errorf("expected 'error' in response, got %v", body)
	}
}

func TestSpotifyLogin_WithCredentials(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/spotify/login", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	loginURL, ok := body["url"].(string)
	if !ok || loginURL == "" {
		t.Errorf("expected 'url' in response, got %v", body)
	}

	parsed, err := url.Parse(loginURL)
	if err != nil {
		t.Fatalf("failed to parse login URL: %v", err)
	}
	if parsed.Host != "accounts.spotify.com" {
		t.Errorf("expected spotify auth URL, got %s", loginURL)
	}
	state := parsed.Query().Get("state")
	if state == "" {
		t.Error("expected state in login URL")
	}
	_ = state // used for callback test below
}

func TestSpotifyCallback_ErrorParam(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/spotify/callback?error=server_error&state=abc123", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rec.Code)
	}

	loc := rec.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected Location header")
	}
	if loc != "http://127.0.0.1:5173?spotify=denied" {
		t.Errorf("expected redirect to denied, got %s", loc)
	}
}

func TestSpotifyCallback_InvalidState(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/spotify/callback?code=testcode&state=invalidstate", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rec.Code)
	}

	loc := rec.Header().Get("Location")
	if loc != "http://127.0.0.1:5173?spotify=state_mismatch" {
		t.Errorf("expected redirect to state_mismatch, got %s", loc)
	}
}

func TestSpotifyCallback_NoCode(t *testing.T) {
	r := setupTestRouter(t)

	// First get a valid state from login
	loginReq := httptest.NewRequest(http.MethodGet, "/api/spotify/login", nil)
	loginReq.Header.Set("Accept", "application/json")
	loginRec := httptest.NewRecorder()
	r.ServeHTTP(loginRec, loginReq)

	var loginBody map[string]interface{}
	if err := json.NewDecoder(loginRec.Body).Decode(&loginBody); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	loginURL := loginBody["url"].(string)
	parsed, _ := url.Parse(loginURL)
	state := parsed.Query().Get("state")

	// Callback with valid state but no code
	req := httptest.NewRequest(http.MethodGet, "/api/spotify/callback?state="+state, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rec.Code)
	}

	loc := rec.Header().Get("Location")
	if loc != "http://127.0.0.1:5173?spotify=no_code" {
		t.Errorf("expected redirect to no_code, got %s", loc)
	}
}

func TestSpotifyStatus_NoAccount(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/spotify/status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["connected"] != false {
		t.Errorf("expected connected false, got %v", body["connected"])
	}
}

func TestSpotifyStatus_WithAccount(t *testing.T) {
	r := setupTestRouter(t)

	acc := &models.SpotifyAccount{
		SpotifyUserID:  "test-user-123",
		DisplayName:    "Test User",
		RefreshToken:   "refresh",
		AccessToken:    "access",
		TokenExpiresAt: time.Now().Add(time.Hour),
	}
	if err := db.DB.Create(acc).Error; err != nil {
		t.Fatalf("failed to create test account: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/spotify/status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["connected"] != true {
		t.Errorf("expected connected true, got %v", body["connected"])
	}
	if body["display_name"] != "Test User" {
		t.Errorf("expected display_name Test User, got %v", body["display_name"])
	}
}

func TestSpotifyMe_NoAccount(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/spotify/me", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if _, ok := body["error"]; !ok {
		t.Errorf("expected 'error' in response, got %v", body)
	}
}
