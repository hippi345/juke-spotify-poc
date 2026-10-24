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
)

func TestVibeSenseEvalLive(t *testing.T) {
	if os.Getenv("JUKE_EVALS_LIVE") != "1" && os.Getenv("GEMINI_API_KEY") == "" {
		t.Skip("live evals: set JUKE_EVALS_LIVE=1 and GEMINI_API_KEY")
	}
	cfg := config.Load()
	if cfg.GeminiAPIKey == "" {
		t.Skip("GEMINI_API_KEY not set")
	}
	vectorOn := os.Getenv("JUKE_EVALS_VECTOR") == "1"
	gen := gemini.NewHTTPGenerator(cfg.GeminiAPIKey, cfg.GeminiModel)
	var searcher TrackSearcher
	if spotifyResolveAvailable(cfg) {
		searcher = SpotifyClientAdapter{Client: spotify.NewClient(cfg)}
	}
	runCfg := RunConfig{
		Short:     false,
		VectorOn:  vectorOn,
		Generator: gen,
		Searcher:  searcher,
		Judge:     NewGeminiJudgeFromEnv(),
		VenueKeys: loadVenueHistoryKeys(),
	}
	if vectorOn {
		if err := SeedLiveEvalVenueIndex(cfg); err != nil {
			t.Fatalf("seed live venue index: %v", err)
		}
		runCfg.InjectVenueCandidates = true
	}
	runCfg = wrapLiveBudget(runCfg)
	results, err := RunSuite(runCfg)
	if err != nil {
		if msg := liveBudgetReport(); msg != "" {
			t.Fatalf("%v (%s)", err, msg)
		}
		t.Fatal(err)
	}
	summary := Summarize(results, "live", vectorOn)
	summary.JudgeErrorCount = 0
	bl, err := LoadBaseline(baselineFile())
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	th, err := LoadThresholds("")
	if err != nil {
		t.Fatalf("thresholds: %v", err)
	}
	gate := LiveGate(summary, results, bl, th)
	_ = WriteReports(reportsDir(), summary, results, gate)
	if msg := liveBudgetReport(); msg != "" {
		t.Log(msg)
	}
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
	if err := SeedLiveEvalVenueIndex(cfg); err != nil {
		t.Fatalf("seed live venue index: %v", err)
	}
	gen := gemini.NewHTTPGenerator(cfg.GeminiAPIKey, cfg.GeminiModel)
	var searcher TrackSearcher
	if spotifyResolveAvailable(cfg) {
		searcher = SpotifyClientAdapter{Client: spotify.NewClient(cfg)}
	}
	runCfg := wrapLiveBudget(RunConfig{
		Short:                 false,
		VectorOnly:            true,
		VectorOn:              true,
		InjectVenueCandidates: true,
		Generator:             gen,
		Searcher:              searcher,
		Judge:                 NewGeminiJudgeFromEnv(),
		VenueKeys:             loadVenueHistoryKeys(),
	})
	results, err := RunSuite(runCfg)
	if err != nil {
		if msg := liveBudgetReport(); msg != "" {
			t.Fatalf("%v (%s)", err, msg)
		}
		t.Fatal(err)
	}
	summary := Summarize(results, "live", true)
	summary.JudgeErrorCount = 0
	bl, err := LoadBaseline(baselineFile())
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	th, err := LoadThresholds("")
	if err != nil {
		t.Fatalf("thresholds: %v", err)
	}
	gate := LiveGate(summary, results, bl, th)
	_ = WriteReports(reportsDir(), summary, results, gate)
	if !gate.Passed {
		t.Fatalf("live vector gate failed: %v", gate.Failures)
	}
	t.Logf("live vector ON: cases=%d overlap=%.3f resolveNA=%v %s", summary.CaseCount, summary.VenueOverlapMean, summary.ResolveRateNA, liveBudgetReport())
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
