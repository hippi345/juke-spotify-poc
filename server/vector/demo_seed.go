package vector

import (
	"context"
	"fmt"
	"log"
	"strings"

	"juke-spotify-poc/server/auth"
	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"
	"juke-spotify-poc/server/search"
)

// Sample demo venue — clearly labeled SAMPLE data (not real venue history).
const (
	SampleDemoVenueName  = "Sample Rooftop (demo data)"
	SampleDemoStaffEmail = "sample-rooftop-demo@jukespotify.local"
	SampleDemoLogPrefix  = "SAMPLE demo data:"
)

// SampleDemoSeedResult is returned after a successful seed.
type SampleDemoSeedResult struct {
	VenueID      uint
	VenueName    string
	TracksIndexed int
	CatalogPath  string
}

// EnsureSampleDemoStaff returns a staff user for the demo venue (creates if missing).
func EnsureSampleDemoStaff() (models.User, error) {
	var u models.User
	err := db.DB.Where("email = ?", SampleDemoStaffEmail).First(&u).Error
	if err == nil {
		return u, nil
	}
	hash, err := auth.HashPassword("sample-demo-not-for-login")
	if err != nil {
		return models.User{}, err
	}
	u = models.User{
		Email:        SampleDemoStaffEmail,
		PasswordHash: hash,
		Role:         "staff",
	}
	if err := db.DB.Create(&u).Error; err != nil {
		return models.User{}, err
	}
	log.Printf("%s created staff placeholder user id=%d (not intended for UI login)", SampleDemoLogPrefix, u.ID)
	return u, nil
}

// EnsureSampleDemoVenue finds or creates the named demo venue.
func EnsureSampleDemoVenue(staffID uint) (models.Venue, error) {
	var v models.Venue
	err := db.DB.Where("name = ?", SampleDemoVenueName).First(&v).Error
	if err == nil {
		return v, nil
	}
	v = models.Venue{
		StaffUserID: staffID,
		Name:        SampleDemoVenueName,
		Latitude:    40.758,
		Longitude:   -73.9855,
	}
	if err := db.DB.Create(&v).Error; err != nil {
		return models.Venue{}, err
	}
	_ = search.Default.IndexVenue(context.Background(), search.VenueHit{
		ID:        v.ID,
		Name:      v.Name,
		Latitude:  v.Latitude,
		Longitude: v.Longitude,
		StaffID:   v.StaffUserID,
	})
	log.Printf("%s created venue id=%d name=%q", SampleDemoLogPrefix, v.ID, v.Name)
	return v, nil
}

// SeedSampleDemoCatalog loads the shared catalog fixture into OpenSearch for the demo venue.
func SeedSampleDemoCatalog(ctx context.Context, catalogPath string) (SampleDemoSeedResult, error) {
	staff, err := EnsureSampleDemoStaff()
	if err != nil {
		return SampleDemoSeedResult{}, err
	}
	venue, err := EnsureSampleDemoVenue(staff.ID)
	if err != nil {
		return SampleDemoSeedResult{}, err
	}
	s := Default()
	if !s.Enabled || s.Indexer == nil {
		return SampleDemoSeedResult{}, fmt.Errorf("vector service not enabled (set OPENSEARCH_URL and run with vector init)")
	}
	doc, err := LoadVenueCatalogFixture(catalogPath)
	if err != nil {
		return SampleDemoSeedResult{}, err
	}
	srcVenue := doc.VenueRooftopID
	if srcVenue == 0 {
		srcVenue = 2
	}
	indexed := 0
	for i, tr := range doc.Tracks {
		if tr.VenueID != srcVenue {
			continue
		}
		if strings.TrimSpace(tr.SpotifyID) == "" {
			continue
		}
		text := TrackEmbedText(tr.Title, tr.Artist, catalogGenres(tr))
		vec, err := s.Embedder.Embed(text)
		if err != nil {
			return SampleDemoSeedResult{}, fmt.Errorf("embed track %q: %w", tr.Title, err)
		}
		docID := fmt.Sprintf("%d-%s", venue.ID, tr.SpotifyID)
		vdoc := VenueTrackDoc{
			VenueID:   venue.ID,
			SpotifyID: tr.SpotifyID,
			Title:     tr.Title,
			Artist:    tr.Artist,
			Genres:    catalogGenres(tr),
			PlayCount: tr.PlayCount,
			VoteCount: tr.VoteCount,
			LastSeen:  tr.LastSeen,
			Embedding: vec,
		}
		if err := s.Indexer.Upsert(ctx, docID, vdoc); err != nil {
			return SampleDemoSeedResult{}, fmt.Errorf("upsert %s: %w", docID, err)
		}
		indexed++
		if (i+1)%500 == 0 {
			log.Printf("%s indexed %d tracks so far…", SampleDemoLogPrefix, indexed)
		}
	}
	log.Printf("%s seeded %d SAMPLE tracks into OpenSearch for venue_id=%d (%q)",
		SampleDemoLogPrefix, indexed, venue.ID, venue.Name)
	return SampleDemoSeedResult{
		VenueID:       venue.ID,
		VenueName:     venue.Name,
		TracksIndexed: indexed,
		CatalogPath:   catalogPath,
	}, nil
}

func catalogGenres(tr CatalogTrack) string {
	g := strings.TrimSpace(tr.Genres)
	if g != "" {
		return g
	}
	if tr.Mood != "" {
		return tr.Mood
	}
	return ""
}

// ResetSampleDemo removes SAMPLE demo vector documents and the demo venue row.
func ResetSampleDemo(ctx context.Context) (uint, int, error) {
	var v models.Venue
	if err := db.DB.Where("name = ?", SampleDemoVenueName).First(&v).Error; err != nil {
		log.Printf("%s no demo venue found — nothing to reset", SampleDemoLogPrefix)
		return 0, 0, nil
	}
	deleted := 0
	s := Default()
	if s.Enabled && s.Indexer != nil {
		n, err := s.Indexer.DeleteByVenueID(ctx, v.ID)
		if err != nil {
			return v.ID, 0, err
		}
		deleted = n
	}
	if err := db.DB.Delete(&v).Error; err != nil {
		return v.ID, deleted, err
	}
	log.Printf("%s removed venue id=%d and %d OpenSearch SAMPLE track docs", SampleDemoLogPrefix, v.ID, deleted)
	return v.ID, deleted, nil
}
