package gemini

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

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

// RedactSecrets removes API keys and key= query fragments from strings intended for logs or clients.
func RedactSecrets(s string, secrets ...string) string {
	for _, sec := range secrets {
		sec = strings.TrimSpace(sec)
		if sec == "" {
			continue
		}
		s = strings.ReplaceAll(s, sec, "[REDACTED]")
		s = strings.ReplaceAll(s, url.QueryEscape(sec), "[REDACTED]")
	}
	s = regexp.MustCompile(`(?i)([?&]key=)[^&\s"']+`).ReplaceAllString(s, "${1}[REDACTED]")
	return s
}
