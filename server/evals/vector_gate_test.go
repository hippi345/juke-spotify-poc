package evals

import (
	"testing"
)

// TestVibeSenseVectorRegressionGate compares vector on/off on venue-history cases using
// deterministic overlap with fixture venue keys (not LLM-judge merit).
func TestVibeSenseVectorRegressionGate(t *testing.T) {
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
	t.Logf("vector regression gate: overlap off=%.3f on=%.3f lift=%.3f | mood %.3f/%.3f crowd %.3f/%.3f resolve %.3f/%.3f",
		offSum.VenueOverlapMean, onSum.VenueOverlapMean, gate.Lift,
		offSum.MoodFitMean, onSum.MoodFitMean,
		offSum.CrowdFitMean, onSum.CrowdFitMean,
		offSum.ResolveRateMean, onSum.ResolveRateMean)
	if !gate.Passed {
		t.Fatalf("vector regression gate failed: %v", gate.Failures)
	}
}
