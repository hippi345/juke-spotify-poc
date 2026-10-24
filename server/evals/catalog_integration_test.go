package evals

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"juke-spotify-poc/server/vector"
)

const (
	catalogRecallMin    = 0.35
	catalogPrecisionMin = 0.20
	catalogP95MaxMs     = 100
)

func TestCatalogVectorRetrievalIntegration(t *testing.T) {
	ret, err := NewCatalogRetriever()
	if err != nil {
		t.Fatal(err)
	}
	query := "sunset vibes people enjoyed at this rooftop bar"
	truth := loadPlantedSunsetKeys()
	if len(truth) == 0 {
		t.Fatal("no planted sunset keys")
	}

	const k = 20
	var vecLat []int64
	var vecHits []CatalogHit
	for i := 0; i < 50; i++ {
		start := time.Now()
		vecHits = ret.QueryVector(canonicalVenueID, query, k, time.Time{})
		vecLat = append(vecLat, time.Since(start).Milliseconds())
	}
	kwHits := ret.QueryKeyword(canonicalVenueID, query, k, time.Time{})
	recall, prec := RecallPrecisionAtK(vecHits, truth, k)
	kwRecall, _ := RecallPrecisionAtK(kwHits, truth, k)

	if recall < catalogRecallMin || prec < catalogPrecisionMin {
		t.Fatalf("vector recall@20=%.3f precision@20=%.3f (min %.2f / %.2f)", recall, prec, catalogRecallMin, catalogPrecisionMin)
	}
	kwPrec, _ := RecallPrecisionAtK(kwHits, truth, k)
	vecVoteAvg := avgVoteCount(vecHits)
	kwVoteAvg := avgVoteCount(kwHits)
	if recall < kwRecall || prec < kwPrec || vecVoteAvg <= kwVoteAvg {
		t.Fatalf("vector should beat keyword: recall %.3f/%.3f prec %.3f/%.3f vote_avg %.0f/%.0f",
			recall, kwRecall, prec, kwPrec, vecVoteAvg, kwVoteAvg)
	}

	// Venue scoping: other venue returns nothing.
	if len(ret.QueryVector(99, query, k, time.Time{})) != 0 {
		t.Fatal("expected no hits for venue_id=99")
	}

	// Vote weighting: top hit should be high-vote planted sunset.
	if len(vecHits) == 0 || vecHits[0].VoteCount < 100 {
		t.Fatalf("expected vote-weighted top hit, got %+v", vecHits[0])
	}

	sort.Slice(vecLat, func(i, j int) bool { return vecLat[i] < vecLat[j] })
	p95 := vecLat[len(vecLat)*95/100]
	if p95 > catalogP95MaxMs {
		t.Fatalf("p95 latency %dms exceeds %dms", p95, catalogP95MaxMs)
	}
	t.Logf("catalog retrieval: recall@20=%.3f precision@20=%.3f kw_recall@20=%.3f p95=%dms", recall, prec, kwRecall, p95)

	if os.Getenv("OPENSEARCH_URL") != "" {
		testCatalogOpenSearchRetrieval(t, query, truth)
	}
}

func testCatalogOpenSearchRetrieval(t *testing.T, query string, truth map[string]bool) {
	if err := vector.Init(true, os.Getenv("OPENSEARCH_URL"), "", ""); err != nil {
		t.Fatalf("vector init: %v", err)
	}
	ctx := context.Background()
	svc := vector.Default()
	if err := svc.Indexer.EnsureIndex(ctx, vector.EmbedDimension(svc.Embedder)); err != nil {
		t.Fatalf("ensure index: %v", err)
	}
	doc, err := loadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	roof := doc.VenueRooftopID
	if roof == 0 {
		roof = canonicalVenueID
	}
	// Index a subset for CI speed (high-vote + planted).
	indexed := 0
	for _, tr := range doc.Tracks {
		if tr.VenueID != roof {
			continue
		}
		if !tr.PlantedSunset && tr.VoteCount < 200 {
			continue
		}
		text := vector.TrackEmbedText(tr.Title, tr.Artist, tr.Genres+" "+tr.Mood)
		vec, err := svc.Embedder.Embed(text)
		if err != nil {
			t.Fatal(err)
		}
		docID := "eval-" + tr.SpotifyID
		vdoc := vector.VenueTrackDoc{
			VenueID:   roof,
			SpotifyID: tr.SpotifyID,
			Title:     tr.Title,
			Artist:    tr.Artist,
			Genres:    tr.Genres,
			VoteCount: tr.VoteCount,
			Embedding: vec,
		}
		if err := svc.Indexer.Upsert(ctx, docID, vdoc); err != nil {
			t.Fatalf("upsert: %v", err)
		}
		indexed++
	}
	if indexed < 50 {
		t.Fatalf("indexed only %d docs", indexed)
	}
	hits, err := svc.Indexer.SearchByText(ctx, roof, query, svc.Embedder, 20)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	var found int
	for _, h := range hits {
		if truth[normalizeKey(h.Title, h.Artist)] {
			found++
		}
	}
	if found < 3 {
		t.Fatalf("opensearch path: only %d planted hits in top 20", found)
	}
	t.Logf("opensearch retrieval: %d planted in top 20 (indexed %d docs)", found, indexed)
}
