package config

import (
	"os"
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
	AppBaseURL          string
}

// Load reads config from environment with sensible defaults for local Docker MySQL
func Load() *Config {
	port := getEnv("PORT", "8080")
	return &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "3306"),
		DBUser:     getEnv("DB_USER", "root"),
		DBPassword: getEnv("DB_PASSWORD", "jukespotify"),
		DBName:     getEnv("DB_NAME", "jukespotify"),
		ServerPort: port,

		SpotifyClientID:     getEnv("SPOTIFY_CLIENT_ID", ""),
		SpotifyClientSecret: getEnv("SPOTIFY_CLIENT_SECRET", ""),
		SpotifyRedirectURI:  getEnv("SPOTIFY_REDIRECT_URI", "http://127.0.0.1:5173/api/spotify/callback"),
		AppBaseURL:          getEnv("APP_BASE_URL", "http://localhost:5173"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
