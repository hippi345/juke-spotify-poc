package evals

import "fmt"

const maxLiveJudgeErrors = 5

// LiveGateOff gates the default (vector OFF) arm: live reference, thresholds, and hard checks.
func LiveGateOff(summary SummaryMetrics, results []CaseResult, bl BaselineFile, th ThresholdsFile) GateResult {
	liveRef := bl.LiveFull
	if liveRef.Mode == "" {
		liveRef = bl.Full
	}
	hard := PerCaseHardGate(results)
	agg := CompareLiveAggregate(summary, bl.LiveFull)
	thr := CompareThresholds(summary, results, liveRef, th)
	var judge GateResult
	if summary.JudgeErrorCount > maxLiveJudgeErrors {
		judge = GateResult{Passed: false, Failures: []string{
			fmt.Sprintf("judge errors %d exceed threshold %d", summary.JudgeErrorCount, maxLiveJudgeErrors),
		}}
	}
	return MergeGateResults(hard, agg, thr, judge)
}

// LiveGateVectorOn gates the vector ON arm: hard checks only (schema, grounding, tool sequence).
// Judge/rubric means are informational; quality regressions do not fail this arm.
func LiveGateVectorOn(results []CaseResult) GateResult {
	var failures []string
	for _, r := range results {
		if expectErrorCase(r.CaseID) {
			continue
		}
		if !r.Hard.SchemaOK {
			failures = append(failures, fmt.Sprintf("case %s: schema check failed", r.CaseID))
		}
		if !r.Hard.GroundingOK {
			failures = append(failures, fmt.Sprintf("case %s: grounding check failed", r.CaseID))
		}
		if !r.Hard.ToolSequenceOK {
			failures = append(failures, fmt.Sprintf("case %s: tool sequence check failed", r.CaseID))
		}
	}
	return GateResult{Passed: len(failures) == 0, Failures: failures}
}
