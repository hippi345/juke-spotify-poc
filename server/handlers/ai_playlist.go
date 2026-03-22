package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/gemini"
	"juke-spotify-poc/server/spotify"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// aiPlaylistJob tracks background AI playlist creation (in-memory; POC single instance).
type aiPlaylistJob struct {
	Status string // pending | running | completed | failed
	Result gin.H
	ErrMsg string
}

var (
	aiJobMu sync.RWMutex
	aiJobs  = make(map[string]*aiPlaylistJob)
)

func setJob(id string, j *aiPlaylistJob) {
	aiJobMu.Lock()
	defer aiJobMu.Unlock()
	aiJobs[id] = j
}

func getJob(id string) (*aiPlaylistJob, bool) {
	aiJobMu.RLock()
	defer aiJobMu.RUnlock()
	j, ok := aiJobs[id]
	return j, ok
}

func updateJob(id string, fn func(*aiPlaylistJob)) {
	aiJobMu.Lock()
	defer aiJobMu.Unlock()
	if j, ok := aiJobs[id]; ok {
		fn(j)
	}
}

const maxPlaylistNameRunes = 100 // Spotify playlist name limit

type aiPlaylistRequest struct {
	PlaylistName   string   `json:"playlist_name"`
	TrackCount     int      `json:"track_count"`
	Description    string   `json:"description"`
	SimilarArtists []string `json:"similar_artists"`
}

// StartAIPlaylistJob validates input, enqueues background work, returns 202 + job_id.
func StartAIPlaylistJob(cfg *config.Config, client *spotify.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.GeminiAPIKey == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "VibeSense is not configured (set GEMINI_API_KEY on the server)"})
			return
		}

		var body aiPlaylistRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
			return
		}

		plName := strings.TrimSpace(body.PlaylistName)
		if plName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "playlist_name is required"})
			return
		}
		if utf8.RuneCountInString(plName) > maxPlaylistNameRunes {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("playlist_name must be at most %d characters", maxPlaylistNameRunes)})
			return
		}

		desc := strings.TrimSpace(body.Description)
		if desc == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "description is required"})
			return
		}

		n := body.TrackCount
		if n < 1 || n > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "track_count must be between 1 and 100"})
			return
		}

		acc, err := client.GetDefaultAccount()
		if err != nil || acc == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "connect Spotify first"})
			return
		}
		_ = acc // validated; job uses client again

		var artists []string
		for _, a := range body.SimilarArtists {
			s := strings.TrimSpace(a)
			if s != "" {
				artists = append(artists, s)
			}
		}

		id := uuid.NewString()
		setJob(id, &aiPlaylistJob{Status: "pending"})

		reqCopy := aiPlaylistRequest{
			PlaylistName:   plName,
			TrackCount:     n,
			Description:    desc,
			SimilarArtists: artists,
		}

		go runAIPlaylistJob(id, cfg, client, reqCopy)

		c.JSON(http.StatusAccepted, gin.H{
			"job_id": id,
			"status": "pending",
		})
	}
}

// AIPlaylistJobStatus returns job state for polling (pending | running | completed | failed).
func AIPlaylistJobStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		j, ok := getJob(id)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
			return
		}
		aiJobMu.RLock()
		status := j.Status
		result := j.Result
		errMsg := j.ErrMsg
		aiJobMu.RUnlock()

		switch status {
		case "completed":
			c.JSON(http.StatusOK, gin.H{"status": "completed", "result": result})
		case "failed":
			c.JSON(http.StatusOK, gin.H{"status": "failed", "error": errMsg})
		default:
			c.JSON(http.StatusOK, gin.H{"status": status})
		}
	}
}

func runAIPlaylistJob(id string, cfg *config.Config, client *spotify.Client, body aiPlaylistRequest) {
	updateJob(id, func(j *aiPlaylistJob) { j.Status = "running" })

	n := body.TrackCount
	desc := body.Description
	artists := body.SimilarArtists
	plTitle := strings.TrimSpace(body.PlaylistName)
	if plTitle == "" {
		failJob(id, "playlist name missing")
		return
	}

	suggestions, err := gemini.GenerateTrackList(cfg.GeminiAPIKey, cfg.GeminiModel, desc, n, artists)
	if err != nil {
		failJob(id, err.Error())
		return
	}

	if acc, err := client.GetDefaultAccount(); err != nil || acc == nil {
		failJob(id, "Spotify account disconnected")
		return
	}

	var uris []string
	var notFound []gin.H
	seenURI := make(map[string]bool)

	for _, s := range suggestions {
		if len(uris) >= n {
			break
		}
		q := fmt.Sprintf("%s %s", s.Artist, s.Title)
		tracks, err := client.SearchTracks(q, 5)
		if err != nil || len(tracks) == 0 {
			notFound = append(notFound, gin.H{"title": s.Title, "artist": s.Artist, "reason": "no search results"})
			time.Sleep(50 * time.Millisecond)
			continue
		}
		t := tracks[0]
		if t.URI == "" {
			notFound = append(notFound, gin.H{"title": s.Title, "artist": s.Artist, "reason": "missing uri"})
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if seenURI[t.URI] {
			notFound = append(notFound, gin.H{"title": s.Title, "artist": s.Artist, "reason": "duplicate"})
			time.Sleep(50 * time.Millisecond)
			continue
		}
		seenURI[t.URI] = true
		uris = append(uris, t.URI)
		time.Sleep(50 * time.Millisecond)
	}

	if len(uris) == 0 {
		failJob(id, "could not resolve any tracks on Spotify; try a different description or shorter list")
		return
	}

	plDesc := fmt.Sprintf("Created with VibeSense — %s", time.Now().Format(time.RFC3339))

	playlistID, err := client.CreatePlaylist(plTitle, plDesc, false)
	if err != nil {
		failJob(id, err.Error())
		return
	}

	for i := 0; i < len(uris); i += 100 {
		end := i + 100
		if end > len(uris) {
			end = len(uris)
		}
		if err := client.AddTracksToPlaylist(playlistID, uris[i:end]); err != nil {
			failJob(id, err.Error())
			return
		}
	}

	result := gin.H{
		"playlist_id":      playlistID,
		"playlist_url":     "https://open.spotify.com/playlist/" + playlistID,
		"name":             plTitle,
		"tracks_added":     len(uris),
		"tracks_requested": n,
		"ai_suggestions":   len(suggestions),
		"not_found":        notFound,
	}
	completeJob(id, result)
}

func completeJob(id string, result gin.H) {
	updateJob(id, func(j *aiPlaylistJob) {
		j.Status = "completed"
		j.Result = result
	})
}

func failJob(id, msg string) {
	updateJob(id, func(j *aiPlaylistJob) {
		j.Status = "failed"
		j.ErrMsg = msg
	})
}

