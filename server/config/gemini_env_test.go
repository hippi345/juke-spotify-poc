package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGeminiKeyFromDotenv_skipsEmptyPlaceholder(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	content := "GEMINI_API_KEY=\nGEMINI_API_KEY=not-empty-test-value\n"
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	got := parseGeminiKeyFromDotenv(p)
	if got != "not-empty-test-value" {
		t.Fatalf("got %q want non-empty second line", got)
	}
}

func TestParseGeminiKeyFromDotenv_spacesAroundEquals(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	content := "GEMINI_API_KEY = spaced-value\n"
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	got := parseGeminiKeyFromDotenv(p)
	if got != "spaced-value" {
		t.Fatalf("got %q", got)
	}
}

func TestParseGeminiKeyFromDotenv_exportPrefix(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	content := "export GEMINI_API_KEY=export-style-value\n"
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	got := parseGeminiKeyFromDotenv(p)
	if got != "export-style-value" {
		t.Fatalf("got %q", got)
	}
}
