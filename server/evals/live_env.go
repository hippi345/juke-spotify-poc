package evals

import (
	"fmt"
	"os"
	"strings"

	"juke-spotify-poc/server/config"
)

const defaultLiveOpenSearchURL = "http://localhost:9200"

// LiveEvalOpenSearchURL returns OPENSEARCH_URL for live eval runs (default localhost:9200).
func LiveEvalOpenSearchURL(cfg *config.Config) string {
	if cfg != nil && strings.TrimSpace(cfg.OpenSearchURL) != "" {
		return strings.TrimSpace(cfg.OpenSearchURL)
	}
	if u := strings.TrimSpace(os.Getenv("OPENSEARCH_URL")); u != "" {
		return u
	}
	if os.Getenv("JUKE_EVALS_LIVE") == "1" || os.Getenv("JUKE_EVALS_VECTOR") == "1" {
		return defaultLiveOpenSearchURL
	}
	return ""
}

// RequireLiveVectorOpenSearch fails when vector live arm is on but OpenSearch URL is missing.
func RequireLiveVectorOpenSearch(cfg *config.Config) error {
	if os.Getenv("JUKE_EVALS_VECTOR") != "1" {
		return nil
	}
	if strings.TrimSpace(LiveEvalOpenSearchURL(cfg)) == "" {
		return fmt.Errorf("JUKE_EVALS_VECTOR=1 requires OPENSEARCH_URL (or use default %s for live evals)", defaultLiveOpenSearchURL)
	}
	return nil
}
