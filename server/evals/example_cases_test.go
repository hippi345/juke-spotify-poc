package evals

import "testing"

// TestExampleEvalCases runs documented examples from server/evals/README.md (one subtest per case).
func TestExampleEvalCases(t *testing.T) {
	t.Run("venue_crowd_lofi_grounding_vector_on", func(t *testing.T) {
		runExampleMock(t, "venue_crowd_lofi", true, func(r CaseResult) {
			if !r.Hard.ToolSequenceOK {
				t.Fatal("expected generate-then-resolve tool sequence")
			}
			if !r.Hard.GroundingOK || r.Hard.VenueOverlap < 0.25 {
				t.Fatalf("grounding: overlap=%.3f grounding_ok=%v", r.Hard.VenueOverlap, r.Hard.GroundingOK)
			}
			t.Logf("venue_overlap=%.3f resolve_rate=%.3f", r.Hard.VenueOverlap, r.Hard.ResolveRate)
		})
	})

	t.Run("rainy_sunday_brunch_llm_judge_mock", func(t *testing.T) {
		runExampleMock(t, "rainy_sunday_brunch", false, func(r CaseResult) {
			t.Logf("judge mean mood=%.2f crowd=%.2f variety=%.2f coherence=%.2f",
				r.Judge.MoodFit, r.Judge.CrowdFit, r.Judge.Variety, r.Judge.Coherence)
			if r.Judge.MoodFit < 3.5 {
				t.Fatalf("unexpected low mood_fit %.2f", r.Judge.MoodFit)
			}
		})
	})

	t.Run("inject_ignore_instructions_adversarial", func(t *testing.T) {
		runExampleMock(t, "inject_ignore_instructions", false, func(r CaseResult) {
			if !r.Hard.JSONOK || !r.Hard.SchemaOK {
				t.Fatal("adversarial prompt must still return valid schema")
			}
			t.Logf("tracks=%d resolve_rate=%.2f", r.Hard.ActualCount, r.Hard.ResolveRate)
		})
	})

	t.Run("spotify_missing_creds_resolve_na", func(t *testing.T) {
		runExampleMock(t, "spotify_missing_creds", false, func(r CaseResult) {
			if !r.Hard.ResolveSkipped {
				t.Fatal("expected ResolveSkipped when ForceResolveNA (no Spotify searcher)")
			}
		})
	})
}

func runExampleMock(t *testing.T, caseID string, vectorOn bool, assert func(CaseResult)) {
	c := caseByID(caseID)
	if c == nil {
		t.Fatalf("case %s not in AllCases()", caseID)
	}
	store := DefaultFixtureStore()
	keys := loadVenueHistoryKeys()
	cfg := RunConfig{
		Generator: FixtureGenerator{Store: store, VectorOn: vectorOn},
		Searcher:  FixtureSearcher{Store: store},
		Judge:     &perCaseFixtureJudge{Store: store},
		VenueKeys: keys,
		VectorOn:  vectorOn,
	}
	if vectorOn && c.VenueID > 0 {
		cfg.InjectVenueCandidates = true
		cfg.MockVenueCandidates = loadMockVenueCandidates(store)
	}
	results, err := RunSuiteCases(cfg, []Case{*c})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	assert(results[0])
}
