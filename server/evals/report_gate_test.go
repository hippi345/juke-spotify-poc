package evals

import (
	"strings"
	"testing"
	"time"

	"juke-spotify-poc/server/gemini"
)

func TestCompareBaseline_failsOnBlankTitles(t *testing.T) {
	c := Case{ID: "ambient_spa", Description: "ambient spa", TrackCount: 10}
	suggestions := []gemini.TrackSuggestion{
		{Title: "Track 1", Artist: "Artist ambientspa 1"},
		{Title: "", Artist: "Artist ambientspa 2"},
		{Title: "Track 3", Artist: "Artist ambientspa 3"},
		{Title: "", Artist: "Artist ambientspa 4"},
		{Title: "Track 5", Artist: "Artist ambientspa 5"},
		{Title: "", Artist: "Artist ambientspa 1"},
		{Title: "Track 7", Artist: "Artist ambientspa 2"},
		{Title: "Track 8", Artist: "Artist ambientspa 3"},
		{Title: "Track 9", Artist: "Artist ambientspa 4"},
		{Title: "Track 10", Artist: "Artist ambientspa 5"},
	}
	hard := RunHardChecks(c, suggestions, nil, nil, time.Now(), nil)
	results := []CaseResult{{CaseID: c.ID, Hard: hard, Judge: JudgeScores{MoodFit: 4, CrowdFit: 4, Variety: 4, Coherence: 4}}}
	summary := Summarize(results, "mocked", false)
	bl := SummaryMetrics{HardPassCount: 1, ResolveRateNA: true, JSONOK: true, CountOK: true, NoDuplicates: true}
	gate := CompareBaseline(summary, bl, results)
	if gate.Passed {
		t.Fatal("expected gate to fail when schema has blank titles")
	}
}

func TestCompareBaseline_failsOnInventedRAGTracks(t *testing.T) {
	c := Case{
		ID:          "rag_lofi_retrieval",
		Description: "RAG grounding — calm study lofi",
		TrackCount:  10,
		VenueID:     1,
		VectorEval:  true,
	}
	keys := loadVenueHistoryKeys()
	suggestions := []gemini.TrackSuggestion{
		{Title: "Phantom Lofi 1", Artist: "Invented Artist"},
		{Title: "Phantom Lofi 2", Artist: "Invented Artist"},
		{Title: "Phantom Lofi 3", Artist: "Invented Artist"},
		{Title: "Phantom Lofi 4", Artist: "Invented Artist"},
		{Title: "Phantom Lofi 5", Artist: "Invented Artist"},
		{Title: "Phantom Lofi 6", Artist: "Invented Artist"},
		{Title: "Phantom Lofi 7", Artist: "Invented Artist"},
		{Title: "Phantom Lofi 8", Artist: "Invented Artist"},
		{Title: "Phantom Lofi 9", Artist: "Invented Artist"},
		{Title: "Phantom Lofi 10", Artist: "Invented Artist"},
	}
	hard := RunHardChecksWithVector(c, suggestions, nil, keys, time.Now(), nil, true, true)
	if hard.GroundingOK {
		t.Fatalf("expected grounding failure for invented tracks, overlap=%.3f", hard.VenueOverlap)
	}
	results := []CaseResult{{CaseID: c.ID, Hard: hard, Judge: JudgeScores{MoodFit: 4, CrowdFit: 4, Variety: 4, Coherence: 4}}}
	summary := Summarize(results, "mocked", true)
	bl := SummaryMetrics{HardPassCount: 1, ResolveRateNA: true, JSONOK: true, CountOK: true, NoDuplicates: true}
	gate := CompareBaseline(summary, bl, results)
	if gate.Passed {
		t.Fatal("expected gate to fail when RAG grounding fails")
	}
	if !gateFailureContains(gate.Failures, "grounding check failed") {
		t.Fatalf("expected grounding check failed in gate failures: %v", gate.Failures)
	}
}

func TestCompareBaseline_groundingFailureMessage(t *testing.T) {
	c := Case{ID: "rag_lofi_retrieval", Description: "RAG", TrackCount: 10, VenueID: 1, VectorEval: true}
	keys := loadVenueHistoryKeys()
	suggestions := []gemini.TrackSuggestion{{Title: "Only Phantom", Artist: "Nobody"}}
	for len(suggestions) < 10 {
		suggestions = append(suggestions, gemini.TrackSuggestion{Title: "Phantom", Artist: "Nobody"})
	}
	hard := RunHardChecksWithVector(c, suggestions, nil, keys, time.Now(), nil, true, true)
	results := []CaseResult{{CaseID: c.ID, Hard: hard}}
	gate := CompareBaseline(Summarize(results, "mocked", true), SummaryMetrics{HardPassCount: 1, ResolveRateNA: true}, results)
	if gate.Passed {
		t.Fatal("expected failure")
	}
	if !gateFailureContains(gate.Failures, "case rag_lofi_retrieval: grounding check failed") {
		t.Fatalf("missing report grounding line: %v", gate.Failures)
	}
}

func gateFailureContains(failures []string, substr string) bool {
	for _, f := range failures {
		if strings.Contains(f, substr) {
			return true
		}
	}
	return false
}
