# VibeSense eval suite

Table-driven quality gates for playlist generation, venue-history RAG grounding, and safety cases.

## Case matrix by feature

| Feature | Cases | Check type | Mode |
|---------|------:|------------|------|
| playlist_mood | 20 | deterministic+llm_judge | mock+live |
| venue_size | 4 | deterministic+llm_judge | mock+live |
| genre | 11 | deterministic+llm_judge | mock+live |
| more_like_this | 4 | deterministic+llm_judge | mock+live |
| crowd_vote | 4 | deterministic | mock+live |
| rag_grounding | 2 | deterministic | mock+live |
| degraded_input | 4 | deterministic | mock+live |
| adversarial_safety | 4 | deterministic | mock+live |

Counts are derived from `FeatureCatalog()` in `catalog.go` (run `go test ./evals/ -run TestCaseCatalog` to verify ≥50 total cases).

## Deterministic gate (mock CI)

- Tool sequence: single Gemini generation, then per-track Spotify search when a searcher is configured.
- Grounding: vector/RAG arms require ≥25% overlap with `venue_history_keys.json` when candidates are injected.
- Schema: every suggestion has non-empty `title` and `artist`.
- Error handling: nonsense/empty prompts fail cleanly; missing Spotify creds set resolve **N/A** (`ResolveSkipped`) without crashing.

## Live gate (`evals_live`)

- Tag: `go test -tags evals_live ./evals/`
- LLM judge runs 3× per case; **judge errors fail the suite** (no default 3/3/3/3).
- Mood rubric applies only when `Case.MoodRubric` is true (skipped for more-like-this cases).
- Vector ON/OFF: set `JUKE_EVALS_VECTOR=1` to inject venue candidates live; default OFF.

## Key leak guard

`TestKeyLeakGuard_*` scans eval reports and error strings for `AIza…`, `?key=`, `sk-…`, and `Bearer` token patterns. Repository CI also runs **gitleaks**.

## Commands

```bash
cd server
go test -short -count=1 ./evals/ -run 'TestVibeSenseEvalGateCI|TestKeyLeakGuard|TestVibeSenseVectorRegressionGate'
go test -count=1 ./evals/ -run TestVibeSenseEvalGate
JUKE_EVALS_LIVE=1 go test -tags evals_live -count=1 ./evals/ -run TestVibeSenseEvalLive -write-reports
JUKE_EVALS_VECTOR=1 JUKE_EVALS_LIVE=1 go test -tags evals_live -count=1 ./evals/ -run TestVibeSenseEvalLiveVectorOn -write-reports
```
