#!/usr/bin/env bash
# Safe leak audit: report file locations only; print at most first 4 chars of AIza matches.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="${1:-$ROOT/artifacts/leak-audit-report.txt}"
mkdir -p "$(dirname "$OUT")"

mask_line() {
  sed -E 's/AIza[0-9A-Za-z_-]+/AIza****/g; s/([?&]key=)[^&[:space:]"'\''\]]+/\1****/g'
}

{
  echo "# Gemini leak audit $(date -u +%Y-%m-%dT%H:%MZ)"
  echo ""
  echo "## Git tree (tracked files, excluding vendor)"
  if git -C "$ROOT" grep -n -E 'AIza[0-9A-Za-z_-]{8,}|(\?|&)key=' -- ':!**/vendor/**' ':!**/*.gz' 2>/dev/null | mask_line | head -200; then
    :
  else
    echo "(no matches in tracked tree)"
  fi
  echo ""
  echo "## Git history (all branches, pickaxe AIza, paths only)"
  git -C "$ROOT" log --all --oneline -S'AIza' --pretty=format:'%h %s' 2>/dev/null | head -50 || true
  echo ""
} > "$OUT"

AUDIT_DIR="$ROOT/.leak-audit-downloads"
rm -rf "$AUDIT_DIR"
mkdir -p "$AUDIT_DIR"

for PR in 46 47 48; do
  echo "## PR #$PR workflow runs" >> "$OUT"
  runs=$(gh pr view "$PR" --repo hippi345/juke-spotify-poc --json statusCheckRollup -q '.statusCheckRollup[].detailsUrl' 2>/dev/null | grep -oE '[0-9]{11}' | sort -u || true)
  gh run list --repo hippi345/juke-spotify-poc --limit 30 --json databaseId,headBranch,event,workflowName,conclusion 2>/dev/null | \
    python3 -c "import json,sys; pr=$PR; data=json.load(sys.stdin); [print(r['databaseId'], r.get('workflowName','')) for r in data]" >> "$OUT" 2>/dev/null || true
  for run_id in $(gh run list --repo hippi345/juke-spotify-poc --limit 40 --json databaseId,headBranch -q '.[] | select(.headBranch | test("vibesense|eval")) | .databaseId' 2>/dev/null | head -15); do
    logdir="$AUDIT_DIR/run-$run_id"
    mkdir -p "$logdir"
    gh run download "$run_id" --repo hippi345/juke-spotify-poc -D "$logdir" 2>/dev/null || true
    gh run view "$run_id" --repo hippi345/juke-spotify-poc --log 2>/dev/null | mask_line | grep -nE 'AIza|key=' | head -20 | while read -r line; do
      echo "PR#$PR run $run_id log: $line" >> "$OUT"
    done || true
    if [[ -d "$logdir" ]]; then
      grep -rnE 'AIza[0-9A-Za-z_-]{8,}|(\?|&)key=' "$logdir" 2>/dev/null | mask_line | head -30 | while read -r line; do
        echo "PR#$PR run $run_id artifact: $line" >> "$OUT"
      done || true
    fi
  done
done

echo "Wrote $OUT"
