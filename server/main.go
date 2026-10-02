package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"juke-spotify-poc/server/auth"
	"juke-spotify-poc/server/cache"
	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/events"
	"juke-spotify-poc/server/handlers"
	"juke-spotify-poc/server/metrics"
	"juke-spotify-poc/server/middleware"
	"juke-spotify-poc/server/models"
	"juke-spotify-poc/server/paidskip"
	"juke-spotify-poc/server/search"
	"juke-spotify-poc/server/spotify"
	"juke-spotify-poc/server/tracing"
	"juke-spotify-poc/server/venue"
	"juke-spotify-poc/server/voting"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
	if err := spotify.BootstrapVenueAccountFromEnv(cfg); err != nil {
		log.Printf("Spotify venue bootstrap: %v", err)
	}
	if err := cache.InitRedis(cfg.RedisAddr); err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}
	if err := search.InitOpenSearch(cfg.OpenSearchURL); err != nil {
		log.Fatalf("OpenSearch init failed: %v", err)
	}
	if err := events.InitKafka(cfg.KafkaBrokers); err != nil {
		log.Fatalf("Kafka init failed: %v", err)
	}

	shutdownTrace, err := tracing.Init(context.Background(), cfg.OTLPEndpoint, "juke-api", cfg.InstanceID)
	if err != nil {
		log.Fatalf("Tracing init failed: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors.Default()) // Allow all origins for local dev
	if cfg.OTLPEndpoint != "" {
		r.Use(otelgin.Middleware("juke-api"))
	}
	if cfg.MetricsEnabled {
		r.Use(metrics.Middleware())
		metrics.Register(r)
	}
	r.Use(middleware.InstanceID(cfg.InstanceID))
	r.Use(auth.Middleware(cfg.AuthSecret))

	authHandlers := &auth.Handlers{Secret: cfg.AuthSecret}
	venueHandlers := &venue.Handlers{}

	// Routes
	r.GET("/", handlers.Root)
	r.GET("/health", handlers.Health)
	r.GET("/health/vibesense", handlers.VibeSenseHealth(cfg))
	r.GET("/api/health/vibesense", handlers.VibeSenseHealth(cfg))
	r.GET("/api/placeholder", handlers.Placeholder)

	// Email accounts (staff + patron)
	r.POST("/api/auth/register", authHandlers.Register)
	r.POST("/api/auth/login", authHandlers.Login)
	r.GET("/api/auth/me", authHandlers.Me)

	// Venues & patron join
	r.POST("/api/venues", venueHandlers.Create)
	r.GET("/api/venues/mine", venueHandlers.Mine)
	r.GET("/api/venues/search", venueHandlers.Search)
	r.GET("/api/venues/nearby", venueHandlers.Nearby)
	r.POST("/api/venues/:id/join", venueHandlers.Join)

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
	voting.RecoverOrphanedActiveSessions(votingManager)
	votingHandlers := &voting.Handlers{Manager: votingManager, Svc: spotifyClient}
	votingTicker := voting.NewTicker(votingManager, spotifyClient)
	votingTicker.Start()

	ctx, stopKafka := context.WithCancel(context.Background())
	events.RunConsumer(ctx, cfg.KafkaBrokers, "juke-api-"+cfg.InstanceID, []string{
		events.TopicPayment,
		events.TopicSession,
		events.TopicVote,
	}, func(c context.Context, topic string, value []byte, _ []kafka.Header) error {
		return handleKafkaMessage(c, topic, value, votingManager)
	})

	r.POST("/api/voting/session/start", votingHandlers.SessionStart)
	r.POST("/api/voting/session/end", votingHandlers.SessionEnd)
	r.GET("/api/voting/state", votingHandlers.State)
	r.GET("/api/voting/playlist-overview", votingHandlers.PlaylistOverview)
	r.POST("/api/voting/trigger-refill", votingHandlers.TriggerRefill)
	r.POST("/api/voting/vote", votingHandlers.Vote)

	paidSkipHandlers := &paidskip.Handlers{Manager: votingManager, Config: cfg}
	r.GET("/api/paid-skip/config", paidSkipHandlers.ConfigInfo)
	r.POST("/api/paid-skip/checkout", paidSkipHandlers.CreateCheckout)
	r.POST("/api/stripe/webhook", paidSkipHandlers.StripeWebhook)

	r.NoRoute(handlers.NotFound)

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
	stopKafka()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if shutdownTrace != nil {
		_ = shutdownTrace(shutdownCtx)
	}

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func handleKafkaMessage(ctx context.Context, topic string, value []byte, mgr *voting.Manager) error {
	tracer := otel.Tracer(tracing.InstrumentationName)
	ctx, span := tracer.Start(ctx, "kafka.consume",
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", topic),
		),
	)
	defer span.End()

	switch topic {
	case events.TopicPayment:
		var p events.PaymentPayload
		if err := json.Unmarshal(value, &p); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
		span.SetAttributes(
			attribute.Int64("paid_skip.id", int64(p.PaidSkipID)),
			attribute.Int64("voting.session_id", int64(p.VotingSessionID)),
		)
		var skip models.PaidSkip
		if err := db.DB.First(&skip, p.PaidSkipID).Error; err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
		if skip.Status != "paid" {
			return nil
		}
		log.Printf("kafka payment: session=%d paid_skip_id=%d track=%s", p.VotingSessionID, p.PaidSkipID, skip.TrackID)
		if err := mgr.ApplyPaidSkipAfterPayment(&skip); err != nil {
			log.Printf("kafka payment: apply paid skip %d: %v", skip.ID, err)
			span.RecordError(err)
		}
	case events.TopicSession:
		var p events.SessionPayload
		if err := json.Unmarshal(value, &p); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
		span.SetAttributes(
			attribute.String("session.action", p.Action),
			attribute.Int64("voting.session_id", int64(p.SessionID)),
		)
		log.Printf("kafka session: action=%s session_id=%d", p.Action, p.SessionID)
		if p.Action == "ended" {
			_ = cache.Default.Delete(ctx, "jukespotify:spotify:currently_playing")
		}
	case events.TopicVote:
		var p events.VotePayload
		if err := json.Unmarshal(value, &p); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
		span.SetAttributes(
			attribute.Int64("voting.session_id", int64(p.SessionID)),
			attribute.Int64("patron.id", int64(p.PatronID)),
			attribute.String("track.id", p.TrackID),
		)
		log.Printf("kafka vote: session=%d patron=%d track=%s", p.SessionID, p.PatronID, p.TrackID)
	}
	return nil
}
