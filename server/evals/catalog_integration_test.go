package evals

import (
	"context"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"juke-spotify-poc/server/vector"
)

const (
	catalogRecallMin    = 0.35
	catalogPrecisionMin = 0.20
	catalogRecallMargin = 0.12
	catalogNDCGMargin   = 0.10
)

// Paraphrased queries: no catalog mood/tag tokens (sunset, nu-disco, chill, etc.).
var catalogParaphraseQueries = []string{
	"golden hour drive with friends",
	"dusk commute after work",
	"warm open-air lounge at twilight",
	"late shift wind-down on the terrace",
}

func TestCatalogVectorRetrievalIntegration(t *testing.T) {
	ret, err := NewCatalogRetriever()
	if err != nil {
		t.Fatal(err)
	}
	truth := loadPlantedSunsetKeys()
	if len(truth) == 0 {
		t.Fatal("no planted sunset keys")
	}

	const k = 20
	var vecRecallSum, kwRecallSum, vecNDCGSum, kwNDCGSum float64

	for _, query := range catalogParaphraseQueries {
		assertNoTagLexicalOverlap(t, query)

		var vecLat []int64
		var vecHits []CatalogHit
		for i := 0; i < 30; i++ {
			start := time.Now()
			vecHits = ret.QueryVector(canonicalVenueID, query, k, time.Time{})
			vecLat = append(vecLat, time.Since(start).Milliseconds())
		}
		kwHits := ret.QueryKeyword(canonicalVenueID, query, k, time.Time{})

		vRec, vPrec := RecallPrecisionAtK(vecHits, truth, k)
		kRec, _ := RecallPrecisionAtK(kwHits, truth, k)
		vNDCG := NDCGAtK(vecHits, truth, k)
		kNDCG := NDCGAtK(kwHits, truth, k)

		if vRec < catalogRecallMin || vPrec < catalogPrecisionMin {
			t.Fatalf("query %q: vector recall@20=%.3f precision@20=%.3f", query, vRec, vPrec)
		}
		if vRec-kRec < catalogRecallMargin && vNDCG-kNDCG < catalogNDCGMargin {
			t.Fatalf("query %q: vector must beat keyword (recall Δ=%.3f need %.2f, nDCG Δ=%.3f need %.2f)",
				query, vRec-kRec, catalogRecallMargin, vNDCG-kNDCG, catalogNDCGMargin)
		}

		vecRecallSum += vRec
		kwRecallSum += kRec
		vecNDCGSum += vNDCG
		kwNDCGSum += kNDCG

		sort.Slice(vecLat, func(i, j int) bool { return vecLat[i] < vecLat[j] })
		p95 := vecLat[len(vecLat)*95/100]
		if p95 > catalogP95MaxMs {
			t.Fatalf("query %q: p95 latency %dms exceeds %dms", query, p95, catalogP95MaxMs)
		}
	}

	nq := float64(len(catalogParaphraseQueries))
	t.Logf("catalog retrieval avg: vec_recall@20=%.3f kw=%.3f vec_nDCG@20=%.3f kw=%.3f",
		vecRecallSum/nq, kwRecallSum/nq, vecNDCGSum/nq, kwNDCGSum/nq)

	testCatalogVoteWeighting(t, ret)
	testCatalogVenueScope(t, ret)

	if os.Getenv("OPENSEARCH_URL") != "" {
		testCatalogOpenSearchRetrieval(t, catalogParaphraseQueries[0], truth)
	}
}

func assertNoTagLexicalOverlap(t *testing.T, query string) {
	banned := []string{"sunset", "nu-disco", "nu disco", "chill", "house", "lofi", "disco", "rooftop", "patio", "brunch"}
	q := strings.ToLower(query)
	for _, w := range banned {
		if strings.Contains(q, w) {
			t.Fatalf("query %q overlaps banned tag token %q", query, w)
		}
	}
}

func testCatalogVoteWeighting(t *testing.T, ret *CatalogRetriever) {
	query := "golden hour drive with friends"
	hits := ret.QueryVector(canonicalVenueID, query, 20, time.Time{})
	var planted []CatalogHit
	for _, h := range hits {
		if h.PlantedSun {
			planted = append(planted, h)
		}
	}
	if len(planted) < 2 {
		t.Fatalf("need 2+ planted hits for vote test, got %d", len(planted))
	}
	if planted[0].VoteCount < planted[1].VoteCount {
		t.Fatalf("vote weighting: top planted vote %d < second %d", planted[0].VoteCount, planted[1].VoteCount)
	}
}

func testCatalogVenueScope(t *testing.T, ret *CatalogRetriever) {
	query := catalogParaphraseQueries[0]
	if len(ret.QueryVector(99, query, 20, time.Time{})) != 0 {
		t.Fatal("expected no hits for venue_id=99")
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
	indexed := 0
	for _, tr := range doc.Tracks {
		if tr.VenueID != roof {
			continue
		}
		if !tr.PlantedSunset && tr.VoteCount < 200 {
			continue
		}
		text := vector.TrackEmbedText(tr.Title, tr.Artist, catalogEmbedMeta(tr))
		vec, err := svc.Embedder.Embed(text)
		if err != nil {
			t.Fatal(err)
		}
		docID := "eval-" + tr.SpotifyID
		lastSeen := tr.LastSeen
		if lastSeen == "" {
			lastSeen = "2025-08-01T00:00:00Z"
		}
		vdoc := vector.VenueTrackDoc{
			VenueID:   roof,
			SpotifyID: tr.SpotifyID,
			Title:     tr.Title,
			Artist:    tr.Artist,
			Genres:    tr.Genres,
			VoteCount: tr.VoteCount,
			LastSeen:  lastSeen,
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
	refreshVenueTracksIndex(t, ctx)
	embedQ := vectorEmbedQueryText(query)
	hits, err := svc.Indexer.SearchByText(ctx, roof, embedQ, svc.Embedder, 20)
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

func refreshVenueTracksIndex(t *testing.T, ctx context.Context) {
	base := strings.TrimRight(os.Getenv("OPENSEARCH_URL"), "/")
	if base == "" {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/venue_tracks/_refresh", nil)
	if err != nil {
		t.Fatalf("refresh request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("refresh status %d: %s", resp.StatusCode, string(b))
	}
}
