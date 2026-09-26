package voting

import (
	"testing"

	"juke-spotify-poc/server/spotify"
)

func TestPickWinner_emptyCandidates(t *testing.T) {
	if pickWinner(&RoundState{Candidates: nil, Votes: map[string]int{}}) != nil {
		t.Fatal("expected nil winner for empty candidates")
	}
}

func TestPickWinner_clearMajority(t *testing.T) {
	candidates := []spotify.Track{
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B"},
		{ID: "c", Name: "C"},
	}
	round := &RoundState{
		Candidates: candidates,
		Votes:      map[string]int{"a": 1, "b": 3, "c": 2},
	}
	w := pickWinner(round)
	if w == nil || w.ID != "b" {
		t.Fatalf("expected track b, got %v", w)
	}
}

func TestPickRandomN_returnsAllWhenFewerThanN(t *testing.T) {
	tracks := []spotify.Track{{ID: "1"}, {ID: "2"}}
	got := pickRandomN(tracks, 5)
	if len(got) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(got))
	}
}

func TestPickRandomStrings_respectsCount(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}
	got := pickRandomStrings(items, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 items, got %d", len(got))
	}
	seen := make(map[string]bool)
	for _, s := range got {
		if seen[s] {
			t.Fatalf("duplicate pick: %s", s)
		}
		seen[s] = true
	}
}

func TestManager_HasActiveSession(t *testing.T) {
	m := NewManager(nil)
	if m.HasActiveSession() {
		t.Fatal("expected no active session initially")
	}
}
