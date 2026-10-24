#!/usr/bin/env python3
"""
Precompute 768-d catalog + eval-query embeddings for VibeSense retrieval tests.

CI uses these committed vectors (no API key). Generation uses a local open model;
production target is gemini-embedding-001 (768-d). Re-run with --provider gemini
once a new GEMINI_API_KEY is available to regenerate the same manifest layout.

Default: sentence-transformers/all-MiniLM-L6-v2 (384-d) zero-padded to 768-d.
"""
from __future__ import annotations

import argparse
import gzip
import hashlib
import json
import struct
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parents[1]
CATALOG = REPO / "server/evals/testdata/vector/venue_catalog.json.gz"
OUT_DIR = REPO / "server/evals/testdata/vector"
MANIFEST = OUT_DIR / "catalog_embedding_manifest.json"
VECTORS = OUT_DIR / "catalog_track_embeddings.f32"

# Keep in sync with evals/catalog_eval_queries.go
EVAL_QUERIES = [
    "crepuscule highway outing with companions",
    "amber sky relaxed social drive",
    "post-shift decompression on the veranda",
    "gentle unwinding after overtime ends",
    "breezy open-air gathering at dayfade",
    "convivial horizon cruise among companions",
    "homeward inbound commute calm-down playlist",
    "leisurely al fresco atmosphere at dayend",
    "coral radiance social ride with mates",
    "vesper veranda decompression interlude",
    "crimson sky companionable road trip",
    "soft glowing commute unwind with crew",
]

STORED_DIM = 768
PRODUCTION_MODEL = "gemini-embedding-001"


def track_embed_text(title: str, artist: str, genres: str, mood: str, tempo: str) -> str:
    meta = " ".join(x for x in (genres, mood, tempo) if x).strip()
    return f"{title} — {artist} — {meta}".strip()  # matches evals.CatalogTrackIndexText / vector.TrackEmbedText


def pad_to_768(vec: list[float], native_dim: int) -> list[float]:
    if len(vec) > STORED_DIM:
        raise ValueError(f"vector dim {len(vec)} exceeds stored dim {STORED_DIM}")
    if len(vec) == STORED_DIM:
        return vec
    return vec + [0.0] * (STORED_DIM - len(vec))


def catalog_sha256(path: Path) -> str:
    """SHA-256 of the committed .json.gz bytes on disk (matches Go verify)."""
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def embed_minilm(texts: list[str]):
    from sentence_transformers import SentenceTransformer

    model = SentenceTransformer("sentence-transformers/all-MiniLM-L6-v2")
    native = model.get_sentence_embedding_dimension()
    raw = model.encode(texts, normalize_embeddings=True, show_progress_bar=len(texts) > 32)
    out = [pad_to_768(v.tolist(), native) for v in raw]
    return out, f"sentence-transformers/all-MiniLM-L6-v2", native


def embed_gemini(texts: list[str], api_key: str):
    import urllib.request

    model = PRODUCTION_MODEL
    native = STORED_DIM
    out: list[list[float]] = []
    for text in texts:
        url = (
            f"https://generativelanguage.googleapis.com/v1beta/models/{model}:embedContent"
        )
        body = json.dumps(
            {
                "model": f"models/{model}",
                "content": {"parts": [{"text": text}]},
                "outputDimensionality": STORED_DIM,
            }
        ).encode()
        req = urllib.request.Request(
            url,
            data=body,
            headers={
                "Content-Type": "application/json",
                "x-goog-api-key": api_key,
            },
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=120) as resp:
            data = json.loads(resp.read())
        emb = data["embedding"]["values"]
        if len(emb) != STORED_DIM:
            raise SystemExit(f"unexpected dim {len(emb)}")
        out.append(emb)
    return out, model, native


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument(
        "--provider",
        choices=("minilm", "gemini"),
        default="minilm",
        help="minilm: local open model (default, no network). gemini: live API (requires key).",
    )
    args = ap.parse_args()

    if not CATALOG.is_file():
        print(f"missing catalog: {CATALOG}", file=sys.stderr)
        return 1

    with gzip.open(CATALOG, "rt", encoding="utf-8") as f:
        doc = json.load(f)
    tracks = doc["tracks"]

    track_texts = [
        track_embed_text(t["title"], t["artist"], t.get("genres", ""), t.get("mood", ""), t.get("tempo", ""))
        for t in tracks
    ]
    query_texts = list(EVAL_QUERIES)

    if args.provider == "gemini":
        import os

        key = os.environ.get("GEMINI_API_KEY", "").strip()
        if not key:
            print("GEMINI_API_KEY required for --provider gemini", file=sys.stderr)
            return 1
        track_vecs, model_name, native_dim = embed_gemini(track_texts, key)
        query_vecs, _, _ = embed_gemini(query_texts, key)
    else:
        all_vecs, model_name, native_dim = embed_minilm(track_texts + query_texts)
        track_vecs = all_vecs[: len(track_texts)]
        query_vecs = all_vecs[len(track_texts) :]

    OUT_DIR.mkdir(parents=True, exist_ok=True)
    with open(VECTORS, "wb") as f:
        for vec in track_vecs:
            if len(vec) != STORED_DIM:
                raise SystemExit("bad track vector dim")
            f.write(struct.pack(f"<{STORED_DIM}f", *vec))

    manifest = {
        "stored_dim": STORED_DIM,
        "production_target_model": PRODUCTION_MODEL,
        "generator_model": model_name,
        "generator_native_dim": native_dim,
        "pad_strategy": "zero_pad_to_stored_dim",
        "catalog_path": "venue_catalog.json.gz",
        "catalog_sha256": catalog_sha256(CATALOG),
        "num_tracks": len(tracks),
        "track_text": "title — artist — genres mood tempo (no eval hints)",
        "eval_queries": {q: query_vecs[i] for i, q in enumerate(EVAL_QUERIES)},
    }
    with open(MANIFEST, "w", encoding="utf-8") as f:
        json.dump(manifest, f, indent=2)
        f.write("\n")

    print(f"wrote {VECTORS} ({len(tracks)} x {STORED_DIM} float32)")
    print(f"wrote {MANIFEST} model={model_name} native_dim={native_dim}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
