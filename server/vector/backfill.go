package vector

import (
	"context"
	"fmt"
	"log"

	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"
	"juke-spotify-poc/server/spotify"
)

func primaryArtist(t spotify.Track) string {
	if len(t.Artists) > 0 {
		return t.Artists[0].Name
	}
	return ""
}

// BackfillVenue indexes playlist tracks from all voting sessions at this venue.
func BackfillVenue(ctx context.Context, venueID uint, client *spotify.Client) (int, error) {
	s := Default()
	if !s.Enabled {
		return 0, fmt.Errorf("vector search disabled (set VIBESENSE_VECTOR_ENABLED=true)")
	}
	var sessions []models.VotingSession
	if err := db.DB.Where("venue_id = ?", venueID).Find(&sessions).Error; err != nil {
		return 0, err
	}
	if len(sessions) == 0 {
		return 0, fmt.Errorf("no voting sessions with venue_id=%d (start a staff session with venue_id set)", venueID)
	}
	if client == nil {
		return 0, fmt.Errorf("spotify client required for backfill")
	}
	indexed := 0
	for _, sess := range sessions {
		offset := 0
		for {
			page, err := client.GetPlaylistTracks(sess.PlaylistID, offset)
			if err != nil {
				log.Printf("vector backfill: playlist %s: %v", sess.PlaylistID, err)
				break
			}
			if page == nil || len(page.Items) == 0 {
				break
			}
			for _, item := range page.Items {
				t := item.Item
				if t == nil {
					t = item.Track
				}
				if t == nil || t.ID == "" {
					continue
				}
				genres := ""
				if t.Album.Name != "" {
					genres = t.Album.Name
				}
				upsertTrack(ctx, s, venueID, t.ID, t.Name, primaryArtist(*t), genres, func(d *VenueTrackDoc) {
					d.PlayCount++
				})
				indexed++
			}
			offset += len(page.Items)
			if page.Next == nil {
				break
			}
		}
		log.Printf("vector backfill: session %d playlist %s processed", sess.ID, sess.PlaylistID)
	}
	return indexed, nil
}
