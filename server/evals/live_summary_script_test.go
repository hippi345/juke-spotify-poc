package evals

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiveSummaryScript_abortedOnArmSkipsVerdict(t *testing.T) {
	root := filepath.Join("..", "..")
	script := filepath.Join(root, "scripts", "summarize-live-eval-report.py")
	off := filepath.Join(t.TempDir(), "off.json")
	on := filepath.Join(t.TempDir(), "on.json")
	offBody := `{"summary":{"vector_on":false,"case_count":58,"mood_fit_mean":3.1,"crowd_fit_mean":3.2,"variety_mean":3.3,"coherence_mean":3.4,"resolve_rate_mean":0.8,"hard_pass_count":50,"judge_error_count":0},"gate":{"passed":false}}`
	if err := os.WriteFile(off, []byte(offBody), 0o644); err != nil {
		t.Fatal(err)
	}
	onBody := `{"summary":{"vector_on":true,"case_count":0,"aborted":true,"abort_reason":"test abort"},"gate":{"passed":false,"failures":["aborted"]}}`
	if err := os.WriteFile(on, []byte(onBody), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("python3", script, off, on).Output()
	if err != nil {
		t.Fatalf("script: %v", err)
	}
	text := string(out)
	if strings.Contains(text, "**Vector verdict: vector neutral**") {
		t.Fatal("aborted ON arm must not print vector neutral verdict")
	}
	if !strings.Contains(text, "did not complete") {
		t.Fatalf("expected aborted message, got:\n%s", text)
	}
}
