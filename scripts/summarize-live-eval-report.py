#!/usr/bin/env python3
"""Print live eval rubric means and baseline gate diff (no secrets)."""
import json
import sys
from pathlib import Path

def load(path):
    with open(path, encoding="utf-8") as f:
        return json.load(f)

def main():
    off_path, on_path = sys.argv[1], sys.argv[2]
    off = load(off_path)
    on = load(on_path)
    baseline_path = Path(__file__).resolve().parents[1] / "server" / "evals" / "baseline.json"
    baseline = load(baseline_path)["full"]

    def block(label, doc):
        s = doc["summary"]
        g = doc.get("gate", {})
        print(f"## {label}")
        print(f"- Cases: {s.get('case_count')}")
        print(f"- Mood / crowd / variety / coherence: {s.get('mood_fit_mean'):.3f} / {s.get('crowd_fit_mean'):.3f} / {s.get('variety_mean'):.3f} / {s.get('coherence_mean'):.3f}")
        print(f"- Resolve mean: {s.get('resolve_rate_mean'):.3f} (NA={s.get('resolve_rate_na')})")
        print(f"- Venue overlap mean: {s.get('venue_overlap_mean', 0):.3f}")
        print(f"- Venue-history share mean: {s.get('venue_history_share_mean', 0):.3f}")
        print(f"- Hard pass count: {s.get('hard_pass_count')}")
        print(f"- Judge errors: {s.get('judge_error_count', 0)}")
        print(f"- Gate passed: {g.get('passed')}")
        if g.get("failures"):
            print("- Gate failures:")
            for f in g["failures"]:
                print(f"  - {f}")
        print()

    print("# Live eval summary\n")
    block("Vector index OFF", off)
    block("Vector index ON", on)
    print("## Baseline reference (mocked full)")
    print(f"- Mood / crowd / variety / coherence: {baseline['mood_fit_mean']:.3f} / {baseline['crowd_fit_mean']:.3f} / {baseline['variety_mean']:.3f} / {baseline['coherence_mean']:.3f}")
    print(f"- Resolve mean: {baseline['resolve_rate_mean']:.3f}")

if __name__ == "__main__":
    main()
