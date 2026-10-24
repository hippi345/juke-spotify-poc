package evals

import (
	"testing"
)

func TestCompareThresholds_failsBelowMeanTarget(t *testing.T) {
	tgt, err := LoadThresholds(thresholdsFile())
	if err != nil {
		t.Fatal(err)
	}
	tgt.MeanTargets.MoodIntent = 4.0
	current := SummaryMetrics{MoodFitMean: 3.0, CrowdFitMean: 4, VarietyMean: 4, CoherenceMean: 4, ResolveRateNA: true}
	g := CompareThresholds(current, nil, SummaryMetrics{}, tgt)
	if g.Passed {
		t.Fatal("expected failure below mood target")
	}
}
