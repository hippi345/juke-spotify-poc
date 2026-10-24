package gemini

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRedactSecrets_removesKeyQueryParam(t *testing.T) {
	key := "AIzaSyTestSecretKey123"
	raw := "embed HTTP 404: https://generativelanguage.googleapis.com/v1beta/models/m:embedContent?key=" + key
	got := RedactSecrets(raw, key)
	if strings.Contains(got, key) {
		t.Fatalf("redacted string still contains key: %s", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("expected redaction marker: %s", got)
	}
}

func TestSetAPIKeyHeader_onlyInHeader(t *testing.T) {
	req := httptest.NewRequest("POST", "https://example.com/v1beta/models/x:generateContent", nil)
	SetAPIKeyHeader(req, "secret-key")
	if req.URL.RawQuery != "" {
		t.Fatalf("unexpected query: %s", req.URL.RawQuery)
	}
	if req.Header.Get(apiKeyHeader) != "secret-key" {
		t.Fatalf("missing header")
	}
}
