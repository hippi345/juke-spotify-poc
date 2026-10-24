package evals

import (
	"regexp"
	"strings"
)

// SecretLeakPatterns detect credential material in HTTP bodies, logs, and reports.
var SecretLeakPatterns = []*regexp.Regexp{
	regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`),
	regexp.MustCompile(`(?i)[?&]key=[A-Za-z0-9_-]{8,}`),
	regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._-]{20,}`),
}

// FindSecretLeaks returns pattern descriptions for any match in s.
func FindSecretLeaks(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var hits []string
	for _, re := range SecretLeakPatterns {
		if re.FindStringIndex(s) != nil {
			hits = append(hits, re.String())
		}
	}
	return hits
}

// AssertNoSecretLeaks fails the scan when any pattern matches.
func AssertNoSecretLeaks(s string) error {
	hits := FindSecretLeaks(s)
	if len(hits) == 0 {
		return nil
	}
	return &SecretLeakError{Patterns: hits}
}

// SecretLeakError is returned by the key-leak guard.
type SecretLeakError struct {
	Patterns []string
}

func (e *SecretLeakError) Error() string {
	return "secret leak guard: matched " + strings.Join(e.Patterns, ", ")
}
