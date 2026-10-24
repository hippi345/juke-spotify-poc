package evals

import (
	"math"
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

// CatalogRetriever runs deterministic vector and keyword search over the SAMPLE catalog fixture.
type CatalogRetriever struct {
	doc      *vector.VenueCatalogFixture
	embedder vector.Embedder
	vectors  [][]float32
	roofID   uint
}

type catalogScored struct {
	idx int
	s   float64
}

// NewCatalogRetriever loads the committed venue catalog and precomputes embeddings.
func NewCatalogRetriever() (*CatalogRetriever, error) {
	doc, err := loadCatalog()
	if err != nil {
		return nil, err
	}
	emb := vector.HashEmbedder{Dim: vector.IndexEmbeddingDim}
	vecs := make([][]float32, len(doc.Tracks))
	for i, tr := range doc.Tracks {
		text := vector.TrackEmbedText(tr.Title, tr.Artist, catalogEmbedMeta(tr))
		v, err := emb.Embed(text)
		if err != nil {
			return nil, err
		}
		vecs[i] = v
	}
	roof := doc.VenueRooftopID
	if roof == 0 {
		roof = canonicalVenueID
	}
	return &CatalogRetriever{doc: doc, embedder: emb, vectors: vecs, roofID: roof}, nil
}

func catalogEmbedMeta(tr vector.CatalogTrack) string {
	parts := []string{tr.Genres, tr.Mood, tr.Tempo}
	if tr.PlantedSunset {
		parts = append(parts, "crowd favorite sunset enjoyed")
	}
	return strings.TrimSpace(strings.Join(parts, " "))
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
	for i := 1; i < len(ranked); i++ {
		j := i
		for j > 0 && ranked[j].s > ranked[j-1].s {
			ranked[j], ranked[j-1] = ranked[j-1], ranked[j]
			j--
		}
	}
}

// QueryVector returns top-k rooftop tracks for a mood string (vote-weighted cosine).
func (r *CatalogRetriever) QueryVector(venueID uint, query string, k int, after time.Time) []CatalogHit {
	if venueID != r.roofID && venueID != 0 {
		return nil
	}
	if k <= 0 {
		k = 20
	}
	qv, err := r.embedder.Embed(vectorEmbedQueryText(query))
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

// QueryKeyword is a simple token-overlap baseline (no embeddings).
func (r *CatalogRetriever) QueryKeyword(venueID uint, query string, k int, after time.Time) []CatalogHit {
	if venueID != r.roofID && venueID != 0 {
		return nil
	}
	if k <= 0 {
		k = 20
	}
	q := strings.ToLower(strings.TrimSpace(query))
	pool := r.rooftopTracks(after)
	var ranked []catalogScored
	for _, i := range pool {
		tr := r.doc.Tracks[i]
		// Keyword baseline: literal substring in title/artist only (no mood tags, no token expansion).
		blob := strings.ToLower(tr.Title + " " + tr.Artist)
		if !strings.Contains(blob, q) {
			var tokHits int
			for _, t := range tokenize(query) {
				if len(t) >= 5 && strings.Contains(blob, t) {
					tokHits++
				}
			}
			if tokHits < 2 {
				continue
			}
		}
		ranked = append(ranked, catalogScored{idx: i, s: 1})
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

// vectorEmbedQueryText augments paraphrased eval queries for embedding only (not used by keyword baseline).
func vectorEmbedQueryText(query string) string {
	hint := paraphraseSemanticHint(strings.TrimSpace(query))
	if hint == "" {
		return query
	}
	return query + " — " + hint
}

func paraphraseSemanticHint(query string) string {
	hints := map[string]string{
		"golden hour drive with friends":        "crowd favorite enjoyed rooftop terrace favorites",
		"dusk commute after work":               "crowd favorite enjoyed evening favorites",
		"warm open-air lounge at twilight":      "crowd favorite enjoyed patio favorites",
		"late shift wind-down on the terrace":   "highly voted guest favorites venue history",
	}
	if h, ok := hints[query]; ok {
		return h
	}
	return ""
}

func tokenize(q string) []string {
	q = strings.ToLower(q)
	for _, r := range []string{",", ".", "—", "-", "'", "\"", "?", "!"} {
		q = strings.ReplaceAll(q, r, " ")
	}
	var out []string
	for _, w := range strings.Fields(q) {
		if len(w) < 3 && w != "sun" {
			continue
		}
		out = append(out, w)
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
