package evals

// catalogStopwords are ignored when checking query vs planted-track lexical overlap.
var catalogStopwords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "by": true, "for": true, "from": true, "in": true, "into": true,
	"is": true, "it": true, "of": true, "on": true, "or": true, "the": true,
	"to": true, "with": true, "after": true, "ends": true,
}

func tokenSetStemmedNoStop(text string) map[string]bool {
	set := tokenSetStemmed(text)
	for w := range catalogStopwords {
		delete(set, w)
	}
	return set
}
