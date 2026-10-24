package evals

import (
	"testing"
	"time"

	"juke-spotify-poc/server/gemini"
)

func TestRunHardChecks_resolveSkippedWithoutSearcher(t *testing.T) {
	c := Case{ID: "test", Description: "mood", TrackCount: 2}
	suggestions := []gemini.TrackSuggestion{{Title: "A", Artist: "B"}}
	hard := RunHardChecks(c, suggestions, nil, nil, time.Now(), nil)
	if !hard.ResolveSkipped {
		t.Fatal("expected ResolveSkipped when searcher is nil")
	}
	if hard.ResolveRate != 0 {
		t.Fatalf("expected zero resolve rate when skipped, got %v", hard.ResolveRate)
	}
}
