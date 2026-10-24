package vector

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
)

const embedURL = "https://generativelanguage.googleapis.com/v1beta/models/%s:embedContent"

// ErrEmbedUnavailable means embeddings cannot run (e.g. missing API key).
var ErrEmbedUnavailable = fmt.Errorf("embeddings unavailable")

// Embedder produces dense vectors for venue track text.
type Embedder interface {
	Embed(text string) ([]float32, error)
}

// GeminiEmbedder uses Google text embedding models via net/http.
type GeminiEmbedder struct {
	APIKey string
	Model  string
	Client *http.Client
}

func NewEmbedderFromConfig(apiKey, provider string) Embedder {
	p := strings.ToLower(strings.TrimSpace(provider))
	if p == "ollama" {
		return OllamaEmbedder{
			BaseURL: strings.TrimSpace(os.Getenv("OLLAMA_BASE_URL")),
			Model:   strings.TrimSpace(os.Getenv("OLLAMA_EMBED_MODEL")),
			Client:  &http.Client{Timeout: 60 * time.Second},
		}
	}
	model := strings.TrimSpace(os.Getenv("GEMINI_EMBED_MODEL"))
	if model == "" {
		model = "text-embedding-004"
	}
	if strings.TrimSpace(apiKey) == "" {
		return HashEmbedder{Dim: 768}
	}
	return GeminiEmbedder{
		APIKey: apiKey,
		Model:  model,
		Client: &http.Client{Timeout: 60 * time.Second},
	}
}

// HashEmbedder builds a deterministic local vector (dev/backfill when GEMINI_API_KEY is unset).
type HashEmbedder struct {
	Dim int
}

func (h HashEmbedder) Embed(text string) ([]float32, error) {
	dim := h.Dim
	if dim <= 0 {
		dim = 768
	}
	vec := make([]float32, dim)
	var seed uint64
	for _, b := range []byte(text) {
		seed = seed*131 + uint64(b)
	}
	for i := 0; i < dim; i++ {
		seed ^= seed << 13
		seed ^= seed >> 7
		seed ^= seed << 17
		vec[i] = float32(int(seed%1000)) / 1000
	}
	return vec, nil
}

func (g GeminiEmbedder) Embed(text string) ([]float32, error) {
	if g.APIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY not set for embeddings")
	}
	body := map[string]interface{}{
		"model":   "models/" + g.Model,
		"content": map[string]interface{}{
			"parts": []map[string]string{{"text": text}},
		},
	}
	raw, _ := json.Marshal(body)
	u := fmt.Sprintf(embedURL+"?key=%s", g.Model, url.QueryEscape(g.APIKey))
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed HTTP %d: %s", resp.StatusCode, truncate(respBody, 300))
	}
	var parsed struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Embedding.Values) == 0 {
		return nil, fmt.Errorf("empty embedding")
	}
	return parsed.Embedding.Values, nil
}

// OllamaEmbedder calls a local Ollama embedding endpoint.
type OllamaEmbedder struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

func (o OllamaEmbedder) Embed(text string) ([]float32, error) {
	base := strings.TrimRight(o.BaseURL, "/")
	if base == "" {
		base = "http://127.0.0.1:11434"
	}
	model := o.Model
	if model == "" {
		model = "nomic-embed-text"
	}
	body := map[string]interface{}{"model": model, "prompt": text}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, base+"/api/embeddings", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embed HTTP %d", resp.StatusCode)
	}
	var parsed struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, err
	}
	return parsed.Embedding, nil
}

func TrackEmbedText(title, artist, genres string) string {
	return strings.TrimSpace(fmt.Sprintf("%s — %s — %s", title, artist, genres))
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
