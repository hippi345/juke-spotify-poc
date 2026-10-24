package vector

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"juke-spotify-poc/server/gemini"
)

// Service wraps venue track vector index + embedder for handlers and VibeSense.
type Service struct {
	Enabled            bool // config flag (same on every API replica)
	Indexer            Indexer
	Embedder           Embedder
	IndexReady         bool
	RetrievalLimit     int
	MinSimilarityScore float64
}

var (
	defaultSvc   Service
	defaultMu    sync.RWMutex
	defaultInited bool
)

// Init configures the global vector service (safe when disabled).
func Init(enabled bool, opensearchURL string, geminiKey string, embedProvider string) error {
	emb := NewEmbedderFromConfig(geminiKey, embedProvider)
	if enabled && strings.TrimSpace(geminiKey) == "" && strings.ToLower(strings.TrimSpace(embedProvider)) != "ollama" {
		log.Printf("vector: enabled but GEMINI_API_KEY is unset — using deterministic HashEmbedder for indexing/search (not production gemini-embedding-001 quality)")
	}
	idx := NewOpenSearchIndexer(opensearchURL, emb)
	indexReady := false
	if !enabled {
		idx = NopIndexer{}
	} else if strings.TrimSpace(opensearchURL) == "" {
		log.Printf("vector: enabled but OPENSEARCH_URL empty — searches will return no hits")
		idx = NopIndexer{}
	} else {
		dim := EmbedDimension(emb)
		if err := idx.EnsureIndex(context.Background(), dim); err != nil {
			log.Printf("vector: index ensure failed at startup (searches may retry): %v", err)
		} else {
			indexReady = true
		}
	}
	defaultMu.Lock()
	defaultSvc = Service{
		Enabled:            enabled,
		Indexer:            idx,
		Embedder:           emb,
		IndexReady:         indexReady,
		RetrievalLimit:     envIntDefault("VIBESENSE_VECTOR_RETRIEVAL_LIMIT", 8),
		MinSimilarityScore: envFloatDefault("VIBESENSE_VECTOR_MIN_SIMILARITY", 0.01),
	}
	defaultInited = true
	defaultMu.Unlock()
	return nil
}

// Default returns the configured vector service.
func Default() Service {
	defaultMu.RLock()
	s := defaultSvc
	defaultMu.RUnlock()
	if !defaultInited {
		return Service{Enabled: false, Indexer: NopIndexer{}}
	}
	return s
}

// SetDefaultService replaces the global vector service (for tests).
func SetDefaultService(s Service) {
	defaultMu.Lock()
	defaultSvc = s
	defaultInited = true
	defaultMu.Unlock()
}

// CandidatesForMood retrieves top-N venue tracks for Gemini grounding.
// limit <= 0 uses RetrievalLimit (default 8, below typical seeded index size).
func (s Service) CandidatesForMood(ctx context.Context, venueID uint, mood string, limit int) []gemini.TrackSuggestion {
	if !s.Enabled || venueID == 0 || s.Indexer == nil {
		return nil
	}
	eff := limit
	if eff <= 0 || eff > s.RetrievalLimit {
		eff = s.RetrievalLimit
	}
	if eff <= 0 {
		eff = 8
	}
	hits, err := s.Indexer.SearchByText(ctx, venueID, mood, s.Embedder, eff)
	if err != nil {
		log.Printf("vector: mood retrieval failed: %v", err)
		return nil
	}
	var out []gemini.TrackSuggestion
	for _, h := range hits {
		if h.Score < s.MinSimilarityScore {
			continue
		}
		out = append(out, gemini.TrackSuggestion{Title: h.Title, Artist: h.Artist})
		if len(out) >= eff {
			break
		}
	}
	return out
}

func envIntDefault(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envFloatDefault(key string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}
