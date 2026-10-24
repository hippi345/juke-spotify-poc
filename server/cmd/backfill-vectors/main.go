package main

import (
	"context"
	"flag"
	"log"
	"os"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/spotify"
	"juke-spotify-poc/server/vector"
)

func main() {
	venueID := flag.Uint("venue-id", 0, "venue id to backfill")
	flag.Parse()
	if *venueID == 0 {
		log.Fatal("venue-id required")
	}
	cfg := config.Load()
	if err := db.Connect(cfg); err != nil {
		log.Fatalf("db: %v", err)
	}
	enabled := os.Getenv("VIBESENSE_VECTOR_ENABLED") == "1" || os.Getenv("VIBESENSE_VECTOR_ENABLED") == "true"
	if err := vector.Init(enabled, cfg.OpenSearchURL, cfg.GeminiAPIKey, cfg.EmbedProvider); err != nil {
		log.Fatalf("vector init: %v", err)
	}
	spotifyClient := spotify.NewClient(cfg)
	n, err := vector.BackfillVenue(context.Background(), uint(*venueID), spotifyClient)
	if err != nil {
		log.Fatalf("backfill: %v", err)
	}
	log.Printf("backfill complete (%d tracks indexed)", n)
}
