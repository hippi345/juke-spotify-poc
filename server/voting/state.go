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

// PlaylistTrackWithMeta extends a track with session metadata
type PlaylistTrackWithMeta struct {
	Track   spotify.Track `json:"track"`
	Played  bool          `json:"played"`
	Refilled bool         `json:"refilled"` // added via "similar vibe" refill
}

// Manager holds session and round state
type Manager struct {
	mu sync.RWMutex

	Session          *models.VotingSession
	Round            *RoundState
	UsedTrackIDs     map[string]bool
	RefilledTrackIDs map[string]bool // tracks added via refill (similar vibe)
	lastAdvanceAt    time.Time
	pendingWinnerID  string // winner added to queue; new round starts when this track begins playing
	svc              *spotify.Client
}

// NewManager creates a new voting manager
func NewManager(svc *spotify.Client) *Manager {
	return &Manager{
		UsedTrackIDs:    make(map[string]bool),
		RefilledTrackIDs: make(map[string]bool),
		svc:             svc,
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
	m.RefilledTrackIDs = make(map[string]bool)
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

// GetPlaylistOverview returns all playlist tracks with played and refilled flags
func (m *Manager) GetPlaylistOverview() ([]PlaylistTrackWithMeta, error) {
	m.mu.RLock()
	session := m.Session
	usedTrackIDs := make(map[string]bool)
	for k, v := range m.UsedTrackIDs {
		usedTrackIDs[k] = v
	}
	refilledTrackIDs := make(map[string]bool)
	for k, v := range m.RefilledTrackIDs {
		refilledTrackIDs[k] = v
	}
	m.mu.RUnlock()

	if session == nil || session.Status != "active" {
		return nil, fmt.Errorf("no active session")
	}

	var result []PlaylistTrackWithMeta
	offset := 0
	for {
		resp, err := m.svc.GetPlaylistTracks(session.PlaylistID, offset)
		if err != nil {
			return nil, err
		}

		for _, pitem := range resp.Items {
			track := pitem.Track
			if track == nil {
				track = pitem.Item
			}
			if track == nil || track.ID == "" {
				continue
			}
			played := usedTrackIDs[track.ID]
			refilled := refilledTrackIDs[track.ID]
			result = append(result, PlaylistTrackWithMeta{
				Track:    *track,
				Played:   played,
				Refilled: refilled,
			})
		}

		if resp.Next == nil || *resp.Next == "" {
			break
		}
		offset += 50
	}

	return result, nil
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
		m.svc.ClearQueue()
	}
	m.Session = nil
	m.Round = nil
	m.UsedTrackIDs = nil
	m.RefilledTrackIDs = nil
	m.pendingWinnerID = ""
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

	// When waiting for winner to start: show "Voting ended", no countdown
	if m.pendingWinnerID != "" {
		return session, round, 0
	}

	// Countdown = time until we advance (add winner). We advance when current song has ~15s left.
	if round != nil {
		if nowPlaying != nil && nowPlaying.Item != nil {
			remainingMs := int64(nowPlaying.Item.DurationMs) - nowPlaying.ProgressMs
			secUntilAdvance := (remainingMs - int64(roundEndBufferMs)) / 1000
			if secUntilAdvance > 0 {
				timeRemainingSec = int(secUntilAdvance)
			}
		} else if !round.RoundEndsAt.IsZero() {
			remaining := time.Until(round.RoundEndsAt)
			if remaining > 0 {
				timeRemainingSec = int(remaining.Seconds())
			}
		}
	}

	return session, round, timeRemainingSec
}

// ShouldAdvanceRound returns true if the round should advance (song has ~15s left, or nothing playing and round expired)
func (m *Manager) ShouldAdvanceRound(cp *spotify.CurrentlyPlaying) bool {
	m.mu.RLock()
	session, round, _ := m.getStateLocked(cp)
	pending := m.pendingWinnerID
	m.mu.RUnlock()
	if session == nil || session.Status != "active" || round == nil {
		return false
	}
	// Waiting for winner to start; don't advance
	if pending != "" {
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

// AdvanceRound ends the current round, picks winner, adds to queue. Does NOT create a new round;
// the new round starts when StartNewRoundIfWinnerPlaying detects the winner is playing.
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

	// Pick winner (only one track goes to queue per round)
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
			// Something playing: add winner to queue (winner only - no other candidates)
			log.Printf("voting: advance - adding to queue: %s (%s)", winner.Name, winner.URI)
			if err := m.svc.AddToQueue(winner.URI); err != nil {
				log.Printf("voting: add to queue FAILED: %v", err)
			} else {
				log.Printf("voting: add to queue OK")
			}
		}
		m.UsedTrackIDs[winner.ID] = true
		m.pendingWinnerID = winner.ID
		log.Printf("voting: voting ended, waiting for winner %s to start", winner.Name)
	}
	// Set debounce immediately after queueing so concurrent ticker/poll doesn't double-add
	m.lastAdvanceAt = time.Now()

	// Keep current round (show "Voting ended"); new round starts when winner begins playing
	return nil
}

// StartNewRoundIfWinnerPlaying creates a new round when the queued winner has started playing.
// Call this from the ticker and state handler when pendingWinnerID is set.
func (m *Manager) StartNewRoundIfWinnerPlaying(cp *spotify.CurrentlyPlaying) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.pendingWinnerID == "" || m.Session == nil || m.Session.Status != "active" {
		return
	}
	if cp == nil || cp.Item == nil || cp.Item.ID != m.pendingWinnerID {
		return
	}

	// Winner is now playing; create new round
	m.pendingWinnerID = ""
	round, err := m.fetchCandidatesLocked(cp.Item)
	if err != nil {
		log.Printf("voting: start new round (winner playing): %v", err)
		m.Round = nil
		return
	}
	log.Printf("voting: new round started with %d candidates (winner now playing)", len(round.Candidates))
	m.Round = round
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

	// Fetch all unused tracks from playlist, then randomly pick 3
	var unused []spotify.Track
	offset := 0
	for {
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
			unused = append(unused, *track)
		}

		if resp.Next == nil || *resp.Next == "" {
			break
		}
		offset += 50
	}

	if len(unused) == 0 {
		return nil, fmt.Errorf("no unused tracks in playlist")
	}

	// Randomly pick 3 from unused pool
	candidates := pickRandomN(unused, 3)

	// Compute round end time: prefer nextSong (winner we just queued), else currently playing
	roundEndsAt := time.Now().Add(60 * time.Second)
	if nextSong != nil && nextSong.DurationMs > 0 {
		// Winner is in queue, not yet playing. Add current song's remaining time before winner's duration.
		delayUntilWinnerStarts := time.Duration(0)
		if cp, err := m.svc.GetCurrentlyPlaying(); err == nil && cp != nil && cp.Item != nil {
			remainingMs := int64(cp.Item.DurationMs) - cp.ProgressMs
			if remainingMs > 0 {
				delayUntilWinnerStarts = time.Duration(remainingMs) * time.Millisecond
			}
		}
		roundEndsAt = time.Now().Add(delayUntilWinnerStarts).Add(time.Duration(nextSong.DurationMs)*time.Millisecond - roundEndBufferMs*time.Millisecond)
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
		m.RefilledTrackIDs[t.ID] = true
		if len(uris) >= refillBatchSize {
			break
		}
	}
	if len(uris) > 0 {
		_ = m.svc.AddTracksToPlaylist(m.Session.PlaylistID, uris)
	}
}

// pickRandomN returns n randomly selected tracks from the slice (or all if fewer than n)
func pickRandomN(tracks []spotify.Track, n int) []spotify.Track {
	if len(tracks) <= n {
		return tracks
	}
	// Fisher-Yates shuffle first n elements (partial shuffle)
	perm := make([]int, len(tracks))
	for i := range perm {
		perm[i] = i
	}
	for i := 0; i < n; i++ {
		j := i + rand.Intn(len(perm)-i)
		perm[i], perm[j] = perm[j], perm[i]
	}
	result := make([]spotify.Track, n)
	for i := 0; i < n; i++ {
		result[i] = tracks[perm[i]]
	}
	return result
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

