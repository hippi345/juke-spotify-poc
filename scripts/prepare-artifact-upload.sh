#!/usr/bin/env bash
# Redact likely secrets in-place, then gitleaks-scan every file before upload-artifact.
# Usage: prepare-artifact-upload.sh path/to/file [more paths...]
set -euo pipefail

if [[ "$#" -lt 1 ]]; then
  echo "usage: prepare-artifact-upload.sh <file-or-dir> ..." >&2
  exit 2
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
STAGE="${ARTIFACT_SCAN_DIR:-$ROOT/.artifact-upload-stage}"
rm -rf "$STAGE"
mkdir -p "$STAGE"

copy_tree() {
  local src="$1"
  local dest="$2"
  if [[ -f "$src" ]]; then
    mkdir -p "$(dirname "$dest")"
    cp -a "$src" "$dest"
  elif [[ -d "$src" ]]; then
    mkdir -p "$dest"
    cp -a "$src"/. "$dest"/
  else
    echo "missing path: $src" >&2
    exit 1
  fi
}

for arg in "$@"; do
  abs="$arg"
  if [[ "$arg" != /* ]]; then
    abs="$ROOT/$arg"
  fi
  base="$(basename "$abs")"
  copy_tree "$abs" "$STAGE/$base"
done

python3 - "$STAGE" <<'PY'
import pathlib, re, sys
stage = pathlib.Path(sys.argv[1])
patterns = [
    (re.compile(r"AIza[0-9A-Za-z_-]{35}"), "AIza[REDACTED]"),
    (re.compile(r"(?i)([?&]key=)[^&\s'\"]+"), r"\1[REDACTED]"),
    (re.compile(r"(?i)Bearer\s+[A-Za-z0-9._-]+"), "Bearer [REDACTED]"),
]
for path in stage.rglob("*"):
    if not path.is_file():
        continue
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        continue
    orig = text
    for rx, repl in patterns:
        text = rx.sub(repl, text)
    if text != orig:
        path.write_text(text, encoding="utf-8")
PY

if ! command -v gitleaks >/dev/null 2>&1; then
  curl -sSfL https://github.com/gitleaks/gitleaks/releases/download/v8.24.2/gitleaks_8.24.2_linux_x64.tar.gz | tar -xz -C /tmp
  GITLEAKS=/tmp/gitleaks
else
  GITLEAKS=gitleaks
fi

echo "gitleaks scan of staged artifact files..."
if ! "$GITLEAKS" detect --no-git --redact --source "$STAGE" --verbose; then
  echo "::error::gitleaks found secrets in artifact upload bundle" >&2
  exit 1
fi

# Fail on patterns gitleaks might miss (Google API key prefix).
if grep -rqE 'AIza[0-9A-Za-z_-]{10,}|(\?|&)key=[^[:space:]"'\''\]]+' "$STAGE" 2>/dev/null; then
  echo "::error::residual secret pattern after redaction" >&2
  exit 1
fi

# Replace originals with redacted copies for upload paths that live under repo.
for arg in "$@"; do
  abs="$arg"
  if [[ "$arg" != /* ]]; then
    abs="$ROOT/$arg"
  fi
  base="$(basename "$abs")"
  if [[ -f "$abs" ]]; then
    cp -a "$STAGE/$base" "$abs"
  fi
done

echo "artifact upload preparation: redacted and clean"
