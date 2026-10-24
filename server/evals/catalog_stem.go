package evals

import (
	"strings"
	"unicode"

	"github.com/kljensen/snowball/english"
)

// catalogIndexTokens is the single tokenizer for BM25 indexing, BM25 queries, and overlap checks.
func catalogIndexTokens(text string) []string {
	raw := tokenizeAndStem(text)
	if len(raw) == 0 {
		return raw
	}
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		if catalogStopwords[t] {
			continue
		}
		out = append(out, t)
	}
	return out
}

func tokenSetFromIndexTokens(text string) map[string]bool {
	toks := catalogIndexTokens(text)
	set := make(map[string]bool, len(toks))
	for _, t := range toks {
		set[t] = true
	}
	return set
}

func tokenizeAndStem(text string) []string {
	text = strings.ToLower(text)
	var tok []rune
	var out []string
	flush := func() {
		if len(tok) == 0 {
			return
		}
		s := stemToken(string(tok))
		if s != "" {
			out = append(out, s)
		}
		tok = nil
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			tok = append(tok, r)
			continue
		}
		flush()
	}
	flush()
	return out
}

func stemToken(tok string) string {
	tok = strings.TrimSpace(strings.ToLower(tok))
	if tok == "" {
		return ""
	}
	return english.Stem(tok, true)
}
