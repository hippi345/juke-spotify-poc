package evals

import (
	"strings"
	"testing"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/spotify"
)

func TestSpotifyClientAdapter_nilDB(t *testing.T) {
	old := db.DB
	db.DB = nil
	defer func() { db.DB = old }()

	cfg := &config.Config{SpotifyClientID: "id", SpotifyClientSecret: "secret"}
	adapter := SpotifyClientAdapter{Client: spotify.NewClient(cfg)}
	_, err := adapter.SearchTracks("artist track", 5)
	if err == nil || !strings.Contains(err.Error(), "database not initialized") {
		t.Fatalf("expected database guard error, got %v", err)
	}
}
