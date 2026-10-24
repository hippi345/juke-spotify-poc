package evals

import (
	"math"
)

// catalogBM25 is Okapi BM25 over stemmed terms (same corpus text as vector embeddings).
type catalogBM25 struct {
	terms   [][]string
	docLen  []float64
	avgDL   float64
	df      map[string]int
	nDocs   int
	k1      float64
	b       float64
}

func newCatalogBM25(docs []string) *catalogBM25 {
	terms := make([][]string, len(docs))
	df := make(map[string]int)
	var totalLen float64
	for i, d := range docs {
		t := catalogIndexTokens(d)
		terms[i] = t
		dl := float64(len(t))
		if dl == 0 {
			dl = 1
		}
		totalLen += dl
		seen := make(map[string]bool)
		for _, term := range t {
			if seen[term] {
				continue
			}
			seen[term] = true
			df[term]++
		}
	}
	n := len(docs)
	avg := 1.0
	if n > 0 {
		avg = totalLen / float64(n)
	}
	return &catalogBM25{
		terms:  terms,
		docLen: docLenSlice(terms),
		avgDL:  avg,
		df:     df,
		nDocs:  n,
		k1:     1.2,
		b:      0.75,
	}
}

func docLenSlice(terms [][]string) []float64 {
	out := make([]float64, len(terms))
	for i, t := range terms {
		l := float64(len(t))
		if l == 0 {
			l = 1
		}
		out[i] = l
	}
	return out
}

func (idx *catalogBM25) scoreQuery(query string, docID int) float64 {
	if docID < 0 || docID >= idx.nDocs {
		return 0
	}
	qTerms := catalogIndexTokens(query)
	if len(qTerms) == 0 {
		return 0
	}
	docTerms := idx.terms[docID]
	if len(docTerms) == 0 {
		return 0
	}
	tf := make(map[string]int)
	for _, t := range docTerms {
		tf[t]++
	}
	dl := idx.docLen[docID]
	var score float64
	for _, term := range qTerms {
		freq := tf[term]
		if freq == 0 {
			continue
		}
		dfi := idx.df[term]
		if dfi == 0 {
			continue
		}
		idf := math.Log(1 + (float64(idx.nDocs)-float64(dfi)+0.5)/(float64(dfi)+0.5))
		denom := float64(freq) + idx.k1*(1-idx.b+idx.b*dl/idx.avgDL)
		score += idf * (float64(freq) * (idx.k1 + 1)) / denom
	}
	return score
}
