package spotify

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestGetDefaultAccount_nilDB(t *testing.T) {
	old := db.DB
	db.DB = nil
	defer func() { db.DB = old }()

	c := NewClient(&config.Config{})
	_, err := c.GetDefaultAccount()
	if err == nil || !strings.Contains(err.Error(), "database not initialized") {
		t.Fatalf("expected database error, got %v", err)
	}
}

func TestSpotify429FailFast_quotaExceeded(t *testing.T) {
	err := spotify429FailFast("5", []byte(`{"error":{"reason":"QUOTA_EXCEEDED"}}`))
	if err == nil || !strings.Contains(err.Error(), "spotify quota exhausted") {
		t.Fatalf("expected quota error, got %v", err)
	}
}

func TestSpotify429FailFast_longRetryAfter(t *testing.T) {
	err := spotify429FailFast("60", []byte(`{"error":{"status":429}}`))
	if err == nil || !strings.Contains(err.Error(), "retry-after") {
		t.Fatalf("expected Retry-After cap error, got %v", err)
	}
}

func TestDoWithRetry_quotaExceededNoRetry(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"reason":"QUOTA_EXCEEDED"}}`))
	}))
	defer srv.Close()

	c, cleanup := testClientWithAccount(t, srv.URL)
	defer cleanup()

	_, err := c.DoWithRetry(http.MethodGet, "/quota-test", nil, 3)
	if err == nil || !strings.Contains(err.Error(), "spotify quota exhausted") {
		t.Fatalf("expected quota exhausted, got %v (hits=%d)", err, hits)
	}
	if hits != 1 {
		t.Fatalf("expected single request (no retries), got %d", hits)
	}
}

func testClientWithAccount(t *testing.T, apiBase string) (*Client, func()) {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := database.AutoMigrate(&models.SpotifyAccount{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB := db.DB
	db.DB = database
	acc := models.SpotifyAccount{
		AccessToken:      "test-access-token",
		RefreshToken:     "test-refresh-token",
		TokenExpiresAt:   time.Now().Add(24 * time.Hour),
		SpotifyUserID:    "user",
		DisplayName:      "Test",
	}
	if err := database.Create(&acc).Error; err != nil {
		t.Fatalf("seed account: %v", err)
	}

	cfg := &config.Config{SpotifyClientID: "id", SpotifyClientSecret: "secret"}
	c := NewClient(cfg)
	// Route API calls to httptest (path is joined with spotifyAPIURL in DoWithRetry).
	c.client = &http.Client{
		Transport: &spotifyAPITestTransport{serverURL: apiBase},
	}
	return c, func() { db.DB = oldDB }
}

type spotifyAPITestTransport struct {
	serverURL string
}

func (st *spotifyAPITestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u, err := url.Parse(st.serverURL)
	if err != nil {
		return nil, err
	}
	req.URL.Scheme = u.Scheme
	req.URL.Host = u.Host
	return http.DefaultTransport.RoundTrip(req)
}
