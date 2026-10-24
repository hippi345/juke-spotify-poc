package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"juke-spotify-poc/server/gemini"
	"juke-spotify-poc/server/spotify"
)

// TestGenerateFixtures writes testdata for all cases (dev only: go test ./evals -run TestGenerateFixtures).
func TestGenerateFixtures(t *testing.T) {
	if os.Getenv("JUKE_EVALS_GEN_FIXTURES") != "1" {
		t.Skip("set JUKE_EVALS_GEN_FIXTURES=1 to regenerate fixtures")
	}
	store := DefaultFixtureStore()
	for _, c := range AllCases() {
		dir := filepath.Join(store.Root, "cases", c.ID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if c.ExpectError {
			continue
		}
		tracks := make([]gemini.TrackSuggestion, 0, c.TrackCount)
		spotifyMap := make(map[string][]spotify.Track)
		for i := 0; i < c.TrackCount; i++ {
			title := fmt.Sprintf("Track %d", i+1)
			artist := fmt.Sprintf("Artist %s %d", slug(c.ID), (i%5)+1)
			tracks = append(tracks, gemini.TrackSuggestion{Title: title, Artist: artist})
			q := strings.ToLower(fmt.Sprintf("%s %s", artist, title))
			spotifyMap[q] = []spotify.Track{{
				ID:      fmt.Sprintf("id-%s-%d", c.ID, i),
				Name:    title,
				Artists: []spotify.Artist{{Name: artist}},
				URI:     fmt.Sprintf("spotify:track:id-%s-%d", c.ID, i),
			}}
		}
		geminiRaw, _ := json.MarshalIndent(tracks, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, "gemini.json"), geminiRaw, 0o644); err != nil {
			t.Fatal(err)
		}
		spRaw, _ := json.MarshalIndent(spotifyMap, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, "spotify.json"), spRaw, 0o644); err != nil {
			t.Fatal(err)
		}
		judge := [][]JudgeScores{
			{
				{MoodFit: 4.2, CrowdFit: 4.1, Variety: 4.0, Coherence: 4.2},
				{MoodFit: 4.3, CrowdFit: 4.0, Variety: 4.1, Coherence: 4.1},
				{MoodFit: 4.1, CrowdFit: 4.2, Variety: 3.9, Coherence: 4.3},
			},
		}
		if !c.MoodRubric {
			for i := range judge[0] {
				judge[0][i].MoodFit = 0
				judge[0][i].MoodReason = "n/a"
			}
		}
		jRaw, _ := json.MarshalIndent(judge, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, "judge.json"), jRaw, 0o644); err != nil {
			t.Fatal(err)
		}
		if c.VectorEval {
			writeVectorFixture(t, dir, c, tracks)
		}
	}
	writeMockVenueCandidates(t, store)
}

func writeVectorFixture(t *testing.T, dir string, c Case, offTracks []gemini.TrackSuggestion) {
	vectorTracks := make([]gemini.TrackSuggestion, len(offTracks))
	prefix, artist := vectorTheme(c.ID)
	for i := range offTracks {
		vectorTracks[i] = gemini.TrackSuggestion{
			Title:  fmt.Sprintf("%s %d", prefix, i+1),
			Artist: artist,
		}
	}
	geminiRaw, _ := json.MarshalIndent(vectorTracks, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "gemini_vector.json"), geminiRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	spotifyMap := make(map[string][]spotify.Track)
	for i, tr := range vectorTracks {
		q := strings.ToLower(fmt.Sprintf("%s %s", tr.Artist, tr.Title))
		spotifyMap[q] = []spotify.Track{{
			ID:      fmt.Sprintf("vec-%s-%d", c.ID, i),
			Name:    tr.Title,
			Artists: []spotify.Artist{{Name: tr.Artist}},
			URI:     fmt.Sprintf("spotify:track:vec-%s-%d", c.ID, i),
		}}
	}
	spPath := filepath.Join(dir, "spotify.json")
	var existing map[string][]spotify.Track
	if raw, err := os.ReadFile(spPath); err == nil {
		_ = json.Unmarshal(raw, &existing)
	}
	if existing == nil {
		existing = make(map[string][]spotify.Track)
	}
	for k, v := range spotifyMap {
		existing[k] = v
	}
	spRaw, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(spPath, spRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	judge := [][]JudgeScores{
		{
			{MoodFit: 4.4, CrowdFit: 4.5, Variety: 4.0, Coherence: 4.2},
			{MoodFit: 4.3, CrowdFit: 4.4, Variety: 4.1, Coherence: 4.1},
			{MoodFit: 4.5, CrowdFit: 4.3, Variety: 4.0, Coherence: 4.3},
		},
	}
	jRaw, _ := json.MarshalIndent(judge, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "judge_vector.json"), jRaw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func vectorTheme(id string) (prefix, artist string) {
	switch {
	case strings.Contains(id, "country") || strings.Contains(id, "honky"):
		return "Honky Tonk", "Nashville Crowd"
	case strings.Contains(id, "edm") || strings.Contains(id, "house"):
		return "House Banger", "DJ Regular"
	case strings.Contains(id, "rock"):
		return "Classic Rock", "Crowd Favorite"
	default:
		return "Lofi Study", "Venue Regular"
	}
}

func writeMockVenueCandidates(t *testing.T, store FixtureStore) {
	var keys []string
	var candidates []gemini.TrackSuggestion
	for _, c := range AllCases() {
		if !c.VectorEval {
			continue
		}
		prefix, artist := vectorTheme(c.ID)
		for i := 1; i <= 10; i++ {
			title := fmt.Sprintf("%s %d", prefix, i)
			keys = append(keys, strings.ToLower(title+"|"+artist))
			candidates = append(candidates, gemini.TrackSuggestion{Title: title, Artist: artist})
		}
	}
	keysPath := filepath.Join(store.Root, "vector", "venue_history_keys.json")
	keyRaw, _ := json.MarshalIndent(keys, "", "  ")
	if err := os.WriteFile(keysPath, keyRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	candPath := filepath.Join(store.Root, "vector", "mock_venue_candidates.json")
	candRaw, _ := json.MarshalIndent(candidates, "", "  ")
	if err := os.WriteFile(candPath, candRaw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func slug(s string) string {
	return strings.ReplaceAll(s, "_", "")
}
