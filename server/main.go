package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/handlers"
	"juke-spotify-poc/server/spotify"
	"juke-spotify-poc/server/voting"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	loadDotEnv()
	// If .env had GEMINI_API_KEY= (empty), godotenv sets the var to "". Unset so our
	// fallback parser can still read a non-empty assignment from the same file.
	if strings.TrimSpace(os.Getenv("GEMINI_API_KEY")) == "" {
		_ = os.Unsetenv("GEMINI_API_KEY")
	}

	cfg := config.Load()
	if cfg.GeminiAPIKey == "" {
		log.Printf("VibeSense: GEMINI_API_KEY is empty — export it in your shell, add it to the IDE run env, or set it in server/.env (see server/.env.example)")
		log.Printf("VibeSense: if you already export GEMINI_API_KEY, IDEs often start Go without your shell profile — set the variable in the launch configuration")
		config.LogGeminiEnvHints()
	}

	if err := db.Connect(cfg); err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors.Default()) // Allow all origins for local dev

	// Routes
	r.GET("/", handlers.Root)
	r.GET("/health", handlers.Health)
	r.GET("/health/vibesense", handlers.VibeSenseHealth(cfg))
	r.GET("/api/health/vibesense", handlers.VibeSenseHealth(cfg))
	r.GET("/api/placeholder", handlers.Placeholder)

	// Spotify OAuth
	spotifyClient := spotify.NewClient(cfg)
	r.GET("/api/spotify/login", handlers.SpotifyLogin(cfg))
	r.GET("/api/spotify/callback", handlers.SpotifyCallback(cfg))
	r.POST("/api/spotify/disconnect", handlers.SpotifyDisconnect())
	r.GET("/api/spotify/status", handlers.SpotifyStatus(spotifyClient))
	r.GET("/api/spotify/devices", handlers.SpotifyDevices(spotifyClient))
	r.POST("/api/spotify/device", handlers.SpotifySetDevice(spotifyClient))
	r.GET("/api/spotify/me", handlers.SpotifyMe(spotifyClient))

	// Player & Playlists
	r.GET("/api/player/now-playing", handlers.PlayerNowPlaying(spotifyClient))
	r.GET("/api/playlists", handlers.PlaylistsList(spotifyClient))
	r.GET("/api/debug/playlist/:id", handlers.DebugPlaylistAccess(spotifyClient))

	r.POST("/api/ai-playlist/create", handlers.StartAIPlaylistJob(cfg, spotifyClient))
	r.GET("/api/ai-playlist/jobs/:id", handlers.AIPlaylistJobStatus())

	// Voting
	votingManager := voting.NewManager(spotifyClient)
	votingHandlers := &voting.Handlers{Manager: votingManager, Svc: spotifyClient}
	votingTicker := voting.NewTicker(votingManager, spotifyClient)
	votingTicker.Start()

	r.POST("/api/voting/session/start", votingHandlers.SessionStart)
	r.POST("/api/voting/session/end", votingHandlers.SessionEnd)
	r.GET("/api/voting/state", votingHandlers.State)
	r.GET("/api/voting/playlist-overview", votingHandlers.PlaylistOverview)
	r.POST("/api/voting/trigger-refill", votingHandlers.TriggerRefill)
	r.POST("/api/voting/vote", votingHandlers.Vote)

	srv := &http.Server{
		Addr:    "0.0.0.0:" + cfg.ServerPort,
		Handler: r,
	}

	go func() {
		log.Printf("Server starting on http://0.0.0.0:%s (also http://127.0.0.1:%s) — PORT env=%q", cfg.ServerPort, cfg.ServerPort, os.Getenv("PORT"))
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
