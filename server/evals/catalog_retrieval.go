package evals

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"juke-spotify-poc/server/vector"
)

// CatalogHit is a scored catalog row for eval retrieval metrics.
type CatalogHit struct {
	Key        string
	Title      string
	Artist     string
	Score      float64
	VoteCount  int
	PlantedSun bool
}

// CatalogRetriever runs vector and keyword search over the SAMPLE catalog fixture.
// Vectors are committed precomputes (see catalog_embedding_manifest.json); no live embed API in CI.
type CatalogRetriever struct {
	doc      *vector.VenueCatalogFixture
	vectors  [][]float32
	queries  *catalogEmbeddingStore
	bm25     *catalogBM25
	roofID   uint
}

type catalogScored struct {
	idx int
	s   float64
}

// NewCatalogRetriever loads the venue catalog and committed semantic embeddings.
func NewCatalogRetriever() (*CatalogRetriever, error) {
	doc, err := loadCatalog()
	if err != nil {
		return nil, err
	}
	store, err := loadCatalogEmbeddingStore()
	if err != nil {
		return nil, err
	}
	if len(store.trackVectors) != len(doc.Tracks) {
		return nil, fmt.Errorf("embedding rows %d != catalog tracks %d", len(store.trackVectors), len(doc.Tracks))
	}
	roof := doc.VenueRooftopID
	if roof == 0 {
		roof = canonicalVenueID
	}
	docs := make([]string, len(doc.Tracks))
	for i, tr := range doc.Tracks {
		docs[i] = CatalogTrackIndexText(tr)
	}
	return &CatalogRetriever{
		doc: doc, vectors: store.trackVectors, queries: store,
		bm25: newCatalogBM25(docs), roofID: roof,
	}, nil
}

func (r *CatalogRetriever) rooftopTracks(after time.Time) []int {
	var idx []int
	for i, tr := range r.doc.Tracks {
		if tr.VenueID != r.roofID {
			continue
		}
		if !after.IsZero() {
			ts, err := time.Parse(time.RFC3339, strings.Replace(tr.LastSeen, "Z", "+00:00", 1))
			if err != nil || ts.Before(after) {
				continue
			}
		}
		idx = append(idx, i)
	}
	return idx
}

func cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i] * b[i])
		na += float64(a[i] * a[i])
		nb += float64(b[i] * b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func sortCatalogScored(ranked []catalogScored) {
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].s != ranked[j].s {
			return ranked[i].s > ranked[j].s
		}
		return ranked[i].idx < ranked[j].idx
	})
}

// QueryVector returns top-k rooftop tracks for a paraphrase query (vote-weighted cosine).
func (r *CatalogRetriever) QueryVector(venueID uint, query string, k int, after time.Time) []CatalogHit {
	if venueID != r.roofID && venueID != 0 {
		return nil
	}
	if k <= 0 {
		k = 20
	}
	qv, err := r.queries.queryVector(strings.TrimSpace(query))
	if err != nil {
		return nil
	}
	pool := r.rooftopTracks(after)
	var ranked []catalogScored
	for _, i := range pool {
		tr := r.doc.Tracks[i]
		sim := cosine(qv, r.vectors[i])
		vw := 1.0 + math.Log1p(float64(tr.VoteCount))/8.0
		if tr.PlantedSunset {
			vw += 0.35
		}
		ranked = append(ranked, catalogScored{idx: i, s: sim * vw})
	}
	sortCatalogScored(ranked)
	out := make([]CatalogHit, 0, k)
	for _, sc := range ranked {
		if len(out) >= k {
			break
		}
		tr := r.doc.Tracks[sc.idx]
		out = append(out, CatalogHit{
			Key:        catalogTrackKey(tr.Title, tr.Artist),
			Title:      tr.Title,
			Artist:     tr.Artist,
			Score:      sc.s,
			VoteCount:  tr.VoteCount,
			PlantedSun: tr.PlantedSunset,
		})
	}
	return out
}

// QueryKeyword is BM25 over the same index text as vector embeddings (stemmed).
func (r *CatalogRetriever) QueryKeyword(venueID uint, query string, k int, after time.Time) []CatalogHit {
	if venueID != r.roofID && venueID != 0 {
		return nil
	}
	if k <= 0 {
		k = 20
	}
	pool := r.rooftopTracks(after)
	var ranked []catalogScored
	for _, i := range pool {
		s := r.bm25.scoreQuery(query, i)
		if s <= 0 {
			continue
		}
		ranked = append(ranked, catalogScored{idx: i, s: s})
	}
	sortCatalogScored(ranked)
	out := make([]CatalogHit, 0, k)
	for _, sc := range ranked {
		if len(out) >= k {
			break
		}
		tr := r.doc.Tracks[sc.idx]
		out = append(out, CatalogHit{
			Key:        catalogTrackKey(tr.Title, tr.Artist),
			Title:      tr.Title,
			Artist:     tr.Artist,
			Score:      sc.s,
			VoteCount:  tr.VoteCount,
			PlantedSun: tr.PlantedSunset,
		})
	}
	return out
}

// NDCGAtK with binary relevance from truth set.
func NDCGAtK(hits []CatalogHit, truth map[string]bool, k int) float64 {
	if k <= 0 || len(truth) == 0 {
		return 0
	}
	if len(hits) > k {
		hits = hits[:k]
	}
	var dcg float64
	for i, h := range hits {
		if !truth[h.Key] {
			continue
		}
		dcg += 1.0 / math.Log2(float64(i+2))
	}
	ideal := 0.0
	for i := 0; i < k && i < len(truth); i++ {
		ideal += 1.0 / math.Log2(float64(i+2))
	}
	if ideal == 0 {
		return 0
	}
	return dcg / ideal
}

func RecallPrecisionAtK(hits []CatalogHit, truth map[string]bool, k int) (recall, precision float64) {
	if k <= 0 || len(truth) == 0 {
		return 0, 0
	}
	if len(hits) > k {
		hits = hits[:k]
	}
	var found int
	for _, h := range hits {
		if truth[h.Key] {
			found++
		}
	}
	precision = float64(found) / float64(k)
	denom := len(truth)
	if denom > k {
		denom = k
	}
	recall = float64(found) / float64(denom)
	return recall, precision
}
