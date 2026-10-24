//go:build evals_live

package evals

import (
	"os"
	"path/filepath"
	"testing"

	"juke-spotify-poc/server/gemini"
	"juke-spotify-poc/server/spotify"
	"juke-spotify-poc/server/config"
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
	if sc := spotify.NewClient(cfg); sc != nil {
		searcher = SpotifyClientAdapter{Client: sc}
	}
	judge := NewGeminiJudgeFromEnv()
	runCfg := RunConfig{
		Short:     false,
		Generator: gen,
		Searcher:  searcher,
		Judge:     judge,
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
