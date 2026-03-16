import { useEffect, useState } from 'react'

type SpotifyStatus = {
  connected: boolean
  display_name?: string
  spotify_id?: string
}

function App() {
  const apiUrl = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'
  const [health, setHealth] = useState<{ status: string; ok: boolean } | null>(null)
  const [spotify, setSpotify] = useState<SpotifyStatus | null>(null)
  const [spotifyError, setSpotifyError] = useState<string | null>(null)
  const spotifyParam = new URLSearchParams(window.location.search).get('spotify')

  const handleConnectSpotify = async () => {
    setSpotifyError(null)
    try {
      const res = await fetch('/api/spotify/login', {
        headers: { Accept: 'application/json' },
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        setSpotifyError(data.error || `Error ${res.status}`)
        return
      }
      if (data.url) {
        window.location.href = data.url
      }
    } catch (e) {
      setSpotifyError(e instanceof Error ? e.message : 'Failed to connect')
    }
  }

  useEffect(() => {
    fetch('/health')
      .then((r) => r.json())
      .then((data) => setHealth({ ...data, ok: true }))
      .catch(() => setHealth({ status: 'unreachable', ok: false }))
  }, [])

  useEffect(() => {
    fetch('/api/spotify/status')
      .then((r) => r.json())
      .then(setSpotify)
      .catch(() => setSpotify({ connected: false }))
  }, [spotifyParam])

  return (
    <div className="min-h-screen bg-gradient-to-br from-[#0a0a0b] via-[#0f0f12] to-[#141416]">
      <main className="mx-auto max-w-4xl px-6 py-20">
        <div className="rounded-2xl border border-white/5 bg-white/[0.02] p-12 shadow-2xl backdrop-blur-sm">
          <h1 className="mb-2 text-4xl font-semibold tracking-tight text-white">
            Juke Spotify POC
          </h1>
          <p className="mb-8 text-lg text-zinc-400">
            Skeleton app ready for implementation
          </p>
          <div className="space-y-4 rounded-xl bg-black/30 p-6">
            <p className="text-sm text-zinc-500">
              API health: {health ? (
                <span className={health.ok ? 'text-green-400' : 'text-red-400'}>
                  {health.ok ? `✓ ${health.status}` : health.status}
                </span>
              ) : (
                <span className="text-zinc-500">checking...</span>
              )}
            </p>
            <p className="text-sm text-zinc-500">
              Spotify: {spotify?.connected ? (
                <span className="text-green-400">
                  ✓ Connected as {spotify.display_name || spotify.spotify_id || 'Spotify user'}
                </span>
              ) : (
                <>
                  <button
                    type="button"
                    onClick={handleConnectSpotify}
                    className="inline-flex items-center gap-2 rounded-lg bg-[#1DB954] px-4 py-2 text-sm font-medium text-white transition hover:bg-[#1ed760]"
                  >
                    Connect Spotify
                  </button>
                  {spotifyError && (
                    <p className="text-sm text-red-400 mt-2">{spotifyError}</p>
                  )}
                </>
              )}
            </p>
            {spotifyParam === 'connected' && (
              <p className="text-sm text-green-400">Spotify connected successfully.</p>
            )}
            <p className="text-sm text-zinc-500">
              API base: <code className="rounded bg-white/10 px-2 py-1 font-mono text-zinc-300">{apiUrl}</code>
            </p>
            <p className="text-sm text-zinc-500">
              Client dev server: <code className="rounded bg-white/10 px-2 py-1 font-mono text-zinc-300">localhost:5173</code>
            </p>
          </div>
        </div>
      </main>
    </div>
  );
}

export default App;
