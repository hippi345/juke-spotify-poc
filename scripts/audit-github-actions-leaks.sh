#!/usr/bin/env bash
# Download logs + artifacts for PR #46/#47/#48 runs; report run_id, file, first 4 chars of match only.
set -euo pipefail
REPO="hippi345/juke-spotify-poc"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/artifacts/leak-audit-github.txt"
DL="$ROOT/.leak-audit-downloads"
mkdir -p "$(dirname "$OUT")"
rm -rf "$DL"
mkdir -p "$DL"

report_match() {
  local run_id="$1"
  local file="$2"
  local kind="$3"
  local line="$4"
  local prefix
  prefix=$(echo "$line" | grep -oE 'AIza[0-9A-Za-z_-]{4}|key=[^&[:space:]"'\'']{0,4}' | head -1 | sed 's/key=/key=/' )
  if echo "$line" | grep -qE 'AIza[0-9A-Za-z_-]{35}'; then
    prefix=$(echo "$line" | grep -oE 'AIza[0-9A-Za-z_-]{4}' | head -1)
  elif echo "$line" | grep -qE '[?&]key='; then
    prefix="key="
  else
    return 0
  fi
  echo "run_id=$run_id kind=$kind file=$file prefix=${prefix}" >> "$OUT"
}

scan_file() {
  local run_id="$1"
  local path="$2"
  local kind="$3"
  [[ -f "$path" ]] || return 0
  while IFS= read -r line; do
    if echo "$line" | grep -qE 'AIza[0-9A-Za-z_-]{35}|[?&]key=[^[:space:]"'\'']+'; then
      report_match "$run_id" "$path" "$kind" "$line"
    fi
  done < "$path"
}

{
  echo "# GitHub Actions leak audit"
  echo "# PRs 46, 47, 48 — $(date -u +%Y-%m-%dT%H:%MZ)"
  echo ""
} > "$OUT"

for PR in 46 47 48; do
  echo "## PR #$PR" >> "$OUT"
  head_branch=$(gh pr view "$PR" --repo "$REPO" --json headRefName -q .headRefName 2>/dev/null || echo "")
  base_branch=$(gh pr view "$PR" --repo "$REPO" --json baseRefName -q .baseRefName 2>/dev/null || echo "")
  for branch in "$head_branch" "$base_branch"; do
    [[ -z "$branch" ]] && continue
    while read -r run_id; do
      [[ -z "$run_id" ]] && continue
      logdir="$DL/pr${PR}-run-${run_id}"
      mkdir -p "$logdir"
      gh run download "$run_id" --repo "$REPO" -D "$logdir/artifacts" 2>/dev/null || true
      gh run view "$run_id" --repo "$REPO" --log 2>/dev/null > "$logdir/full.log" || true
      scan_file "$run_id" "$logdir/full.log" "log"
      find "$logdir" -type f 2>/dev/null | while read -r f; do
        scan_file "$run_id" "$f" "artifact"
      done
    done < <(gh run list --repo "$REPO" --branch "$branch" --limit 50 --json databaseId -q '.[].databaseId' 2>/dev/null || true)
  done
  echo "" >> "$OUT"
done

if ! grep -q '^run_id=' "$OUT" 2>/dev/null; then
  echo "(no AIza{35} or key= matches in scanned logs/artifacts)" >> "$OUT"
fi
echo "Wrote $OUT"
