package vector

import (
	"context"

	"juke-spotify-poc/server/events"
)

// HandleVoteEvent indexes or updates venue_tracks from a Kafka vote payload.
func HandleVoteEvent(ctx context.Context, p events.VotePayload) {
	if p.VenueID == 0 || p.TrackID == "" {
		return
	}
	IngestVote(ctx, p.VenueID, p.TrackID, p.Title, p.Artist, p.Genres)
}
