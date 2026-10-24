#!/usr/bin/env python3
"""Print pass/fail table for vector OFF arm vs thresholds; ON arm informational."""
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
TH = json.loads((ROOT / "server/evals/thresholds.json").read_text(encoding="utf-8"))
TARGETS = TH["mean_targets"]


def load_report(path):
    return json.loads(Path(path).read_text(encoding="utf-8"))


def row(label, value, target, ok):
    status = "PASS" if ok else "FAIL"
    print(f"| {label} | {value} | {target} | {status} |")


def main():
    off_path = ROOT / "artifacts/live-evals/report-vector-off.json"
    on_path = ROOT / "artifacts/live-evals/report-vector-on.json"
    summary_path = ROOT / "artifacts/live-evals/summary.md"
    if not off_path.exists():
        print("missing report-vector-off.json", file=sys.stderr)
        sys.exit(1)
    off = load_report(off_path)
    on = load_report(on_path) if on_path.exists() else None
    so, go = off["summary"], off.get("gate", {})
    print("\n## Live eval vs thresholds (vector OFF only)\n")
    print("| Metric | Value | Target | Status |")
    print("|--------|-------|--------|--------|")
    row("mood/intent", f"{so['mood_fit_mean']:.3f}", f"{TARGETS['mood_intent']:.2f}", so["mood_fit_mean"] >= TARGETS["mood_intent"])
    row("crowd fit", f"{so['crowd_fit_mean']:.3f}", f"{TARGETS['crowd_fit']:.2f}", so["crowd_fit_mean"] >= TARGETS["crowd_fit"])
    row("accuracy/coherence", f"{so['coherence_mean']:.3f}", f"{TARGETS['accuracy_coherence']:.2f}", so["coherence_mean"] >= TARGETS["accuracy_coherence"])
    row("variety", f"{so['variety_mean']:.3f}", f"{TARGETS['variety']:.2f}", so["variety_mean"] >= TARGETS["variety"])
    if not so.get("resolve_rate_na"):
        row("resolve rate", f"{so['resolve_rate_mean']:.3f}", f"{TARGETS['resolve_rate']:.2f}", so["resolve_rate_mean"] >= TARGETS["resolve_rate"])
    row("gate (baseline+thresholds)", str(go.get("passed")), "true", bool(go.get("passed")))
    row("judge errors", str(so.get("judge_error_count", 0)), "0", so.get("judge_error_count", 0) == 0)
    if on:
        sn = on["summary"]
        gn = on.get("gate", {})
        print("\n## Vector ON arm (informational — hard gate only)\n")
        print(f"- Hard gate passed: {gn.get('passed')}")
        print(f"- Mood / crowd / variety / coherence: {sn['mood_fit_mean']:.3f} / {sn['crowd_fit_mean']:.3f} / {sn['variety_mean']:.3f} / {sn['coherence_mean']:.3f}")
        print(f"- Venue overlap: {sn.get('venue_overlap_mean', 0):.3f}")
        print(f"- Venue-history share: {sn.get('venue_history_share_mean', 0):.3f}")
    if summary_path.exists():
        print("\n(See artifacts/live-evals/summary.md for OFF vs ON deltas and vector verdict.)\n")
    print()
    if not go.get("passed"):
        sys.exit(1)
    if on and not on.get("gate", {}).get("passed"):
        sys.exit(1)


if __name__ == "__main__":
    main()
