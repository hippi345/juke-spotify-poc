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

// vectorEvalVenueIDs returns distinct venue IDs referenced by vector eval cases.
func vectorEvalVenueIDs() []uint {
	seen := make(map[uint]bool)
	for _, c := range AllCases() {
		if c.VectorEval && c.VenueID > 0 {
			seen[c.VenueID] = true
		}
	}
	var out []uint
	for id := range seen {
		out = append(out, id)
	}
	return out
}

type liveVenueTrackJSON struct {
	SpotifyID string `json:"spotify_id"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Genres    string `json:"genres"`
}

// SeedLiveEvalVenueIndexes upserts SAMPLE catalog tracks for every vector-eval venue (1 and 2).
func SeedLiveEvalVenueIndexes(cfg *config.Config) error {
	osURL := LiveEvalOpenSearchURL(cfg)
	if err := vector.Init(true, osURL, cfg.GeminiAPIKey, cfg.EmbedProvider); err != nil {
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
		roof = canonicalVenueID
	}
	ctx := context.Background()
	want := vectorEvalVenueIDs()
	indexedByVenue := make(map[uint]int)
	for _, venueID := range want {
		if venueID == 1 {
			n, err := seedLiveVenueBarTracks(ctx, svc, venueID)
			if err != nil {
				return err
			}
			indexedByVenue[venueID] = n
			continue
		}
		n, err := seedCatalogVenueTracks(ctx, svc, doc, venueID, roof)
		if err != nil {
			return err
		}
		indexedByVenue[venueID] = n
	}
	for _, venueID := range want {
		if indexedByVenue[venueID] == 0 {
			return fmt.Errorf("no tracks indexed for live eval venue_id=%d", venueID)
		}
	}
	return nil
}

func seedCatalogVenueTracks(ctx context.Context, svc vector.Service, doc *vector.VenueCatalogFixture, venueID, roof uint) (int, error) {
	_ = roof
	indexed := 0
	for _, tr := range doc.Tracks {
		if tr.VenueID != venueID {
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
			return 0, fmt.Errorf("embed %s: %w", tr.Title, err)
		}
		docID := fmt.Sprintf("%d-%s", venueID, tr.SpotifyID)
		vdoc := vector.VenueTrackDoc{
			VenueID:   venueID,
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
			return 0, fmt.Errorf("upsert %s: %w", docID, err)
		}
		indexed++
	}
	return indexed, nil
}

func seedLiveVenueBarTracks(ctx context.Context, svc vector.Service, venueID uint) (int, error) {
	path := filepath.Join(fixtureRoot(), "vector", "live_venue_tracks.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var tracks []liveVenueTrackJSON
	if err := json.Unmarshal(raw, &tracks); err != nil {
		return 0, err
	}
	indexed := 0
	for _, tr := range tracks {
		if tr.SpotifyID == "" {
			continue
		}
		text := vector.TrackEmbedText(tr.Title, tr.Artist, tr.Genres)
		vec, err := svc.Embedder.Embed(text)
		if err != nil {
			return 0, err
		}
		docID := fmt.Sprintf("%d-%s", venueID, tr.SpotifyID)
		vdoc := vector.VenueTrackDoc{
			VenueID:   venueID,
			SpotifyID: tr.SpotifyID,
			Title:     tr.Title,
			Artist:    tr.Artist,
			Genres:    tr.Genres,
			VoteCount: 500,
			LastSeen:  time.Now().UTC().Format(time.RFC3339),
			Embedding: vec,
		}
		if err := svc.Indexer.Upsert(ctx, docID, vdoc); err != nil {
			return 0, err
		}
		indexed++
	}
	return indexed, nil
}
