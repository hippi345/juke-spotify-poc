package evals

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"juke-spotify-poc/server/gemini"
)

// JudgeScores are LLM-as-judge dimensions (1–5).
type JudgeScores struct {
	MoodFit    float64 `json:"mood_fit"`
	CrowdFit   float64 `json:"crowd_fit"`
	Variety    float64 `json:"variety"`
	Coherence  float64 `json:"coherence"`
	MoodReason string  `json:"mood_reason"`
}

// JudgeVerdict is one judge call with strict JSON fields.
type JudgeVerdict struct {
	MoodFit     int    `json:"mood_fit"`
	CrowdFit    int    `json:"crowd_fit"`
	Variety     int    `json:"variety"`
	Coherence   int    `json:"coherence"`
	MoodReason  string `json:"mood_reason"`
	CrowdReason string `json:"crowd_reason"`
	VarietyReason string `json:"variety_reason"`
	CoherenceReason string `json:"coherence_reason"`
}

// Judge runs mood/crowd/variety/coherence scoring.
type Judge interface {
	Score(description string, suggestions []gemini.TrackSuggestion) (JudgeScores, error)
}

// FixtureJudge returns canned scores from testdata.
type FixtureJudge struct {
	Runs [][]JudgeScores
	Call int
}

func (f *FixtureJudge) Score(description string, suggestions []gemini.TrackSuggestion) (JudgeScores, error) {
	if len(f.Runs) == 0 {
		return JudgeScores{MoodFit: 4, CrowdFit: 4, Variety: 4, Coherence: 4, MoodReason: "fixture default"}, nil
	}
	idx := f.Call % len(f.Runs)
	f.Call++
	if len(f.Runs[idx]) == 0 {
		return JudgeScores{MoodFit: 4, CrowdFit: 4, Variety: 4, Coherence: 4}, nil
	}
	runIdx := (f.Call - 1) % len(f.Runs[idx])
	return f.Runs[idx][runIdx], nil
}

// GeminiJudge calls Gemini at temperature 0.
type GeminiJudge struct {
	APIKey string
	Model  string
	Client *http.Client
}

func NewGeminiJudgeFromEnv() *GeminiJudge {
	model := strings.TrimSpace(os.Getenv("EVAL_JUDGE_MODEL"))
	if model == "" {
		model = strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	}
	if model == "" {
		model = "gemini-2.5-flash"
	}
	return &GeminiJudge{
		APIKey: strings.TrimSpace(os.Getenv("GEMINI_API_KEY")),
		Model:  model,
		Client: &http.Client{Timeout: 90 * time.Second},
	}
}

func (j *GeminiJudge) Score(description string, suggestions []gemini.TrackSuggestion) (JudgeScores, error) {
	if j.APIKey == "" {
		return JudgeScores{}, fmt.Errorf("GEMINI_API_KEY not set for judge")
	}
	var trackLines []string
	for _, s := range suggestions {
		trackLines = append(trackLines, fmt.Sprintf("- %s — %s", s.Title, s.Artist))
	}
	prompt := fmt.Sprintf(`You are evaluating an AI-generated playlist for a venue jukebox.

Playlist brief:
%s

Tracks:
%s

Score each dimension 1-5 (integers). Return ONLY JSON:
{"mood_fit":3,"crowd_fit":3,"variety":3,"coherence":3,"mood_reason":"...","crowd_reason":"...","variety_reason":"...","coherence_reason":"..."}
`, description, strings.Join(trackLines, "\n"))

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]string{{"text": prompt}}},
		},
		"generationConfig": map[string]interface{}{
			"temperature":      0,
			"responseMimeType": "application/json",
		},
	}
	raw, _ := json.Marshal(body)
	u := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", j.Model, url.QueryEscape(j.APIKey))
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return JudgeScores{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := j.Client.Do(req)
	if err != nil {
		return JudgeScores{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return JudgeScores{}, fmt.Errorf("judge gemini HTTP %d: %s", resp.StatusCode, truncate(respBody, 200))
	}
	text, err := extractGeminiText(respBody)
	if err != nil {
		return JudgeScores{}, err
	}
	var v JudgeVerdict
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return JudgeScores{}, fmt.Errorf("judge parse: %w", err)
	}
	return JudgeScores{
		MoodFit:    float64(v.MoodFit),
		CrowdFit:   float64(v.CrowdFit),
		Variety:    float64(v.Variety),
		Coherence:  float64(v.Coherence),
		MoodReason: v.MoodReason,
	}, nil
}

func extractGeminiText(respBody []byte) (string, error) {
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty judge response")
	}
	return strings.TrimSpace(parsed.Candidates[0].Content.Parts[0].Text), nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}

// AggregateJudgeRuns computes mean scores over N judge calls.
func AggregateJudgeRuns(runs []JudgeScores) JudgeScores {
	if len(runs) == 0 {
		return JudgeScores{}
	}
	var m JudgeScores
	for _, r := range runs {
		m.MoodFit += r.MoodFit
		m.CrowdFit += r.CrowdFit
		m.Variety += r.Variety
		m.Coherence += r.Coherence
	}
	n := float64(len(runs))
	m.MoodFit /= n
	m.CrowdFit /= n
	m.Variety /= n
	m.Coherence /= n
	return m
}
