package evals

import (
	"testing"
	"time"

	"juke-spotify-poc/server/gemini"
)

func TestLiveGateVectorOn_failsSchemaNotRubric(t *testing.T) {
	c := Case{ID: "ambient_spa", Description: "ambient spa", TrackCount: 10}
	suggestions := []gemini.TrackSuggestion{{Title: "", Artist: "A"}}
	hard := RunHardChecks(c, suggestions, nil, nil, time.Now(), nil)
	results := []CaseResult{{CaseID: c.ID, Hard: hard, Judge: JudgeScores{MoodFit: 1}}}
	g := LiveGateVectorOn(results)
	if g.Passed {
		t.Fatal("expected schema failure on vector ON arm")
	}
}
