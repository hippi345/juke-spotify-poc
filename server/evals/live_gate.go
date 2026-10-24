package evals

import "fmt"

// LiveGateOff gates the default (vector OFF) arm: baseline, thresholds, and hard checks.
func LiveGateOff(summary SummaryMetrics, results []CaseResult, bl BaselineFile, th ThresholdsFile) GateResult {
	base := CompareBaseline(summary, bl.Full, results)
	thr := CompareThresholds(summary, results, bl.Full, th)
	return MergeGateResults(base, thr)
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
