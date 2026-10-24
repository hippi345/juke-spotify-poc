package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// VectorGateResult compares vector-on vs vector-off eval runs.
type VectorGateResult struct {
	Passed   bool     `json:"passed"`
	Failures []string `json:"failures,omitempty"`
	Off      SummaryMetrics `json:"off"`
	On       SummaryMetrics `json:"on"`
	Lift     float64        `json:"venue_history_lift"`
}

// CompareVectorGate decides whether vector grounding helps enough to ship.
func CompareVectorGate(off, on SummaryMetrics) VectorGateResult {
	var failures []string
	if on.MoodFitMean < off.MoodFitMean-0.01 {
		failures = append(failures, fmt.Sprintf("mood_fit dropped with vector: %.3f -> %.3f", off.MoodFitMean, on.MoodFitMean))
	}
	if on.CrowdFitMean < off.CrowdFitMean-0.01 {
		failures = append(failures, fmt.Sprintf("crowd_fit dropped with vector: %.3f -> %.3f", off.CrowdFitMean, on.CrowdFitMean))
	}
	if on.ResolveRateMean < off.ResolveRateMean-0.001 {
		failures = append(failures, fmt.Sprintf("resolve rate dropped with vector: %.3f -> %.3f", off.ResolveRateMean, on.ResolveRateMean))
	}
	lift := on.VenueHistoryShare - off.VenueHistoryShare
	if lift < 0.05 {
		failures = append(failures, fmt.Sprintf("venue history share lift %.3f below 0.05 threshold", lift))
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
