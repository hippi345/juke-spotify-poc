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

scan_file() {
  local run_id="$1"
  local path="$2"
  [[ -f "$path" ]] || return 0
  local rel="${path#"$DL"/}"
  grep -aHnoE 'AIza[0-9A-Za-z_-]{35}|key=[^&[:space:]"'\'']+' "$path" 2>/dev/null | while IFS= read -r line; do
    local file="${line%%:*}"
    local match
    match=$(echo "$line" | sed 's/^[^:]*://')
    local prefix="${match:0:4}"
    local relfile="${file#"$DL"/}"
    echo "${run_id} ${relfile} ${prefix}" >> "$OUT"
  done
}

collect_run() {
  local run_id="$1"
  [[ -z "$run_id" ]] && return 0
  local logdir="$DL/run-${run_id}"
  mkdir -p "$logdir"
  gh run download "$run_id" --repo "$REPO" -D "$logdir/artifacts" 2>/dev/null || true
  gh run view "$run_id" --repo "$REPO" --log 2>/dev/null > "$logdir/full.log" || true
  scan_file "$run_id" "$logdir/full.log"
  find "$logdir" -type f 2>/dev/null | while read -r f; do
    scan_file "$run_id" "$f"
  done
}

{
  echo "# GitHub Actions leak audit (run_id file prefix)"
  echo "# PRs 46, 47, 48 — $(date -u +%Y-%m-%dT%H:%MZ)"
  echo ""
} > "$OUT"

declare -A SEEN_RUN=()

for PR in 46 47 48; do
  echo "## PR #$PR" >> "$OUT"
  head_branch=$(gh pr view "$PR" --repo "$REPO" --json headRefName -q .headRefName 2>/dev/null || echo "")
  base_branch=$(gh pr view "$PR" --repo "$REPO" --json baseRefName -q .baseRefName 2>/dev/null || echo "")
  branches=()
  [[ -n "$head_branch" ]] && branches+=("$head_branch")
  [[ -n "$base_branch" ]] && branches+=("$base_branch")
  for branch in "${branches[@]}"; do
    while read -r run_id; do
      [[ -z "$run_id" ]] && continue
      [[ -n "${SEEN_RUN[$run_id]:-}" ]] && continue
      SEEN_RUN[$run_id]=1
      collect_run "$run_id"
    done < <(gh run list --repo "$REPO" --branch "$branch" --limit 100 --json databaseId -q '.[].databaseId' 2>/dev/null || true)
  done
  while read -r sha; do
    [[ -z "$sha" ]] && continue
    while read -r run_id; do
      [[ -z "$run_id" ]] && continue
      [[ -n "${SEEN_RUN[$run_id]:-}" ]] && continue
      SEEN_RUN[$run_id]=1
      collect_run "$run_id"
    done < <(gh run list --repo "$REPO" --commit "$sha" --limit 50 --json databaseId -q '.[].databaseId' 2>/dev/null || true)
  done < <(gh pr view "$PR" --repo "$REPO" --json commits -q '.commits[].oid' 2>/dev/null || true)
  echo "" >> "$OUT"
done

if ! grep -qE '^[0-9]+ ' "$OUT" 2>/dev/null; then
  echo "(no AIza{35} or key= matches in scanned logs/artifacts)" >> "$OUT"
fi
echo "Wrote $OUT"
