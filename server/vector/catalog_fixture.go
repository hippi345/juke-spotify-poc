package vector

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// VenueCatalogFixture is the committed eval/demo venue history (SAMPLE data).
type VenueCatalogFixture struct {
	SchemaVersion      int      `json:"schema_version"`
	GeneratedSeed      int      `json:"generated_seed"`
	VenueRooftopID     uint     `json:"venue_rooftop_id"`
	TrackCount         int      `json:"track_count"`
	PlantedSunsetCount int      `json:"planted_sunset_count"`
	PlantedSunsetKeys  []string `json:"planted_sunset_keys"`
	HighlyVotedKeys    []string `json:"highly_voted_keys"`
	Tracks             []CatalogTrack `json:"tracks"`
}

// CatalogTrack is one row in venue_catalog.json.gz.
type CatalogTrack struct {
	SpotifyID     string `json:"spotify_id"`
	VenueID       uint   `json:"venue_id"`
	Title         string `json:"title"`
	Artist        string `json:"artist"`
	Genres        string `json:"genres"`
	Mood          string `json:"mood"`
	Tempo         string `json:"tempo"`
	PlayCount     int    `json:"play_count"`
	VoteCount     int    `json:"vote_count"`
	LastSeen      string `json:"last_seen"`
	PlantedSunset bool   `json:"planted_sunset"`
}

// DefaultCatalogFixturePath points at the shared eval/demo gzip fixture.
func DefaultCatalogFixturePath() string {
	if p := os.Getenv("VENUE_CATALOG_FIXTURE"); p != "" {
		return p
	}
	candidates := []string{
		filepath.Join("evals", "testdata", "vector", "venue_catalog.json.gz"),
		filepath.Join("..", "evals", "testdata", "vector", "venue_catalog.json.gz"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return candidates[0]
}

// LoadVenueCatalogFixture reads and parses venue_catalog.json.gz.
func LoadVenueCatalogFixture(path string) (*VenueCatalogFixture, error) {
	if path == "" {
		path = DefaultCatalogFixturePath()
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()
	var doc VenueCatalogFixture
	if err := json.NewDecoder(gz).Decode(&doc); err != nil {
		return nil, err
	}
	if len(doc.Tracks) == 0 {
		return nil, fmt.Errorf("catalog fixture %s: no tracks", path)
	}
	return &doc, nil
}
