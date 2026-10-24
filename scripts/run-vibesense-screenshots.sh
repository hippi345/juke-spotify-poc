#!/usr/bin/env bash
# Capture VibeSense UI screenshots (mock API + Vite + Playwright headless).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "Installing client dependencies…"
(cd client && npm ci)

echo "Installing screenshot script dependencies…"
(cd scripts && npm install --no-fund --no-audit)
(cd scripts && npx playwright install chromium)

MOCK_PID=""
VITE_PID=""
cleanup() {
  [[ -n "$VITE_PID" ]] && kill "$VITE_PID" 2>/dev/null || true
  [[ -n "$MOCK_PID" ]] && kill "$MOCK_PID" 2>/dev/null || true
}
trap cleanup EXIT

node scripts/mock-vibesense-api.mjs &
MOCK_PID=$!
for i in $(seq 1 30); do
  curl -sf "http://127.0.0.1:8081/health" >/dev/null && break
  sleep 0.2
done

(cd client && VITE_API_URL=http://127.0.0.1:8081 npm run dev -- --host 127.0.0.1 --port 5173) &
VITE_PID=$!
for i in $(seq 1 60); do
  curl -sf "http://127.0.0.1:5173/" >/dev/null && break
  sleep 0.5
done

export USE_MOCK_API=1
export MOCK_API_CONTROL_URL=http://127.0.0.1:8081/mock/control
node scripts/capture-vibesense-screenshots.mjs

echo "Screenshots written to artifacts/screenshots/"
