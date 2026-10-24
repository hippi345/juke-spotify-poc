package vector

import (
	"context"
	"testing"
)

type scoreIndexer struct {
	hits []SearchHit
}

func (s scoreIndexer) EnsureIndex(context.Context, int) error              { return nil }
func (s scoreIndexer) Get(context.Context, string) (*VenueTrackDoc, error) { return nil, nil }
func (s scoreIndexer) Upsert(context.Context, string, VenueTrackDoc) error { return nil }
func (s scoreIndexer) DeleteByVenueID(context.Context, uint) (int, error)  { return 0, nil }
func (s scoreIndexer) SearchSimilar(context.Context, uint, []float32, int) ([]SearchHit, error) {
	return s.hits, nil
}
func (s scoreIndexer) SearchByText(context.Context, uint, string, Embedder, int) ([]SearchHit, error) {
	return s.hits, nil
}

func TestCandidatesForMood_respectsLimitAndSimilarity(t *testing.T) {
	SetDefaultService(Service{
		Enabled:            true,
		Indexer: scoreIndexer{
			hits: []SearchHit{
				{Title: "A", Artist: "One", Score: 0.9},
				{Title: "B", Artist: "Two", Score: 0.8},
				{Title: "C", Artist: "Three", Score: 0.05},
				{Title: "D", Artist: "Four", Score: 0.7},
			},
		},
		RetrievalLimit:     2,
		MinSimilarityScore: 0.5,
	})
	out := Default().CandidatesForMood(context.Background(), 1, "lofi", 0)
	if len(out) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(out))
	}
	if out[0].Title != "A" || out[1].Title != "B" {
		t.Fatalf("unexpected picks: %+v", out)
	}
}
