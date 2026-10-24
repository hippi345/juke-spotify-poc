package vector

import (
	"testing"
)

func TestValidateEmbedVector_rejectsWrongDimension(t *testing.T) {
	err := ValidateEmbedVector(make([]float32, 3072))
	if err == nil {
		t.Fatal("expected dimension mismatch error")
	}
}

func TestNewEmbedderFromConfig_defaultModel(t *testing.T) {
	emb := NewEmbedderFromConfig("test-key", "gemini")
	ge, ok := emb.(GeminiEmbedder)
	if !ok {
		t.Fatalf("expected GeminiEmbedder, got %T", emb)
	}
	if ge.Model != DefaultGeminiEmbedModel {
		t.Fatalf("model: %s", ge.Model)
	}
}
