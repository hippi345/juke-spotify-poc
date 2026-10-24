package vector

import (
	"log"

	"juke-spotify-poc/server/gemini"
)

const DegradedSemanticSearch = "semantic_search_unavailable"

// LogEmbedError logs a redacted upstream error (never logs API keys).
func LogEmbedError(prefix string, err error, apiKey string) {
	if err == nil {
		return
	}
	log.Printf("%s: %s", prefix, gemini.RedactSecrets(err.Error(), apiKey))
}
