package evals

import (
	"testing"
)

// TestVibeSenseVectorGate compares vector-on vs vector-off on venue-history eval cases only.
func TestVibeSenseVectorGate(t *testing.T) {
	offResults, err := RunSuite(vectorEvalConfig(false))
	if err != nil {
		t.Fatal(err)
	}
	offSum := Summarize(offResults, "mocked", false)

	onResults, err := RunSuite(vectorEvalConfig(true))
	if err != nil {
		t.Fatal(err)
	}
	onSum := Summarize(onResults, "mocked", true)

	gate := CompareVectorGate(offSum, onSum)
	t.Logf("vector gate: mood off=%.3f on=%.3f | crowd off=%.3f on=%.3f | resolve off=%.3f on=%.3f | venue off=%.3f on=%.3f",
		offSum.MoodFitMean, onSum.MoodFitMean,
		offSum.CrowdFitMean, onSum.CrowdFitMean,
		offSum.ResolveRateMean, onSum.ResolveRateMean,
		offSum.VenueHistoryShare, onSum.VenueHistoryShare)
	if !gate.Passed {
		t.Fatalf("vector gate failed (no measurable quality gain): %v", gate.Failures)
	}
}
