//go:build evals_live

package evals

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/db"
	"juke-spotify-poc/server/gemini"
	"juke-spotify-poc/server/spotify"
)

// TestVibeSenseEvalLive is the full live suite with vector index OFF (default product path).
func TestVibeSenseEvalLive(t *testing.T) {
	if os.Getenv("JUKE_EVALS_LIVE") != "1" && os.Getenv("GEMINI_API_KEY") == "" {
		t.Skip("live evals: set JUKE_EVALS_LIVE=1 and GEMINI_API_KEY")
	}
	if os.Getenv("JUKE_EVALS_VECTOR") == "1" {
		t.Skip("vector OFF arm: unset JUKE_EVALS_VECTOR")
	}
	cfg := config.Load()
	if cfg.GeminiAPIKey == "" {
		t.Skip("GEMINI_API_KEY not set")
	}
	cfg.OpenSearchURL = LiveEvalOpenSearchURL(cfg)
	if !spotifyResolveAvailable(cfg) {
		t.Fatal("live evals require SPOTIFY_CLIENT_ID and SPOTIFY_CLIENT_SECRET (resolve check must not be silently skipped)")
	}
	requireLiveEvalDatabase(t, cfg)
	gen := gemini.NewHTTPGenerator(cfg.GeminiAPIKey, cfg.GeminiModel)
	searcher := SpotifyClientAdapter{Client: spotify.NewClient(cfg)}
	runCfg := wrapLiveBudget(RunConfig{
		Short:     false,
		VectorOn:  false,
		Generator: gen,
		Searcher:  searcher,
		Judge:     NewGeminiJudgeFromEnv(),
		VenueKeys: loadVenueHistoryKeys(),
	})
	results, err := RunSuite(runCfg)
	if err != nil {
		if msg := liveBudgetReport(); msg != "" {
			t.Fatalf("%v (%s)", err, msg)
		}
		t.Fatal(err)
	}
	summary := Summarize(results, "live", false)
	bl, err := LoadBaseline(baselineFile())
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	th, err := LoadThresholds("")
	if err != nil {
		t.Fatalf("thresholds: %v", err)
	}
	gate := LiveGateOff(summary, results, bl, th)
	_ = WriteReports(reportsDir(), summary, results, gate)
	if msg := liveBudgetReport(); msg != "" {
		t.Log(msg)
	}
	if !gate.Passed {
		t.Fatalf("live gate (vector OFF) failed: %v", gate.Failures)
	}
	scanLiveReportsForLeaks(t)
}

// TestVibeSenseEvalLiveVectorOn is the full live suite with vector index ON (informational rubrics).
func TestVibeSenseEvalLiveVectorOn(t *testing.T) {
	if os.Getenv("JUKE_EVALS_LIVE") != "1" && os.Getenv("GEMINI_API_KEY") == "" {
		t.Skip("live evals: set JUKE_EVALS_LIVE=1 and GEMINI_API_KEY")
	}
	if os.Getenv("JUKE_EVALS_VECTOR") != "1" {
		t.Skip("vector ON arm: set JUKE_EVALS_VECTOR=1")
	}
	cfg := config.Load()
	if cfg.GeminiAPIKey == "" {
		t.Skip("GEMINI_API_KEY not set")
	}
	cfg.OpenSearchURL = LiveEvalOpenSearchURL(cfg)
	if err := RequireLiveVectorOpenSearch(cfg); err != nil {
		t.Fatal(err)
	}
	if !spotifyResolveAvailable(cfg) {
		t.Fatal("live evals require SPOTIFY_CLIENT_ID and SPOTIFY_CLIENT_SECRET (resolve check must not be silently skipped)")
	}
	requireLiveEvalDatabase(t, cfg)
	if err := SeedLiveEvalVenueIndex(cfg); err != nil {
		t.Fatalf("live venue index seed failed: %v", err)
	}
	gen := gemini.NewHTTPGenerator(cfg.GeminiAPIKey, cfg.GeminiModel)
	searcher := SpotifyClientAdapter{Client: spotify.NewClient(cfg)}
	runCfg := wrapLiveBudget(RunConfig{
		Short:                    false,
		VectorOn:                 true,
		InjectVenueCandidates:    true,
		LiveOpenSearchCandidates: true,
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
	gate := LiveGateVectorOn(results)
	_ = WriteReports(reportsDir(), summary, results, gate)
	if !gate.Passed {
		t.Fatalf("live vector ON hard gate failed: %v", gate.Failures)
	}
	t.Logf("live vector ON: cases=%d overlap=%.3f venue_share=%.3f %s",
		summary.CaseCount, summary.VenueOverlapMean, summary.VenueHistoryShare, liveBudgetReport())
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

func requireLiveEvalDatabase(t *testing.T, cfg *config.Config) {
	t.Helper()
	if db.DB != nil {
		return
	}
	if err := db.Connect(cfg); err != nil {
		t.Fatalf("live evals require MySQL (Spotify account storage): %v", err)
	}
	if db.DB == nil {
		t.Fatal("live evals require MySQL (Spotify account storage): database not initialized")
	}
}
