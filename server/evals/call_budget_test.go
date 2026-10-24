package evals

import (
	"testing"

	"juke-spotify-poc/server/gemini"
)

func TestLiveCallBudget_enforcesCap(t *testing.T) {
	t.Setenv("JUKE_EVALS_LIVE", "1")
	t.Setenv("JUKE_EVALS_LIVE_CALL_CAP", "2")
	resetLiveCallBudget()
	if err := consumeLiveCall(); err != nil {
		t.Fatal(err)
	}
	if err := consumeLiveCall(); err != nil {
		t.Fatal(err)
	}
	if err := consumeLiveCall(); err == nil {
		t.Fatal("expected third call to exceed budget")
	}
}

func TestLiveBudgetStatus_inactiveByDefault(t *testing.T) {
	t.Setenv("JUKE_EVALS_LIVE", "")
	liveBudget = nil
	if LiveBudgetStatus() != "" {
		t.Fatalf("expected empty status when live budget inactive, got %q", LiveBudgetStatus())
	}
}

func TestBudgetedGenerator_countsCalls(t *testing.T) {
	t.Setenv("JUKE_EVALS_LIVE", "1")
	t.Setenv("JUKE_EVALS_LIVE_CALL_CAP", "1")
	inner := stubGen{}
	cfg := wrapLiveBudget(RunConfig{Generator: inner})
	if _, err := cfg.Generator.GenerateTrackList("x", 1, nil, gemini.GenerateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.Generator.GenerateTrackList("x", 1, nil, gemini.GenerateOptions{}); err == nil {
		t.Fatal("expected budget error on second generation")
	}
}

type stubGen struct{}

func (stubGen) GenerateTrackList(string, int, []string, gemini.GenerateOptions) ([]gemini.TrackSuggestion, error) {
	return []gemini.TrackSuggestion{{Title: "A", Artist: "B"}}, nil
}
