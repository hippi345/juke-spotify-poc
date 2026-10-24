package main

import (
	"context"
	"log"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/vector"
)

func main() {
	cfg := config.Load()
	if err := db.Connect(cfg); err != nil {
		log.Fatalf("%s MySQL: %v", vector.SampleDemoLogPrefix, err)
	}
	if cfg.OpenSearchURL != "" {
		if err := vector.Init(true, cfg.OpenSearchURL, cfg.GeminiAPIKey, cfg.EmbedProvider); err != nil {
			log.Fatalf("%s vector init: %v", vector.SampleDemoLogPrefix, err)
		}
	} else {
		log.Printf("%s OPENSEARCH_URL unset — will only remove MySQL demo venue row if present", vector.SampleDemoLogPrefix)
	}
	venueID, deleted, err := vector.ResetSampleDemo(context.Background())
	if err != nil {
		log.Fatalf("%s reset failed: %v", vector.SampleDemoLogPrefix, err)
	}
	if venueID == 0 {
		log.Printf("%s reset complete (no demo venue was present)", vector.SampleDemoLogPrefix)
		return
	}
	log.Printf("%s reset complete — removed venue_id=%d and %d OpenSearch docs", vector.SampleDemoLogPrefix, venueID, deleted)
}
