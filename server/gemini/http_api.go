package gemini

import "net/http"

const apiKeyHeader = "x-goog-api-key"

// GenerativeModelURL builds a Gemini REST URL without embedding the API key in the query string.
func GenerativeModelURL(model, method string) string {
	return "https://generativelanguage.googleapis.com/v1beta/models/" + model + ":" + method
}

// SetAPIKeyHeader sends the Gemini API key only via header (never in the URL).
func SetAPIKeyHeader(req *http.Request, apiKey string) {
	if apiKey != "" {
		req.Header.Set(apiKeyHeader, apiKey)
	}
}
