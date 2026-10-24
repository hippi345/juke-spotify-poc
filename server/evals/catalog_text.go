package evals

import (
	"strings"

	"juke-spotify-poc/server/vector"
)

// CatalogTrackIndexText is the lexical field set indexed for BM25 and embedded for vectors.
// The SAMPLE catalog has no album/tags columns; genres+mood+tempo carry tag-like metadata.
func CatalogTrackIndexText(tr vector.CatalogTrack) string {
	meta := strings.TrimSpace(strings.Join([]string{tr.Genres, tr.Mood, tr.Tempo}, " "))
	return vector.TrackEmbedText(tr.Title, tr.Artist, meta)
}
