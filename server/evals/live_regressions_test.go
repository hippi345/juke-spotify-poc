package evals

import (
	"fmt"
	"testing"
	"time"

	"juke-spotify-poc/server/gemini"
)

func TestLiveGateOff_usesLiveBaselineNotMockedHardPass(t *testing.T) {
	bl, err := LoadBaseline(baselineFile())
	if err != nil {
		t.Fatal(err)
	}
	if bl.LiveFull.HardPassCount != 53 {
		t.Fatalf("expected live_full hard_pass 53, got %d", bl.LiveFull.HardPassCount)
	}
	// Mocked full baseline is 58/58; live run at 37/58 must fail live ref, not pass mocked aggregate only.
	summary := SummaryMetrics{
		Mode:          "live",
		HardPassCount: 37,
		ResolveRateMean: 0.76,
		P50LatencyMs:  5917,
	}
	gate := CompareLiveAggregate(summary, bl.LiveFull)
	if gate.Passed {
		t.Fatal("expected live aggregate failure when hard pass below live ref")
	}
	mocked := CompareBaseline(summary, bl.Full, nil)
	if mocked.Passed {
		t.Fatal("mocked baseline should also fail hard pass 37 vs 58")
	}
}

func TestCompareThresholds_skipsMoodFloorForMoreLikeThis(t *testing.T) {
	th, err := LoadThresholds("")
	if err != nil {
		t.Fatal(err)
	}
	results := []CaseResult{{
		CaseID: "similar_artists_radiohead",
		Judge:  JudgeScores{MoodFit: 0, CrowdFit: 4, Variety: 4, Coherence: 4},
	}}
	summary := Summarize(results, "live", false)
	gate := CompareThresholds(summary, results, SummaryMetrics{}, th)
	for _, f := range gate.Failures {
		if f == "" {
			continue
		}
		if f == "case similar_artists_radiohead: mood/intent 0.00 below floor 2.00" {
			t.Fatalf("mood floor should not apply to more-like-this cases: %v", gate.Failures)
		}
	}
}

func TestSummarize_excludesMoreLikeThisFromMoodMean(t *testing.T) {
	results := []CaseResult{
		{CaseID: "rainy_sunday_brunch", Judge: JudgeScores{MoodFit: 4, CrowdFit: 4, Variety: 4, Coherence: 4}},
		{CaseID: "similar_artists_radiohead", Judge: JudgeScores{MoodFit: 0, CrowdFit: 4, Variety: 4, Coherence: 4}},
	}
	s := Summarize(results, "mocked", false)
	if s.MoodFitMean != 4.0 {
		t.Fatalf("expected mood mean 4.0 excluding more-like-this, got %.3f", s.MoodFitMean)
	}
}

func TestRunHardChecks_generationErrorMarksToolSequenceOK(t *testing.T) {
	c := Case{ID: "count_max", Description: "x", TrackCount: 100}
	hard := RunHardChecks(c, nil, nil, nil, time.Now(), fmt.Errorf("parse tracks: not a valid JSON array"))
	if !hard.ToolSequenceOK {
		t.Fatal("expected tool sequence OK when generator was invoked")
	}
	if !hard.GroundingOK {
		t.Fatal("non-vector case should not fail grounding on gen error")
	}
	if hard.ErrorMessage == "" {
		t.Fatal("expected generation error message")
	}
}

func TestRunner_liveJudgeErrorsDoNotAbortSuite(t *testing.T) {
	t.Setenv("JUKE_EVALS_LIVE", "1")
	t.Setenv("JUKE_EVALS_JUDGE_RUNS", "1")
	failJudge := stubFailJudge{}
	okGen := stubGen{}
	cfg := RunConfig{
		Generator: okGen,
		Searcher:  nil,
		Judge:     failJudge,
	}
	cases := []Case{{ID: "count_two", Description: "two songs", TrackCount: 2, Feature: FeatureMood, MoodRubric: true, CI: true}}
	results, err := RunSuiteCases(cfg, cases)
	if err != nil {
		t.Fatalf("live judge errors should not abort suite: %v", err)
	}
	if len(results) != 1 || results[0].JudgeErrorCount != 1 {
		t.Fatalf("expected one judge error, got %+v", results[0])
	}
}

type stubFailJudge struct{}

func (stubFailJudge) Score(string, []gemini.TrackSuggestion) (JudgeScores, error) {
	return JudgeScores{}, fmt.Errorf("context deadline exceeded")
}
