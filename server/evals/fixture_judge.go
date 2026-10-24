package evals

import "juke-spotify-poc/server/gemini"

// perCaseFixtureJudge loads judge scores from per-case fixtures (mocked eval runs).
type perCaseFixtureJudge struct {
	Store    FixtureStore
	vectorOn bool
	callMap  map[string]int
}

func (j *perCaseFixtureJudge) Score(description string, suggestions []gemini.TrackSuggestion) (JudgeScores, error) {
	id := currentCaseID
	runs := LoadJudgeFixture(j.Store, id, j.vectorOn)
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
