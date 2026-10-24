package evals

// catalogStopwords are dropped in catalogIndexTokens (BM25 + overlap checks).
var catalogStopwords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "by": true, "for": true, "from": true, "in": true, "into": true,
	"is": true, "it": true, "of": true, "on": true, "or": true, "the": true,
	"to": true, "with": true, "after": true, "our": true, "that": true, "this": true,
	"do": true, "what": true, "here": true, "but": true, "not": true, "too": true,
}
