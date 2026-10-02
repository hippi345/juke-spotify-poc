package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const venuesIndex = "jukespotify-venues"

type osIndexer struct {
	baseURL    string
	httpClient *http.Client
}

// InitOpenSearch configures the venue search index. Empty url leaves the no-op indexer.
func InitOpenSearch(url string) error {
	if strings.TrimSpace(url) == "" {
		return nil
	}
	base := strings.TrimRight(url, "/")
	idx := &osIndexer{
		baseURL: base,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	if err := idx.ensureIndex(context.Background()); err != nil {
		return err
	}
	Default = idx
	return nil
}

func (o *osIndexer) ensureIndex(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, o.baseURL+"/"+venuesIndex, nil)
	if err != nil {
		return err
	}
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	body := map[string]interface{}{
		"settings": map[string]interface{}{
			"index": map[string]interface{}{
				"number_of_shards":   1,
				"number_of_replicas": 0,
			},
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"name":      map[string]string{"type": "text"},
				"latitude":  map[string]string{"type": "double"},
				"longitude": map[string]string{"type": "double"},
				"staff_id":  map[string]string{"type": "long"},
			},
		},
	}
	raw, _ := json.Marshal(body)
	put, err := http.NewRequestWithContext(ctx, http.MethodPut, o.baseURL+"/"+venuesIndex, bytes.NewReader(raw))
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
		return fmt.Errorf("create index: %s", string(b))
	}
	return nil
}

func (o *osIndexer) IndexVenue(ctx context.Context, v VenueHit) error {
	doc := map[string]interface{}{
		"name":      v.Name,
		"latitude":  v.Latitude,
		"longitude": v.Longitude,
		"staff_id":  v.StaffID,
	}
	raw, _ := json.Marshal(doc)
	url := fmt.Sprintf("%s/%s/_doc/%d", o.baseURL, venuesIndex, v.ID)
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
		return fmt.Errorf("index venue: %s", string(b))
	}
	return nil
}

func (o *osIndexer) SearchVenues(ctx context.Context, query string, limit int) ([]VenueHit, error) {
	if limit <= 0 {
		limit = 20
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	payload := map[string]interface{}{
		"size": limit,
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  q,
				"fields": []string{"name^2"},
			},
		},
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/"+venuesIndex+"/_search", bytes.NewReader(raw))
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
		return nil, fmt.Errorf("search venues: %s", string(b))
	}
	var parsed struct {
		Hits struct {
			Hits []struct {
				Source struct {
					Name      string  `json:"name"`
					Latitude  float64 `json:"latitude"`
					Longitude float64 `json:"longitude"`
					StaffID   uint    `json:"staff_id"`
				} `json:"_source"`
				ID string `json:"_id"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	out := make([]VenueHit, 0, len(parsed.Hits.Hits))
	for _, h := range parsed.Hits.Hits {
		id64, _ := strconv.ParseUint(h.ID, 10, 64)
		out = append(out, VenueHit{
			ID:        uint(id64),
			Name:      h.Source.Name,
			Latitude:  h.Source.Latitude,
			Longitude: h.Source.Longitude,
			StaffID:   h.Source.StaffID,
		})
	}
	return out, nil
}
