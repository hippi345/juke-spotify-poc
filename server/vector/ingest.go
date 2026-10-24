package vector

import (
	"context"
	"fmt"
	"log"
	"time"

	"juke-spotify-poc/server/events"
)

// IngestVote bumps vote_count for a venue track document.
func IngestVote(ctx context.Context, venueID uint, trackID, title, artist, genres string) {
	s := Default()
	if !s.Enabled || venueID == 0 || trackID == "" {
		return
	}
	upsertTrack(ctx, s, venueID, trackID, title, artist, genres, func(d *VenueTrackDoc) {
		d.VoteCount++
	})
}

// IngestSessionPlay records a track heard during a session.
func IngestSessionPlay(ctx context.Context, venueID uint, trackID, title, artist, genres string) {
	s := Default()
	if !s.Enabled || venueID == 0 || trackID == "" {
		return
	}
	upsertTrack(ctx, s, venueID, trackID, title, artist, genres, func(d *VenueTrackDoc) {
		d.PlayCount++
	})
}

func upsertTrack(ctx context.Context, s Service, venueID uint, trackID, title, artist, genres string, bump func(*VenueTrackDoc)) {
	docID := fmt.Sprintf("%d-%s", venueID, trackID)
	text := TrackEmbedText(title, artist, genres)
	vec, err := s.Embedder.Embed(text)
	if err != nil {
		log.Printf("vector: embed %s: %v", docID, err)
		return
	}
	doc := VenueTrackDoc{
		VenueID:   venueID,
		SpotifyID: trackID,
		Title:     title,
		Artist:    artist,
		Genres:    genres,
		LastSeen:  time.Now().UTC().Format(time.RFC3339),
		Embedding: vec,
	}
	bump(&doc)
	if err := s.Indexer.Upsert(ctx, docID, doc); err != nil {
		log.Printf("vector: upsert %s: %v", docID, err)
	}
}

// KafkaHooks returns optional event handlers for vector ingest.
func KafkaHooks() (onVote func(events.VotePayload), onSession func(events.SessionPayload)) {
	return func(p events.VotePayload) {
		// track metadata filled by caller when available
		IngestVote(context.Background(), 0, p.TrackID, "", "", "")
	}, func(p events.SessionPayload) {
		_ = p
	}
}
