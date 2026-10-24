#!/usr/bin/env python3
"""Print pass/fail table for live eval reports vs thresholds.json."""
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
    if not off_path.exists():
        print("missing report-vector-off.json", file=sys.stderr)
        sys.exit(1)
    off = load_report(off_path)
    on = load_report(on_path) if on_path.exists() else None
    so, go = off["summary"], off.get("gate", {})
    print("\n## Live eval vs thresholds (vector OFF)\n")
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
        print("\n## Vector ON arm\n")
        print(f"- venue overlap mean: {sn.get('venue_overlap_mean', 0):.3f} (target {TARGETS.get('venue_overlap_vector_on', 0):.2f})")
        print(f"- venue-history share: {sn.get('venue_history_share_mean', 0):.3f}")
    print()
    if not go.get("passed"):
        sys.exit(1)


if __name__ == "__main__":
    main()
