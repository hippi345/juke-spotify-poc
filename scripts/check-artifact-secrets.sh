#!/usr/bin/env bash
# Fail if eval reports/logs contain likely API key material.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
shopt -s nullglob
files=(
  "$ROOT"/artifacts/live-evals/*
  "$ROOT"/server/evals/out/report.json
  "$ROOT"/server/evals/out/report.md
)
found=0
for f in "${files[@]}"; do
  [[ -f "$f" ]] || continue
  if grep -qE 'AIza|(\?|&)key=|Bearer ' "$f" 2>/dev/null; then
    echo "secret pattern found in $f" >&2
    found=1
  fi
done
if [[ "$found" -ne 0 ]]; then
  exit 1
fi
echo "artifact secret scan: clean"
