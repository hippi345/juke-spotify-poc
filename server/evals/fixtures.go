package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"juke-spotify-poc/server/gemini"
	"juke-spotify-poc/server/spotify"
)

// FixtureStore loads recorded Gemini/Spotify/judge data from testdata/.
type FixtureStore struct {
	Root string
}

func DefaultFixtureStore() FixtureStore {
	return FixtureStore{Root: fixtureRoot()}
}

// FixtureGenerator returns suggestions from gemini.json.
type FixtureGenerator struct {
	Store FixtureStore
}

func (g FixtureGenerator) GenerateTrackList(description string, count int, similarArtists []string, opts gemini.GenerateOptions) ([]gemini.TrackSuggestion, error) {
	id := caseIDFromDescription(description, count, similarArtists)
	if id == "" {
		return nil, fmt.Errorf("description required")
	}
	if strings.TrimSpace(description) == "" || description == "   " {
		return nil, fmt.Errorf("description required")
	}
	if description == "asdf qwerty zxcv" {
		return nil, fmt.Errorf("nonsense prompt rejected")
	}
	path := g.Store.Root + "/cases/" + id + "/gemini.json"
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("fixture %s: %w", id, err)
	}
	var tracks []gemini.TrackSuggestion
	if err := json.Unmarshal(raw, &tracks); err != nil {
		return nil, err
	}
	return tracks, nil
}

// FixtureSearcher resolves via spotify.json map keyed by "artist title".
type FixtureSearcher struct {
	Store  FixtureStore
	CaseID string
}

func (s FixtureSearcher) SearchTracks(query string, limit int) ([]spotify.Track, error) {
	path := s.Store.Root + "/cases/" + s.CaseID + "/spotify.json"
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	var m map[string][]spotify.Track
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if tracks, ok := m[q]; ok {
		if limit > 0 && len(tracks) > limit {
			return tracks[:limit], nil
		}
		return tracks, nil
	}
	for k, tracks := range m {
		if strings.Contains(q, strings.ToLower(k)) || strings.Contains(strings.ToLower(k), q) {
			if limit > 0 && len(tracks) > limit {
				return tracks[:limit], nil
			}
			return tracks, nil
		}
	}
	return nil, nil
}

func LoadJudgeFixture(store FixtureStore, caseID string) [][]JudgeScores {
	path := store.Root + "/cases/" + caseID + "/judge.json"
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var runs [][]JudgeScores
	if err := json.Unmarshal(raw, &runs); err != nil {
		var flat []JudgeScores
		if json.Unmarshal(raw, &flat) == nil {
			return [][]JudgeScores{flat, flat, flat}
		}
		return nil
	}
	return runs
}

// caseIDFromDescription is set per-run via RunContext; FixtureGenerator uses global run context.
var currentCaseID string

func SetRunCaseID(id string) { currentCaseID = id }

func caseIDFromDescription(description string, count int, similarArtists []string) string {
	if currentCaseID != "" {
		return currentCaseID
	}
	for _, c := range AllCases() {
		if c.Description == description && c.TrackCount == count {
			if len(c.SimilarArtists) != len(similarArtists) {
				continue
			}
			match := true
			for i := range c.SimilarArtists {
				if c.SimilarArtists[i] != similarArtists[i] {
					match = false
					break
				}
			}
			if match {
				return c.ID
			}
		}
	}
	return ""
}
