package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"juke-spotify-poc/server/gemini"
)

// VectorGateResult compares vector-on vs vector-off eval runs (mocked regression).
type VectorGateResult struct {
	Passed   bool           `json:"passed"`
	Failures []string       `json:"failures,omitempty"`
	Off      SummaryMetrics `json:"off"`
	On       SummaryMetrics `json:"on"`
	Lift     float64        `json:"venue_overlap_lift"`
}

const vectorOverlapMinLift = 0.25

// CompareVectorGate is a mocked regression gate: deterministic overlap with venue-history
// fixtures must improve; judge mood/crowd are not used to claim merit (same fixture judge for on/off).
func CompareVectorGate(off, on SummaryMetrics) VectorGateResult {
	var failures []string
	lift := on.VenueOverlapMean - off.VenueOverlapMean

	if lift < vectorOverlapMinLift {
		failures = append(failures, fmt.Sprintf(
			"venue-history overlap lift %.3f below %.2f (off=%.3f on=%.3f)",
			lift, vectorOverlapMinLift, off.VenueOverlapMean, on.VenueOverlapMean))
	}
	if !on.ResolveRateNA && !off.ResolveRateNA && on.ResolveRateMean < off.ResolveRateMean-0.05 {
		failures = append(failures, fmt.Sprintf("resolve rate regressed: %.3f -> %.3f", off.ResolveRateMean, on.ResolveRateMean))
	}
	if on.MoodFitMean < off.MoodFitMean-0.01 {
		failures = append(failures, fmt.Sprintf("mood_fit regressed: %.3f -> %.3f", off.MoodFitMean, on.MoodFitMean))
	}
	if on.CrowdFitMean < off.CrowdFitMean-0.01 {
		failures = append(failures, fmt.Sprintf("crowd_fit regressed: %.3f -> %.3f", off.CrowdFitMean, on.CrowdFitMean))
	}
	if on.P50LatencyMs > off.P50LatencyMs+1000 {
		failures = append(failures, fmt.Sprintf("p50 latency rose %dms -> %dms", off.P50LatencyMs, on.P50LatencyMs))
	}
	return VectorGateResult{
		Passed:   len(failures) == 0,
		Failures: failures,
		Off:      off,
		On:       on,
		Lift:     lift,
	}
}

func loadVenueHistoryKeys() map[string]bool {
	path := filepath.Join(fixtureRoot(), "vector", "venue_history_keys.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var keys []string
	if err := json.Unmarshal(raw, &keys); err != nil {
		return nil
	}
	out := make(map[string]bool, len(keys))
	for _, k := range keys {
		out[k] = true
	}
	return out
}

func vectorEvalConfig(vectorOn bool) RunConfig {
	store := DefaultFixtureStore()
	keys := loadVenueHistoryKeys()
	mockCandidates := loadMockVenueCandidates(store)
	return RunConfig{
		VectorOnly:            true,
		VectorOn:              vectorOn,
		InjectVenueCandidates: vectorOn,
		MockVenueCandidates:   mockCandidates,
		Generator:             FixtureGenerator{Store: store, VectorOn: vectorOn},
		Searcher:              FixtureSearcher{Store: store},
		Judge:                 &perCaseFixtureJudge{Store: store, vectorOn: false},
		VenueKeys:             keys,
		FixtureStore:          store,
	}
}

func loadMockVenueCandidates(store FixtureStore) []gemini.TrackSuggestion {
	path := filepath.Join(store.Root, "vector", "mock_venue_candidates.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []gemini.TrackSuggestion
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}
