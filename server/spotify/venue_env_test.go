package spotify

import (
	"testing"

	"juke-spotify-poc/server/config"
)

func TestBootstrapVenueAccountFromEnv_skipsWhenUnset(t *testing.T) {
	if err := BootstrapVenueAccountFromEnv(&config.Config{}); err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if err := BootstrapVenueAccountFromEnv(&config.Config{
		SpotifyClientID:          "id",
		SpotifyClientSecret:      "secret",
		SpotifyVenueRefreshToken: "",
	}); err != nil {
		t.Fatalf("expected nil err when refresh unset, got %v", err)
	}
}
