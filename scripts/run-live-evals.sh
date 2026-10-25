#!/usr/bin/env bash
# Run live VibeSense eval suite (vector OFF and ON). Requires GEMINI_API_KEY in env or server/.env.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/server"

if [[ -z "${GEMINI_API_KEY:-}" ]] && [[ -f "$ROOT/server/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/server/.env"
  set +a
fi
if [[ -z "${GEMINI_API_KEY:-}" ]] && [[ -f "$ROOT/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/.env"
  set +a
fi
if [[ -z "${GEMINI_API_KEY:-}" ]]; then
  echo "GEMINI_API_KEY is not set (env or .env)" >&2
  exit 1
fi

mkdir -p "$ROOT/artifacts/live-evals"

export JUKE_EVALS_LIVE=1
: "${OPENSEARCH_URL:=http://localhost:9200}"
export OPENSEARCH_URL

OFF_EC=0
ON_EC=0

echo "=== Live evals: vector index OFF ==="
unset JUKE_EVALS_VECTOR
set +e
go test -tags evals_live -count=1 -timeout 120m ./evals/ -run TestVibeSenseEvalLive -write-reports -v 2>&1 | tee "$ROOT/artifacts/live-evals/off.log"
OFF_EC=${PIPESTATUS[0]}
set -e
cp evals/out/report.json "$ROOT/artifacts/live-evals/report-vector-off.json"
cp evals/out/report.md "$ROOT/artifacts/live-evals/report-vector-off.md"

echo "=== Live evals: vector index ON (full suite, same cases as OFF) ==="
export JUKE_EVALS_VECTOR=1
set +e
go test -tags evals_live -count=1 -timeout 120m ./evals/ -run TestVibeSenseEvalLiveVectorOn -write-reports -v 2>&1 | tee "$ROOT/artifacts/live-evals/on.log"
ON_EC=${PIPESTATUS[0]}
set -e
cp evals/out/report.json "$ROOT/artifacts/live-evals/report-vector-on.json"
cp evals/out/report.md "$ROOT/artifacts/live-evals/report-vector-on.md"

python3 "$ROOT/scripts/summarize-live-eval-report.py" \
  "$ROOT/artifacts/live-evals/report-vector-off.json" \
  "$ROOT/artifacts/live-evals/report-vector-on.json" \
  > "$ROOT/artifacts/live-evals/summary.md"

cat "$ROOT/artifacts/live-evals/summary.md"

FAIL=0
if [[ "$OFF_EC" -ne 0 ]]; then
  echo "vector OFF arm failed (exit $OFF_EC)" >&2
  FAIL=1
fi
if [[ "$ON_EC" -ne 0 ]]; then
  echo "vector ON arm failed (exit $ON_EC)" >&2
  FAIL=1
fi
exit "$FAIL"
