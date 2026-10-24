package evals

import (
	"testing"
)

// TestVibeSenseVectorGate compares mocked evals with fixture venue index on vs off.
func TestVibeSenseVectorGate(t *testing.T) {
	offCfg := mockRunConfig(true)
	offCfg.VectorOn = false
	offResults, err := RunSuite(offCfg)
	if err != nil {
		t.Fatal(err)
	}
	offSum := Summarize(offResults, "mocked", false)

	onCfg := mockRunConfig(true)
	onCfg.VectorOn = true
	onCfg.VenueKeys = loadVenueHistoryKeys()
	onResults, err := RunSuite(onCfg)
	if err != nil {
		t.Fatal(err)
	}
	onSum := Summarize(onResults, "mocked", true)

	gate := CompareVectorGate(offSum, onSum)
	t.Logf("vector gate lift=%.3f off_venue=%.3f on_venue=%.3f resolve_off=%.3f resolve_on=%.3f",
		gate.Lift, offSum.VenueHistoryShare, onSum.VenueHistoryShare, offSum.ResolveRateMean, onSum.ResolveRateMean)
	if !gate.Passed {
		t.Fatalf("vector gate failed (PR should not merge): %v", gate.Failures)
	}
}
