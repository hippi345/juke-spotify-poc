package evals

// catalogEvalQueries are natural guest-style paraphrases for planted-sunset retrieval evals.
// Each query must have zero catalogIndexTokens overlap with every planted track index text
// (see TestCatalogEvalQueriesNoStemOverlapWithPlanted).
var catalogEvalQueries = []string{
	"something easygoing for after work drinks",
	"playlist while the sky turns orange",
	"laid back stuff for hanging on the deck",
	"what is spun a lot during the pre-dinner window",
	"need soft songs for chatting with your group",
	"music for easing into supper",
	"well liked picks that seem cozy and kind",
	"gentle background for our table tonight",
	"bouncy but not too loud for the deck",
	"what fits a low key hangout",
	"tunes patrons keep asking us to play",
	"play whatever fits an unhurried kind vibe",
	"suggestions for a snug open air meetup",
}
