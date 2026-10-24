package vector

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

func TestInit_warnsWhenVectorOnWithoutGeminiKey(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	_ = Init(true, "", "", "gemini")
	msg := buf.String()
	if !strings.Contains(msg, "GEMINI_API_KEY is unset") {
		t.Fatalf("expected startup warning, got: %q", msg)
	}
	if strings.Contains(msg, "AIza") {
		t.Fatal("warning must not contain key material")
	}
}
