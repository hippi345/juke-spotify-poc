package vector

import (
	"context"
	"testing"

	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type memVenueIndexer struct {
	docs map[string]VenueTrackDoc
}

func (m *memVenueIndexer) EnsureIndex(context.Context, int) error { return nil }
func (m *memVenueIndexer) Get(_ context.Context, docID string) (*VenueTrackDoc, error) {
	if d, ok := m.docs[docID]; ok {
		return &d, nil
	}
	return nil, nil
}
func (m *memVenueIndexer) Upsert(_ context.Context, docID string, doc VenueTrackDoc) error {
	m.docs[docID] = doc
	return nil
}
func (m *memVenueIndexer) DeleteByVenueID(_ context.Context, venueID uint) (int, error) {
	var n int
	for id, d := range m.docs {
		if d.VenueID == venueID {
			delete(m.docs, id)
			n++
		}
	}
	return n, nil
}
func (m *memVenueIndexer) SearchSimilar(context.Context, uint, []float32, int) ([]SearchHit, error) {
	return nil, nil
}
func (m *memVenueIndexer) SearchByText(context.Context, uint, string, Embedder, int) ([]SearchHit, error) {
	return nil, nil
}

func TestSampleDemoSeedAndReset(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&models.User{}, &models.Venue{}); err != nil {
		t.Fatal(err)
	}
	db.SetDB(database)

	idx := &memVenueIndexer{docs: map[string]VenueTrackDoc{}}
	SetDefaultService(Service{
		Enabled:  true,
		Indexer:  idx,
		Embedder: HashEmbedder{Dim: IndexEmbeddingDim},
	})

	path := DefaultCatalogFixturePath()
	if _, err := LoadVenueCatalogFixture(path); err != nil {
		t.Fatalf("fixture %s: %v", path, err)
	}

	ctx := context.Background()
	res, err := SeedSampleDemoCatalog(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if res.VenueName != SampleDemoVenueName {
		t.Fatalf("venue name %q", res.VenueName)
	}
	if res.TracksIndexed < 3000 {
		t.Fatalf("expected 3k+ tracks, got %d", res.TracksIndexed)
	}
	if len(idx.docs) != res.TracksIndexed {
		t.Fatalf("indexer doc count %d != reported %d", len(idx.docs), res.TracksIndexed)
	}
	for _, d := range idx.docs {
		if d.VenueID != res.VenueID {
			t.Fatalf("doc venue_id %d != demo %d", d.VenueID, res.VenueID)
		}
	}

	venueID, deleted, err := ResetSampleDemo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if venueID != res.VenueID || deleted != res.TracksIndexed {
		t.Fatalf("reset venue=%d deleted=%d want venue=%d deleted=%d", venueID, deleted, res.VenueID, res.TracksIndexed)
	}
	var count int64
	if err := db.DB.Model(&models.Venue{}).Where("name = ?", SampleDemoVenueName).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("venue row still present")
	}

	staff, err := EnsureSampleDemoStaff()
	if err != nil {
		t.Fatal(err)
	}
	if staff.Role != "staff" || staff.Email != SampleDemoStaffEmail {
		t.Fatalf("staff %+v", staff)
	}
}
