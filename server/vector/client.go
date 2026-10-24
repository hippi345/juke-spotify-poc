package vector

import (
	"context"
	"log"
	"strings"
	"sync"

	"juke-spotify-poc/server/gemini"
)

// Service wraps venue track vector index + embedder for handlers and VibeSense.
type Service struct {
	Enabled    bool // config flag (same on every API replica)
	Indexer    Indexer
	Embedder   Embedder
	IndexReady bool
}

var (
	defaultSvc   Service
	defaultMu    sync.RWMutex
	defaultInited bool
)

// Init configures the global vector service (safe when disabled).
func Init(enabled bool, opensearchURL string, geminiKey string, embedProvider string) error {
	emb := NewEmbedderFromConfig(geminiKey, embedProvider)
	idx := NewOpenSearchIndexer(opensearchURL, emb)
	indexReady := false
	if !enabled {
		idx = NopIndexer{}
	} else if strings.TrimSpace(opensearchURL) == "" {
		log.Printf("vector: enabled but OPENSEARCH_URL empty — searches will return no hits")
		idx = NopIndexer{}
	} else {
		if err := idx.EnsureIndex(context.Background(), 768); err != nil {
			log.Printf("vector: index ensure failed at startup (searches may retry): %v", err)
		} else {
			indexReady = true
		}
	}
	defaultMu.Lock()
	defaultSvc = Service{Enabled: enabled, Indexer: idx, Embedder: emb, IndexReady: indexReady}
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

// CandidatesForMood retrieves top-N venue tracks for Gemini grounding.
func (s Service) CandidatesForMood(ctx context.Context, venueID uint, mood string, limit int) []gemini.TrackSuggestion {
	if !s.Enabled || venueID == 0 || s.Indexer == nil {
		return nil
	}
	hits, err := s.Indexer.SearchByText(ctx, venueID, mood, s.Embedder, limit)
	if err != nil {
		log.Printf("vector: mood retrieval failed: %v", err)
		return nil
	}
	var out []gemini.TrackSuggestion
	for _, h := range hits {
		out = append(out, gemini.TrackSuggestion{Title: h.Title, Artist: h.Artist})
	}
	return out
}
