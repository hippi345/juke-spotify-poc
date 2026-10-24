package evals

// LiveGate evaluates baseline + thresholds + hard checks for a live report.
func LiveGate(summary SummaryMetrics, results []CaseResult, bl BaselineFile, th ThresholdsFile) GateResult {
	base := CompareBaseline(summary, bl.Full, results)
	thr := CompareThresholds(summary, results, bl.Full, th)
	return MergeGateResults(base, thr)
}
