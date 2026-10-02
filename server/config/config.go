package config

import (
	"log"
	"os"
	"strings"
)

// Config holds application configuration from environment
type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	ServerPort string

	SpotifyClientID     string
	SpotifyClientSecret string
	SpotifyRedirectURI  string
	// Long-lived refresh token for the venue playback account (automation / CI only).
	SpotifyVenueRefreshToken string
	AppBaseURL          string

	// Google AI (Gemini) — used for AI-generated playlists (server-side only)
	GeminiAPIKey  string
	GeminiModel   string

	// HMAC secret for email login tokens (set AUTH_SECRET in production).
	AuthSecret string

	// Stripe test-mode secret for paid-skip demo (read from STRIPE_TEST_SECRET_KEY only).
	StripeTestSecretKey string
	StripeWebhookSecret string

	RedisAddr       string
	OpenSearchURL   string
	KafkaBrokers    string
	MetricsEnabled  bool
	InstanceID      string

	// OTLP gRPC endpoint for Tempo (e.g. tempo:4317). Empty disables export.
	OTLPEndpoint string
}

// Load reads config from environment with sensible defaults for local Docker MySQL
func Load() *Config {
	// Default 8081: on Windows, 8080 is often bound by PostgreSQL/EnterpriseDB (not this API).
	port := getEnv("PORT", "8081")
	return &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "3306"),
		DBUser:     getEnv("DB_USER", "root"),
		DBPassword: getEnv("DB_PASSWORD", "jukespotify"),
		DBName:     getEnv("DB_NAME", "jukespotify"),
		ServerPort: port,

		SpotifyClientID:          getEnv("SPOTIFY_CLIENT_ID", ""),
		SpotifyClientSecret:      getEnv("SPOTIFY_CLIENT_SECRET", ""),
		SpotifyRedirectURI:       getEnv("SPOTIFY_REDIRECT_URI", "http://127.0.0.1:5173/api/spotify/callback"),
		SpotifyVenueRefreshToken: strings.TrimSpace(getEnv("SPOTIFY_REFRESH_TOKEN", "")),
		AppBaseURL:          getEnv("APP_BASE_URL", "http://localhost:5173"),

		GeminiAPIKey: loadGeminiAPIKey(),
		GeminiModel:  getEnv("GEMINI_MODEL", "gemini-2.5-flash"),

		AuthSecret: loadAuthSecret(),

		StripeTestSecretKey: strings.TrimSpace(getEnv("STRIPE_TEST_SECRET_KEY", "")),
		StripeWebhookSecret: strings.TrimSpace(getEnv("STRIPE_WEBHOOK_SECRET", "")),

		RedisAddr:      strings.TrimSpace(getEnv("REDIS_ADDR", "")),
		OpenSearchURL:  strings.TrimSpace(getEnv("OPENSEARCH_URL", "")),
		KafkaBrokers:   strings.TrimSpace(getEnv("KAFKA_BROKERS", "")),
		MetricsEnabled: getEnv("METRICS_ENABLED", "") == "1" || strings.EqualFold(getEnv("METRICS_ENABLED", ""), "true"),
		InstanceID:     getEnv("INSTANCE_ID", "api"),
		OTLPEndpoint:   strings.TrimSpace(getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "")),
	}
}

func loadAuthSecret() string {
	s := getEnv("AUTH_SECRET", "")
	if s != "" {
		return s
	}
	log.Printf("AUTH_SECRET is empty — using insecure local-dev default; set AUTH_SECRET in production")
	return "local-dev-only-not-for-production"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadGeminiAPIKey prefers the process environment (GEMINI_API_KEY) so you can export
// the key in your shell or IDE without putting it in .env. godotenv.Load does not override
// an existing env var. File fallback is only used when the variable is unset/empty.
func loadGeminiAPIKey() string {
	k := strings.TrimSpace(getEnv("GEMINI_API_KEY", ""))
	if k != "" {
		return k
	}
	return strings.TrimSpace(geminiKeyFromEnvFiles())
}
