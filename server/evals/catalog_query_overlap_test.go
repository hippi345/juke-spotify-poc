package evals

import (
	"testing"

	"juke-spotify-poc/server/vector"
)

func TestPlantedSunsetGroundTruthNotFromEmbeddings(t *testing.T) {
	doc, err := loadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	var flagged int
	for _, tr := range doc.Tracks {
		if tr.PlantedSunset {
			flagged++
		}
	}
	if flagged != doc.PlantedSunsetCount || len(doc.PlantedSunsetKeys) != doc.PlantedSunsetCount {
		t.Fatalf("planted counts: tracks=%d keys=%d expected=%d", flagged, len(doc.PlantedSunsetKeys), doc.PlantedSunsetCount)
	}
	t.Logf("planted sunset ground truth: %d keys from catalog generator (independent of embedding model)", len(doc.PlantedSunsetKeys))
}

func TestCatalogEvalQueriesNoStemOverlapWithPlanted(t *testing.T) {
	doc, err := loadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	var planted []vector.CatalogTrack
	for _, tr := range doc.Tracks {
		if tr.PlantedSunset {
			planted = append(planted, tr)
		}
	}
	if len(planted) == 0 {
		t.Fatal("no planted tracks")
	}
	for _, query := range catalogEvalQueries {
		qTok := tokenSetFromIndexTokens(query) // same pipeline as catalogIndexTokens / BM25
		for _, tr := range planted {
			docTok := tokenSetFromIndexTokens(CatalogTrackIndexText(tr))
			for tok := range qTok {
				if docTok[tok] {
					t.Fatalf("query %q shares stemmed token %q with planted track %q — %q",
						query, tok, tr.Title, CatalogTrackIndexText(tr))
				}
			}
		}
	}
}
