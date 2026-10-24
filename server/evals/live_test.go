//go:build evals_live

package evals

import (
	"os"
	"strings"
	"testing"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/gemini"
	"juke-spotify-poc/server/spotify"
	"juke-spotify-poc/server/vector"
)

func TestVibeSenseEvalLive(t *testing.T) {
	if os.Getenv("JUKE_EVALS_LIVE") != "1" && os.Getenv("GEMINI_API_KEY") == "" {
		t.Skip("live evals: set JUKE_EVALS_LIVE=1 and GEMINI_API_KEY")
	}
	cfg := config.Load()
	if cfg.GeminiAPIKey == "" {
		t.Skip("GEMINI_API_KEY not set")
	}
	gen := gemini.NewHTTPGenerator(cfg.GeminiAPIKey, cfg.GeminiModel)
	var searcher TrackSearcher
	if spotifyResolveAvailable(cfg) {
		searcher = SpotifyClientAdapter{Client: spotify.NewClient(cfg)}
	}
	judge := NewGeminiJudgeFromEnv()
	runCfg := RunConfig{
		Short:     false,
		Generator: gen,
		Searcher:  searcher,
		Judge:     judge,
	}
	if os.Getenv("JUKE_EVALS_VECTOR") == "1" {
		_ = vector.Init(true, cfg.OpenSearchURL, cfg.GeminiAPIKey, cfg.EmbedProvider)
		runCfg.InjectVenueCandidates = true
	}
	results, err := RunSuite(runCfg)
	if err != nil {
		t.Fatal(err)
	}
	summary := Summarize(results, "live", false)
	bl, err := LoadBaseline(baselineFile())
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	gate := CompareBaseline(summary, bl.Full)
	_ = WriteReports(reportsDir(), summary, results, gate)
	if !gate.Passed {
		t.Fatalf("live gate failed: %v", gate.Failures)
	}
}

func spotifyResolveAvailable(cfg *config.Config) bool {
	return strings.TrimSpace(cfg.SpotifyClientID) != "" && strings.TrimSpace(cfg.SpotifyClientSecret) != ""
}
