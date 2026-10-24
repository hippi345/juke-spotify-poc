package vector

import "fmt"

// IndexEmbeddingDim is the OpenSearch k-NN vector size for venue_tracks.
const IndexEmbeddingDim = 768

// DefaultGeminiEmbedModel is the Gemini embedding model used when GEMINI_EMBED_MODEL is unset.
const DefaultGeminiEmbedModel = "gemini-embedding-001"

// EmbedDimension returns the vector length this embedder produces for the search index.
func EmbedDimension(e Embedder) int {
	switch e.(type) {
	case GeminiEmbedder, HashEmbedder:
		return IndexEmbeddingDim
	default:
		return IndexEmbeddingDim
	}
}

// ValidateEmbedVector ensures a produced embedding matches the index dimension.
func ValidateEmbedVector(vec []float32) error {
	if len(vec) != IndexEmbeddingDim {
		return fmt.Errorf("embedding dimension %d does not match index dimension %d", len(vec), IndexEmbeddingDim)
	}
	return nil
}
