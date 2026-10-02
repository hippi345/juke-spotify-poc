package voting

import (
	"log"

	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"
)

type playlistTrackRemover interface {
	RemoveTracksFromPlaylist(playlistID string, trackURIs []string) error
}

func persistSessionRefillTracks(sessionID uint, trackIDs, uris []string) error {
	if len(uris) == 0 {
		return nil
	}
	rows := make([]models.SessionRefillTrack, len(uris))
	for i := range uris {
		tid := ""
		if i < len(trackIDs) {
			tid = trackIDs[i]
		}
		rows[i] = models.SessionRefillTrack{
			VotingSessionID: sessionID,
			TrackID:         tid,
			TrackURI:        uris[i],
		}
	}
	return db.DB.Create(&rows).Error
}

func loadSessionRefillTrackURIs(sessionID uint) ([]string, error) {
	var rows []models.SessionRefillTrack
	if err := db.DB.Where("voting_session_id = ?", sessionID).Find(&rows).Error; err != nil {
		return nil, err
	}
	uris := make([]string, 0, len(rows))
	for _, r := range rows {
		uris = append(uris, r.TrackURI)
	}
	return uris, nil
}

func deleteSessionRefillTrackRecords(sessionID uint) error {
	return db.DB.Where("voting_session_id = ?", sessionID).Delete(&models.SessionRefillTrack{}).Error
}

// removeRefillTracksForSession removes persisted refill URIs from the playlist when keep-refill is off.
func removeRefillTracksForSession(remover playlistTrackRemover, session *models.VotingSession) error {
	if session.KeepRefillTracks {
		return deleteSessionRefillTrackRecords(session.ID)
	}
	uris, err := loadSessionRefillTrackURIs(session.ID)
	if err != nil {
		return err
	}
	if len(uris) == 0 {
		return nil
	}
	if remover != nil {
		for i := 0; i < len(uris); i += 100 {
			end := i + 100
			if end > len(uris) {
				end = len(uris)
			}
			batch := uris[i:end]
			if err := remover.RemoveTracksFromPlaylist(session.PlaylistID, batch); err != nil {
				log.Printf("voting: remove refilled tracks (session %d): %v", session.ID, err)
				return err
			}
			log.Printf("voting: removed %d refilled tracks from playlist (session %d)", len(batch), session.ID)
		}
	}
	return deleteSessionRefillTrackRecords(session.ID)
}

// RecoverOrphanedActiveSessions ends DB-active sessions left after a crash and cleans up refill tracks when configured.
func RecoverOrphanedActiveSessions(m *Manager) {
	var sessions []models.VotingSession
	if err := db.DB.Where("status = ?", "active").Find(&sessions).Error; err != nil {
		log.Printf("voting: list orphaned sessions: %v", err)
		return
	}
	for i := range sessions {
		s := sessions[i]
		log.Printf("voting: finalizing orphaned active session %d (playlist %s)", s.ID, s.PlaylistID)
		if err := removeRefillTracksForSession(m.playlistRemover(), &s); err != nil {
			log.Printf("voting: orphaned session %d refill cleanup: %v", s.ID, err)
		}
		s.Status = "ended"
		if err := db.DB.Save(&s).Error; err != nil {
			log.Printf("voting: mark session %d ended: %v", s.ID, err)
		}
		_ = db.DB.Model(&models.User{}).Where("joined_voting_session_id = ?", s.ID).
			Update("joined_voting_session_id", nil).Error
	}
}
