//go:build evals_live

package evals

import "juke-spotify-poc/server/config"

// SeedLiveEvalVenueIndex seeds OpenSearch for all vector-eval venue IDs (see venue_live_seed.go).
func SeedLiveEvalVenueIndex(cfg *config.Config) error {
	return SeedLiveEvalVenueIndexes(cfg)
}
