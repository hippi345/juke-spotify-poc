package config

import "path/filepath"

// DotenvLoadPaths returns .env files for godotenv in merge order (later overrides earlier).
func DotenvLoadPaths(wd string) []string {
	if filepath.Base(wd) == "server" {
		return []string{
			filepath.Join(wd, "..", ".env"),
			filepath.Join(wd, ".env"),
		}
	}
	return []string{
		filepath.Join(wd, ".env"),
		filepath.Join(wd, "server", ".env"),
	}
}

// GeminiKeyFromEnvSearchPaths returns .env paths to scan for GEMINI_API_KEY (first non-empty wins).
func GeminiKeyFromEnvSearchPaths(wd string) []string {
	if filepath.Base(wd) == "server" {
		return []string{
			filepath.Join(wd, ".env"),
			filepath.Join(wd, "..", ".env"),
		}
	}
	return []string{
		filepath.Join(wd, "server", ".env"),
		filepath.Join(wd, ".env"),
	}
}
