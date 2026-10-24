package evals

import "juke-spotify-poc/server/spotify"

// TrackSearcher resolves model suggestions via Spotify search (mockable in evals).
type TrackSearcher interface {
	SearchTracks(query string, limit int) ([]spotify.Track, error)
}

// SpotifyClientAdapter wraps the production Spotify client.
type SpotifyClientAdapter struct {
	Client *spotify.Client
}

func (a SpotifyClientAdapter) SearchTracks(query string, limit int) ([]spotify.Track, error) {
	if a.Client == nil {
		return nil, nil
	}
	return a.Client.SearchTracks(query, limit)
}
