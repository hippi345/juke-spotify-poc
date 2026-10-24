package vector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNopIndexer(t *testing.T) {
	var idx Indexer = NopIndexer{}
	if err := idx.EnsureIndex(context.Background(), 768); err != nil {
		t.Fatal(err)
	}
	hits, err := idx.SearchSimilar(context.Background(), 1, []float32{0.1, 0.2}, 5)
	if err != nil || len(hits) != 0 {
		t.Fatalf("nop search: %v %d", err, len(hits))
	}
}

func TestEnsureIndexCreatesMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	idx := NewOpenSearchIndexer(srv.URL, nil)
	if err := idx.EnsureIndex(context.Background(), 4); err != nil {
		t.Fatal(err)
	}
}
