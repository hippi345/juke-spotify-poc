//go:build race

package evals

// Race detector adds overhead; keep a finite bound without failing CI.
const catalogP95MaxMs = 250
