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
		jRaw, _ := json.MarshalIndent(judge, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, "judge.json"), jRaw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func slug(s string) string {
	return strings.ReplaceAll(s, "_", "")
}
