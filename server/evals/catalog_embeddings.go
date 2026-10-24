package evals

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"juke-spotify-poc/server/vector"
)

type catalogEmbeddingManifest struct {
	StoredDim            int                 `json:"stored_dim"`
	ProductionTargetModel string             `json:"production_target_model"`
	GeneratorModel       string              `json:"generator_model"`
	GeneratorNativeDim   int                 `json:"generator_native_dim"`
	PadStrategy          string              `json:"pad_strategy"`
	CatalogPath          string              `json:"catalog_path"`
	CatalogSHA256        string              `json:"catalog_sha256"`
	NumTracks            int                 `json:"num_tracks"`
	EvalQueries          map[string][]float32 `json:"eval_queries"`
}

type catalogEmbeddingStore struct {
	trackVectors [][]float32
	queryVectors map[string][]float32
}

func catalogVectorDir() string {
	return filepath.Dir(vector.DefaultCatalogFixturePath())
}

func defaultManifestPath() string {
	return filepath.Join(catalogVectorDir(), "catalog_embedding_manifest.json")
}

func defaultVectorsPath() string {
	return filepath.Join(catalogVectorDir(), "catalog_track_embeddings.f32")
}

func loadCatalogEmbeddingStore() (*catalogEmbeddingStore, error) {
	manifestPath := defaultManifestPath()
	vecPath := defaultVectorsPath()
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read embedding manifest: %w", err)
	}
	var man catalogEmbeddingManifest
	if err := json.Unmarshal(raw, &man); err != nil {
		return nil, fmt.Errorf("parse embedding manifest: %w", err)
	}
	if man.StoredDim != vector.IndexEmbeddingDim {
		return nil, fmt.Errorf("manifest stored_dim %d != index dim %d", man.StoredDim, vector.IndexEmbeddingDim)
	}
	catPath := vector.DefaultCatalogFixturePath()
	if err := verifyCatalogSHA256(catPath, man.CatalogSHA256); err != nil {
		return nil, err
	}
	doc, err := loadCatalog()
	if err != nil {
		return nil, err
	}
	if len(doc.Tracks) != man.NumTracks {
		return nil, fmt.Errorf("catalog tracks %d != manifest num_tracks %d", len(doc.Tracks), man.NumTracks)
	}
	trackVectors, err := readFloat32Matrix(vecPath, man.NumTracks, man.StoredDim)
	if err != nil {
		return nil, err
	}
	qv := make(map[string][]float32, len(man.EvalQueries))
	for q, v := range man.EvalQueries {
		if len(v) != man.StoredDim {
			return nil, fmt.Errorf("query %q: dim %d", q, len(v))
		}
		qv[q] = v
	}
	return &catalogEmbeddingStore{trackVectors: trackVectors, queryVectors: qv}, nil
}

func verifyCatalogSHA256(catalogPath, want string) error {
	if want == "" {
		return nil
	}
	f, err := os.Open(catalogPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := fmt.Sprintf("%x", h.Sum(nil))
	if got != want {
		return fmt.Errorf("catalog sha256 mismatch (regenerate embeddings): got %s want %s", got, want)
	}
	return nil
}

func readFloat32Matrix(path string, rows, cols int) ([][]float32, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	want := rows * cols * 4
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if len(data) != want {
		return nil, fmt.Errorf("%s: size %d != %d floats", path, len(data), want)
	}
	out := make([][]float32, rows)
	for i := 0; i < rows; i++ {
		row := make([]float32, cols)
		off := i * cols * 4
		for j := 0; j < cols; j++ {
			row[j] = mathFloat32FromBytes(data[off+j*4 : off+j*4+4])
		}
		out[i] = row
	}
	return out, nil
}

func mathFloat32FromBytes(b []byte) float32 {
	return float32(binary.LittleEndian.Uint32(b))
}

func (s *catalogEmbeddingStore) queryVector(query string) ([]float32, error) {
	v, ok := s.queryVectors[query]
	if !ok {
		return nil, fmt.Errorf("no precomputed embedding for eval query (regenerate manifest): %q", query)
	}
	return v, nil
}
