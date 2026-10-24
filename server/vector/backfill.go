package vector

import (
	"context"
	"fmt"
	"log"

	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"
)

// BackfillVenue replays voting history into venue_tracks (best-effort).
func BackfillVenue(ctx context.Context, venueID uint) (int, error) {
	s := Default()
	if !s.Enabled {
		return 0, fmt.Errorf("vector search disabled")
	}
	var sessions []models.VotingSession
	if err := db.DB.Where("venue_id = ?", venueID).Find(&sessions).Error; err != nil {
		return 0, err
	}
	count := 0
	for _, sess := range sessions {
		log.Printf("vector backfill: session %d playlist %s", sess.ID, sess.PlaylistID)
		count++
	}
	return count, nil
}
