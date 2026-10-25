package evals

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"juke-spotify-poc/server/config"
	"juke-spotify-poc/server/vector"
)

// Live-path regression: OpenSearch must return candidates per vector-eval venue after seeding.
func TestLiveVector_CandidateInjectionPerVenue_OpenSearch(t *testing.T) {
	osURL := strings.TrimSpace(os.Getenv("OPENSEARCH_URL"))
	if osURL == "" {
		t.Skip("OPENSEARCH_URL not set")
	}
	cfg := &config.Config{
		OpenSearchURL: osURL,
		EmbedProvider: "hash",
	}
	if err := SeedLiveEvalVenueIndexes(cfg); err != nil {
		t.Fatalf("seed: %v", err)
	}
	refreshVenueTracksIndex(t, context.Background())
	svc := vector.Default()
	ctx := context.Background()
	for _, c := range FilterVectorCases() {
		if c.VenueID == 0 {
			continue
		}
		var after time.Time
		if c.HistoryAfter != "" {
			after, _ = time.Parse(time.RFC3339, c.HistoryAfter)
		}
		cands := svc.CandidatesForMood(ctx, c.VenueID, c.Description, 12, after)
		if len(cands) == 0 {
			t.Fatalf("case %s venue_id=%d: expected non-empty OpenSearch candidates", c.ID, c.VenueID)
		}
	}
}

func TestLiveEvalOpenSearchURL_defaultsForLive(t *testing.T) {
	t.Setenv("JUKE_EVALS_LIVE", "1")
	t.Setenv("OPENSEARCH_URL", "")
	cfg := &config.Config{}
	if LiveEvalOpenSearchURL(cfg) != defaultLiveOpenSearchURL {
		t.Fatalf("expected default %s", defaultLiveOpenSearchURL)
	}
}

func TestRequireLiveVectorOpenSearch(t *testing.T) {
	t.Setenv("JUKE_EVALS_VECTOR", "1")
	cfg := &config.Config{OpenSearchURL: "http://127.0.0.1:9200"}
	if err := RequireLiveVectorOpenSearch(cfg); err != nil {
		t.Fatal(err)
	}
}
