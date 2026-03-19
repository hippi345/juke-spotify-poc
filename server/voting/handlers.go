package voting

import (
	"net/http"
	"strings"

	"juke-spotify-poc/server/spotify"

	"github.com/gin-gonic/gin"
)

// Handlers holds dependencies for voting HTTP handlers
type Handlers struct {
	Manager *Manager
	Svc     *spotify.Client
}

// SessionStart starts a new voting session
func (h *Handlers) SessionStart(c *gin.Context) {
	var body struct {
		PlaylistID      string `json:"playlist_id" binding:"required"`
		PlaylistName    string `json:"playlist_name"`
		RefillThreshold int    `json:"refill_threshold"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "playlist_id required"})
		return
	}

	if err := h.Manager.StartSession(body.PlaylistID, body.PlaylistName, body.RefillThreshold); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "403") {
			errMsg = "Cannot access this playlist (403). In Spotify Developer Dashboard: 1) Add your Spotify email under User Management if in Development Mode, 2) Ensure redirect URI is http://127.0.0.1:5173/api/spotify/callback, 3) Disconnect and reconnect to refresh permissions."
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "started"})
}

// SessionEnd ends the current voting session
func (h *Handlers) SessionEnd(c *gin.Context) {
	h.Manager.EndSession()
	c.JSON(http.StatusOK, gin.H{"status": "ended"})
}

// State returns the current voting state for polling.
// Also triggers advance check (same logic as ticker) so rounds advance when client polls.
func (h *Handlers) State(c *gin.Context) {
	cp, _ := h.Svc.GetCurrentlyPlaying()

	// Start new round when winner (queued) has begun playing
	h.Manager.StartNewRoundIfWinnerPlaying(cp)

	session, round, timeRemainingSec := h.Manager.GetState(cp)

	// Run advance check when we have session+round (piggyback on polling - ensures advance happens)
	if session != nil && session.Status == "active" && round != nil {
		shouldAdvance := false
		if cp == nil || cp.Item == nil {
			shouldAdvance = h.Manager.ShouldAdvanceRound(nil)
		} else {
			shouldAdvance = h.Manager.ShouldAdvanceRound(cp)
		}
		if shouldAdvance {
			_ = h.Manager.AdvanceRound()
		}
		// Re-fetch state after potential advance
		session, round, timeRemainingSec = h.Manager.GetState(cp)
	}

	resp := gin.H{
		"session":          nil,
		"now_playing":      nil,
		"candidates":      nil,
		"votes":            nil,
		"time_remaining_sec": timeRemainingSec,
	}

	if cp != nil {
		resp["now_playing"] = gin.H{
			"playing":      cp.IsPlaying,
			"progress_ms":  cp.ProgressMs,
			"item":         cp.Item,
		}
	}

	if session != nil {
		resp["session"] = gin.H{
			"id":               session.ID,
			"playlist_id":      session.PlaylistID,
			"playlist_name":    session.PlaylistName,
			"refill_threshold": session.RefillThreshold,
			"status":           session.Status,
		}
	}

	if round != nil {
		resp["candidates"] = round.Candidates
		resp["votes"] = round.Votes
	}

	c.JSON(http.StatusOK, resp)
}

// PlaylistOverview returns all playlist tracks with played/refilled status
func (h *Handlers) PlaylistOverview(c *gin.Context) {
	tracks, err := h.Manager.GetPlaylistOverview()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tracks": tracks})
}

// Vote records a vote for a track
func (h *Handlers) Vote(c *gin.Context) {
	var body struct {
		TrackID string `json:"track_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "track_id required"})
		return
	}

	if err := h.Manager.Vote(body.TrackID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "voted"})
}
