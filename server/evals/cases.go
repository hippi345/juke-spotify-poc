package evals

// Case is one VibeSense eval scenario.
type Case struct {
	ID              string
	Description     string
	TrackCount      int
	SimilarArtists  []string
	ExpectError     bool
	CI              bool // included in -short / CI gate subset
	VenueID         uint // optional; used for vector-grounding evals (PR2)
}

// AllCases returns the full eval matrix (~20 moods + edge cases).
func AllCases() []Case {
	return []Case{
		{ID: "rainy_sunday_brunch", Description: "rainy Sunday brunch — mellow acoustic and soft indie", TrackCount: 15, CI: true},
		{ID: "90s_hiphop_pregame", Description: "90s hip-hop pregame energy", TrackCount: 20, CI: true},
		{ID: "late_night_lofi_study", Description: "late-night lo-fi study session", TrackCount: 12, CI: true},
		{ID: "dance_floor_peak", Description: "peak-hour dance floor bangers", TrackCount: 25},
		{ID: "coffee_shop_morning", Description: "quiet coffee shop morning jazz and folk", TrackCount: 10},
		{ID: "indie_roadtrip", Description: "indie road trip windows-down", TrackCount: 18},
		{ID: "80s_synth_nostalgia", Description: "80s synth nostalgia night", TrackCount: 16},
		{ID: "country_honkytonk", Description: "country honky-tonk Friday", TrackCount: 14},
		{ID: "reggae_sunset", Description: "reggae sunset patio", TrackCount: 12},
		{ID: "metal_workout", Description: "heavy metal workout grind", TrackCount: 15},
		{ID: "kpop_party", Description: "K-pop dance party", TrackCount: 20},
		{ID: "classical_dinner", Description: "elegant classical dinner background", TrackCount: 10},
		{ID: "latin_salsa_night", Description: "Latin salsa social dancing", TrackCount: 18},
		{ID: "punk_basement", Description: "punk basement show energy", TrackCount: 14},
		{ID: "chill_rnb_date", Description: "chill R&B date night", TrackCount: 12},
		{ID: "edm_festival_warmup", Description: "EDM festival warmup", TrackCount: 22},
		{ID: "folk_campfire", Description: "folk campfire singalong", TrackCount: 10},
		{ID: "similar_artists_radiohead", Description: "atmospheric rock for a rainy bar", TrackCount: 12, SimilarArtists: []string{"Radiohead", "Portishead"}, CI: true},
		{ID: "non_english_prompt", Description: "soirée française — chanson et électro douce", TrackCount: 10, CI: true},
		{ID: "count_two", Description: "two songs only — quick opener", TrackCount: 2, CI: true},
		{ID: "count_max", Description: "pack the list — crowd pleasers", TrackCount: 100},
		{ID: "obscure_tuvan", Description: "Tuvan throat-singing and experimental folk", TrackCount: 8},
		{ID: "nonsense_input", Description: "asdf qwerty zxcv", TrackCount: 5, ExpectError: true, CI: true},
		{ID: "empty_description", Description: "   ", TrackCount: 5, ExpectError: true, CI: true},
	}
}

// FilterCases returns cases for the current run (CI subset when short).
func FilterCases(short bool) []Case {
	all := AllCases()
	if !short {
		return all
	}
	var out []Case
	for _, c := range all {
		if c.CI {
			out = append(out, c)
		}
	}
	return out
}
