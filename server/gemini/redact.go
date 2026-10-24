package gemini

import (
	"net/url"
	"regexp"
	"strings"
)

// Standard patterns for Google/Gemini credential leaks in errors and reports.
var (
	googleAPIKeyPattern = regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`)
	urlKeyParamPattern  = regexp.MustCompile(`(?i)([?&]key=)[^&\s'"]+`)
)

// RedactSecrets removes API keys and key= query fragments from strings intended for logs, clients, and eval reports.
func RedactSecrets(s string, secrets ...string) string {
	for _, sec := range secrets {
		sec = strings.TrimSpace(sec)
		if sec == "" {
			continue
		}
		s = strings.ReplaceAll(s, sec, "[REDACTED]")
		s = strings.ReplaceAll(s, url.QueryEscape(sec), "[REDACTED]")
	}
	s = googleAPIKeyPattern.ReplaceAllString(s, "AIza[REDACTED]")
	s = urlKeyParamPattern.ReplaceAllString(s, "${1}[REDACTED]")
	return s
}
