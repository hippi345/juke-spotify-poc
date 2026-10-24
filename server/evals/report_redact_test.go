package evals

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteReports_redactsErrorDetails(t *testing.T) {
	key := "AIza" + strings.Repeat("y", 35)
	results := []CaseResult{{
		CaseID: "x",
		Hard: HardCheckResult{
			ErrorMessage: "gemini HTTP 403: " + key,
		},
	}}
	dir := t.TempDir()
	gate := GateResult{
		Passed:   false,
		Failures: []string{"case x: " + key},
	}
	if err := WriteReports(dir, SummaryMetrics{Mode: "test"}, results, gate); err != nil {
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
		if strings.Contains(string(raw), key) {
			t.Fatalf("%s still contains raw key material", name)
		}
	}
}
