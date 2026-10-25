package evals

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"juke-spotify-poc/server/gemini"
	"juke-spotify-poc/server/vector"
)

const judgeRunsPerCase = 3

func judgeRunsForSuite() int {
	if os.Getenv("JUKE_EVALS_LIVE") != "1" {
		return judgeRunsPerCase
	}
	if v := stringsTrimSpace(os.Getenv("JUKE_EVALS_JUDGE_RUNS")); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			return n
		}
	}
	return judgeRunsPerCase
}

func stringsTrimSpace(s string) string {
	return strings.TrimSpace(s)
}

// CaseResult is one case after generation, checks, and judging.
type CaseResult struct {
	CaseID          string
	Hard            HardCheckResult
	Judge           JudgeScores
	JudgeSpread     JudgeSpread
	JudgeErrorCount int // failed judge HTTP calls for this case (live)
	VectorOn        bool
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
	VectorOnly  bool
	VectorOn    bool
	InjectVenueCandidates bool // vector index ON: inject venue history candidates like AI playlist handler
	LiveOpenSearchCandidates bool // live vector arm: retrieve from OpenSearch (not offline catalog retriever)
	MockVenueCandidates   []gemini.TrackSuggestion // mocked vector inject (eval fixtures)
	Generator   gemini.PlaylistGenerator
	Searcher    TrackSearcher
	Judge       Judge
	VenueKeys   map[string]bool
	FixtureStore FixtureStore
}

// RunSuite executes all cases and returns aggregated results.
func RunSuite(cfg RunConfig) ([]CaseResult, error) {
	cases := FilterCases(cfg.Short)
	if cfg.VectorOnly {
		cases = FilterVectorCases()
	}
	return RunSuiteCases(cfg, cases)
}

// RunSuiteCases runs an explicit case list (used by example/doc tests).
func RunSuiteCases(cfg RunConfig, cases []Case) ([]CaseResult, error) {
	var out []CaseResult
	for _, c := range cases {
		SetRunCaseID(c.ID)
		start := time.Now()
		var suggestions []gemini.TrackSuggestion
		var genErr error
		genOpts := gemini.GenerateOptions{}
		if cfg.InjectVenueCandidates && c.VenueID > 0 {
			if cfg.LiveOpenSearchCandidates {
				var after time.Time
				if c.HistoryAfter != "" {
					after, _ = time.Parse(time.RFC3339, c.HistoryAfter)
				}
				genOpts.PreferredCandidates = vector.Default().CandidatesForMood(context.Background(), c.VenueID, c.Description, 0, after)
			} else if c.CanonicalRAG {
				genOpts.PreferredCandidates = CandidatesForCanonicalCase(c, 0)
			} else if len(cfg.MockVenueCandidates) > 0 {
				genOpts.PreferredCandidates = cfg.MockVenueCandidates
			} else {
				genOpts.PreferredCandidates = vector.Default().CandidatesForMood(context.Background(), c.VenueID, c.Description, 0, time.Time{})
			}
		}
		if c.ExpectError {
			_, genErr = cfg.Generator.GenerateTrackList(c.Description, c.TrackCount, c.SimilarArtists, genOpts)
		} else {
			suggestions, genErr = cfg.Generator.GenerateTrackList(c.Description, c.TrackCount, c.SimilarArtists, genOpts)
		}
		searcher := cfg.Searcher
		if c.ForceResolveNA {
			searcher = nil
		}
		if fs, ok := searcher.(FixtureSearcher); ok {
			fs.CaseID = c.ID
			searcher = fs
		}
		venueKeys := cfg.VenueKeys
		if c.CanonicalRAG {
			venueKeys = loadCanonicalHistoryKeys()
		}
		hard := RunHardChecksWithVector(c, suggestions, searcher, venueKeys, start, genErr, cfg.VectorOn, cfg.InjectVenueCandidates)

		var judgeRuns []JudgeScores
		judgeErrs := 0
		if !c.ExpectError && len(suggestions) > 0 && cfg.Judge != nil {
			for i := 0; i < judgeRunsForSuite(); i++ {
				sc, err := cfg.Judge.Score(c.Description, suggestions)
				if err != nil {
					judgeErrs++
					if liveCallsEnabled() {
						continue
					}
					return nil, fmt.Errorf("case %s judge run %d: %w", c.ID, i+1, err)
				}
				judgeRuns = append(judgeRuns, sc)
			}
		} else if c.ExpectError {
			judgeRuns = []JudgeScores{{MoodFit: 0, CrowdFit: 0, Variety: 0, Coherence: 0}}
		} else if genErr == nil && len(suggestions) > 0 {
			judgeRuns = []JudgeScores{{MoodFit: 4, CrowdFit: 4, Variety: 4, Coherence: 4}}
		}
		agg := AggregateJudgeRuns(judgeRuns)
		spread := judgeSpread(judgeRuns)
		out = append(out, CaseResult{
			CaseID:          c.ID,
			Hard:            hard,
			Judge:           agg,
			JudgeSpread:     spread,
			JudgeErrorCount: judgeErrs,
			VectorOn:        cfg.VectorOn,
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
