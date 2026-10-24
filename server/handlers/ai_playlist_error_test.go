package handlers

import (
	"fmt"
	"strings"
	"testing"

	"juke-spotify-poc/server/config"
)

func TestFailJobGemini_clientMessageNeverContainsAPIKey(t *testing.T) {
	const leakKey = "AIzaSyTESTKEYFORLEAKGUARD123456789"
	cfg := &config.Config{GeminiAPIKey: leakKey}
	id := "job-redact-test"
	setJob(id, &aiPlaylistJob{Status: "pending"})
	failJobGemini(id, cfg, fmt.Errorf("upstream gemini HTTP 400: %s", leakKey))

	j, ok := getJob(id)
	if !ok {
		t.Fatal("job missing")
	}
	if j.ErrMsg == "" {
		t.Fatal("expected error message")
	}
	if strings.Contains(j.ErrMsg, leakKey) || strings.Contains(j.ErrMsg, "AIza") {
		t.Fatalf("client error leaked secret: %q", j.ErrMsg)
	}
	if !strings.Contains(j.ErrMsg, "VibeSense could not generate") {
		t.Fatalf("unexpected message: %q", j.ErrMsg)
	}
}
