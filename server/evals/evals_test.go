package evals

import (
	"flag"
	"os"
	"testing"

	"juke-spotify-poc/server/gemini"
)

var (
	updateBaseline = flag.Bool("update-baseline", false, "rewrite baseline.json from current run")
	writeReports   = flag.Bool("write-reports", false, "write out/report.json and report.md")
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func mockRunConfig(short bool) RunConfig {
	store := DefaultFixtureStore()
	return RunConfig{
		Short:        short,
		Generator:    FixtureGenerator{Store: store},
		Searcher:     FixtureSearcher{Store: store},
		Judge:        &perCaseFixtureJudge{Store: store},
		FixtureStore: store,
	}
}

type perCaseFixtureJudge struct {
	Store   FixtureStore
	callMap map[string]int
}

func (j *perCaseFixtureJudge) Score(description string, suggestions []gemini.TrackSuggestion) (JudgeScores, error) {
	id := currentCaseID
	runs := LoadJudgeFixture(j.Store, id)
	if len(runs) == 0 {
		return JudgeScores{MoodFit: 4.2, CrowdFit: 4.1, Variety: 4.0, Coherence: 4.3, MoodReason: "default mock judge"}, nil
	}
	if j.callMap == nil {
		j.callMap = make(map[string]int)
	}
	call := j.callMap[id]
	j.callMap[id] = call + 1
	run := runs[0]
	if len(run) == 0 {
		return JudgeScores{MoodFit: 4, CrowdFit: 4, Variety: 4, Coherence: 4}, nil
	}
	return run[call%len(run)], nil
}

// TestVibeSenseEvalGateCI is the required CI subset (-short).
func TestVibeSenseEvalGateCI(t *testing.T) {
	runGate(t, true)
}

// TestVibeSenseEvalGate runs the full mocked matrix (local/PR validation).
func TestVibeSenseEvalGate(t *testing.T) {
	if testing.Short() {
		t.Skip("full suite skipped in -short")
	}
	runGate(t, false)
}

func runGate(t *testing.T, short bool) {
	cfg := mockRunConfig(short)
	results, err := RunSuite(cfg)
	if err != nil {
		t.Fatal(err)
	}
	summary := Summarize(results, "mocked", false)
	baselinePath := baselineFile()
	bl, err := LoadBaseline(baselinePath)
	if err != nil && !*updateBaseline {
		t.Fatalf("load baseline: %v (run with -update-baseline once)", err)
	}
	if *updateBaseline {
		if short {
			bl.CI = summary
		} else {
			bl.Full = summary
		}
		if err := SaveBaseline(baselinePath, bl); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated baseline (%s) at %s", map[bool]string{true: "ci", false: "full"}[short], baselinePath)
	}
	ref := bl.Full
	if short {
		ref = bl.CI
	}
	gate := CompareBaseline(summary, ref)
	if *writeReports || os.Getenv("JUKE_EVALS_WRITE_REPORTS") == "1" {
		_ = WriteReports(reportsDir(), summary, results, gate)
	}
	if !gate.Passed {
		t.Fatalf("eval gate failed: %v", gate.Failures)
	}
}
