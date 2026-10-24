#!/usr/bin/env node
/**
 * Captures browser-viewport PNGs of VibeSense UI states (mocked API; no secrets).
 * Usage: node scripts/capture-vibesense-screenshots.mjs
 * Requires: Vite dev server on http://127.0.0.1:5173
 */
import { chromium } from 'playwright'
import { mkdir } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const outDir = path.join(__dirname, '..', 'artifacts', 'screenshots')
const baseURL = 'http://127.0.0.1:5173'
const useMockApi = process.env.USE_MOCK_API === '1'
const mockControlUrl = process.env.MOCK_API_CONTROL_URL || 'http://127.0.0.1:8081/mock/control'

async function setMockControl(vectorOn, jobFail) {
  if (!useMockApi) return
  const res = await fetch(mockControlUrl, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ vectorIndex: vectorOn, jobFail }),
  })
  if (!res.ok) {
    throw new Error(`mock control failed: ${res.status}`)
  }
}

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

function json(body, status = 200) {
  return {
    status,
    contentType: 'application/json',
    body: JSON.stringify(body),
  }
}

function installRoutes(page, { vectorOn, jobFail }) {
  return page.route('**/*', async (route) => {
    const url = route.request().url()
    const u = new URL(url)
    const p = u.pathname

    if (p === '/api/spotify/status') {
      return route.fulfill(json({ connected: true, display_name: 'Demo Host', spotify_id: 'demo' }))
    }
    if (p === '/api/auth/me') {
      return route.fulfill(json(demoUser))
    }
    if (p === '/api/venues/mine') {
      return route.fulfill(json(demoVenues))
    }
    if (p === '/api/voting/state') {
      return route.fulfill(json({ session: null, candidates: [], votes: {}, time_remaining_sec: 0 }))
    }
    if (p === '/api/spotify/devices') {
      return route.fulfill(json({ devices: [] }))
    }
    if (p.startsWith('/api/venues/1/tracks/similar')) {
      if (!vectorOn) {
        return route.fulfill(json({ error: 'Semantic search is not available for this venue' }, 503))
      }
      return route.fulfill(json(similarTracksOn))
    }
    if (p === '/api/ai-playlist/create' && route.request().method() === 'POST') {
      if (jobFail) {
        return route.fulfill(json({ error: 'description required' }, 400))
      }
      return route.fulfill({ ...json({ job_id: 'job-demo-1', status: 'pending' }), status: 202 })
    }
    if (p.startsWith('/api/ai-playlist/jobs/')) {
      if (jobFail) {
        return route.fulfill(json({ status: 'failed', error: 'description required' }))
      }
      return route.fulfill(
        json({
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
      )
    }
    if (url.includes('127.0.0.1:5173') || url.includes('localhost:5173')) {
      return route.continue()
    }
    return route.continue()
  })
}

async function prepPage(browser, opts) {
  const context = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 1,
  })
  await context.addInitScript(() => {
    localStorage.setItem('juke_staff_token', 'demo-screenshot-token-not-real')
  })
  const page = await context.newPage()
  if (useMockApi) {
    await setMockControl(opts.vectorOn, opts.jobFail)
  } else {
    await installRoutes(page, opts)
  }
  await page.goto(baseURL, { waitUntil: 'networkidle' })
  await page.getByRole('button', { name: 'Create playlist with AI VibeSense' }).click()
  return { context, page }
}

async function fillCommon(page, { name, count, desc, artists = [] }) {
  await page.locator('#ai-playlist-name').fill(name)
  await page.locator('#ai-track-count').fill(String(count))
  await page.locator('#ai-desc').fill(desc)
  for (const a of artists) {
    await page.getByPlaceholder('Artist name, then Add').fill(a)
    await page.getByRole('button', { name: 'Add' }).click()
  }
}

async function main() {
  await mkdir(outDir, { recursive: true })
  const browser = await chromium.launch({ headless: true })

  const shots = []

  // 1 Mood — rainy brunch
  {
    const { context, page } = await prepPage(browser, { vectorOn: false, jobFail: false })
    await fillCommon(page, {
      name: 'Rainy Brunch Mix',
      count: 15,
      desc: 'Rainy Sunday brunch — mellow acoustic and soft indie',
    })
    await page.getByRole('button', { name: 'Generate with VibeSense' }).click()
    await page.getByText('VibeSense playlist ready').waitFor({ timeout: 15000 })
    const file = '01-mood-rainy-brunch-complete.png'
    await page.screenshot({ path: path.join(outDir, file) })
    shots.push({ file, caption: 'Mood playlist (rainy brunch): generation completes and success toast appears.' })
    await context.close()
  }

  // 2 Venue size — large club
  {
    const { context, page } = await prepPage(browser, { vectorOn: false, jobFail: false })
    await fillCommon(page, {
      name: 'Peak Hour Club',
      count: 22,
      desc: '400-person nightclub — high-energy house and hip-hop mashups',
    })
    await page.screenshot({ path: path.join(outDir, '02-venue-size-large-club-form.png') })
    shots.push({
      file: '02-venue-size-large-club-form.png',
      caption: 'Large-venue sizing: 22-track request for a high-capacity club mood.',
    })
    await context.close()
  }

  // 3 More-like-this
  {
    const { context, page } = await prepPage(browser, { vectorOn: false, jobFail: false })
    await fillCommon(page, {
      name: 'Atmospheric Bar',
      count: 12,
      desc: 'Atmospheric rock for a rainy bar',
      artists: ['Radiohead', 'Portishead'],
    })
    await page.screenshot({ path: path.join(outDir, '03-more-like-this-artists.png') })
    shots.push({
      file: '03-more-like-this-artists.png',
      caption: 'More-like-this: inspiration artists attached before generation.',
    })
    await context.close()
  }

  // 4 Vector index ON — similar tracks
  {
    const { context, page } = await prepPage(browser, { vectorOn: true, jobFail: false })
    await fillCommon(page, {
      name: 'Study Lofi',
      count: 10,
      desc: 'Late-night study session — calm lofi focus',
    })
    await page.getByPlaceholder('Describe a track vibe or paste a track name').fill('calm lofi study')
    await page.getByRole('button', { name: 'Find similar' }).click()
    await page.getByText('Lofi Study Beat').waitFor({ timeout: 10000 })
    await page.getByText('Lofi Study Beat').scrollIntoViewIfNeeded()
    await page.screenshot({ path: path.join(outDir, '04-vector-index-on-similar-tracks.png') })
    shots.push({
      file: '04-vector-index-on-similar-tracks.png',
      caption: 'Vector index ON: venue “More like this” returns semantic similar tracks.',
    })
    await context.close()
  }

  // 5 Vector index OFF — degraded similar search
  {
    const { context, page } = await prepPage(browser, { vectorOn: false, jobFail: false })
    await fillCommon(page, {
      name: 'Study Lofi',
      count: 10,
      desc: 'Late-night study session — calm lofi focus',
    })
    await page.getByPlaceholder('Describe a track vibe or paste a track name').fill('calm lofi study')
    await page.getByRole('button', { name: 'Find similar' }).click()
    await page.getByText('Semantic search is not available').waitFor()
    await page.screenshot({ path: path.join(outDir, '05-vector-index-off-degraded.png') })
    shots.push({
      file: '05-vector-index-off-degraded.png',
      caption: 'Vector index OFF (default): similar-track search degrades gracefully with an inline error.',
    })
    await context.close()
  }

  // 6 Degraded playlist input (server rejects after client validation)
  {
    const { context, page } = await prepPage(browser, { vectorOn: false, jobFail: false })
    await setMockControl(false, true)
    await fillCommon(page, {
      name: 'Bad Input Test',
      count: 5,
      desc: 'asdf qwerty zxcv',
    })
    await page.getByRole('button', { name: 'Generate with VibeSense' }).click()
    await page.locator('form .text-red-400', { hasText: 'description required' }).waitFor()
    await page.screenshot({ path: path.join(outDir, '06-degraded-empty-description-error.png') })
    shots.push({
      file: '06-degraded-empty-description-error.png',
      caption: 'Degraded input: invalid generation request surfaces a clean API error (no crash).',
    })
    await context.close()
  }

  await browser.close()
  console.log(JSON.stringify({ outDir, shots }, null, 2))
}

main().catch((e) => {
  console.error(e)
  process.exit(1)
})
