#!/usr/bin/env node
/**
 * Minimal mock HTTP API for VibeSense UI screenshots (no Gemini/Spotify keys).
 * Control plane: POST http://127.0.0.1:8081/mock/control
 *   { "vectorIndex": true|false, "jobFail": true|false }
 */
import http from 'node:http'

const PORT = Number(process.env.MOCK_API_PORT || 8081)

let vectorIndex = process.env.MOCK_VECTOR_INDEX === '1'
let jobFail = false

const demoUser = { user: { id: 1, email: 'demo@example.com', role: 'host' } }
const demoVenues = {
  venues: [{ id: 1, name: 'Demo Lounge', latitude: 40.7, longitude: -74.0 }],
}
const similarTracksOn = {
  tracks: [
    { spotify_id: 't1', title: 'Lofi Study Beat', artist: 'Venue Regular', score: 0.92 },
    { spotify_id: 't2', title: 'Midnight Focus', artist: 'Venue Regular', score: 0.88 },
    { spotify_id: 't3', title: 'Rainy Window', artist: 'Venue Regular', score: 0.85 },
  ],
}

function send(res, status, body) {
  const raw = JSON.stringify(body)
  res.writeHead(status, { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(raw) })
  res.end(raw)
}

async function readBody(req) {
  const chunks = []
  for await (const c of req) chunks.push(c)
  if (chunks.length === 0) return {}
  try {
    return JSON.parse(Buffer.concat(chunks).toString('utf8'))
  } catch {
    return {}
  }
}

const server = http.createServer(async (req, res) => {
  const u = new URL(req.url || '/', `http://127.0.0.1:${PORT}`)
  const p = u.pathname

  if (p === '/health') {
    return send(res, 200, { status: 'ok', mock: true })
  }
  if (p === '/mock/control' && req.method === 'POST') {
    const body = await readBody(req)
    if (typeof body.vectorIndex === 'boolean') vectorIndex = body.vectorIndex
    if (typeof body.jobFail === 'boolean') jobFail = body.jobFail
    return send(res, 200, { vectorIndex, jobFail })
  }
  if (p === '/api/spotify/status') {
    return send(res, 200, { connected: true, display_name: 'Demo Host', spotify_id: 'demo' })
  }
  if (p === '/api/auth/me') {
    return send(res, 200, demoUser)
  }
  if (p === '/api/venues/mine') {
    return send(res, 200, demoVenues)
  }
  if (p === '/api/voting/state') {
    return send(res, 200, { session: null, candidates: [], votes: {}, time_remaining_sec: 0 })
  }
  if (p === '/api/spotify/devices') {
    return send(res, 200, { devices: [] })
  }
  if (p.startsWith('/api/venues/1/tracks/similar')) {
    if (!vectorIndex) {
      return send(res, 503, { error: 'Semantic search is not available for this venue' })
    }
    return send(res, 200, similarTracksOn)
  }
  if (p === '/api/ai-playlist/create' && req.method === 'POST') {
    const body = await readBody(req)
    const desc = String(body.description || '')
    if (jobFail || desc.trim() === '') {
      return send(res, 400, { error: 'description required' })
    }
    return send(res, 202, { job_id: 'job-demo-1', status: 'pending' })
  }
  if (p.startsWith('/api/ai-playlist/jobs/')) {
    if (jobFail) {
      return send(res, 200, { status: 'failed', error: 'description required' })
    }
    return send(res, 200, {
      status: 'completed',
      result: {
        playlist_id: 'demo-playlist-id',
        playlist_url: 'https://open.spotify.com/playlist/demo-playlist-id',
        name: 'Demo VibeSense Mix',
        tracks_added: 12,
        tracks_requested: 12,
        ai_suggestions: 12,
      },
    })
  }
  send(res, 404, { error: 'not found', path: p })
})

server.listen(PORT, '127.0.0.1', () => {
  console.log(`mock vibesense api listening on http://127.0.0.1:${PORT}`)
})
