package vector

import (
	"context"
	"log"
	"sync"

	"juke-spotify-poc/server/gemini"
)

// Service wraps venue track vector index + embedder for handlers and VibeSense.
type Service struct {
	Enabled  bool
	Indexer  Indexer
	Embedder Embedder
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
	if enabled && opensearchURL != "" {
		if err := idx.EnsureIndex(context.Background(), 768); err != nil {
			log.Printf("vector: index init failed (continuing without vectors): %v", err)
			idx = NopIndexer{}
			enabled = false
		}
	} else {
		enabled = false
		idx = NopIndexer{}
	}
	defaultMu.Lock()
	defaultSvc = Service{Enabled: enabled, Indexer: idx, Embedder: emb}
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
