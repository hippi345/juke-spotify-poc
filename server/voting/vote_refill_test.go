package voting

import (
	"errors"
	"testing"

	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"
	"juke-spotify-poc/server/spotify"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupVotingTestDB(t *testing.T) {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := database.AutoMigrate(
		&models.VotingSession{},
		&models.SessionRefillTrack{},
		&models.User{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db.SetDB(database)
}

func TestManager_Vote_onePerPatronPerRound(t *testing.T) {
	m := NewManager(nil)
	m.mu.Lock()
	m.Round = &RoundState{
		Candidates: []spotify.Track{
			{ID: "a", Name: "A"},
			{ID: "b", Name: "B"},
		},
		Votes:          make(map[string]int),
		VotedPatronIDs: make(map[uint]bool),
	}
	m.mu.Unlock()

	if err := m.Vote(1, "a"); err != nil {
		t.Fatalf("first vote: %v", err)
	}
	if err := m.Vote(2, "b"); err != nil {
		t.Fatalf("second patron vote: %v", err)
	}
	if err := m.Vote(1, "b"); !errors.Is(err, ErrAlreadyVotedThisRound) {
		t.Fatalf("expected ErrAlreadyVotedThisRound on duplicate patron, got %v", err)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.Round.Votes["a"] != 1 || m.Round.Votes["b"] != 1 {
		t.Fatalf("unexpected vote counts: %v", m.Round.Votes)
	}
}

type fakePlaylistRemover struct {
	removedBatches [][]string
	playlistID     string
}

func (f *fakePlaylistRemover) RemoveTracksFromPlaylist(playlistID string, trackURIs []string) error {
	f.playlistID = playlistID
	f.removedBatches = append(f.removedBatches, append([]string(nil), trackURIs...))
	return nil
}

func TestRemoveRefillTracksAfterRestart_keepRefillOff(t *testing.T) {
	setupVotingTestDB(t)
	session := &models.VotingSession{
		PlaylistID:       "playlist-1",
		Status:           "active",
		KeepRefillTracks: false,
	}
	if err := db.DB.Create(session).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []models.SessionRefillTrack{
		{VotingSessionID: session.ID, TrackID: "t1", TrackURI: "spotify:track:t1"},
		{VotingSessionID: session.ID, TrackID: "t2", TrackURI: "spotify:track:t2"},
	} {
		if err := db.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}

	fake := &fakePlaylistRemover{}
	if err := removeRefillTracksForSession(fake, session); err != nil {
		t.Fatal(err)
	}
	if fake.playlistID != "playlist-1" {
		t.Fatalf("expected playlist-1, got %q", fake.playlistID)
	}
	if len(fake.removedBatches) != 1 || len(fake.removedBatches[0]) != 2 {
		t.Fatalf("expected one batch of 2 URIs, got %#v", fake.removedBatches)
	}

	var remaining int64
	if err := db.DB.Model(&models.SessionRefillTrack{}).Where("voting_session_id = ?", session.ID).Count(&remaining).Error; err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("expected refill rows deleted, got %d", remaining)
	}
}

func TestRemoveRefillTracksAfterRestart_keepRefillOn(t *testing.T) {
	setupVotingTestDB(t)
	session := &models.VotingSession{
		PlaylistID:       "playlist-2",
		Status:           "active",
		KeepRefillTracks: true,
	}
	if err := db.DB.Create(session).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&models.SessionRefillTrack{
		VotingSessionID: session.ID,
		TrackID:         "t9",
		TrackURI:        "spotify:track:t9",
	}).Error; err != nil {
		t.Fatal(err)
	}

	fake := &fakePlaylistRemover{}
	if err := removeRefillTracksForSession(fake, session); err != nil {
		t.Fatal(err)
	}
	if len(fake.removedBatches) != 0 {
		t.Fatalf("keep-refill on should not remove from playlist, got %#v", fake.removedBatches)
	}
	var remaining int64
	if err := db.DB.Model(&models.SessionRefillTrack{}).Where("voting_session_id = ?", session.ID).Count(&remaining).Error; err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("expected tracking rows cleared, got %d", remaining)
	}
}

func TestRecoverOrphanedActiveSessions_cleansRefillTracks(t *testing.T) {
	setupVotingTestDB(t)
	fake := &fakePlaylistRemover{}
	session := &models.VotingSession{
		PlaylistID:       "orphan-pl",
		Status:           "active",
		KeepRefillTracks: false,
	}
	if err := db.DB.Create(session).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&models.SessionRefillTrack{
		VotingSessionID: session.ID,
		TrackID:         "x",
		TrackURI:        "spotify:track:x",
	}).Error; err != nil {
		t.Fatal(err)
	}

	m := NewManager(nil)
	m.removerOverride = fake
	RecoverOrphanedActiveSessions(m)

	if len(fake.removedBatches) != 1 || len(fake.removedBatches[0]) != 1 {
		t.Fatalf("expected refill removal on startup recover, got %#v", fake.removedBatches)
	}
	if err := db.DB.First(&session, session.ID).Error; err != nil {
		t.Fatal(err)
	}
	if session.Status != "ended" {
		t.Fatalf("expected session ended after recover, got %q", session.Status)
	}
	var refillRows int64
	if err := db.DB.Model(&models.SessionRefillTrack{}).Where("voting_session_id = ?", session.ID).Count(&refillRows).Error; err != nil {
		t.Fatal(err)
	}
	if refillRows != 0 {
		t.Fatalf("expected refill rows cleared, got %d", refillRows)
	}
}
