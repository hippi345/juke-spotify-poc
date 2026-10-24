package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BaselineFile holds CI and full-suite reference metrics.
type BaselineFile struct {
	CI   SummaryMetrics `json:"ci"`
	Full SummaryMetrics `json:"full"`
}

// SummaryMetrics are gate inputs compared against baseline.json.
type SummaryMetrics struct {
	GeneratedAt       string  `json:"generated_at"`
	Mode              string  `json:"mode"`
	VectorOn          bool    `json:"vector_on"`
	CaseCount         int     `json:"case_count"`
	ResolveRateMean   float64 `json:"resolve_rate_mean"`
	MoodFitMean       float64 `json:"mood_fit_mean"`
	CrowdFitMean      float64 `json:"crowd_fit_mean"`
	VarietyMean       float64 `json:"variety_mean"`
	CoherenceMean     float64 `json:"coherence_mean"`
	P50LatencyMs      int64   `json:"p50_latency_ms"`
	P95LatencyMs      int64   `json:"p95_latency_ms"`
	HardPassCount     int     `json:"hard_pass_count"`
	VenueHistoryShare float64 `json:"venue_history_share_mean"`
	VenueOverlapMean  float64 `json:"venue_overlap_mean"`
	JSONOK            bool    `json:"json_ok"`
	CountOK           bool    `json:"count_ok"`
	NoDuplicates      bool    `json:"no_duplicates"`
}

// GateResult explains baseline comparison outcome.
type GateResult struct {
	Passed   bool     `json:"passed"`
	Failures []string `json:"failures,omitempty"`
}

func Summarize(results []CaseResult, mode string, vectorOn bool) SummaryMetrics {
	var resolveSum, moodSum, crowdSum, varietySum, coherenceSum, venueSum, overlapSum float64
	var latencies []int64
	hardPass := 0
	jsonOK := true
	countOK := true
	dupOK := true
	var venueN int
	for _, r := range results {
		resolveSum += r.Hard.ResolveRate
		moodSum += r.Judge.MoodFit
		crowdSum += r.Judge.CrowdFit
		varietySum += r.Judge.Variety
		coherenceSum += r.Judge.Coherence
		latencies = append(latencies, r.Hard.LatencyMs)
		if r.Hard.JSONOK && r.Hard.CountOK && r.Hard.NoDuplicates && (r.Hard.ErrorClean || !expectErrorCase(r.CaseID)) {
			if r.Hard.ResolveRate >= 0.85 || expectErrorCase(r.CaseID) {
				hardPass++
			}
		}
		if !r.Hard.JSONOK {
			jsonOK = false
		}
		if !r.Hard.CountOK && !expectErrorCase(r.CaseID) {
			countOK = false
		}
		if !r.Hard.NoDuplicates {
			dupOK = false
		}
		if r.Hard.ResolvedFromVenue > 0 {
			venueSum += r.Hard.ResolvedFromVenue
			venueN++
		}
		overlapSum += r.Hard.VenueOverlap
	}
	n := float64(len(results))
	if n == 0 {
		n = 1
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p50 := latencies[len(latencies)/2]
	p95 := latencies[len(latencies)*95/100]
	venueMean := 0.0
	if venueN > 0 {
		venueMean = venueSum / float64(venueN)
	}
	return SummaryMetrics{
		GeneratedAt:       time.Now().UTC().Format(time.RFC3339),
		Mode:              mode,
		VectorOn:          vectorOn,
		CaseCount:         len(results),
		ResolveRateMean:   resolveSum / n,
		MoodFitMean:       moodSum / n,
		CrowdFitMean:      crowdSum / n,
		VarietyMean:       varietySum / n,
		CoherenceMean:     coherenceSum / n,
		P50LatencyMs:      p50,
		P95LatencyMs:      p95,
		HardPassCount:     hardPass,
		VenueHistoryShare: venueMean,
		VenueOverlapMean:  overlapSum / n,
		JSONOK:            jsonOK,
		CountOK:           countOK,
		NoDuplicates:      dupOK,
	}
}

func expectErrorCase(id string) bool {
	for _, c := range AllCases() {
		if c.ID == id {
			return c.ExpectError
		}
	}
	return false
}

// CompareBaseline applies regression thresholds from the spec.
func CompareBaseline(current, baseline SummaryMetrics) GateResult {
	var failures []string
	if current.ResolveRateMean < baseline.ResolveRateMean-0.05 {
		failures = append(failures, fmt.Sprintf("resolve rate dropped %.3f -> %.3f (max -0.05)", baseline.ResolveRateMean, current.ResolveRateMean))
	}
	if current.MoodFitMean < baseline.MoodFitMean-0.2 {
		failures = append(failures, fmt.Sprintf("mood_fit mean dropped %.3f -> %.3f", baseline.MoodFitMean, current.MoodFitMean))
	}
	if current.CrowdFitMean < baseline.CrowdFitMean-0.2 {
		failures = append(failures, fmt.Sprintf("crowd_fit mean dropped %.3f -> %.3f", baseline.CrowdFitMean, current.CrowdFitMean))
	}
	if baseline.JSONOK && !current.JSONOK {
		failures = append(failures, "JSON checks regressed")
	}
	if baseline.CountOK && !current.CountOK {
		failures = append(failures, "count checks regressed")
	}
	if baseline.NoDuplicates && !current.NoDuplicates {
		failures = append(failures, "duplicate checks regressed")
	}
	if current.P50LatencyMs > baseline.P50LatencyMs+1000 {
		failures = append(failures, fmt.Sprintf("p50 latency rose %dms -> %dms (max +1000ms)", baseline.P50LatencyMs, current.P50LatencyMs))
	}
	return GateResult{Passed: len(failures) == 0, Failures: failures}
}

func LoadBaseline(path string) (BaselineFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return BaselineFile{}, err
	}
	var m BaselineFile
	if err := json.Unmarshal(raw, &m); err != nil {
		// legacy single-object baseline
		var single SummaryMetrics
		if err2 := json.Unmarshal(raw, &single); err2 != nil {
			return BaselineFile{}, err
		}
		return BaselineFile{CI: single, Full: single}, nil
	}
	return m, nil
}

func SaveBaseline(path string, b BaselineFile) error {
	raw, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func WriteReports(dir string, summary SummaryMetrics, results []CaseResult, gate GateResult) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	report := map[string]interface{}{
		"summary": summary,
		"gate":    gate,
		"cases":   results,
	}
	raw, _ := json.MarshalIndent(report, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "report.json"), raw, 0o644); err != nil {
		return err
	}
	md := FormatMarkdownReport(summary, gate)
	return os.WriteFile(filepath.Join(dir, "report.md"), []byte(md), 0o644)
}

func FormatMarkdownReport(summary SummaryMetrics, gate GateResult) string {
	var b strings.Builder
	b.WriteString("# VibeSense eval report\n\n")
	b.WriteString(fmt.Sprintf("- Generated: %s\n", summary.GeneratedAt))
	b.WriteString(fmt.Sprintf("- Mode: %s (vector=%v)\n", summary.Mode, summary.VectorOn))
	b.WriteString(fmt.Sprintf("- Cases: %d\n", summary.CaseCount))
	b.WriteString(fmt.Sprintf("- Resolve rate (mean): %.3f\n", summary.ResolveRateMean))
	b.WriteString(fmt.Sprintf("- Judge mood/crowd/variety/coherence: %.2f / %.2f / %.2f / %.2f\n",
		summary.MoodFitMean, summary.CrowdFitMean, summary.VarietyMean, summary.CoherenceMean))
	b.WriteString(fmt.Sprintf("- Latency p50/p95: %dms / %dms\n", summary.P50LatencyMs, summary.P95LatencyMs))
	if gate.Passed {
		b.WriteString("\n**Gate: PASSED**\n")
	} else {
		b.WriteString("\n**Gate: FAILED**\n")
		for _, f := range gate.Failures {
			b.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}
	return b.String()
}
