package evals

import (
	"fmt"
	"strings"
	"time"

	"juke-spotify-poc/server/gemini"
	"juke-spotify-poc/server/spotify"
)

// HardCheckResult captures deterministic quality gates for one case run.
type HardCheckResult struct {
	JSONOK              bool
	CountOK             bool
	NoDuplicates        bool
	RequestedCount      int
	ActualCount         int
	ResolveRate         float64
	SimilarArtistRate   float64
	LatencyMs           int64
	ErrorClean          bool
	ErrorMessage        string
	ResolvedFromVenue   float64 // share of resolved tracks matching venue history (vector eval)
}

func normalizeKey(title, artist string) string {
	return strings.ToLower(strings.TrimSpace(title) + "|" + strings.TrimSpace(artist))
}

func fuzzyMatchSuggestion(s gemini.TrackSuggestion, tracks []spotify.Track) bool {
	wantT := strings.ToLower(strings.TrimSpace(s.Title))
	wantA := strings.ToLower(strings.TrimSpace(s.Artist))
	for _, t := range tracks {
		if trackMatches(wantT, wantA, t) {
			return true
		}
	}
	return false
}

func primaryArtist(t spotify.Track) string {
	if len(t.Artists) > 0 {
		return t.Artists[0].Name
	}
	return ""
}

func trackMatches(wantTitle, wantArtist string, t spotify.Track) bool {
	title := strings.ToLower(strings.TrimSpace(t.Name))
	artist := strings.ToLower(strings.TrimSpace(primaryArtist(t)))
	if title == wantTitle && artist == wantArtist {
		return true
	}
	if title == wantTitle && strings.Contains(artist, wantArtist) {
		return true
	}
	if strings.Contains(title, wantTitle) && strings.Contains(artist, wantArtist) {
		return true
	}
	return false
}

func similarArtistAdherence(suggestions []gemini.TrackSuggestion, artists []string) float64 {
	if len(artists) == 0 || len(suggestions) == 0 {
		return 1
	}
	var want []string
	for _, a := range artists {
		w := strings.ToLower(strings.TrimSpace(a))
		if w != "" {
			want = append(want, w)
		}
	}
	if len(want) == 0 {
		return 1
	}
	var hits int
	for _, s := range suggestions {
		art := strings.ToLower(strings.TrimSpace(s.Artist))
		for _, w := range want {
			if art == w || strings.Contains(art, w) || strings.Contains(w, art) {
				hits++
				break
			}
		}
	}
	return float64(hits) / float64(len(suggestions))
}

// RunHardChecks validates suggestions and optional Spotify resolution.
func RunHardChecks(c Case, suggestions []gemini.TrackSuggestion, search TrackSearcher, venueTrackKeys map[string]bool, started time.Time, genErr error) HardCheckResult {
	res := HardCheckResult{
		RequestedCount: c.TrackCount,
		LatencyMs:      time.Since(started).Milliseconds(),
	}
	if c.ExpectError {
		res.ErrorClean = genErr != nil
		if genErr != nil {
			res.ErrorMessage = genErr.Error()
		}
		res.JSONOK = true
		res.CountOK = true
		res.NoDuplicates = true
		return res
	}
	if genErr != nil {
		res.ErrorMessage = genErr.Error()
		return res
	}
	res.JSONOK = len(suggestions) > 0
	res.ActualCount = len(suggestions)
	res.CountOK = res.ActualCount == c.TrackCount
	seen := make(map[string]bool)
	res.NoDuplicates = true
	for _, s := range suggestions {
		k := normalizeKey(s.Title, s.Artist)
		if seen[k] {
			res.NoDuplicates = false
			break
		}
		seen[k] = true
	}

	if search == nil {
		res.ResolveRate = 1
		res.SimilarArtistRate = similarArtistAdherence(suggestions, c.SimilarArtists)
		return res
	}

	var resolved int
	var venueHits int
	for _, s := range suggestions {
		q := fmt.Sprintf("%s %s", s.Artist, s.Title)
		tracks, err := search.SearchTracks(q, 5)
		if err != nil || len(tracks) == 0 {
			continue
		}
		if fuzzyMatchSuggestion(s, tracks) {
			resolved++
			k := normalizeKey(s.Title, s.Artist)
			if venueTrackKeys != nil && venueTrackKeys[k] {
				venueHits++
			}
		}
	}
	if len(suggestions) > 0 {
		res.ResolveRate = float64(resolved) / float64(len(suggestions))
		if resolved > 0 && venueTrackKeys != nil {
			res.ResolvedFromVenue = float64(venueHits) / float64(resolved)
		}
	}
	res.SimilarArtistRate = similarArtistAdherence(suggestions, c.SimilarArtists)
	return res
}
