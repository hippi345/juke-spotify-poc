#!/usr/bin/env python3
"""Set mean_targets in thresholds.json from two live report JSON files (lower run minus margin)."""
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MARGIN = 0.15


def means(doc):
    s = doc["summary"]
    return {
        "mood_intent": s["mood_fit_mean"],
        "crowd_fit": s["crowd_fit_mean"],
        "accuracy_coherence": s["coherence_mean"],
        "variety": s["variety_mean"],
        "resolve_rate": s.get("resolve_rate_mean", 0),
    }


def main():
    if len(sys.argv) != 3:
        print("usage: calibrate-thresholds-from-reports.py run1.json run2.json", file=sys.stderr)
        sys.exit(2)
    a = means(json.loads(Path(sys.argv[1]).read_text(encoding="utf-8")))
    b = means(json.loads(Path(sys.argv[2]).read_text(encoding="utf-8")))
    out = {k: round(min(a[k], b[k]) - MARGIN, 2) for k in a}
    out["resolve_rate"] = round(max(0.5, min(a["resolve_rate"], b["resolve_rate"]) - MARGIN), 2)
    th_path = ROOT / "server/evals/thresholds.json"
    th = json.loads(th_path.read_text(encoding="utf-8"))
    th["mean_targets"].update(out)
    th_path.write_text(json.dumps(th, indent=2) + "\n", encoding="utf-8")
    print("updated mean_targets:", out)


if __name__ == "__main__":
    main()
