package search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOsIndexerSearchAndIndex(t *testing.T) {
	docs := map[string]map[string]interface{}{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case r.Method == http.MethodHead && strings.HasSuffix(path, venuesIndex):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPut && strings.HasSuffix(path, venuesIndex):
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && strings.Contains(path, "/_doc/"):
			id := strings.TrimPrefix(path, "/"+venuesIndex+"/_doc/")
			var doc map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&doc)
			docs[id] = doc
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/_search"):
			var req struct {
				Query struct {
					MultiMatch struct {
						Query string `json:"query"`
					} `json:"multi_match"`
				} `json:"query"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			q := strings.ToLower(req.Query.MultiMatch.Query)
			hits := []map[string]interface{}{}
			for id, doc := range docs {
				name, _ := doc["name"].(string)
				if strings.Contains(strings.ToLower(name), q) {
					hits = append(hits, map[string]interface{}{
						"_id": id,
						"_source": doc,
					})
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"hits": map[string]interface{}{"hits": hits},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	prev := Default
	t.Cleanup(func() { Default = prev })

	if err := InitOpenSearch(srv.URL); err != nil {
		t.Fatalf("InitOpenSearch: %v", err)
	}

	ctx := context.Background()
	if err := Default.IndexVenue(ctx, VenueHit{
		ID: 42, Name: "Neon Lounge", Latitude: 1, Longitude: 2, StaffID: 3,
	}); err != nil {
		t.Fatalf("IndexVenue: %v", err)
	}

	hits, err := Default.SearchVenues(ctx, "neon", 5)
	if err != nil {
		t.Fatalf("SearchVenues: %v", err)
	}
	if len(hits) != 1 || hits[0].ID != 42 || hits[0].Name != "Neon Lounge" {
		t.Fatalf("unexpected hits: %+v", hits)
	}

	empty, err := Default.SearchVenues(ctx, "   ", 5)
	if err != nil {
		t.Fatalf("SearchVenues empty query: %v", err)
	}
	if empty != nil {
		t.Fatalf("expected nil for blank query, got %+v", empty)
	}
}
