import { useCallback, useEffect, useState } from 'react'
import { DevicePicker } from './components/DevicePicker'
import { NowPlaying } from './components/NowPlaying'
import { PlaylistOverview } from './components/PlaylistOverview'
import { SessionControls } from './components/SessionControls'
import { VotingRound } from './components/VotingRound'
import { useVotingState } from './hooks/useVotingState'

type SpotifyStatus = {
  connected: boolean
  display_name?: string
  spotify_id?: string
  active_device_id?: string | null
}

function App() {
  const [spotify, setSpotify] = useState<SpotifyStatus | null>(null)
  const [spotifyError, setSpotifyError] = useState<string | null>(null)
  const [disconnecting, setDisconnecting] = useState(false)
  const searchParams = new URLSearchParams(window.location.search)
  const spotifyParam = searchParams.get('spotify')
  const spotifyHint = searchParams.get('hint')

  const fetchSpotifyStatus = useCallback(() => {
    return fetch('/api/spotify/status')
      .then((r) => r.json())
      .then(setSpotify)
      .catch(() => setSpotify({ connected: false }))
  }, [])

  const { state, error: stateError, refetch } = useVotingState(true)

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

  const handleSetDevice = useCallback(async (deviceId: string) => {
    const res = await fetch('/api/spotify/device', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ device_id: deviceId }),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new Error((data as { error?: string }).error || 'Failed to set device')
    }
    await fetchSpotifyStatus()
  }, [fetchSpotifyStatus])

  const handleDisconnectSpotify = async () => {
    setSpotifyError(null)
    setDisconnecting(true)
    try {
      const res = await fetch('/api/spotify/disconnect', { method: 'POST' })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        setSpotifyError((data as { error?: string }).error || 'Failed to disconnect')
        return
      }
      await fetchSpotifyStatus()
    } catch (e) {
      setSpotifyError(e instanceof Error ? e.message : 'Failed to disconnect')
    } finally {
      setDisconnecting(false)
    }
  }

  const handleSessionStart = useCallback(
    async (playlistId: string, playlistName: string, refillThreshold: number) => {
      const res = await fetch('/api/voting/session/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          playlist_id: playlistId,
          playlist_name: playlistName,
          refill_threshold: refillThreshold,
        }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        throw new Error(data.error || `Error ${res.status}`)
      }
      refetch()
    },
    [refetch]
  )

  const handleSessionEnd = useCallback(async () => {
    const res = await fetch('/api/voting/session/end', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    })
    if (!res.ok) {
      throw new Error('Failed to end session')
    }
    refetch()
  }, [refetch])

  const handleVote = useCallback(
    async (trackId: string) => {
      const res = await fetch('/api/voting/vote', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ track_id: trackId }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        throw new Error(data.error || 'Vote failed')
      }
      refetch()
    },
    [refetch]
  )

  useEffect(() => {
    fetchSpotifyStatus()
  }, [spotifyParam, fetchSpotifyStatus])

  const sessionActive = state?.session?.status === 'active'
  const nowPlaying = state?.now_playing

  return (
    <div className="min-h-screen bg-gradient-to-br from-[#050506] via-[#0c0c0e] to-[#12121a]">
      <main className="mx-auto max-w-4xl px-6 py-12">
        <div className="mb-8">
          <h1 className="mb-2 text-4xl font-semibold tracking-tight text-white/95">
            Juke Spotify POC
          </h1>
          <p className="text-lg text-zinc-400/90">
            Vote for the next song. Winner gets added to the queue.
          </p>
        </div>

        <div className="space-y-6">
          {/* Spotify connection */}
          <div className="glass-panel p-6">
            <p className="text-sm text-zinc-500">
              Spotify:{' '}
              {spotify?.connected ? (
                <span className="flex flex-wrap items-center gap-3">
                  <span className="text-green-400">
                    ✓ Connected as {spotify.display_name || spotify.spotify_id || 'Spotify user'}
                  </span>
                  <button
                    type="button"
                    onClick={(e) => {
                      e.preventDefault()
                      e.stopPropagation()
                      handleDisconnectSpotify()
                    }}
                    disabled={disconnecting}
                    className="rounded-lg border border-white/15 bg-white/[0.06] px-3 py-1 text-sm text-zinc-300 backdrop-blur-sm hover:bg-white/10 hover:text-white disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {disconnecting ? 'Disconnecting...' : 'Disconnect'}
                  </button>
                  {spotifyError && (
                    <span className="w-full text-sm text-red-400">{spotifyError}</span>
                  )}
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
                    <p className="mt-2 text-sm text-red-400">{spotifyError}</p>
                  )}
                </>
              )}
            </p>
            {spotifyParam === 'connected' && (
              <p className="mt-2 text-sm text-green-400">Spotify connected successfully.</p>
            )}
            {spotify?.connected && (
              <div className="mt-4">
                <DevicePicker
                  activeDeviceId={spotify.active_device_id ?? null}
                  onDeviceSelect={handleSetDevice}
                />
              </div>
            )}
            {spotifyParam === 'error' && (
              <p className="mt-2 text-sm text-red-400">
                {spotifyHint === 'redirect_uri_mismatch' ? (
                  <>
                    Connection failed (400). Check that your Spotify app redirect URI exactly matches:{' '}
                    <code className="rounded bg-white/[0.08] px-1 py-0.5 font-mono text-xs backdrop-blur-sm">
                      http://127.0.0.1:5173/api/spotify/callback
                    </code>
                    . Add it in the Spotify Developer Dashboard under your app settings.
                  </>
                ) : (
                  'Connection failed. Check the server logs for details.'
                )}
              </p>
            )}
          </div>

          {/* Session controls (admin) */}
          {spotify?.connected && (
            <SessionControls
              sessionActive={!!sessionActive}
              onSessionStart={handleSessionStart}
              onSessionEnd={handleSessionEnd}
            />
          )}

          {/* Now Playing */}
          {spotify?.connected && sessionActive && (
            <NowPlaying
              item={nowPlaying?.item ?? null}
              progressMs={nowPlaying?.progress_ms ?? 0}
              isPlaying={nowPlaying?.playing ?? false}
            />
          )}

          {/* Voting round */}
          {spotify?.connected && sessionActive && state && (
            <VotingRound
              candidates={state.candidates}
              votes={state.votes}
              timeRemainingSec={state.time_remaining_sec}
              onVote={handleVote}
            />
          )}

          {/* Playlist overview */}
          {spotify?.connected && sessionActive && (
            <PlaylistOverview sessionActive={!!sessionActive} />
          )}

          {stateError && (
            <p className="text-sm text-red-400">Failed to load state: {stateError}</p>
          )}
        </div>
      </main>
    </div>
  )
}

export default App
