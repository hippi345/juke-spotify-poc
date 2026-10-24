# VibeSense UI screenshots (mock API)

Captured with `scripts/run-vibesense-screenshots.sh` (mock HTTP API on `:8081`, Vite on `:5173`, Playwright headless viewport 1280×800). No real API keys or Spotify credentials.

| File | Caption |
|------|---------|
| `01-mood-rainy-brunch-complete.png` | Mood playlist (rainy brunch): generation completes and success toast appears. |
| `02-venue-size-large-club-form.png` | Large-venue sizing: 22-track request for a high-capacity club mood. |
| `03-more-like-this-artists.png` | More-like-this: inspiration artists attached before generation. |
| `04-vector-index-on-similar-tracks.png` | Vector index ON: venue “More like this” returns semantic similar tracks. |
| `05-vector-index-off-degraded.png` | Vector index OFF (default): similar-track search degrades gracefully with an inline error. |
| `06-degraded-empty-description-error.png` | Degraded input: invalid generation request surfaces a clean API error (no crash). |
