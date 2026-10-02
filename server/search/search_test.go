package search

import (
	"context"
	"testing"
)

func TestNopIndexer(t *testing.T) {
	ctx := context.Background()
	var idx Indexer = nopIndexer{}

	if err := idx.IndexVenue(ctx, VenueHit{ID: 1, Name: "x"}); err != nil {
		t.Fatalf("IndexVenue: %v", err)
	}
	hits, err := idx.SearchVenues(ctx, "x", 10)
	if err != nil {
		t.Fatalf("SearchVenues: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected no hits from nop, got %d", len(hits))
	}
}

func TestInitOpenSearchEmptyURL(t *testing.T) {
	prev := Default
	t.Cleanup(func() { Default = prev })

	if err := InitOpenSearch(""); err != nil {
		t.Fatalf("InitOpenSearch empty: %v", err)
	}
}
