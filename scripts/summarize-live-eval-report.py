#!/usr/bin/env python3
"""Print live eval rubric means, OFF vs ON deltas, and vector verdict (no secrets)."""
import json
import sys
from pathlib import Path

def load(path):
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def fmt_delta(off, on):
    d = on - off
    sign = "+" if d >= 0 else ""
    return f"{sign}{d:.3f}"


def vector_verdict(so, sn):
    mood_d = sn["mood_fit_mean"] - so["mood_fit_mean"]
    crowd_d = sn["crowd_fit_mean"] - so["crowd_fit_mean"]
    coh_d = sn["coherence_mean"] - so["coherence_mean"]
    var_d = sn["variety_mean"] - so["variety_mean"]
    overlap_d = sn.get("venue_overlap_mean", 0) - so.get("venue_overlap_mean", 0)
    share_d = sn.get("venue_history_share_mean", 0) - so.get("venue_history_share_mean", 0)
    resolve_d = 0.0
    if not so.get("resolve_rate_na") and not sn.get("resolve_rate_na"):
        resolve_d = sn["resolve_rate_mean"] - so["resolve_rate_mean"]

    rubric_ok = mood_d >= -0.1 and crowd_d >= -0.1 and coh_d >= -0.1 and var_d >= -0.1 and resolve_d >= -0.05
    rubric_hurt = mood_d < -0.15 or crowd_d < -0.15 or coh_d < -0.15 or resolve_d < -0.08
    grounding_up = overlap_d >= 0.08 or share_d >= 0.05

    if rubric_hurt:
        return "vector hurts", mood_d, crowd_d, coh_d, var_d, overlap_d, share_d, resolve_d
    if grounding_up and rubric_ok:
        return "vector helps", mood_d, crowd_d, coh_d, var_d, overlap_d, share_d, resolve_d
    return "vector neutral", mood_d, crowd_d, coh_d, var_d, overlap_d, share_d, resolve_d


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
        print(f"- Vector index: {'ON' if s.get('vector_on') else 'OFF'}")
        print(f"- Cases: {s.get('case_count')}")
        print(f"- Mood / crowd / variety / coherence: {s.get('mood_fit_mean'):.3f} / {s.get('crowd_fit_mean'):.3f} / {s.get('variety_mean'):.3f} / {s.get('coherence_mean'):.3f}")
        print(f"- Resolve mean: {s.get('resolve_rate_mean'):.3f} (NA={s.get('resolve_rate_na')})")
        print(f"- Venue overlap mean: {s.get('venue_overlap_mean', 0):.3f}")
        print(f"- Venue-history share mean: {s.get('venue_history_share_mean', 0):.3f}")
        print(f"- Hard pass count: {s.get('hard_pass_count')}")
        print(f"- Judge errors: {s.get('judge_error_count', 0)}")
        print(f"- Gate passed: {g.get('passed')} (OFF arm uses thresholds; ON arm hard checks only)")
        if g.get("failures"):
            print("- Gate failures:")
            for f in g["failures"]:
                print(f"  - {f}")
        print()

    print("# Live eval summary\n")
    block("Vector index OFF (default — thresholds gate)", off)
    block("Vector index ON (informational rubrics)", on)

    so, sn = off["summary"], on["summary"]
    verdict, mood_d, crowd_d, coh_d, var_d, overlap_d, share_d, resolve_d = vector_verdict(so, sn)
    print("## OFF vs ON deltas (ON − OFF)\n")
    print(f"- Mood/intent: {fmt_delta(so['mood_fit_mean'], sn['mood_fit_mean'])}")
    print(f"- Crowd fit: {fmt_delta(so['crowd_fit_mean'], sn['crowd_fit_mean'])}")
    print(f"- Accuracy/coherence: {fmt_delta(so['coherence_mean'], sn['coherence_mean'])}")
    print(f"- Variety: {fmt_delta(so['variety_mean'], sn['variety_mean'])}")
    if not so.get("resolve_rate_na") and not sn.get("resolve_rate_na"):
        print(f"- Resolve rate: {fmt_delta(so['resolve_rate_mean'], sn['resolve_rate_mean'])}")
    print(f"- Venue overlap mean: {fmt_delta(so.get('venue_overlap_mean', 0), sn.get('venue_overlap_mean', 0))}")
    print(f"- Venue-history share mean: {fmt_delta(so.get('venue_history_share_mean', 0), sn.get('venue_history_share_mean', 0))}")
    print(f"\n**Vector verdict: {verdict}**\n")

    print("## Baseline reference (mocked full)")
    print(f"- Mood / crowd / variety / coherence: {baseline['mood_fit_mean']:.3f} / {baseline['crowd_fit_mean']:.3f} / {baseline['variety_mean']:.3f} / {baseline['coherence_mean']:.3f}")
    print(f"- Resolve mean: {baseline['resolve_rate_mean']:.3f}")

if __name__ == "__main__":
    main()
