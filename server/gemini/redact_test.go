package gemini

import (
	"strings"
	"testing"
)

func TestRedactSecrets_googleAPIKey35(t *testing.T) {
	// AIza + exactly 35 suffix characters (standard Google API key shape).
	key := "AIza" + strings.Repeat("x", 35)
	raw := "gemini: HTTP 403: " + key + " in body"
	got := RedactSecrets(raw)
	if got == raw || strings.Contains(got, key) {
		t.Fatalf("expected key redacted, got %q", got)
	}
	if !strings.Contains(got, "AIza[REDACTED]") {
		t.Fatalf("expected AIza[REDACTED], got %q", got)
	}
}

func TestRedactSecrets_keyQueryParam(t *testing.T) {
	raw := "embed HTTP 404: https://generativelanguage.googleapis.com/v1beta/models/m:embedContent?key=SECRETVALUE&other=1"
	got := RedactSecrets(raw)
	if strings.Contains(got, "SECRETVALUE") {
		t.Fatalf("key param not redacted: %q", got)
	}
	if !strings.Contains(got, "key=[REDACTED]") {
		t.Fatalf("expected key=[REDACTED], got %q", got)
	}
}
