package search

import (
	"context"
)

// VenueHit is a venue document returned from search.
type VenueHit struct {
	ID        uint
	Name      string
	Latitude  float64
	Longitude float64
	StaffID   uint
}

// Indexer indexes and searches venues.
type Indexer interface {
	IndexVenue(ctx context.Context, v VenueHit) error
	SearchVenues(ctx context.Context, query string, limit int) ([]VenueHit, error)
}

type nopIndexer struct{}

func (nopIndexer) IndexVenue(context.Context, VenueHit) error { return nil }
func (nopIndexer) SearchVenues(context.Context, string, int) ([]VenueHit, error) {
	return nil, nil
}

// Default is a no-op until InitOpenSearch succeeds.
var Default Indexer = nopIndexer{}
