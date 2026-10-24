package evals

import (
	"strings"
	"time"

	"juke-spotify-poc/server/gemini"
	"juke-spotify-poc/server/vector"
)

const canonicalVenueID = 2

func loadCatalog() (*vector.VenueCatalogFixture, error) {
	return vector.LoadVenueCatalogFixture(vector.DefaultCatalogFixturePath())
}

func loadCanonicalHighlyVotedKeys() map[string]bool {
	doc, err := loadCatalog()
	if err != nil {
		return nil
	}
	out := make(map[string]bool, len(doc.HighlyVotedKeys))
	for _, k := range doc.HighlyVotedKeys {
		out[k] = true
	}
	return out
}

func loadCanonicalHistoryKeys() map[string]bool {
	doc, err := loadCatalog()
	if err != nil {
		return nil
	}
	out := make(map[string]bool, len(doc.Tracks))
	roof := doc.VenueRooftopID
	if roof == 0 {
		roof = canonicalVenueID
	}
	for _, tr := range doc.Tracks {
		if tr.VenueID != roof {
			continue
		}
		out[normalizeKey(tr.Title, tr.Artist)] = true
	}
	return out
}

func loadPlantedSunsetKeys() map[string]bool {
	doc, err := loadCatalog()
	if err != nil {
		return nil
	}
	out := make(map[string]bool, len(doc.PlantedSunsetKeys))
	for _, k := range doc.PlantedSunsetKeys {
		out[k] = true
	}
	return out
}

// CandidatesForCanonicalCase returns mock/live candidate injection for canonical RAG cases.
func CandidatesForCanonicalCase(c Case, limit int) []gemini.TrackSuggestion {
	if limit <= 0 {
		limit = 20
	}
	ret, err := NewCatalogRetriever()
	if err != nil {
		return nil
	}
	var after time.Time
	if c.HistoryAfter != "" {
		after, _ = time.Parse(time.RFC3339, c.HistoryAfter)
	}
	hits := ret.QueryVector(canonicalVenueID, c.Description, limit*3, after)
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]gemini.TrackSuggestion, 0, len(hits))
	for _, h := range hits {
		out = append(out, gemini.TrackSuggestion{Title: h.Title, Artist: h.Artist})
	}
	return out
}

func catalogTrackKey(title, artist string) string {
	return strings.ToLower(strings.TrimSpace(title) + "|" + strings.TrimSpace(artist))
}

func highlyVotedShare(suggestions []gemini.TrackSuggestion, keys map[string]bool) float64 {
	if len(suggestions) == 0 || len(keys) == 0 {
		return 0
	}
	var hits int
	for _, s := range suggestions {
		if keys[normalizeKey(s.Title, s.Artist)] {
			hits++
		}
	}
	return float64(hits) / float64(len(suggestions))
}
