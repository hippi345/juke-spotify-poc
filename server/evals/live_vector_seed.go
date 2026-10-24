//go:build evals_live

package evals

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/vector"
)

const liveEvalVenueID = 2 // canonical SAMPLE rooftop (shared catalog)

// SeedLiveEvalVenueIndex upserts SAMPLE catalog tracks for venue 2 (live vector arm).
func SeedLiveEvalVenueIndex(cfg *config.Config) error {
	if err := vector.Init(true, cfg.OpenSearchURL, cfg.GeminiAPIKey, cfg.EmbedProvider); err != nil {
		return err
	}
	svc := vector.Default()
	if !svc.Enabled || svc.Indexer == nil {
		return fmt.Errorf("vector service not enabled for live seed")
	}
	path := filepath.Join(fixtureRoot(), "vector", "venue_catalog.json.gz")
	if _, err := os.Stat(path); err != nil {
		path = vector.DefaultCatalogFixturePath()
	}
	doc, err := vector.LoadVenueCatalogFixture(path)
	if err != nil {
		return err
	}
	roof := doc.VenueRooftopID
	if roof == 0 {
		roof = liveEvalVenueID
	}
	ctx := context.Background()
	indexed := 0
	for _, tr := range doc.Tracks {
		if tr.VenueID != roof {
			continue
		}
		if tr.SpotifyID == "" {
			continue
		}
		meta := tr.Genres + " " + tr.Mood
		if tr.PlantedSunset {
			meta += " crowd favorite sunset enjoyed"
		}
		text := vector.TrackEmbedText(tr.Title, tr.Artist, meta)
		vec, err := svc.Embedder.Embed(text)
		if err != nil {
			return fmt.Errorf("embed %s: %w", tr.Title, err)
		}
		docID := fmt.Sprintf("%d-%s", liveEvalVenueID, tr.SpotifyID)
		vdoc := vector.VenueTrackDoc{
			VenueID:   liveEvalVenueID,
			SpotifyID: tr.SpotifyID,
			Title:     tr.Title,
			Artist:    tr.Artist,
			Genres:    tr.Genres,
			PlayCount: tr.PlayCount,
			VoteCount: tr.VoteCount,
			LastSeen:  tr.LastSeen,
			Embedding: vec,
		}
		if vdoc.LastSeen == "" {
			vdoc.LastSeen = time.Now().UTC().Format(time.RFC3339)
		}
		if err := svc.Indexer.Upsert(ctx, docID, vdoc); err != nil {
			return fmt.Errorf("upsert %s: %w", docID, err)
		}
		indexed++
	}
	if indexed == 0 {
		return fmt.Errorf("no SAMPLE catalog tracks indexed")
	}
	return nil
}
