package vector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const venueTracksIndex = "venue_tracks"

// VenueTrackDoc is stored in OpenSearch for k-NN retrieval.
type VenueTrackDoc struct {
	VenueID    uint    `json:"venue_id"`
	SpotifyID  string  `json:"spotify_id"`
	Title      string  `json:"title"`
	Artist     string  `json:"artist"`
	Genres     string  `json:"genres"`
	PlayCount  int     `json:"play_count"`
	VoteCount  int     `json:"vote_count"`
	QueueCount int     `json:"queue_count"`
	LastSeen   string  `json:"last_seen"`
	Embedding  []float32 `json:"embedding"`
}

// SearchHit is a retrieved venue track.
type SearchHit struct {
	SpotifyID string
	Title     string
	Artist    string
	Score     float64
}

// Indexer upserts and searches venue_tracks.
type Indexer interface {
	EnsureIndex(ctx context.Context, dimension int) error
	Upsert(ctx context.Context, docID string, doc VenueTrackDoc) error
	SearchSimilar(ctx context.Context, venueID uint, queryVec []float32, limit int) ([]SearchHit, error)
	SearchByText(ctx context.Context, venueID uint, q string, embedder Embedder, limit int) ([]SearchHit, error)
}

type osIndexer struct {
	baseURL    string
	httpClient *http.Client
	embedder   Embedder
}

// NewOpenSearchIndexer connects to OpenSearch for venue_tracks k-NN.
func NewOpenSearchIndexer(baseURL string, embedder Embedder) Indexer {
	if strings.TrimSpace(baseURL) == "" {
		return NopIndexer{}
	}
	return &osIndexer{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
		embedder:   embedder,
	}
}

// NopIndexer is used when vector search is disabled or OpenSearch is unavailable.
type NopIndexer struct{}

func (NopIndexer) EnsureIndex(context.Context, int) error { return nil }
func (NopIndexer) Upsert(context.Context, string, VenueTrackDoc) error { return nil }
func (NopIndexer) SearchSimilar(context.Context, uint, []float32, int) ([]SearchHit, error) {
	return nil, nil
}
func (NopIndexer) SearchByText(context.Context, uint, string, Embedder, int) ([]SearchHit, error) {
	return nil, nil
}

func (o *osIndexer) EnsureIndex(ctx context.Context, dimension int) error {
	if dimension <= 0 {
		dimension = 768
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, o.baseURL+"/"+venueTracksIndex, nil)
	if err != nil {
		return err
	}
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	body := map[string]interface{}{
		"settings": map[string]interface{}{
			"index": map[string]interface{}{
				"knn":                true,
				"number_of_shards":   1,
				"number_of_replicas": 0,
			},
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"venue_id":    map[string]string{"type": "long"},
				"spotify_id":  map[string]string{"type": "keyword"},
				"title":       map[string]string{"type": "text"},
				"artist":      map[string]string{"type": "text"},
				"genres":      map[string]string{"type": "text"},
				"play_count":  map[string]string{"type": "integer"},
				"vote_count":  map[string]string{"type": "integer"},
				"queue_count": map[string]string{"type": "integer"},
				"last_seen":   map[string]string{"type": "date"},
				"embedding": map[string]interface{}{
					"type":      "knn_vector",
					"dimension": dimension,
					"method": map[string]interface{}{
						"name":       "hnsw",
						"space_type": "cosinesimil",
						"engine":     "nmslib",
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(body)
	put, err := http.NewRequestWithContext(ctx, http.MethodPut, o.baseURL+"/"+venueTracksIndex, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	put.Header.Set("Content-Type", "application/json")
	putResp, err := o.httpClient.Do(put)
	if err != nil {
		return err
	}
	defer putResp.Body.Close()
	if putResp.StatusCode >= 300 {
		b, _ := io.ReadAll(putResp.Body)
		return fmt.Errorf("create venue_tracks index: %s", string(b))
	}
	return nil
}

func (o *osIndexer) Upsert(ctx context.Context, docID string, doc VenueTrackDoc) error {
	raw, _ := json.Marshal(doc)
	url := fmt.Sprintf("%s/%s/_doc/%s", o.baseURL, venueTracksIndex, docID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upsert venue track: %s", string(b))
	}
	return nil
}

func (o *osIndexer) SearchSimilar(ctx context.Context, venueID uint, queryVec []float32, limit int) ([]SearchHit, error) {
	if limit <= 0 {
		limit = 20
	}
	payload := map[string]interface{}{
		"size": limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{
						"knn": map[string]interface{}{
							"embedding": map[string]interface{}{
								"vector": queryVec,
								"k":      limit,
							},
						},
					},
				},
				"filter": []interface{}{
					map[string]interface{}{
						"term": map[string]interface{}{"venue_id": venueID},
					},
				},
			},
		},
	}
	return o.runSearch(ctx, payload)
}

func (o *osIndexer) SearchByText(ctx context.Context, venueID uint, q string, embedder Embedder, limit int) ([]SearchHit, error) {
	emb := embedder
	if emb == nil {
		emb = o.embedder
	}
	if emb == nil {
		return nil, fmt.Errorf("embedder not configured")
	}
	vec, err := emb.Embed(q)
	if err != nil {
		return nil, err
	}
	return o.SearchSimilar(ctx, venueID, vec, limit)
}

func (o *osIndexer) runSearch(ctx context.Context, payload map[string]interface{}) ([]SearchHit, error) {
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/"+venueTracksIndex+"/_search", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vector search: %s", string(b))
	}
	var parsed struct {
		Hits struct {
			Hits []struct {
				Score  float64 `json:"_score"`
				Source struct {
					SpotifyID string `json:"spotify_id"`
					Title     string `json:"title"`
					Artist    string `json:"artist"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	out := make([]SearchHit, 0, len(parsed.Hits.Hits))
	for _, h := range parsed.Hits.Hits {
		out = append(out, SearchHit{
			SpotifyID: h.Source.SpotifyID,
			Title:     h.Source.Title,
			Artist:    h.Source.Artist,
			Score:     h.Score,
		})
	}
	return out, nil
}
