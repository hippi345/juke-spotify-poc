package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ThresholdsFile is the committed qualitative + regression gate for live runs.
type ThresholdsFile struct {
	Calibration           ThresholdCalibration `json:"calibration"`
	MeanTargets           MeanTargets          `json:"mean_targets"`
	PerCaseFloors         PerCaseFloors        `json:"per_case_floors"`
	RegressionTolerance   RegressionTolerance  `json:"regression_tolerance"`
	HardGates             HardGateFlags        `json:"hard_gates"`
}

type ThresholdCalibration struct {
	Method             string `json:"method"`
	LiveRunsAveraged   int    `json:"live_runs_averaged"`
	JudgeRunsPerCase   int    `json:"judge_runs_per_case"`
	CalibratedAt       string `json:"calibrated_at,omitempty"`
}

type MeanTargets struct {
	MoodIntent         float64 `json:"mood_intent"`
	CrowdFit           float64 `json:"crowd_fit"`
	AccuracyCoherence  float64 `json:"accuracy_coherence"`
	Variety            float64 `json:"variety"`
	ResolveRate        float64 `json:"resolve_rate"`
	VenueOverlapVector float64 `json:"venue_overlap_vector_on,omitempty"`
}

type PerCaseFloors struct {
	MoodIntent        float64 `json:"mood_intent"`
	CrowdFit          float64 `json:"crowd_fit"`
	AccuracyCoherence float64 `json:"accuracy_coherence"`
	Variety           float64 `json:"variety"`
}

type RegressionTolerance struct {
	MoodIntentDrop    float64 `json:"mood_intent_drop"`
	CrowdFitDrop      float64 `json:"crowd_fit_drop"`
	CoherenceDrop     float64 `json:"accuracy_coherence_drop"`
	VarietyDrop       float64 `json:"variety_drop"`
	ResolveRateDrop   float64 `json:"resolve_rate_drop"`
	HardPassCountDrop int     `json:"hard_pass_count_drop"`
}

type HardGateFlags struct {
	SchemaPerCase        bool `json:"schema_per_case"`
	GroundingPerCase     bool `json:"grounding_per_case"`
	ToolSequencePerCase  bool `json:"tool_sequence_per_case"`
	KeyLeakScan          bool `json:"key_leak_scan"`
}

func thresholdsFile() string {
	return filepath.Join(filepath.Dir(baselineFile()), "thresholds.json")
}

func LoadThresholds(path string) (ThresholdsFile, error) {
	if path == "" {
		path = thresholdsFile()
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ThresholdsFile{}, err
	}
	var t ThresholdsFile
	if err := json.Unmarshal(raw, &t); err != nil {
		return ThresholdsFile{}, err
	}
	return t, nil
}

// CompareThresholds checks mean targets, per-case floors, and regression vs baseline (live).
func CompareThresholds(current SummaryMetrics, results []CaseResult, baseline SummaryMetrics, t ThresholdsFile) GateResult {
	var failures []string

	if t.MeanTargets.MoodIntent > 0 && current.MoodFitMean < t.MeanTargets.MoodIntent {
		failures = append(failures, fmt.Sprintf("mood/intent mean %.3f below target %.3f", current.MoodFitMean, t.MeanTargets.MoodIntent))
	}
	if t.MeanTargets.CrowdFit > 0 && current.CrowdFitMean < t.MeanTargets.CrowdFit {
		failures = append(failures, fmt.Sprintf("crowd fit mean %.3f below target %.3f", current.CrowdFitMean, t.MeanTargets.CrowdFit))
	}
	if t.MeanTargets.AccuracyCoherence > 0 && current.CoherenceMean < t.MeanTargets.AccuracyCoherence {
		failures = append(failures, fmt.Sprintf("accuracy/coherence mean %.3f below target %.3f", current.CoherenceMean, t.MeanTargets.AccuracyCoherence))
	}
	if t.MeanTargets.Variety > 0 && current.VarietyMean < t.MeanTargets.Variety {
		failures = append(failures, fmt.Sprintf("variety mean %.3f below target %.3f", current.VarietyMean, t.MeanTargets.Variety))
	}
	if t.MeanTargets.ResolveRate > 0 && !current.ResolveRateNA && current.ResolveRateMean < t.MeanTargets.ResolveRate {
		failures = append(failures, fmt.Sprintf("resolve rate %.3f below target %.3f", current.ResolveRateMean, t.MeanTargets.ResolveRate))
	}
	if current.VectorOn && t.MeanTargets.VenueOverlapVector > 0 && current.VenueOverlapMean > 0 &&
		current.VenueOverlapMean < t.MeanTargets.VenueOverlapVector {
		failures = append(failures, fmt.Sprintf("venue overlap %.3f below vector-on target %.3f", current.VenueOverlapMean, t.MeanTargets.VenueOverlapVector))
	}

	for _, r := range results {
		if expectErrorCase(r.CaseID) {
			continue
		}
		if caseUsesMoodRubric(r.CaseID) && t.PerCaseFloors.MoodIntent > 0 && r.Judge.MoodFit < t.PerCaseFloors.MoodIntent {
			failures = append(failures, fmt.Sprintf("case %s: mood/intent %.2f below floor %.2f", r.CaseID, r.Judge.MoodFit, t.PerCaseFloors.MoodIntent))
		}
		if t.PerCaseFloors.CrowdFit > 0 && r.Judge.CrowdFit < t.PerCaseFloors.CrowdFit {
			failures = append(failures, fmt.Sprintf("case %s: crowd fit %.2f below floor %.2f", r.CaseID, r.Judge.CrowdFit, t.PerCaseFloors.CrowdFit))
		}
		if t.PerCaseFloors.AccuracyCoherence > 0 && r.Judge.Coherence < t.PerCaseFloors.AccuracyCoherence {
			failures = append(failures, fmt.Sprintf("case %s: coherence %.2f below floor %.2f", r.CaseID, r.Judge.Coherence, t.PerCaseFloors.AccuracyCoherence))
		}
		if t.PerCaseFloors.Variety > 0 && r.Judge.Variety < t.PerCaseFloors.Variety {
			failures = append(failures, fmt.Sprintf("case %s: variety %.2f below floor %.2f", r.CaseID, r.Judge.Variety, t.PerCaseFloors.Variety))
		}
	}

	rt := t.RegressionTolerance
	if rt.MoodIntentDrop > 0 && current.MoodFitMean < baseline.MoodFitMean-rt.MoodIntentDrop {
		failures = append(failures, fmt.Sprintf("mood/intent regressed %.3f -> %.3f (max drop %.2f)", baseline.MoodFitMean, current.MoodFitMean, rt.MoodIntentDrop))
	}
	if rt.CrowdFitDrop > 0 && current.CrowdFitMean < baseline.CrowdFitMean-rt.CrowdFitDrop {
		failures = append(failures, fmt.Sprintf("crowd fit regressed %.3f -> %.3f", baseline.CrowdFitMean, current.CrowdFitMean))
	}
	if rt.CoherenceDrop > 0 && current.CoherenceMean < baseline.CoherenceMean-rt.CoherenceDrop {
		failures = append(failures, fmt.Sprintf("coherence regressed %.3f -> %.3f", baseline.CoherenceMean, current.CoherenceMean))
	}
	if rt.VarietyDrop > 0 && current.VarietyMean < baseline.VarietyMean-rt.VarietyDrop {
		failures = append(failures, fmt.Sprintf("variety regressed %.3f -> %.3f", baseline.VarietyMean, current.VarietyMean))
	}
	if rt.ResolveRateDrop > 0 && !current.ResolveRateNA && !baseline.ResolveRateNA &&
		current.ResolveRateMean < baseline.ResolveRateMean-rt.ResolveRateDrop {
		failures = append(failures, fmt.Sprintf("resolve rate regressed %.3f -> %.3f", baseline.ResolveRateMean, current.ResolveRateMean))
	}
	if current.HardPassCount < baseline.HardPassCount-rt.HardPassCountDrop {
		failures = append(failures, fmt.Sprintf("hard pass count regressed %d -> %d", baseline.HardPassCount, current.HardPassCount))
	}

	return GateResult{Passed: len(failures) == 0, Failures: failures}
}

// MergeGateResults combines multiple gate outcomes.
func MergeGateResults(parts ...GateResult) GateResult {
	var failures []string
	for _, p := range parts {
		failures = append(failures, p.Failures...)
	}
	return GateResult{Passed: len(failures) == 0, Failures: failures}
}
