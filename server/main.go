package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/handlers"
	"juke-spotify-poc/server/spotify"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	if err := db.Connect(cfg); err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors.Default()) // Allow all origins for local dev

	// Routes
	r.GET("/health", handlers.Health)
	r.GET("/api/placeholder", handlers.Placeholder)

	// Spotify OAuth
	spotifyClient := spotify.NewClient(cfg)
	r.GET("/api/spotify/login", handlers.SpotifyLogin(cfg))
	r.GET("/api/spotify/callback", handlers.SpotifyCallback(cfg))
	r.GET("/api/spotify/status", handlers.SpotifyStatus(spotifyClient))
	r.GET("/api/spotify/me", handlers.SpotifyMe(spotifyClient))

	srv := &http.Server{
		Addr:    "0.0.0.0:" + cfg.ServerPort,
		Handler: r,
	}

	go func() {
		log.Printf("Server starting on http://0.0.0.0:%s (also http://127.0.0.1:%s)", cfg.ServerPort, cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
