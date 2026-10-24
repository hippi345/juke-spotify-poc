package gemini

// GenerateOptions tunes playlist generation (optional venue-grounding candidates).
type GenerateOptions struct {
	PreferredCandidates []TrackSuggestion
}

// PlaylistGenerator produces track suggestions for VibeSense evals and production.
type PlaylistGenerator interface {
	GenerateTrackList(description string, count int, similarArtists []string, opts GenerateOptions) ([]TrackSuggestion, error)
}

// HTTPGenerator calls the Gemini REST API (production default).
type HTTPGenerator struct {
	APIKey string
	Model  string
}

func (g HTTPGenerator) GenerateTrackList(description string, count int, similarArtists []string, opts GenerateOptions) ([]TrackSuggestion, error) {
	return generateTrackListHTTP(g.APIKey, g.Model, description, count, similarArtists, opts)
}

// NewHTTPGenerator returns a generator backed by Gemini net/http.
func NewHTTPGenerator(apiKey, model string) PlaylistGenerator {
	return HTTPGenerator{APIKey: apiKey, Model: model}
}
