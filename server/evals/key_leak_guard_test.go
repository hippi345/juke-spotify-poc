package evals

import (
	"os"
	"path/filepath"
	"testing"

	"juke-spotify-poc/server/gemini"
)

func TestKeyLeakGuard_RedactAndPatterns(t *testing.T) {
	fake := "AIzaSyDummyKeyForUnitTestOnly123456"
	s := "url?key=" + fake
	red := gemini.RedactSecrets(s, fake)
	if err := AssertNoSecretLeaks(red); err != nil {
		t.Fatalf("redacted string should pass: %v", err)
	}
	if err := AssertNoSecretLeaks(s); err == nil {
		t.Fatal("expected leak detection on raw string")
	}
	if err := AssertNoSecretLeaks("Bearer sk-abcdefghijklmnopqrstuvwxyz"); err == nil {
		t.Fatal("expected Bearer/sk detection")
	}
}

func TestKeyLeakGuard_EvalReports(t *testing.T) {
	cfg := mockRunConfig(true)
	results, err := RunSuite(cfg)
	if err != nil {
		t.Fatal(err)
	}
	summary := Summarize(results, "mocked", false)
	dir := t.TempDir()
	gate := GateResult{Passed: true}
	if err := WriteReports(dir, summary, results, gate); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"report.json", "report.md"} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := AssertNoSecretLeaks(string(raw)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for _, r := range results {
		if r.Hard.ErrorMessage != "" {
			if err := AssertNoSecretLeaks(r.Hard.ErrorMessage); err != nil {
				t.Fatalf("case %s error message: %v", r.CaseID, err)
			}
		}
	}
}

func TestCaseCatalog_AtLeastFifty(t *testing.T) {
	if len(AllCases()) < 50 {
		t.Fatalf("expected 50+ cases, got %d", len(AllCases()))
	}
}
