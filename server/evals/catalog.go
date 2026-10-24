package evals

import "sort"

// Feature groups eval cases for reporting and README tables.
type Feature string

const (
	FeatureMood          Feature = "playlist_mood"
	FeatureVenueSize     Feature = "venue_size"
	FeatureGenre         Feature = "genre"
	FeatureCrowdVote     Feature = "crowd_vote"
	FeatureMoreLikeThis  Feature = "more_like_this"
	FeatureRAGGrounding  Feature = "rag_grounding"
	FeatureDegraded      Feature = "degraded_input"
	FeatureAdversarial   Feature = "adversarial_safety"
)

// CheckType describes which gates apply to a feature bucket in docs.
type CheckType string

const (
	CheckDeterministic   CheckType = "deterministic"
	CheckLLMJudge        CheckType = "llm_judge"
	CheckDeterministicLLM CheckType = "deterministic+llm_judge"
)

// EvalMode is mock vs live participation for a feature row.
type EvalMode string

const (
	ModeMock EvalMode = "mock"
	ModeLive EvalMode = "live"
	ModeBoth EvalMode = "mock+live"
)

// FeatureRow is one README table line (aggregated counts).
type FeatureRow struct {
	Feature    Feature
	CaseCount  int
	CheckType  CheckType
	Mode       EvalMode
}

// FeatureCatalog aggregates AllCases() into documentation rows.
func FeatureCatalog() []FeatureRow {
	counts := make(map[Feature]int)
	for _, c := range AllCases() {
		counts[c.Feature]++
	}
	var features []Feature
	for f := range counts {
		features = append(features, f)
	}
	sort.Slice(features, func(i, j int) bool { return features[i] < features[j] })
	var rows []FeatureRow
	for _, f := range features {
		rows = append(rows, FeatureRow{
			Feature:   f,
			CaseCount: counts[f],
			CheckType: checkTypeForFeature(f),
			Mode:      modeForFeature(f),
		})
	}
	return rows
}

func checkTypeForFeature(f Feature) CheckType {
	switch f {
	case FeatureDegraded, FeatureAdversarial:
		return CheckDeterministic
	case FeatureRAGGrounding, FeatureCrowdVote:
		return CheckDeterministic
	default:
		return CheckDeterministicLLM
	}
}

func modeForFeature(f Feature) EvalMode {
	switch f {
	case FeatureRAGGrounding, FeatureCrowdVote:
		return ModeBoth
	default:
		return ModeBoth
	}
}

func caseByID(id string) *Case {
	for _, c := range AllCases() {
		if c.ID == id {
			return &c
		}
	}
	return nil
}
