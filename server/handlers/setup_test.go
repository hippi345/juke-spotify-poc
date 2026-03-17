package handlers

import (
	"testing"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"
	"juke-spotify-poc/server/spotify"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestRouter(t *testing.T) *gin.Engine {
	t.Helper()

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	if err := database.AutoMigrate(&models.SpotifyAccount{}, &models.VotingSession{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	db.SetDB(database)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	cfg := &config.Config{
		SpotifyClientID:     "test-client-id",
		SpotifyClientSecret: "test-client-secret",
		SpotifyRedirectURI:  "http://127.0.0.1:5173/api/spotify/callback",
		AppBaseURL:         "http://127.0.0.1:5173",
	}

	spotifyClient := spotify.NewClient(cfg)

	r.GET("/health", Health)
	r.GET("/api/placeholder", Placeholder)
	r.GET("/api/spotify/login", SpotifyLogin(cfg))
	r.GET("/api/spotify/callback", SpotifyCallback(cfg))
	r.GET("/api/spotify/status", SpotifyStatus(spotifyClient))
	r.GET("/api/spotify/me", SpotifyMe(spotifyClient))

	return r
}

func setupTestRouterNoSpotifyCreds(t *testing.T) *gin.Engine {
	t.Helper()

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	if err := database.AutoMigrate(&models.SpotifyAccount{}, &models.VotingSession{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	db.SetDB(database)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	cfg := &config.Config{
		SpotifyClientID:     "",
		SpotifyClientSecret: "",
		SpotifyRedirectURI:  "http://127.0.0.1:5173/api/spotify/callback",
		AppBaseURL:         "http://127.0.0.1:5173",
	}

	spotifyClient := spotify.NewClient(cfg)

	r.GET("/api/spotify/login", SpotifyLogin(cfg))
	r.GET("/api/spotify/callback", SpotifyCallback(cfg))
	r.GET("/api/spotify/status", SpotifyStatus(spotifyClient))
	r.GET("/api/spotify/me", SpotifyMe(spotifyClient))

	return r
}
