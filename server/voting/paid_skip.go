package voting

import (
	"fmt"
	"log"
	"time"

	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"
)

// ActiveSessionID returns the in-memory active voting session id, or 0.
func (m *Manager) ActiveSessionID() uint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.Session == nil || m.Session.Status != "active" {
		return 0
	}
	return m.Session.ID
}

// PlaylistTrackURI returns the Spotify URI for trackID if it appears on the active session playlist.
func (m *Manager) PlaylistTrackURI(trackID string) (string, error) {
	m.mu.RLock()
	session := m.Session
	m.mu.RUnlock()
	if session == nil || session.Status != "active" {
		return "", fmt.Errorf("no active session")
	}
	return m.trackURIOnPlaylist(session.PlaylistID, trackID)
}

// TrackURIForVotingSession validates trackID against a session playlist (works across API replicas).
func (m *Manager) TrackURIForVotingSession(votingSessionID uint, trackID string) (string, error) {
	var session models.VotingSession
	if err := db.DB.First(&session, votingSessionID).Error; err != nil {
		return "", fmt.Errorf("session not found")
	}
	if session.Status != "active" {
		return "", fmt.Errorf("session not active")
	}
	return m.trackURIOnPlaylist(session.PlaylistID, trackID)
}

func (m *Manager) trackURIOnPlaylist(playlistID, trackID string) (string, error) {
	offset := 0
	for {
		resp, err := m.svc.GetPlaylistTracks(playlistID, offset)
		if err != nil {
			return "", err
		}
		for _, pitem := range resp.Items {
			track := pitem.Track
			if track == nil {
				track = pitem.Item
			}
			if track != nil && track.ID == trackID {
				if track.URI == "" {
					return "", fmt.Errorf("track missing uri")
				}
				return track.URI, nil
			}
		}
		if resp.Next == nil || *resp.Next == "" {
			break
		}
		offset += 50
	}
	return "", fmt.Errorf("track not on venue playlist")
}

// QueuePaidSkipsBeforeWinner adds fulfilled paid skips (FIFO) then the vote winner to Spotify.
func (m *Manager) queuePaidSkipsBeforeWinner(winnerURI, winnerID string) {
	skips := m.listPaidSkipsPendingQueue()
	for _, skip := range skips {
		if err := m.svc.AddToQueue(skip.TrackURI); err != nil {
			log.Printf("voting: paid skip queue failed track %s: %v", skip.TrackID, err)
			continue
		}
		now := time.Now()
		skip.QueuedAt = &now
		db.DB.Save(&skip)
		m.UsedTrackIDs[skip.TrackID] = true
		log.Printf("voting: paid skip queued (advance): %s", skip.TrackID)
	}

	if winnerURI != "" {
		if err := m.svc.AddToQueue(winnerURI); err != nil {
			log.Printf("voting: add to queue FAILED: %v", err)
		} else {
			log.Printf("voting: add to queue OK (winner after paid skips)")
		}
		if winnerID != "" {
			m.UsedTrackIDs[winnerID] = true
		}
	}
}

// listPaidSkipsPendingQueue returns paid skips for the active session not yet sent to Spotify, oldest first.
func (m *Manager) listPaidSkipsPendingQueue() []models.PaidSkip {
	if m.Session == nil {
		return nil
	}
	var skips []models.PaidSkip
	_ = db.DB.Where(
		"voting_session_id = ? AND status = ? AND queued_at IS NULL",
		m.Session.ID, "paid",
	).Order("paid_at ASC, id ASC").Find(&skips).Error
	return skips
}

// ApplyPaidSkipAfterPayment queues a track immediately after successful Stripe payment.
func (m *Manager) ApplyPaidSkipAfterPayment(skip *models.PaidSkip) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Session == nil || m.Session.Status != "active" || m.Session.ID != skip.VotingSessionID {
		return fmt.Errorf("no active session")
	}
	if m.UsedTrackIDs[skip.TrackID] {
		return fmt.Errorf("track already played or queued")
	}
	if m.pendingWinnerID != "" {
		// Winner already queued for this round; new paid skip still goes to Spotify queue (best effort).
		log.Printf("voting: paid skip after vote winner queued — may play after pending winner")
	}
	if err := m.svc.AddToQueue(skip.TrackURI); err != nil {
		return err
	}
	now := time.Now()
	skip.QueuedAt = &now
	if err := db.DB.Save(skip).Error; err != nil {
		return err
	}
	m.UsedTrackIDs[skip.TrackID] = true
	log.Printf("voting: paid skip queued (payment): %s", skip.TrackID)
	return nil
}
