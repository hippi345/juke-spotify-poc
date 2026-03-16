# Juke Spotify POC

A proof-of-concept monorepo with a Go/Gin API server, React client, and MySQL for local development.

## Prerequisites

- **Go** 1.21+ (for server)
- **Node.js** 18+ (for client; Vite 5 requires it). Use `nvm use` in `client/` if you have nvm, or install from [nodejs.org](https://nodejs.org/)
- **Docker** (for MySQL)

### Docker in WSL

If `sudo service docker start` gives "unrecognized service":

**Option A – Start daemon manually:**
```bash
sudo dockerd &
```
Wait a few seconds, then run `docker ps` to verify.

**Option B – Use Docker Desktop (Windows):** Install [Docker Desktop](https://www.docker.com/products/docker-desktop/), enable WSL 2 integration, and start Docker Desktop. WSL will use it automatically.

**Option C – Enable systemd** (WSL 11.0+): Add `systemd=true` under `[boot]` in `/etc/wsl.conf`, run `wsl --shutdown`, reopen WSL, then `sudo systemctl start docker`.

## Quick Start

### 1. Start MySQL

**Option A – Docker Compose** (if `docker compose` is available):
```bash
docker compose up -d
```

**Option B – Plain Docker** (no Compose plugin):
```bash
bash scripts/start-mysql.sh
```

Wait for MySQL to be healthy (about 10–15 seconds). The database `jukespotify` is created automatically.

### 2. Run the Server

```bash
cd server
go mod tidy
go run .
```

Server runs at **http://localhost:8080**.

**Environment variables** (optional, defaults work with Docker Compose):

| Variable              | Default                              | Description                    |
|-----------------------|--------------------------------------|--------------------------------|
| DB_HOST               | localhost                            | MySQL host                     |
| DB_PORT               | 3306                                 | MySQL port                     |
| DB_USER               | root                                 | MySQL user                     |
| DB_PASSWORD           | jukespotify                          | MySQL password                 |
| DB_NAME               | jukespotify                          | Database name                  |
| PORT                  | 8080                                 | API server port                |
| SPOTIFY_CLIENT_ID     | (required for Spotify)               | From [Spotify Dashboard](https://developer.spotify.com/dashboard) |
| SPOTIFY_CLIENT_SECRET | (required for Spotify)               | From Spotify Dashboard         |
| SPOTIFY_REDIRECT_URI  | http://127.0.0.1:5173/api/spotify/callback | OAuth callback URL        |
| APP_BASE_URL          | http://localhost:5173                | Client URL for post-auth redirect |

### 3. Run the Client

```bash
cd client
npm install
npm run dev
```

Client runs at **http://localhost:5173**.

Optional: copy `.env.example` to `.env` and set `VITE_API_URL` if the API is not at `http://localhost:8080`.

## Project Structure

```
juke-spotify-poc/
├── server/           # Go + Gin API
├── client/           # React + Vite + Tailwind
├── docker-compose.yml
└── README.md
```

## Spotify Setup

1. Create an app at [Spotify Dashboard](https://developer.spotify.com/dashboard)
2. Add redirect URI: `http://127.0.0.1:5173/api/spotify/callback` (goes through Vite proxy to server)
3. Set `SPOTIFY_CLIENT_ID` and `SPOTIFY_CLIENT_SECRET` when running the server
4. Click "Connect Spotify" in the client to authorize

## API Endpoints

- `GET /health` – Health check
- `GET /api/placeholder` – Placeholder route for future implementation
- `GET /api/spotify/login` – Redirects to Spotify OAuth
- `GET /api/spotify/callback` – OAuth callback (handles token exchange)
- `GET /api/spotify/status` – Returns whether a Spotify account is connected
- `GET /api/spotify/me` – Fetches current user profile from Spotify (test endpoint)

## Testing

Run the API tests (no MySQL or Spotify credentials needed; uses SQLite in-memory):

```bash
cd server
go mod tidy
go test ./...
```

Tests cover health, placeholder, Spotify login (with/without credentials), callback (error, invalid state, no code), status (with/without account), and me (no account).
