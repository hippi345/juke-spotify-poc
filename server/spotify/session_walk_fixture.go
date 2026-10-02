package spotify

import (
	"sync"
	"time"
)

// SessionWalkPlaylistID is the fixed playlist id used by scripts/compose-session-walk.sh
// when Spotify is not connected. Playback calls are no-ops; tracks are served locally.
const SessionWalkPlaylistID = "compose-session-walk-demo"

var (
	sessionWalkMu           sync.Mutex
	sessionWalkFixtureActive bool
	sessionWalkNowPlaying   *Track
	sessionWalkNowPlayingAt time.Time
)

func isSessionWalkPlaylist(playlistID string) bool {
	return playlistID == SessionWalkPlaylistID
}

func sessionWalkTracks() []Track {
	return []Track{
		{ID: "walk-track-alpha", Name: "Walk Alpha", URI: "spotify:track:walk-track-alpha", DurationMs: 180000},
		{ID: "walk-track-bravo", Name: "Walk Bravo", URI: "spotify:track:walk-track-bravo", DurationMs: 180000},
		{ID: "walk-track-charlie", Name: "Walk Charlie", URI: "spotify:track:walk-track-charlie", DurationMs: 180000},
		{ID: "walk-track-delta", Name: "Walk Delta", URI: "spotify:track:walk-track-delta", DurationMs: 180000},
		{ID: "walk-track-echo", Name: "Walk Echo", URI: "spotify:track:walk-track-echo", DurationMs: 180000},
	}
}

func getSessionWalkPlaylistTracks(offset int) (*PlaylistTracksResponse, error) {
	sessionWalkMu.Lock()
	sessionWalkFixtureActive = true
	sessionWalkMu.Unlock()
	all := sessionWalkTracks()
	if offset >= len(all) {
		return &PlaylistTracksResponse{Items: nil, Next: nil, Total: len(all)}, nil
	}
	items := make([]PlaylistTrackItem, 0, len(all)-offset)
	for _, tr := range all[offset:] {
		t := tr
		items = append(items, PlaylistTrackItem{Track: &t})
	}
	var next *string
	if offset+len(items) < len(all) {
		empty := ""
		next = &empty
	}
	return &PlaylistTracksResponse{
		Items: items,
		Next:  next,
		Total: len(all),
	}, nil
}

func setSessionWalkNowPlaying(trackURI string) {
	sessionWalkMu.Lock()
	defer sessionWalkMu.Unlock()
	for _, tr := range sessionWalkTracks() {
		if tr.URI == trackURI {
			t := tr
			sessionWalkNowPlaying = &t
			sessionWalkNowPlayingAt = time.Now()
			return
		}
	}
}

func isSessionWalkTrackURI(uri string) bool {
	for _, tr := range sessionWalkTracks() {
		if tr.URI == uri {
			return true
		}
	}
	return false
}

func sessionWalkPlaybackEnabled() bool {
	sessionWalkMu.Lock()
	defer sessionWalkMu.Unlock()
	return sessionWalkFixtureActive
}

func getSessionWalkCurrentlyPlaying() (*CurrentlyPlaying, error) {
	sessionWalkMu.Lock()
	defer sessionWalkMu.Unlock()
	if sessionWalkNowPlaying == nil {
		// Default idle state: first track at start so round timing works.
		t := sessionWalkTracks()[0]
		sessionWalkNowPlaying = &t
		sessionWalkNowPlayingAt = time.Now()
	}
	return &CurrentlyPlaying{
		IsPlaying:  true,
		ProgressMs: 0,
		Item:       sessionWalkNowPlaying,
	}, nil
}
