# Juke Spotify POC

[![CI](https://github.com/hippi345/juke-spotify-poc/actions/workflows/ci.yml/badge.svg)](https://github.com/hippi345/juke-spotify-poc/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A proof-of-concept **collaborative jukebox**: a Go API connects to Spotify, runs voting rounds on playlist tracks, refills playlists with AI-assisted suggestions (VibeSense / Gemini), and exposes a React web UI plus an optional Android venue client.

## Features

- **Spotify OAuth** — connect an account, pick a device, and control playback
- **Voting sessions** — guests vote on the next track from a curated candidate set
- **Playlist refill** — optional “similar vibe” track suggestions via Google Gemini
- **AI playlist jobs** — background jobs to build playlists from a text prompt
- **Web client** — React + Vite + Tailwind for the host UI
- **Android client** — Compose app for venue displays (`mobile-client/`)

## Requirements

| Component | Version |
|-----------|---------|
| Go | 1.22+ (see `server/go.mod`) |
| Node.js | 22 LTS (see `client/.nvmrc`) |
| Docker | For local MySQL via Compose |
| Spotify app | [Developer Dashboard](https://developer.spotify.com/dashboard) credentials |
| Google AI (optional) | `GEMINI_API_KEY` for VibeSense / AI playlists |

## Quick start

### 1. MySQL

```bash
docker compose up -d
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

### Client

| Variable | Default | Description |
|----------|---------|-------------|
| `VITE_API_URL` | `http://127.0.0.1:8081` | API base URL |

## Usage

1. Configure Spotify redirect URI: `http://127.0.0.1:5173/api/spotify/callback`
2. Start MySQL, server, and client as above
3. In the UI, **Connect Spotify**, choose a device and playlist, then start a voting session

### API highlights

- `GET /health` — liveness
- `GET /api/spotify/login` — start OAuth
- `GET /api/spotify/status` — connection state
- `POST /api/voting/session/start` — begin voting
- `GET /api/voting/state` — poll session / round state
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
├── docker-compose.yml   # Local MySQL
├── .github/workflows/   # CI
└── SECURITY.md          # Vulnerability reporting
```

## License

[MIT](LICENSE) — Copyright (c) 2026 Joel Shearon
