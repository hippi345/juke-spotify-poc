//go:build evals_live

package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/vector"
)

const liveEvalVenueID = 1

type liveVenueTrack struct {
	SpotifyID string `json:"spotify_id"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Genres    string `json:"genres"`
}

// SeedLiveEvalVenueIndex upserts well-known tracks for venue 1 (live vector arm).
func SeedLiveEvalVenueIndex(cfg *config.Config) error {
	if err := vector.Init(true, cfg.OpenSearchURL, cfg.GeminiAPIKey, cfg.EmbedProvider); err != nil {
		return err
	}
	svc := vector.Default()
	if !svc.Enabled || svc.Indexer == nil {
		return fmt.Errorf("vector service not enabled for live seed")
	}
	path := filepath.Join(fixtureRoot(), "vector", "live_venue_tracks.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var tracks []liveVenueTrack
	if err := json.Unmarshal(raw, &tracks); err != nil {
		return err
	}
	ctx := context.Background()
	for _, tr := range tracks {
		if tr.SpotifyID == "" {
			continue
		}
		text := vector.TrackEmbedText(tr.Title, tr.Artist, tr.Genres)
		vec, err := svc.Embedder.Embed(text)
		if err != nil {
			return fmt.Errorf("embed %s: %w", tr.Title, err)
		}
		docID := fmt.Sprintf("%d-%s", liveEvalVenueID, tr.SpotifyID)
		doc := vector.VenueTrackDoc{
			VenueID:   liveEvalVenueID,
			SpotifyID: tr.SpotifyID,
			Title:     tr.Title,
			Artist:    tr.Artist,
			Genres:    tr.Genres,
			LastSeen:  time.Now().UTC().Format(time.RFC3339),
			Embedding: vec,
		}
		if err := svc.Indexer.Upsert(ctx, docID, doc); err != nil {
			return fmt.Errorf("upsert %s: %w", docID, err)
		}
	}
	return nil
}
