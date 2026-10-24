package evals

import (
	"fmt"

	"juke-spotify-poc/server/gemini"
	"juke-spotify-poc/server/spotify"
)

type budgetedGenerator struct {
	inner gemini.PlaylistGenerator
}

func (b budgetedGenerator) GenerateTrackList(description string, count int, similarArtists []string, opts gemini.GenerateOptions) ([]gemini.TrackSuggestion, error) {
	if err := consumeLiveCall(); err != nil {
		return nil, err
	}
	return b.inner.GenerateTrackList(description, count, similarArtists, opts)
}

type budgetedJudge struct {
	inner Judge
}

func (b budgetedJudge) Score(description string, suggestions []gemini.TrackSuggestion) (JudgeScores, error) {
	if err := consumeLiveCall(); err != nil {
		return JudgeScores{}, err
	}
	return b.inner.Score(description, suggestions)
}

type budgetedSearcher struct {
	inner TrackSearcher
}

func (b budgetedSearcher) SearchTracks(query string, limit int) ([]spotify.Track, error) {
	if err := consumeLiveCall(); err != nil {
		return nil, err
	}
	return b.inner.SearchTracks(query, limit)
}

func wrapLiveBudget(cfg RunConfig) RunConfig {
	if !liveCallsEnabled() {
		return cfg
	}
	resetLiveCallBudget()
	if cfg.Generator != nil {
		cfg.Generator = budgetedGenerator{inner: cfg.Generator}
	}
	if cfg.Judge != nil {
		cfg.Judge = budgetedJudge{inner: cfg.Judge}
	}
	if cfg.Searcher != nil {
		cfg.Searcher = budgetedSearcher{inner: cfg.Searcher}
	}
	return cfg
}

func liveBudgetReport() string {
	if liveBudget == nil {
		return ""
	}
	return fmt.Sprintf("live_api_calls=%d cap=%d", liveCallCount(), liveBudget.cap)
}
