package main

import (
	"context"
	"log"
	"os"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/vector"
)

func main() {
	cfg := config.Load()
	if cfg.OpenSearchURL == "" {
		log.Fatalf("%s OPENSEARCH_URL is required (e.g. http://127.0.0.1:9200)", vector.SampleDemoLogPrefix)
	}
	if err := db.Connect(cfg); err != nil {
		log.Fatalf("%s MySQL: %v", vector.SampleDemoLogPrefix, err)
	}
	// Force vector on for seeding SAMPLE demo catalog (independent of API default).
	if err := vector.Init(true, cfg.OpenSearchURL, cfg.GeminiAPIKey, cfg.EmbedProvider); err != nil {
		log.Fatalf("%s vector init: %v", vector.SampleDemoLogPrefix, err)
	}
	catalogPath := vector.DefaultCatalogFixturePath()
	if len(os.Args) > 1 {
		catalogPath = os.Args[1]
	}
	log.Printf("%s loading fixture %s into OpenSearch (not real venue data)", vector.SampleDemoLogPrefix, catalogPath)
	res, err := vector.SeedSampleDemoCatalog(context.Background(), catalogPath)
	if err != nil {
		log.Fatalf("%s seed failed: %v", vector.SampleDemoLogPrefix, err)
	}
	log.Printf("%s done — venue_id=%d name=%q tracks=%d",
		vector.SampleDemoLogPrefix, res.VenueID, res.VenueName, res.TracksIndexed)
	log.Printf("%s try AI playlist with venue_id=%d and VIBESENSE_VECTOR_ENABLED=true", vector.SampleDemoLogPrefix, res.VenueID)
}
