# Juke Spotify POC

[![CI](https://github.com/hippi345/juke-spotify-poc/actions/workflows/ci.yml/badge.svg)](https://github.com/hippi345/juke-spotify-poc/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A proof-of-concept **collaborative jukebox**: a Go API connects to Spotify, runs voting rounds on playlist tracks, refills playlists with AI-assisted suggestions (VibeSense / Gemini), and exposes a React web UI plus an optional Android patron client.

**Spotify use:** This project is for **personal, non-commercial demonstration only**. It is not licensed for commercial use, public performance, or shared venue playback beyond your own Spotify account terms. Do not deploy it as a product or multi-tenant service.

## Features

- **Spotify OAuth** — connect an account, pick a device, and control playback
- **Voting sessions** — guests vote on the next track from a curated candidate set
- **Playlist refill** — optional “similar vibe” track suggestions via Google Gemini
- **AI playlist jobs** — background jobs to build playlists from a text prompt
- **Web client** — React + Vite + Tailwind for the host UI
- **Email accounts** — staff (web host) and patrons (Android) register with email/password on the same API
- **Venue sessions** — staff set a venue location; sessions can be open or protected with a join password; patrons discover nearby active sessions and join before voting
- **Paid skip (demo)** — joined patrons pay **$1.00 USD** (Stripe **test mode**) to queue a playlist track next, ahead of the vote winner; only tracks already on the venue playlist are accepted
- **Android client** — Compose patron app: sign in, find nearby sessions, join, vote, paid skip (`mobile-client/`)

## Requirements

| Component | Version |
|-----------|---------|
| Go | 1.22+ (see `server/go.mod`) |
| Node.js | 22 LTS (see `client/.nvmrc`) |
| Docker | For local MySQL via Compose |
| Spotify app | [Developer Dashboard](https://developer.spotify.com/dashboard) credentials |
| Google AI (optional) | `GEMINI_API_KEY` for VibeSense / AI playlists |

## Quick start

### 1. Full stack (API + MySQL + web)

```bash
docker compose up -d --build
```

This starts MySQL, the Go API on **http://127.0.0.1:8081**, and the Vite dev host on **http://localhost:5173**. Compose loads **`docker-compose.env.example`** for placeholder env vars; copy it to **`docker-compose.env`** (gitignored) only when you want to override Spotify, Gemini, or Stripe settings locally.

**Spotify use:** Personal demo only — not for commercial use, shared venues, or multi-tenant deployment (see intro above).

MySQL only:

```bash
docker compose up -d mysql
# or: bash scripts/start-mysql.sh
```

### 2. API server

```bash
cd server
cp .env.example .env   # edit Spotify (and optional Gemini) values
go run .
```

Default API: **http://127.0.0.1:8081** (`PORT` overrides; avoid `8080` on Windows if another service uses it).

### 3. Web client

```bash
cd client
npm ci
npm run dev
```

Open **http://localhost:5173**. Copy `client/.env.example` to `client/.env` if the API is not at `http://127.0.0.1:8081`.

### 4. Android (optional)

See [mobile-client/README.md](mobile-client/README.md). Set `JUKE_API_BASE` in `gradle.properties` to your machine’s API URL (emulator default: `http://10.0.2.2:8081/`).

## Configuration

Secrets are **never** committed. Use environment variables or gitignored `.env` files.

### Server (`server/.env` or env)

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | `localhost` | MySQL host |
| `DB_PORT` | `3306` | MySQL port |
| `DB_USER` | `root` | MySQL user |
| `DB_PASSWORD` | `jukespotify` | MySQL password (local Docker only) |
| `DB_NAME` | `jukespotify` | Database name |
| `PORT` | `8081` | HTTP listen port |
| `SPOTIFY_CLIENT_ID` | — | Spotify app client ID |
| `SPOTIFY_CLIENT_SECRET` | — | Spotify app client secret |
| `SPOTIFY_REDIRECT_URI` | `http://127.0.0.1:5173/api/spotify/callback` | Must match Spotify app settings |
| `APP_BASE_URL` | `http://localhost:5173` | Post-auth redirect base |
| `GEMINI_API_KEY` | — | Google AI key for VibeSense / AI playlists |
| `GEMINI_MODEL` | `gemini-2.5-flash` | Optional model override |
| `AUTH_SECRET` | — | HMAC secret for staff/patron login tokens (required for email auth) |
| `STRIPE_TEST_SECRET_KEY` | — | Stripe **test** secret key for paid-skip checkout (set in your environment only; never commit) |
| `STRIPE_WEBHOOK_SECRET` | — | Stripe webhook signing secret for `POST /api/stripe/webhook` (environment only) |

### Client

| Variable | Default | Description |
|----------|---------|-------------|
| `VITE_API_URL` | `http://127.0.0.1:8081` | API base URL |

## Usage

1. Configure Spotify redirect URI: `http://127.0.0.1:5173/api/spotify/callback`
2. Start MySQL, server, and client as above
3. Register a **staff** account, create a **venue** (location), **Connect Spotify**, choose a device and playlist, then start a voting session (optional join password)
4. On Android, register a **patron** account, search **nearby** sessions, **join**, then vote or use **paid skip** on a playlist track (requires Stripe test keys on the API)

### API highlights

- `GET /health` — liveness
- `GET /api/spotify/login` — start OAuth
- `GET /api/spotify/status` — connection state
- `POST /api/auth/register` / `POST /api/auth/login` — email accounts (`role`: `staff` or `patron`)
- `POST /api/venues` — staff creates a venue with lat/lng
- `GET /api/venues/nearby?lat=&lng=` — patron discovers active sessions
- `POST /api/venues/:id/join` — patron joins (optional `join_password`)
- `POST /api/voting/session/start` — begin voting (staff: include `venue_id`, optional `join_password`)
- `GET /api/voting/state` — poll session / round state
- `GET /api/paid-skip/config` — paid-skip price and whether Stripe is configured
- `POST /api/paid-skip/checkout` — patron starts Stripe Checkout for a playlist `track_id` (must have joined the session)
- `POST /api/stripe/webhook` — Stripe webhook (forward test events with the Stripe CLI)
- `POST /api/ai-playlist/create` — start AI playlist job (requires Gemini)

## Development

### Tests

**Server** (SQLite in-memory; no Spotify or MySQL):

```bash
cd server
go test ./...
```

**Client**:

```bash
cd client
npm test
```

### CI

On push and pull requests to `main`, [`.github/workflows/ci.yml`](.github/workflows/ci.yml) runs:

| Job | What it does |
|-----|----------------|
| Go server | `go mod verify`, vet, golangci-lint, `go test -race` |
| React client | `npm ci`, lint, test, production build |
| Android mobile client | `./gradlew testDebugUnitTest assembleDebug` in `mobile-client/` (JDK 21, Android SDK via Actions) |

### Lint

```bash
cd client && npm run lint
cd server && golangci-lint run   # install: https://golangci-lint.run/welcome/install/
```

### Stack check (all services running)

```bash
./verify-stack.sh
```

## Project structure

```
juke-spotify-poc/
├── client/              # React + Vite web UI
├── server/              # Go + Gin API (vendored deps in vendor/)
├── mobile-client/       # Kotlin / Jetpack Compose Android app
├── scripts/             # MySQL / Docker helpers
├── docker-compose.yml   # MySQL + API + web (Vite)
├── .github/workflows/   # CI
└── SECURITY.md          # Vulnerability reporting
```

## License

[MIT](LICENSE) — Copyright (c) 2026 Joel Shearon
