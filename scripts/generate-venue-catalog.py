#!/usr/bin/env python3
"""Deterministic shared venue catalog (3k–5k tracks) + planted sunset ground truth. No PII."""
import gzip
import hashlib
import json
import random
from datetime import datetime, timedelta, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
OUT_DIR = ROOT / "server/evals/testdata/vector"
SEED = 20261025
VENUE_ROOFTOP = 2
VENUE_OTHER = 99
TARGET = 4000
PLANTED_SUNSET = 50

POOL = [
    ("Blinding Lights", "The Weeknd", "pop", "upbeat", "mid"),
    ("One More Time", "Daft Punk", "house", "euphoric", "fast"),
    ("Get Lucky", "Daft Punk", "nu-disco", "sunset", "mid"),
    ("Titanium", "David Guetta", "edm", "anthem", "fast"),
    ("Heat Waves", "Glass Animals", "indie", "dreamy", "mid"),
    ("Coffee", "beabadoobee", "indie", "mellow", "slow"),
    ("Resonance", "HOME", "synthwave", "nostalgic", "mid"),
    ("Nightcall", "Kavinsky", "synthwave", "sunset", "mid"),
    ("Sunset Lover", "Petit Biscuit", "chill", "sunset", "slow"),
    ("Shelter", "Roosevelt", "nu-disco", "sunset", "mid"),
    ("Monte Carlo", "Roosevelt", "nu-disco", "warm", "mid"),
    ("Midnight City", "M83", "electronic", "night", "mid"),
    ("Redbone", "Childish Gambino", "funk", "smooth", "mid"),
    ("Electric Feel", "MGMT", "psychedelic", "groovy", "mid"),
    ("Mr. Brightside", "The Killers", "rock", "anthem", "fast"),
    ("Tennessee Whiskey", "Chris Stapleton", "country", "warm", "slow"),
    ("Levitating", "Dua Lipa", "disco-pop", "party", "fast"),
    ("Good 4 U", "Olivia Rodrigo", "pop-punk", "energetic", "fast"),
    ("As It Was", "Harry Styles", "pop", "breezy", "mid"),
    ("Flowers", "Miley Cyrus", "pop", "empowering", "mid"),
    ("Rooftop Session (Live at The Echo)", "Local Venue Regulars", "local-indie", "sunset", "mid"),
    ("Neon Patio", "Downtown Sound Collective", "local-nu-disco", "sunset", "mid"),
    ("Golden Hour (Venue Edit)", "Sunset Room DJs", "local-house", "sunset", "mid"),
    ("Echo Park Nights", "Silver Lake Rotation", "local-indie", "night", "mid"),
    ("Brunch Bell", "Patio Friends", "local-folk", "brunch", "slow"),
]

# Hard decoys: lexically sunset-adjacent but wrong daypart / vibe (not in planted ground truth).
DECOY_POOL = [
    ("Sunrise Brew", "Early Birds Collective", "acoustic", "morning-coffee", "slow"),
    ("Dawn Commute", "Kitchen Radio", "folk", "sunrise", "slow"),
    ("2AM Club Rush", "Nightfloor DJs", "techno", "late-night-club", "fast"),
    ("Warehouse Peak", "Afterhours Unit", "house", "late-night-club", "fast"),
    ("Rainy Evening Drizzle", "Umbrella Trio", "jazz", "rainy-evening", "slow"),
    ("Storm Window Blues", "Grey Sky Band", "blues", "rainy-evening", "mid"),
    ("Morning Pastry Hour", "Brunch Circuit", "indie", "morning-coffee", "slow"),
    ("Neon After Midnight", "Basement Pulse", "edm", "late-night-club", "fast"),
]


def spotify_id(title: str, artist: str) -> str:
    return hashlib.sha256(f"{title}|{artist}".encode()).hexdigest()[:22]


def key(title: str, artist: str) -> str:
    return f"{title.lower()}|{artist.lower()}"


def main():
    rng = random.Random(SEED)
    start = datetime(2024, 1, 1, tzinfo=timezone.utc)
    end = datetime(2025, 10, 1, tzinfo=timezone.utc)
    span = int((end - start).total_seconds())

    tracks = []
    planted_keys = []

    sparse_planted_idx = {3, 9, 17, 23, 31, 39, 47}

    for i in range(PLANTED_SUNSET):
        base = POOL[i % len(POOL)]
        title = f"{base[0]} (Rooftop Sunset Mix)" if i > len(POOL) else base[0]
        if i >= len(POOL):
            title = f"Golden Hour Track {i+1}"
        artist = base[1] if i < len(POOL) else "Sunset Room Residents"
        genre, mood, tempo = base[2], "sunset", base[4]
        if i in sparse_planted_idx:
            genre, mood, tempo = "", "", ""
        votes = rng.randint(800, 2500)
        plays = votes * rng.randint(3, 8)
        ts = start + timedelta(seconds=rng.randint(span // 2, span))
        sid = spotify_id(title, artist)
        k = key(title, artist)
        planted_keys.append(k)
        tracks.append({
            "spotify_id": sid,
            "venue_id": VENUE_ROOFTOP,
            "title": title,
            "artist": artist,
            "genres": genre,
            "mood": mood,
            "tempo": tempo,
            "play_count": plays,
            "vote_count": votes,
            "last_seen": ts.isoformat().replace("+00:00", "Z"),
            "planted_sunset": True,
        })

    # Hard decoys at rooftop with strong votes (compete with retrieval).
    for j in range(120):
        base = DECOY_POOL[j % len(DECOY_POOL)]
        title = f"{base[0]} #{j+1}"
        artist = base[1]
        genre, mood, tempo = base[2], base[3], base[4]
        votes = rng.randint(600, 2200)
        plays = votes * rng.randint(2, 7)
        ts = start + timedelta(seconds=rng.randint(0, span))
        tracks.append({
            "spotify_id": spotify_id(title, artist),
            "venue_id": VENUE_ROOFTOP,
            "title": title,
            "artist": artist,
            "genres": genre,
            "mood": mood,
            "tempo": tempo,
            "play_count": plays,
            "vote_count": votes,
            "last_seen": ts.isoformat().replace("+00:00", "Z"),
            "planted_sunset": False,
        })

    while len(tracks) < TARGET:
        base = POOL[rng.randint(0, len(POOL) - 1)]
        title, artist, genre, mood, tempo = base[0], base[1], base[2], base[3], base[4]
        venue = VENUE_ROOFTOP if rng.random() < 0.85 else VENUE_OTHER
        votes = rng.randint(1, 120)
        if mood in ("sunset", "chill", "nu-disco") and venue == VENUE_ROOFTOP:
            votes = int(votes * rng.uniform(1.2, 2.5))
        plays = votes * rng.randint(1, 10)
        ts = start + timedelta(seconds=rng.randint(0, span))
        tracks.append({
            "spotify_id": spotify_id(f"{title}-{len(tracks)}", artist),
            "venue_id": venue,
            "title": title,
            "artist": artist,
            "genres": genre,
            "mood": mood,
            "tempo": tempo,
            "play_count": plays,
            "vote_count": votes,
            "last_seen": ts.isoformat().replace("+00:00", "Z"),
            "planted_sunset": False,
        })

    highly = sorted(tracks, key=lambda t: t["vote_count"], reverse=True)[:300]
    highly_keys = [key(t["title"], t["artist"]) for t in highly]

    catalog = {
        "schema_version": 2,
        "generated_seed": SEED,
        "venue_rooftop_id": VENUE_ROOFTOP,
        "track_count": len(tracks),
        "planted_sunset_count": len(planted_keys),
        "planted_sunset_keys": planted_keys,
        "highly_voted_keys": highly_keys,
        "tracks": tracks,
    }

    OUT_DIR.mkdir(parents=True, exist_ok=True)
    raw = json.dumps(catalog, separators=(",", ":")).encode("utf-8")
    gz_path = OUT_DIR / "venue_catalog.json.gz"
    with gzip.open(gz_path, "wb") as f:
        f.write(raw)
    meta = {
        "path": "venue_catalog.json.gz",
        "track_count": len(tracks),
        "planted_sunset_keys": planted_keys,
        "highly_voted_keys": highly_keys,
        "regenerate": "python3 scripts/generate-venue-catalog.py",
    }
    (OUT_DIR / "venue_catalog_manifest.json").write_text(json.dumps(meta, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {gz_path} ({len(raw)/1024/1024:.2f} MB raw, {gz_path.stat().st_size/1024:.0f} KB gz)")
    print(f"planted_sunset={len(planted_keys)} decoys=120 highly_voted={len(highly_keys)}")


if __name__ == "__main__":
    main()
