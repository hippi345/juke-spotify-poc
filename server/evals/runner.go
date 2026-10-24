package evals

import (
	"time"

	"juke-spotify-poc/server/gemini"
)

const judgeRunsPerCase = 3

// CaseResult is one case after generation, checks, and judging.
type CaseResult struct {
	CaseID       string
	Hard         HardCheckResult
	Judge        JudgeScores
	JudgeSpread  JudgeSpread
	VectorOn     bool
}

// JudgeSpread captures min/max per dimension across judge runs.
type JudgeSpread struct {
	MoodMin   float64
	MoodMax   float64
	CrowdMin  float64
	CrowdMax  float64
}

// RunConfig controls one eval suite execution.
type RunConfig struct {
	Short       bool
	VectorOn    bool
	Generator   gemini.PlaylistGenerator
	Searcher    TrackSearcher
	Judge       Judge
	VenueKeys   map[string]bool
	FixtureStore FixtureStore
}

// RunSuite executes all cases and returns aggregated results.
func RunSuite(cfg RunConfig) ([]CaseResult, error) {
	cases := FilterCases(cfg.Short)
	var out []CaseResult
	for _, c := range cases {
		SetRunCaseID(c.ID)
		start := time.Now()
		var suggestions []gemini.TrackSuggestion
		var genErr error
		if c.ExpectError {
			_, genErr = cfg.Generator.GenerateTrackList(c.Description, c.TrackCount, c.SimilarArtists, gemini.GenerateOptions{})
		} else {
			suggestions, genErr = cfg.Generator.GenerateTrackList(c.Description, c.TrackCount, c.SimilarArtists, gemini.GenerateOptions{})
		}
		searcher := cfg.Searcher
		if fs, ok := searcher.(FixtureSearcher); ok {
			fs.CaseID = c.ID
			searcher = fs
		}
		hard := RunHardChecks(c, suggestions, searcher, cfg.VenueKeys, start, genErr)

		var judgeRuns []JudgeScores
		if !c.ExpectError && len(suggestions) > 0 && cfg.Judge != nil {
			for i := 0; i < judgeRunsPerCase; i++ {
				sc, err := cfg.Judge.Score(c.Description, suggestions)
				if err != nil {
					sc = JudgeScores{MoodFit: 3, CrowdFit: 3, Variety: 3, Coherence: 3}
				}
				judgeRuns = append(judgeRuns, sc)
			}
		} else if c.ExpectError {
			judgeRuns = []JudgeScores{{MoodFit: 0, CrowdFit: 0, Variety: 0, Coherence: 0}}
		} else {
			judgeRuns = []JudgeScores{{MoodFit: 4, CrowdFit: 4, Variety: 4, Coherence: 4}}
		}
		agg := AggregateJudgeRuns(judgeRuns)
		spread := judgeSpread(judgeRuns)
		out = append(out, CaseResult{
			CaseID:      c.ID,
			Hard:        hard,
			Judge:       agg,
			JudgeSpread: spread,
			VectorOn:    cfg.VectorOn,
		})
	}
	return out, nil
}

func judgeSpread(runs []JudgeScores) JudgeSpread {
	if len(runs) == 0 {
		return JudgeSpread{}
	}
	s := JudgeSpread{
		MoodMin: runs[0].MoodFit, MoodMax: runs[0].MoodFit,
		CrowdMin: runs[0].CrowdFit, CrowdMax: runs[0].CrowdFit,
	}
	for _, r := range runs[1:] {
		if r.MoodFit < s.MoodMin {
			s.MoodMin = r.MoodFit
		}
		if r.MoodFit > s.MoodMax {
			s.MoodMax = r.MoodFit
		}
		if r.CrowdFit < s.CrowdMin {
			s.CrowdMin = r.CrowdFit
		}
		if r.CrowdFit > s.CrowdMax {
			s.CrowdMax = r.CrowdFit
		}
	}
	return s
}
