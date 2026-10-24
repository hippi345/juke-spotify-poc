//go:build evals_live

package evals

import (
	"os"
	"path/filepath"
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
	scanLiveReportsForLeaks(t)
}

func TestVibeSenseEvalLiveVectorOn(t *testing.T) {
	if os.Getenv("JUKE_EVALS_LIVE") != "1" && os.Getenv("GEMINI_API_KEY") == "" {
		t.Skip("live evals: set JUKE_EVALS_LIVE=1 and GEMINI_API_KEY")
	}
	if os.Getenv("JUKE_EVALS_VECTOR") != "1" {
		t.Skip("vector live arm: set JUKE_EVALS_VECTOR=1")
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
	_ = vector.Init(true, cfg.OpenSearchURL, cfg.GeminiAPIKey, cfg.EmbedProvider)
	runCfg := RunConfig{
		Short:                 false,
		VectorOnly:            true,
		VectorOn:              true,
		InjectVenueCandidates: true,
		Generator:             gen,
		Searcher:              searcher,
		Judge:                 NewGeminiJudgeFromEnv(),
		VenueKeys:             loadVenueHistoryKeys(),
	}
	results, err := RunSuite(runCfg)
	if err != nil {
		t.Fatal(err)
	}
	summary := Summarize(results, "live", true)
	t.Logf("live vector ON: cases=%d overlap=%.3f resolveNA=%v", summary.CaseCount, summary.VenueOverlapMean, summary.ResolveRateNA)
	scanLiveReportsForLeaks(t)
}

func scanLiveReportsForLeaks(t *testing.T) {
	dir := reportsDir()
	for _, name := range []string{"report.json", "report.md"} {
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if err := AssertNoSecretLeaks(string(raw)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func spotifyResolveAvailable(cfg *config.Config) bool {
	return strings.TrimSpace(cfg.SpotifyClientID) != "" && strings.TrimSpace(cfg.SpotifyClientSecret) != ""
}
