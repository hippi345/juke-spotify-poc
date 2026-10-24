# Live eval thresholds (`thresholds.json`)

Committed targets for **live** VibeSense runs (`JUKE_EVALS_LIVE=1`, `-tags evals_live`). Mocked CI continues to use `baseline.json` only.

## How thresholds were picked

1. Run **two** full live suites (vector index **OFF**) with **`JUKE_EVALS_JUDGE_RUNS=2`** so each case is judged twice and scores are averaged (less variance than a single judge call).
2. Record mean mood/intent (`mood_fit`), crowd fit, accuracy/coherence (`coherence`), variety, and resolve rate from each run.
3. Set **mean targets** to roughly **0.15 below the lower of the two run means** (conservative margin).
4. Set **per-case floors** to **2.0** on each judge dimension so one bad case cannot hide behind a high average.
5. **Regression tolerance** matches the mocked baseline gate: up to **0.2** drop on judge means and **0.05** on resolve rate vs `baseline.json` `full`, and **no** drop in hard-pass count.
6. **Hard gates** (always on): per-case schema, grounding (when vector injection applies), tool sequence, and key-leak scans on reports.

Re-calibrate after changing prompts, models, or case matrix:

```bash
export JUKE_EVALS_LIVE=1 JUKE_EVALS_JUDGE_RUNS=2 GEMINI_API_KEY=...
for i in 1 2; do
  go test -tags evals_live -count=1 ./evals/ -run TestVibeSenseEvalLive -write-reports
  cp evals/out/report.json /tmp/live-cal-$i.json
done
# Update thresholds.json mean_targets from the lower run means minus margin.
```

## Rubric mapping

| Threshold field | Judge / metric | Intent |
|-----------------|----------------|--------|
| `mood_intent` | `mood_fit` | Prompt / vibe match |
| `crowd_fit` | `crowd_fit` | Venue / audience fit |
| `accuracy_coherence` | `coherence` | Sensible, non-contradictory picks |
| `variety` | `variety` | Track diversity |
| `resolve_rate` | Spotify resolve | Real tracks found |
| Grounding hard gate | `grounding_ok` | No invented venue/RAG phantoms (no false catalog claims) |
