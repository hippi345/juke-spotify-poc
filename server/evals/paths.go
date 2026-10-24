package evals

import (
	"path/filepath"
	"runtime"
)

// rootDir is the server/evals package directory (stable regardless of test cwd).
var rootDir = func() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}()

func baselineFile() string   { return filepath.Join(rootDir, "baseline.json") }
func reportsDir() string     { return filepath.Join(rootDir, "out") }
func fixtureRoot() string    { return filepath.Join(rootDir, "testdata") }
