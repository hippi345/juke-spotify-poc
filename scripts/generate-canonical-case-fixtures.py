#!/usr/bin/env python3
"""Write eval fixtures for rag_canonical_* cases from venue_catalog.json.gz."""
import gzip
import json
import os
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CATALOG = ROOT / "server/evals/testdata/vector/venue_catalog.json.gz"
CASES = ROOT / "server/evals/testdata/cases"

CASE_SPECS = [
    ("rag_canonical_sunset", "sunset"),
    ("rag_canonical_late_night", "night"),
    ("rag_canonical_sunday_brunch", "brunch"),
    ("rag_canonical_last_summer", "summer"),
    ("rag_canonical_rooftop_disco", "disco"),
]


def key(t, a):
    return f"{t.lower()}|{a.lower()}"


def main():
    with gzip.open(CATALOG, "rt", encoding="utf-8") as f:
        cat = json.load(f)
    roof = cat.get("venue_rooftop_id", 2)
    tracks = [t for t in cat["tracks"] if t["venue_id"] == roof]
    planted = [t for t in tracks if t.get("planted_sunset")]
    highly = sorted(tracks, key=lambda t: t["vote_count"], reverse=True)[:80]

    def pick_vector(n, mood_hint):
        pool = [t for t in highly if mood_hint in (t.get("mood") or "") + (t.get("genres") or "")]
        if len(pool) < n:
            pool = highly
        return pool[:n]

    judge_block = [
        [
            {"MoodFit": 4.3, "CrowdFit": 4.4, "Variety": 4.0, "Coherence": 4.2},
            {"MoodFit": 4.2, "CrowdFit": 4.3, "Variety": 4.1, "Coherence": 4.1},
            {"MoodFit": 4.4, "CrowdFit": 4.2, "Variety": 3.9, "Coherence": 4.3},
        ]
    ]

    for case_id, hint in CASE_SPECS:
        d = CASES / case_id
        d.mkdir(parents=True, exist_ok=True)
        n = 12 if case_id != "rag_canonical_late_night" else 14
        vec_pool = pick_vector(n, hint)
        if case_id == "rag_canonical_sunset":
            vec_pool = planted[:n] if len(planted) >= n else planted + highly[: max(0, n - len(planted))]
        vector_tracks = [{"title": t["title"], "artist": t["artist"]} for t in vec_pool[:n]]
        off_tracks = [
            {"title": f"Generic Off {i+1}", "artist": f"Outside Artist {i+1}"}
            for i in range(n)
        ]
        spotify = {}
        for i, tr in enumerate(vector_tracks):
            q = f"{tr['artist'].lower()} {tr['title'].lower()}"
            spotify[q] = [{
                "id": f"vec-{case_id}-{i}",
                "name": tr["title"],
                "artists": [{"name": tr["artist"]}],
                "uri": f"spotify:track:vec-{case_id}-{i}",
            }]
        for i, tr in enumerate(off_tracks):
            q = f"{tr['artist'].lower()} {tr['title'].lower()}"
            spotify[q] = [{
                "id": f"off-{case_id}-{i}",
                "name": tr["title"],
                "artists": [{"name": tr["artist"]}],
                "uri": f"spotify:track:off-{case_id}-{i}",
            }]
        (d / "gemini.json").write_text(json.dumps(off_tracks, indent=2) + "\n", encoding="utf-8")
        (d / "gemini_vector.json").write_text(json.dumps(vector_tracks, indent=2) + "\n", encoding="utf-8")
        (d / "spotify.json").write_text(json.dumps(spotify, indent=2) + "\n", encoding="utf-8")
        (d / "judge.json").write_text(json.dumps(judge_block, indent=2) + "\n", encoding="utf-8")
        (d / "judge_vector.json").write_text(json.dumps(judge_block, indent=2) + "\n", encoding="utf-8")
        print(f"wrote {case_id} ({n} tracks)")


if __name__ == "__main__":
    main()
