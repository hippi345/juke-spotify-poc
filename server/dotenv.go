package main

import (
	"log"
	"os"
	"path/filepath"

	"juke-spotify-poc/server/config"

	"github.com/joho/godotenv"
)

// loadDotEnv merges env files so VibeSense works no matter where you start the server from.
// Later files override earlier for duplicate keys (so server/.env wins over repo .env).
func loadDotEnv() {
	wd, err := os.Getwd()
	var paths []string
	if err != nil {
		paths = []string{".env", "server/.env"}
	} else {
		paths = config.DotenvLoadPaths(wd)
	}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), ".env"))
	}

	seen := make(map[string]struct{})
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if _, dup := seen[abs]; dup {
			continue
		}
		st, err := os.Stat(abs)
		if err != nil || st.IsDir() {
			continue
		}
		seen[abs] = struct{}{}
		if err := godotenv.Load(abs); err != nil {
			log.Printf("env: could not load %q: %v", abs, err)
			continue
		}
		log.Printf("env: loaded %q", abs)
	}
}
