package config

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// geminiKeyFromEnvFiles reads GEMINI_API_KEY from a .env file when the process env is empty.
// This is a fallback when godotenv did not run, failed to parse, or the IDE did not inherit the shell env.
func geminiKeyFromEnvFiles() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	candidates := GeminiKeyFromEnvSearchPaths(wd)

	seen := make(map[string]struct{})
	for _, p := range candidates {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if _, dup := seen[abs]; dup {
			continue
		}
		seen[abs] = struct{}{}
		if k := parseGeminiKeyFromDotenv(abs); k != "" {
			return k
		}
	}
	return ""
}

func parseGeminiKeyFromDotenv(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimSuffix(sc.Text(), "\r"))
		line = strings.TrimPrefix(line, "\ufeff")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export"))
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		if !strings.EqualFold(key, "GEMINI_API_KEY") {
			continue
		}
		val := strings.TrimSpace(line[idx+1:])
		val = strings.Trim(val, `"'`)
		// Skip empty placeholders; keep scanning for a later non-empty assignment.
		if val != "" {
			return val
		}
	}
	return ""
}

// LogGeminiEnvHints prints where we looked for .env (no secret values).
func LogGeminiEnvHints() {
	wd, err := os.Getwd()
	if err != nil {
		log.Printf("env: Getwd: %v", err)
		return
	}
	log.Printf("env: cwd=%q", wd)
	for _, p := range GeminiKeyFromEnvSearchPaths(wd) {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		st, err := os.Stat(abs)
		if err != nil {
			log.Printf("env: no file %q", abs)
			continue
		}
		if st.IsDir() {
			log.Printf("env: skip directory %q", abs)
			continue
		}
		log.Printf("env: found %q (%d bytes) — use: GEMINI_API_KEY=<key> (spaces around = are OK)", abs, st.Size())
	}
}
