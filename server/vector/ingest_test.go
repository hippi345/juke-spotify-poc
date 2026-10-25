package vector

import (
	"context"
	"sync"
	"testing"
	"time"
)

type memIndexer struct {
	mu   sync.Mutex
	docs map[string]VenueTrackDoc
}

func (m *memIndexer) EnsureIndex(context.Context, int) error { return nil }

func (m *memIndexer) Get(_ context.Context, docID string) (*VenueTrackDoc, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.docs[docID]; ok {
		cp := d
		return &cp, nil
	}
	return nil, nil
}

func (m *memIndexer) Upsert(_ context.Context, docID string, doc VenueTrackDoc) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.docs[docID] = doc
	return nil
}

func (m *memIndexer) DeleteByVenueID(_ context.Context, venueID uint) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n int
	for id, d := range m.docs {
		if d.VenueID == venueID {
			delete(m.docs, id)
			n++
		}
	}
	return n, nil
}

func (m *memIndexer) SearchSimilar(context.Context, uint, []float32, int, time.Time) ([]SearchHit, error) {
	return nil, nil
}

func (m *memIndexer) SearchByText(context.Context, uint, string, Embedder, int, time.Time) ([]SearchHit, error) {
	return nil, nil
}

func TestUpsertPreservesVoteCountOnBackfill(t *testing.T) {
	idx := &memIndexer{docs: map[string]VenueTrackDoc{}}
	svc := Service{
		Enabled:  true,
		Indexer:  idx,
		Embedder: HashEmbedder{Dim: 8},
	}
	ctx := context.Background()
	docID := "1-walk-track-bravo"
	_ = idx.Upsert(ctx, docID, VenueTrackDoc{
		VenueID: 1, SpotifyID: "walk-track-bravo", Title: "Walk Track Bravo", Artist: "Walk Artist",
		VoteCount: 5, PlayCount: 1, Embedding: []float32{0.1, 0.2},
	})

	upsertTrack(ctx, svc, 1, "walk-track-bravo", "Walk Track Bravo", "Walk Artist", "country", func(d *VenueTrackDoc) {
		d.PlayCount++
	})

	got, _ := idx.Get(ctx, docID)
	if got == nil {
		t.Fatal("missing doc")
	}
	if got.VoteCount != 5 {
		t.Fatalf("vote_count wiped: got %d want 5", got.VoteCount)
	}
	if got.PlayCount != 2 {
		t.Fatalf("play_count: got %d want 2", got.PlayCount)
	}
}
