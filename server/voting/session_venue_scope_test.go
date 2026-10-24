package voting

import (
	"sync"
	"testing"

	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupVenueScopeTestDB(t *testing.T) {
	t.Helper()
	database, err := gorm.Open(sqlite.Open("file:venue_scope?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("sql DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := database.AutoMigrate(
		&models.VotingSession{},
		&models.SessionRefillTrack{},
		&models.User{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db.SetDB(database)
}

func TestVenueIDsMatch(t *testing.T) {
	v12, v13 := uint(12), uint(13)
	p12, p13 := &v12, &v13
	if !venueIDsMatch(nil, nil) {
		t.Fatal("nil,nil should match")
	}
	if !venueIDsMatch(p12, p12) {
		t.Fatal("same venue should match")
	}
	if venueIDsMatch(p12, p13) || venueIDsMatch(p12, nil) || venueIDsMatch(nil, p13) {
		t.Fatal("different or mixed nil venues should not match")
	}
}

func TestEndStaleDBActiveSessionsForVenue_onlySameVenue(t *testing.T) {
	setupVenueScopeTestDB(t)
	v12, v13 := uint(12), uint(13)
	s12 := &models.VotingSession{VenueID: &v12, PlaylistID: "p12", Status: "active"}
	s13 := &models.VotingSession{VenueID: &v13, PlaylistID: "p13", Status: "active"}
	if err := db.DB.Create(s12).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(s13).Error; err != nil {
		t.Fatal(err)
	}

	m := NewManager(nil)
	endStaleDBActiveSessionsForVenue(m, &v13)

	var got12, got13 models.VotingSession
	if err := db.DB.First(&got12, s12.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.First(&got13, s13.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got12.Status != "active" {
		t.Fatalf("venue 12 session should stay active, got %q", got12.Status)
	}
	if got13.Status != "ended" {
		t.Fatalf("venue 13 session should be ended, got %q", got13.Status)
	}
}

func TestStartSession_inMemoryDoesNotEndOtherVenue(t *testing.T) {
	v12, v13 := uint(12), uint(13)
	m := NewManager(nil)
	m.mu.Lock()
	m.Session = &models.VotingSession{
		ID:         5,
		VenueID:    &v12,
		PlaylistID: "p12",
		Status:     "active",
	}
	startVenue := &v13
	if m.Session != nil && m.Session.Status == "active" && venueIDsMatch(m.Session.VenueID, startVenue) {
		m.endActiveSessionLocked()
	}
	m.mu.Unlock()

	if m.Session == nil || m.Session.Status != "active" || m.Session.ID != 5 {
		t.Fatalf("in-memory session for venue 12 should remain active, got %+v", m.Session)
	}
}

func TestTwoVenues_endStaleConcurrent(t *testing.T) {
	setupVenueScopeTestDB(t)
	v12, v13 := uint(12), uint(13)
	s12 := &models.VotingSession{VenueID: &v12, PlaylistID: "p12", Status: "active"}
	s13 := &models.VotingSession{VenueID: &v13, PlaylistID: "p13", Status: "active"}
	if err := db.DB.Create(s12).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(s13).Error; err != nil {
		t.Fatal(err)
	}

	m12 := NewManager(nil)
	m13 := NewManager(nil)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		endStaleDBActiveSessionsForVenue(m12, &v12)
	}()
	go func() {
		defer wg.Done()
		endStaleDBActiveSessionsForVenue(m13, &v13)
	}()
	wg.Wait()

	for _, id := range []uint{s12.ID, s13.ID} {
		var s models.VotingSession
		if err := db.DB.First(&s, id).Error; err != nil {
			t.Fatal(err)
		}
		if s.Status != "ended" {
			t.Fatalf("session %d should be ended, got %q", id, s.Status)
		}
	}
}
