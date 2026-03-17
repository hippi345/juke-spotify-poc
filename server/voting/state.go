package voting

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"
	"juke-spotify-poc/server/spotify"
)

const (
	roundEndBufferMs   = 15000 // End voting 15 seconds before song ends
	refillBatchSize    = 20    // Tracks to add when refilling
	advanceDebounceSec = 8     // Don't advance again within this many seconds
)

// RoundState holds the current voting round
type RoundState struct {
	Candidates  []spotify.Track
	Votes       map[string]int
	RoundEndsAt time.Time
}

// Manager holds session and round state
type Manager struct {
	mu sync.RWMutex

	Session       *models.VotingSession
	Round         *RoundState
	UsedTrackIDs  map[string]bool
	lastAdvanceAt time.Time
	svc           *spotify.Client
}

// NewManager creates a new voting manager
func NewManager(svc *spotify.Client) *Manager {
	return &Manager{
		UsedTrackIDs: make(map[string]bool),
		svc:          svc,
	}
}

// StartSession creates a new voting session and fetches initial candidates
func (m *Manager) StartSession(playlistID, playlistName string, refillThreshold int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Session != nil && m.Session.Status == "active" {
		return fmt.Errorf("session already active")
	}

	session := &models.VotingSession{
		PlaylistID:      playlistID,
		PlaylistName:    playlistName,
		RefillThreshold: refillThreshold,
		Status:          "active",
	}
	if err := db.DB.Create(session).Error; err != nil {
		return err
	}

	m.Session = session
	m.UsedTrackIDs = make(map[string]bool)
	m.Round = nil

	round, err := m.fetchCandidatesLocked(nil)
	if err != nil {
		log.Printf("voting: start session failed (playlist %s): %v", m.Session.PlaylistID, err)
		m.Session.Status = "ended"
		db.DB.Save(m.Session)
		m.Session = nil
		return err
	}

	// Always kick off with a random song from the playlist; remove it from voting pool
	if len(round.Candidates) > 0 {
		idx := rand.Intn(len(round.Candidates))
		kickoff := &round.Candidates[idx]
		if err := m.svc.StartPlayback(kickoff.URI); err != nil {
			log.Printf("voting: kickoff playback failed: %v", err)
		} else {
			m.UsedTrackIDs[kickoff.ID] = true
			// Fetch new candidates for voting (excluding kickoff); round ends when kickoff has ~15s left
			round, err = m.fetchCandidatesLocked(kickoff)
			if err != nil {
				log.Printf("voting: fetch candidates after kickoff: %v", err)
				m.Round = nil
				return err
			}
		}
	}

	m.Round = round
	return nil
}

// RecoverRound creates a round when session is active but round is nil (e.g. after fetch failure)
func (m *Manager) RecoverRound() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Session == nil || m.Session.Status != "active" || m.Round != nil {
		return nil
	}
	round, err := m.fetchCandidatesLocked(nil)
	if err != nil {
		return err
	}
	m.Round = round
	return nil
}

// EndSession ends the current session and stops playback
func (m *Manager) EndSession() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Session != nil && m.Session.Status == "active" {
		m.Session.Status = "ended"
		db.DB.Save(m.Session)
		if err := m.svc.PausePlayback(); err != nil {
			log.Printf("voting: pause playback on session end: %v", err)
		}
	}
	m.Session = nil
	m.Round = nil
	m.UsedTrackIDs = nil
}

// Vote records a vote for a track
func (m *Manager) Vote(trackID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Round == nil {
		return fmt.Errorf("no active round")
	}

	// Verify track is a candidate
	found := false
	for _, t := range m.Round.Candidates {
		if t.ID == trackID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("track not a candidate")
	}

	if m.Round.Votes == nil {
		m.Round.Votes = make(map[string]int)
	}
	m.Round.Votes[trackID]++
	return nil
}

// GetState returns the current session and round state for API responses
func (m *Manager) GetState(nowPlaying *spotify.CurrentlyPlaying) (session *models.VotingSession, round *RoundState, timeRemainingSec int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session = m.Session
	round = m.Round

	if round != nil && !round.RoundEndsAt.IsZero() {
		remaining := time.Until(round.RoundEndsAt)
		if remaining > 0 {
			timeRemainingSec = int(remaining.Seconds())
		}
	}

	return session, round, timeRemainingSec
}

// ShouldAdvanceRound returns true if the round should advance (song has ~15s left, or nothing playing and round expired)
func (m *Manager) ShouldAdvanceRound(cp *spotify.CurrentlyPlaying) bool {
	m.mu.RLock()
	session, round, _ := m.getStateLocked(cp)
	m.mu.RUnlock()
	if session == nil || session.Status != "active" || round == nil {
		return false
	}
	if time.Since(m.lastAdvanceAt) < advanceDebounceSec*time.Second {
		return false
	}
	if cp == nil || cp.Item == nil {
		return time.Now().After(round.RoundEndsAt)
	}
	remainingMs := int64(cp.Item.DurationMs) - cp.ProgressMs
	return remainingMs <= roundEndBufferMs
}

// getStateLocked returns state; caller must hold at least RLock
func (m *Manager) getStateLocked(nowPlaying *spotify.CurrentlyPlaying) (session *models.VotingSession, round *RoundState, timeRemainingSec int) {
	session = m.Session
	round = m.Round
	if round != nil && !round.RoundEndsAt.IsZero() {
		remaining := time.Until(round.RoundEndsAt)
		if remaining > 0 {
			timeRemainingSec = int(remaining.Seconds())
		}
	}
	return session, round, timeRemainingSec
}

// AdvanceRound ends the current round, picks winner, adds to queue, fetches new candidates
func (m *Manager) AdvanceRound() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Session == nil || m.Session.Status != "active" || m.Round == nil {
		return nil
	}

	// Debounce: don't advance again too soon (prevents double-advance when round just changed)
	if time.Since(m.lastAdvanceAt) < advanceDebounceSec*time.Second {
		return nil
	}

	// Pick winner
	winner := pickWinner(m.Round)
	if winner != nil {
		cp, _ := m.svc.GetCurrentlyPlaying()
		if cp == nil || cp.Item == nil {
			// Nothing playing: start playback with winner directly
			log.Printf("voting: advance - starting playback: %s", winner.Name)
			if err := m.svc.StartPlayback(winner.URI); err != nil {
				log.Printf("voting: start playback (winner): %v", err)
			}
		} else {
			// Something playing: add winner to queue
			log.Printf("voting: advance - adding to queue: %s (%s)", winner.Name, winner.URI)
			if err := m.svc.AddToQueue(winner.URI); err != nil {
				log.Printf("voting: add to queue FAILED: %v", err)
			} else {
				log.Printf("voting: add to queue OK")
			}
		}
		m.UsedTrackIDs[winner.ID] = true
	}

	// Fetch new candidates; use winner's duration for round end (when winner will have ~15s left)
	round, err := m.fetchCandidatesLocked(winner)
	if err != nil {
		log.Printf("voting: fetch candidates after advance FAILED: %v", err)
		m.Round = nil
		return err
	}
	log.Printf("voting: new round with %d candidates", len(round.Candidates))
	m.Round = round
	m.lastAdvanceAt = time.Now()

	return nil
}

// fetchCandidatesLocked fetches 3 candidates, filtering used tracks. Caller must hold lock.
// nextSong: when set (e.g. winner we just queued), round end is based on when that song has ~15s left.
func (m *Manager) fetchCandidatesLocked(nextSong *spotify.Track) (*RoundState, error) {
	if m.Session == nil {
		return nil, fmt.Errorf("no session")
	}

	// Check refill threshold
	if m.Session.RefillThreshold > 0 {
		total, err := m.getPlaylistTotalTracks()
		if err == nil {
			unused := total - len(m.UsedTrackIDs)
			if unused < m.Session.RefillThreshold {
				m.refillPlaylistLocked()
			}
		}
	}

	// Fetch tracks, filter used, take 3
	var candidates []spotify.Track
	offset := 0
	for len(candidates) < 3 {
		resp, err := m.svc.GetPlaylistTracks(m.Session.PlaylistID, offset)
		if err != nil {
			return nil, err
		}

		for _, pitem := range resp.Items {
			track := pitem.Track
			if track == nil {
				track = pitem.Item // new /items endpoint uses "item"
			}
			if track == nil || track.ID == "" {
				continue
			}
			if m.UsedTrackIDs[track.ID] {
				continue
			}
			candidates = append(candidates, *track)
			if len(candidates) >= 3 {
				break
			}
		}

		if resp.Next == nil || *resp.Next == "" {
			break
		}
		offset += 50
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no unused tracks in playlist")
	}

	// Compute round end time: prefer nextSong (winner we just queued), else currently playing
	roundEndsAt := time.Now().Add(60 * time.Second)
	if nextSong != nil && nextSong.DurationMs > 0 {
		// Winner will play next; round ends when it has ~15s left
		roundEndsAt = time.Now().Add(time.Duration(nextSong.DurationMs)*time.Millisecond - roundEndBufferMs*time.Millisecond)
	} else {
		cp, err := m.svc.GetCurrentlyPlaying()
		if err == nil && cp != nil && cp.Item != nil {
			remainingMs := int64(cp.Item.DurationMs) - cp.ProgressMs
			if remainingMs > roundEndBufferMs {
				roundEndsAt = time.Now().Add(time.Duration(remainingMs-roundEndBufferMs) * time.Millisecond)
			}
		}
	}

	return &RoundState{
		Candidates:  candidates,
		Votes:       make(map[string]int),
		RoundEndsAt: roundEndsAt,
	}, nil
}

func (m *Manager) getPlaylistTotalTracks() (int, error) {
	resp, err := m.svc.GetPlaylistTracks(m.Session.PlaylistID, 0)
	if err != nil {
		return 0, err
	}
	return resp.Total, nil
}

func (m *Manager) refillPlaylistLocked() {
	// Get 2-3 seed tracks from playlist (prefer unused, but we can use any)
	var seedIDs []string
	offset := 0
	for len(seedIDs) < 3 {
		resp, err := m.svc.GetPlaylistTracks(m.Session.PlaylistID, offset)
		if err != nil {
			return
		}
		for _, pitem := range resp.Items {
			track := pitem.Track
			if track == nil {
				track = pitem.Item
			}
			if track != nil && track.ID != "" {
				seedIDs = append(seedIDs, track.ID)
				if len(seedIDs) >= 3 {
					break
				}
			}
		}
		if resp.Next == nil || *resp.Next == "" {
			break
		}
		offset += 50
	}
	if len(seedIDs) == 0 {
		return
	}

	recs, err := m.svc.GetRecommendations(seedIDs, refillBatchSize)
	if err != nil {
		return
	}

	// Filter: exclude already in playlist or used
	playlistTrackIDs := make(map[string]bool)
	offset = 0
	for {
		resp, err := m.svc.GetPlaylistTracks(m.Session.PlaylistID, offset)
		if err != nil {
			break
		}
		for _, pitem := range resp.Items {
			track := pitem.Track
			if track == nil {
				track = pitem.Item
			}
			if track != nil && track.ID != "" {
				playlistTrackIDs[track.ID] = true
			}
		}
		if resp.Next == nil || *resp.Next == "" {
			break
		}
		offset += 50
	}

	var uris []string
	for _, t := range recs {
		if playlistTrackIDs[t.ID] || m.UsedTrackIDs[t.ID] {
			continue
		}
		uris = append(uris, t.URI)
		if len(uris) >= refillBatchSize {
			break
		}
	}
	if len(uris) > 0 {
		_ = m.svc.AddTracksToPlaylist(m.Session.PlaylistID, uris)
	}
}

// pickWinner returns the track with most votes, or random among ties / all if no votes
func pickWinner(round *RoundState) *spotify.Track {
	if len(round.Candidates) == 0 {
		return nil
	}

	maxVotes := -1
	var winners []*spotify.Track
	for i := range round.Candidates {
		t := &round.Candidates[i]
		v := round.Votes[t.ID]
		if v > maxVotes {
			maxVotes = v
			winners = []*spotify.Track{t}
		} else if v == maxVotes && maxVotes >= 0 {
			winners = append(winners, t)
		}
	}

	if len(winners) == 0 {
		// No votes - pick random from all
		idx := rand.Intn(len(round.Candidates))
		return &round.Candidates[idx]
	}
	// Pick random among ties
	return winners[rand.Intn(len(winners))]
}

