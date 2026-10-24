package evals

import (
	"strings"
	"unicode"

	"github.com/kljensen/snowball/english"
)

func stemToken(tok string) string {
	tok = strings.TrimSpace(strings.ToLower(tok))
	if tok == "" {
		return ""
	}
	return english.Stem(tok, true)
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
