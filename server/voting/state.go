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
	advanceDebounceSec = 8     // Don't advance again within this many seconds
	cpCacheTTL         = 4 * time.Second // Fresher now-playing for responsive UI
	refillDelayMs      = 80    // Brief delay between refill API calls (parallel fetches reduce total time)
	refillCountMax     = 10    // Max tracks to add per refill (API throttling)
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
const kickoffOverrideSec = 6 // Use kickoff as now_playing for this long after session start (Spotify lags)

type Manager struct {
	mu sync.RWMutex

	Session          *models.VotingSession
	Round            *RoundState
	UsedTrackIDs     map[string]bool
	RefilledTrackIDs  map[string]bool // tracks added via refill (for UI badge)
	RefilledTrackURIs []string        // URIs of refilled tracks (removed on session end)
	lastAdvanceAt     time.Time
	pendingWinnerID  string // winner added to queue; new round starts when this track begins playing
	svc              *spotify.Client

	// Cache for GetCurrentlyPlaying to avoid duplicate calls from ticker + state handler
	lastCp   *spotify.CurrentlyPlaying
	lastCpAt time.Time

	// Kickoff override: Spotify lags when we start playback; use kickoff as now_playing for first few seconds
	kickoffTrack   *spotify.Track
	sessionStartAt time.Time
}

// HasActiveSession returns true if there is an active voting session (avoids Spotify calls when idle)
func (m *Manager) HasActiveSession() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Session != nil && m.Session.Status == "active"
}

// GetOrFetchCurrentlyPlaying returns cached currently-playing if fresh (< cpCacheTTL), else fetches from Spotify.
// Skips cache when waiting for winner so we detect the new track as soon as it starts.
func (m *Manager) GetOrFetchCurrentlyPlaying() (*spotify.CurrentlyPlaying, error) {
	m.mu.RLock()
	skipCache := m.pendingWinnerID != ""
	ttl := cpCacheTTL
	if skipCache {
		ttl = 0 // Always fetch fresh when waiting for winner
	}
	if !skipCache && m.lastCp != nil && time.Since(m.lastCpAt) < ttl {
		cp := m.lastCp
		m.mu.RUnlock()
		return cp, nil
	}
	m.mu.RUnlock()

	cp, err := m.svc.GetCurrentlyPlaying()
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.lastCp = cp
	m.lastCpAt = time.Now()
	m.mu.Unlock()
	return cp, nil
}

// NewManager creates a new voting manager
func NewManager(svc *spotify.Client) *Manager {
	return &Manager{
		UsedTrackIDs:    make(map[string]bool),
		RefilledTrackIDs: make(map[string]bool),
		svc:             svc,
	}
}

// StartSession creates a new voting session and fetches initial candidates.
// Returns the kickoff track (now playing) so the client can update UI immediately.
func (m *Manager) StartSession(playlistID, playlistName string, refillThreshold, refillCount int) (*spotify.Track, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Session != nil && m.Session.Status == "active" {
		return nil, fmt.Errorf("session already active")
	}

	// Invalidate now-playing cache so next poll fetches fresh data (avoids showing previous session's track)
	m.lastCp = nil
	m.lastCpAt = time.Time{}

	if refillCount <= 0 {
		refillCount = 10
	}
	if refillCount > refillCountMax {
		refillCount = refillCountMax
	}
	session := &models.VotingSession{
		PlaylistID:      playlistID,
		PlaylistName:    playlistName,
		RefillThreshold: refillThreshold,
		RefillCount:     refillCount,
		Status:          "active",
	}
	if err := db.DB.Create(session).Error; err != nil {
		return nil, err
	}

	m.Session = session
	m.UsedTrackIDs = make(map[string]bool)
	m.RefilledTrackIDs = make(map[string]bool)
	m.RefilledTrackURIs = nil
	m.Round = nil

	// Refill at session start if playlist is already below threshold
	if refillThreshold > 0 && refillCount > 0 {
		total, err := m.getPlaylistTotalTracks()
		if err == nil {
			if total < refillThreshold {
				_ = m.refillPlaylistLocked()
			}
		}
	}

	round, err := m.fetchCandidatesLocked(nil)
	if err != nil {
		log.Printf("voting: start session failed (playlist %s): %v", m.Session.PlaylistID, err)
		m.Session.Status = "ended"
		db.DB.Save(m.Session)
		m.Session = nil
		return nil, err
	}

	var kickoff *spotify.Track
	if len(round.Candidates) > 0 {
		idx := rand.Intn(len(round.Candidates))
		kickoff = &round.Candidates[idx]
		if err := m.svc.StartPlayback(kickoff.URI); err != nil {
			log.Printf("voting: kickoff playback failed: %v", err)
		} else {
			m.UsedTrackIDs[kickoff.ID] = true
			m.kickoffTrack = kickoff
			m.sessionStartAt = time.Now()
			// Fetch new candidates for voting (excluding kickoff); round ends when kickoff has ~15s left
			round, err = m.fetchCandidatesLocked(kickoff)
			if err != nil {
				log.Printf("voting: fetch candidates after kickoff: %v", err)
				m.Round = nil
				return nil, err
			}
		}
	}

	m.Round = round
	return kickoff, nil
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

// TriggerRefill manually runs the vibe fill logic (adds similar tracks to playlist). For testing.
func (m *Manager) TriggerRefill() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Session == nil || m.Session.Status != "active" {
		return fmt.Errorf("no active session")
	}
	return m.refillPlaylistLocked()
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

// EndSession ends the current session and stops playback.
// We never remove original playlist tracks during the session—only add refill tracks.
// Before closing, we remove the refilled tracks from the playlist so it returns to its original state.
func (m *Manager) EndSession() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Session != nil && m.Session.Status == "active" {
		m.Session.Status = "ended"
		db.DB.Save(m.Session)

		// Pause immediately to stop playback (no ClearQueue - skipping causes brief audio bursts)
		_ = m.svc.PausePlayback()
		time.Sleep(300 * time.Millisecond) // let pause propagate

		// Remove refilled tracks from the playlist so it returns to its original state
		if len(m.RefilledTrackURIs) > 0 {
			for i := 0; i < len(m.RefilledTrackURIs); i += 100 {
				end := i + 100
				if end > len(m.RefilledTrackURIs) {
					end = len(m.RefilledTrackURIs)
				}
				batch := m.RefilledTrackURIs[i:end]
				if err := m.svc.RemoveTracksFromPlaylist(m.Session.PlaylistID, batch); err != nil {
					log.Printf("voting: remove refilled tracks on session end: %v", err)
				} else {
					log.Printf("voting: removed %d refilled tracks from playlist", len(batch))
				}
			}
		}
	}
	m.Session = nil
	m.Round = nil
	m.UsedTrackIDs = nil
	m.RefilledTrackIDs = nil
	m.RefilledTrackURIs = nil
	m.pendingWinnerID = ""
	m.kickoffTrack = nil
	m.sessionStartAt = time.Time{}
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

// GetEffectiveNowPlaying returns kickoff as now_playing for the first kickoffOverrideSec after session start
// (Spotify lags when we start playback). Otherwise returns the real cp.
func (m *Manager) GetEffectiveNowPlaying(cp *spotify.CurrentlyPlaying) *spotify.CurrentlyPlaying {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.kickoffTrack != nil && !m.sessionStartAt.IsZero() && time.Since(m.sessionStartAt) < kickoffOverrideSec*time.Second {
		return &spotify.CurrentlyPlaying{
			IsPlaying:  true,
			ProgressMs: 0,
			Item:       m.kickoffTrack,
		}
	}
	return cp
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

	// During kickoff override window, use kickoff for timing (Spotify may still return old track)
	effectiveItem := nowPlaying
	if m.kickoffTrack != nil && !m.sessionStartAt.IsZero() && time.Since(m.sessionStartAt) < kickoffOverrideSec*time.Second {
		effectiveItem = &spotify.CurrentlyPlaying{IsPlaying: true, ProgressMs: 0, Item: m.kickoffTrack}
	}

	// Countdown = time until we advance (add winner). We advance when current song has ~15s left.
	if round != nil {
		if effectiveItem != nil && effectiveItem.Item != nil {
			remainingMs := int64(effectiveItem.Item.DurationMs) - effectiveItem.ProgressMs
			secUntilAdvance := (remainingMs - int64(roundEndBufferMs)) / 1000
			if secUntilAdvance > 0 {
				timeRemainingSec = int(secUntilAdvance)
			} else if !round.RoundEndsAt.IsZero() {
				// cp may be wrong (e.g. old track during kickoff); fall back to RoundEndsAt
				remaining := time.Until(round.RoundEndsAt)
				if remaining > 0 {
					timeRemainingSec = int(remaining.Seconds())
				}
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
	lastAdvanceAt := m.lastAdvanceAt
	// During kickoff override, use kickoff for timing (Spotify may return old track)
	useCp := cp
	if m.kickoffTrack != nil && !m.sessionStartAt.IsZero() && time.Since(m.sessionStartAt) < kickoffOverrideSec*time.Second {
		useCp = &spotify.CurrentlyPlaying{IsPlaying: true, ProgressMs: 0, Item: m.kickoffTrack}
	}
	m.mu.RUnlock()
	if session == nil || session.Status != "active" || round == nil {
		return false
	}
	if pending != "" {
		return false
	}
	if time.Since(lastAdvanceAt) < advanceDebounceSec*time.Second {
		return false
	}
	if useCp == nil || useCp.Item == nil {
		return time.Now().After(round.RoundEndsAt)
	}
	remainingMs := int64(useCp.Item.DurationMs) - useCp.ProgressMs
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

		// Invalidate cp cache so next poll/tick fetches fresh data and detects winner sooner
		m.lastCp = nil
		m.lastCpAt = time.Time{}

		// Refill when unused tracks (total - played) falls below threshold. We only add; never remove original playlist tracks.
		if m.Session.RefillThreshold > 0 && m.Session.RefillCount > 0 {
			total, err := m.getPlaylistTotalTracks()
			if err == nil {
				unused := total - len(m.UsedTrackIDs)
				if unused < m.Session.RefillThreshold {
					if refillErr := m.refillPlaylistLocked(); refillErr != nil {
						log.Printf("voting: refill at round end: %v", refillErr)
					}
				}
			}
		}
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

	// Refill is triggered at end of round (in AdvanceRound), not here

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

func (m *Manager) refillPlaylistLocked() error {
	refillCount := m.Session.RefillCount
	if refillCount <= 0 {
		refillCount = 10
	}
	if refillCount > refillCountMax {
		refillCount = refillCountMax
	}
	delay := time.Duration(refillDelayMs) * time.Millisecond

	// Single fetch: collect artist IDs and playlist track IDs for dedup
	allArtistIDs := make(map[string]bool)
	playlistTrackIDs := make(map[string]bool)
	offset := 0
	for {
		time.Sleep(delay)
		resp, err := m.svc.GetPlaylistTracks(m.Session.PlaylistID, offset)
		if err != nil {
			log.Printf("voting: refill get playlist failed: %v", err)
			return fmt.Errorf("get playlist: %w", err)
		}
		for _, pitem := range resp.Items {
			track := pitem.Track
			if track == nil {
				track = pitem.Item
			}
			if track != nil && track.ID != "" {
				playlistTrackIDs[track.ID] = true
				for _, a := range track.Artists {
					if a.ID != "" {
						allArtistIDs[a.ID] = true
					}
				}
			}
		}
		if resp.Next == nil || *resp.Next == "" {
			break
		}
		offset += 50
	}
	if len(allArtistIDs) == 0 {
		return fmt.Errorf("no artists in playlist to use as seeds")
	}

	// Use similar-artists method (works when Recommendations API returns 404 for new apps)
	artistIDSlice := make([]string, 0, len(allArtistIDs))
	for id := range allArtistIDs {
		artistIDSlice = append(artistIDSlice, id)
	}
	seedArtistCount := refillCount + 8 // extra buffer for 403/404 on Top Tracks, Albums, Search
	if seedArtistCount > len(artistIDSlice) {
		seedArtistCount = len(artistIDSlice)
	}
	seedArtistIDs := pickRandomStrings(artistIDSlice, seedArtistCount)

	time.Sleep(delay)
	recs, err := m.svc.GetRefillTracksFromArtists(seedArtistIDs, refillCount*2)
	if err != nil {
		log.Printf("voting: refill GetRefillTracksFromArtists failed: %v", err)
		return fmt.Errorf("get refill tracks: %w", err)
	}

	var uris []string
	tracksPerArtist := make(map[string]bool) // 1 track per artist: N refill = N unique artists
	for _, t := range recs {
		if playlistTrackIDs[t.ID] || m.UsedTrackIDs[t.ID] {
			continue
		}
		artistID := ""
		if len(t.Artists) > 0 {
			artistID = t.Artists[0].ID
		}
		if artistID != "" && tracksPerArtist[artistID] {
			continue // already have a track from this artist
		}
		uris = append(uris, t.URI)
		m.RefilledTrackIDs[t.ID] = true
		m.RefilledTrackURIs = append(m.RefilledTrackURIs, t.URI)
		if artistID != "" {
			tracksPerArtist[artistID] = true
		}
		if len(uris) >= refillCount {
			break
		}
	}
	if len(uris) == 0 {
		return fmt.Errorf("no new tracks to add (all refill candidates already in playlist)")
	}
	time.Sleep(delay)
	if err := m.svc.AddTracksToPlaylist(m.Session.PlaylistID, uris); err != nil {
		log.Printf("voting: refill AddTracksToPlaylist failed: %v", err)
		return fmt.Errorf("add to playlist: %w", err)
	}
	log.Printf("voting: refill added %d tracks to playlist", len(uris))
	return nil
}

// pickRandomStrings returns n randomly selected strings from the slice (or all if fewer than n)
func pickRandomStrings(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	perm := make([]int, len(items))
	for i := range perm {
		perm[i] = i
	}
	for i := 0; i < n; i++ {
		j := i + rand.Intn(len(perm)-i)
		perm[i], perm[j] = perm[j], perm[i]
	}
	result := make([]string, n)
	for i := 0; i < n; i++ {
		result[i] = items[perm[i]]
	}
	return result
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

