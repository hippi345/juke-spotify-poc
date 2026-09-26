# Security Policy

## Supported versions

Security fixes are applied on the `main` branch. There are no long-term release branches for this proof-of-concept repository.

## Reporting a vulnerability

If you discover a security issue, please **do not** open a public GitHub issue with exploit details.

Contact the repository owner privately (for example via GitHub Security Advisories on this repo, or direct message if you already have a channel). Include:

- A description of the issue and impact
- Steps to reproduce
- Any suggested mitigation

We will acknowledge reports as soon as possible and work on a fix.

## Secrets and credentials

- Never commit API keys, OAuth client secrets, or database passwords. Use environment variables or local `.env` files (see `server/.env.example` and `client/.env.example`).
- The Docker Compose file uses a **local-only** MySQL password (`jukespotify`) suitable for development; do not reuse it in production.
- If you believe a secret was ever committed to git history, rotate it immediately in the provider dashboard (Spotify, Google AI, etc.) even after the secret is removed from the tree.
