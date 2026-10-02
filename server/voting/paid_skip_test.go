package voting

import (
	"testing"
	"time"

	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestListPaidSkipsPendingQueue_order(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := database.AutoMigrate(&models.VotingSession{}, &models.PaidSkip{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db.SetDB(database)

	session := &models.VotingSession{PlaylistID: "p1", Status: "active"}
	if err := db.DB.Create(session).Error; err != nil {
		t.Fatal(err)
	}

	t1 := mustTime("2026-01-01T10:00:00Z")
	t2 := mustTime("2026-01-01T10:01:00Z")
	for _, row := range []models.PaidSkip{
		{VotingSessionID: session.ID, PatronUserID: 1, TrackID: "b", TrackURI: "spotify:track:b", Status: "paid", PaidAt: &t2, StripeCheckoutSessionID: "cs_b"},
		{VotingSessionID: session.ID, PatronUserID: 2, TrackID: "a", TrackURI: "spotify:track:a", Status: "paid", PaidAt: &t1, StripeCheckoutSessionID: "cs_a"},
	} {
		if err := db.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}

	m := NewManager(nil)
	m.mu.Lock()
	m.Session = session
	m.mu.Unlock()

	got := m.listPaidSkipsPendingQueue()
	if len(got) != 2 {
		t.Fatalf("expected 2 skips, got %d", len(got))
	}
	if got[0].TrackID != "a" || got[1].TrackID != "b" {
		t.Fatalf("FIFO order wrong: %s then %s", got[0].TrackID, got[1].TrackID)
	}
}

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
