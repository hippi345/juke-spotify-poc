package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// TrackSuggestion is one track from the model (title + primary artist).
type TrackSuggestion struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

const generateURL = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"

const maxGeminiRetries = 4

// GenerateTrackList asks Gemini for exactly `count` real songs matching the description and optional artist inspirations.
func GenerateTrackList(apiKey, model string, description string, count int, similarArtists []string) ([]TrackSuggestion, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is not set")
	}
	if strings.TrimSpace(description) == "" {
		return nil, fmt.Errorf("description required")
	}
	if count <= 0 {
		return nil, fmt.Errorf("count must be positive")
	}
	if model == "" {
		model = "gemini-2.5-flash"
	}

	var artistsLine string
	if len(similarArtists) > 0 {
		var cleaned []string
		for _, a := range similarArtists {
			s := strings.TrimSpace(a)
			if s != "" {
				cleaned = append(cleaned, s)
			}
		}
		if len(cleaned) > 0 {
			artistsLine = "Inspiration artists (vibe only; do not only pick songs by these names unless they fit): " + strings.Join(cleaned, ", ") + ".\n"
		}
	}

	prompt := fmt.Sprintf(`You are a music curator. Suggest exactly %d distinct songs that fit the playlist description.

Playlist description:
%s
%s
Rules:
- Return ONLY a JSON array (no markdown fences, no commentary). Each element must be: {"title":"Song Title","artist":"Primary Artist Name"}.
- Use well-known tracks that are likely on Spotify. Vary artists; avoid duplicate songs.
- "artist" is the main credited performer or band name as commonly shown on Spotify.
`, count, strings.TrimSpace(description), artistsLine)

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":      0.9,
			"responseMimeType": "application/json",
			"maxOutputTokens":  4096,
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	u := fmt.Sprintf(generateURL+"?key=%s", model, url.QueryEscape(apiKey))
	client := &http.Client{Timeout: 120 * time.Second}

	var respBody []byte
	var status int
	for attempt := 0; attempt < maxGeminiRetries; attempt++ {
		req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		respBody, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		status = resp.StatusCode

		if status == http.StatusOK {
			return decodeGenerateResponse(respBody)
		}

		if status == http.StatusTooManyRequests && attempt < maxGeminiRetries-1 {
			delay := retryDelayFromGeminiError(respBody)
			log.Printf("gemini: HTTP 429, backing off %v before retry %d/%d", delay, attempt+2, maxGeminiRetries)
			time.Sleep(delay)
			continue
		}

		return nil, friendlyGeminiError(status, respBody)
	}
	return nil, fmt.Errorf("gemini: exhausted retries without a successful response")
}

func retryDelayFromGeminiError(body []byte) time.Duration {
	var root struct {
		Error struct {
			Details []struct {
				Type       string `json:"@type"`
				RetryDelay string `json:"retryDelay"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &root); err != nil {
		return 55 * time.Second
	}
	for _, d := range root.Error.Details {
		if strings.Contains(d.Type, "RetryInfo") && d.RetryDelay != "" {
			if dur, err := time.ParseDuration(d.RetryDelay); err == nil && dur > 0 {
				if dur > 120*time.Second {
					return 120 * time.Second
				}
				if dur < 5*time.Second {
					return 5 * time.Second
				}
				return dur
			}
		}
	}
	return 55 * time.Second
}

func friendlyGeminiError(status int, body []byte) error {
	s := string(body)
	if status == http.StatusNotFound {
		if strings.Contains(s, "NOT_FOUND") || strings.Contains(s, "not found") {
			return fmt.Errorf("Gemini model not found (HTTP 404). Set GEMINI_MODEL to a current model id (default: gemini-2.5-flash). See https://ai.google.dev/gemini-api/docs/models")
		}
	}
	if status == http.StatusTooManyRequests {
		if strings.Contains(s, "quota") || strings.Contains(s, "RESOURCE_EXHAUSTED") || strings.Contains(s, "Quota exceeded") {
			return fmt.Errorf("Gemini quota or rate limit (HTTP 429). Wait and retry, or try GEMINI_MODEL=gemini-2.5-flash-lite, and check usage: https://ai.google.dev/gemini-api/docs/rate-limits")
		}
		return fmt.Errorf("Gemini rate limited (HTTP 429). Retry shortly or change GEMINI_MODEL. Details: %s", truncateErrBody(s))
	}
	return fmt.Errorf("gemini: HTTP %d %s", status, truncateErrBody(s))
}

func truncateErrBody(s string) string {
	const max = 400
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func decodeGenerateResponse(respBody []byte) ([]TrackSuggestion, error) {
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
		return nil, fmt.Errorf("gemini response: %w", err)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini: empty candidates")
	}
	text := strings.TrimSpace(parsed.Candidates[0].Content.Parts[0].Text)
	tracks, err := parseTrackJSONArray(text)
	if err != nil {
		return nil, err
	}
	if len(tracks) == 0 {
		return nil, fmt.Errorf("gemini: no tracks in response")
	}
	return tracks, nil
}

func parseTrackJSONArray(text string) ([]TrackSuggestion, error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(text)), "json") {
			text = strings.TrimSpace(text[4:])
		}
		if idx := strings.LastIndex(text, "```"); idx >= 0 {
			text = strings.TrimSpace(text[:idx])
		}
	}
	var out []TrackSuggestion
	if err := json.Unmarshal([]byte(text), &out); err == nil && len(out) > 0 {
		return normalizeSuggestions(out), nil
	}
	re := regexp.MustCompile(`\[[\s\S]*\]`)
	m := re.FindString(text)
	if m == "" {
		return nil, fmt.Errorf("parse tracks: not a valid JSON array")
	}
	if err := json.Unmarshal([]byte(m), &out); err != nil {
		return nil, fmt.Errorf("parse tracks: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("parse tracks: empty array")
	}
	return normalizeSuggestions(out), nil
}

func normalizeSuggestions(in []TrackSuggestion) []TrackSuggestion {
	var out []TrackSuggestion
	seen := make(map[string]bool)
	for _, t := range in {
		title := strings.TrimSpace(t.Title)
		artist := strings.TrimSpace(t.Artist)
		if title == "" || artist == "" {
			continue
		}
		key := strings.ToLower(title + "|" + artist)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, TrackSuggestion{Title: title, Artist: artist})
	}
	return out
}
