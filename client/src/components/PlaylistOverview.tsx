import { useCallback, useEffect, useState } from 'react'

type Track = {
  id: string
  name: string
  uri: string
  artists: { id: string; name: string }[]
  album: { id: string; name: string; images: { url: string; width: number; height: number }[] }
  duration_ms: number
}

type TrackWithMeta = {
  track: Track
  played: boolean
  refilled: boolean
}

type PlaylistOverviewProps = {
  sessionActive: boolean
  /** When this changes (e.g. new round), refetch playlist overview */
  roundKey: string
}

export function PlaylistOverview({ sessionActive, roundKey }: PlaylistOverviewProps) {
  const [tracks, setTracks] = useState<TrackWithMeta[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [refilling, setRefilling] = useState(false)

  const fetchTracks = useCallback(async () => {
    if (!sessionActive) return
    setLoading(true)
    setError(null)
    try {
      const res = await fetch('/api/voting/playlist-overview')
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        setError((data as { error?: string }).error || 'Failed to load playlist')
        setTracks([])
        return
      }
      setTracks((data as { tracks?: TrackWithMeta[] }).tracks ?? [])
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load playlist')
      setTracks([])
    } finally {
      setLoading(false)
    }
  }, [sessionActive])

  useEffect(() => {
    if (!sessionActive) {
      setTracks([])
      return
    }
    fetchTracks()
  }, [sessionActive, roundKey, fetchTracks])

  const handleTriggerRefill = async () => {
    setRefilling(true)
    setError(null)
    try {
      const res = await fetch('/api/voting/trigger-refill', { method: 'POST' })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        setError((data as { error?: string }).error || 'Refill failed')
        return
      }
      setError(null)
      await fetchTracks()
    } finally {
      setRefilling(false)
    }
  }

  if (!sessionActive) return null

  return (
    <div className="glass-panel p-6">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-lg font-medium text-white">Playlist</h3>
        <div className="flex flex-wrap items-center gap-3">
          <div className="flex flex-wrap gap-3 text-xs text-zinc-500">
            <span title="Songs played this session">
              <span className="text-[#1DB954]">▶</span> Played
            </span>
            <span title="Added when playlist ran low">
              <span className="text-[#1DB954]">=</span> Vibe Fill
            </span>
          </div>
          <button
            type="button"
            onClick={() => fetchTracks()}
            disabled={loading}
            className="rounded-lg border border-white/15 bg-white/[0.06] px-3 py-1.5 text-xs text-zinc-400 transition hover:bg-white/10 hover:text-white disabled:opacity-50"
            title="Refresh playlist"
          >
            Refresh
          </button>
          <button
            type="button"
            onClick={handleTriggerRefill}
            disabled={refilling}
            className="rounded-lg border border-[#1DB954]/50 bg-[#1DB954]/10 px-3 py-1.5 text-xs font-medium text-[#1DB954] transition hover:bg-[#1DB954]/20 disabled:opacity-50"
            title="Manually trigger vibe fill to test (adds similar tracks to playlist)"
          >
            {refilling ? 'Refilling...' : 'Trigger refill (test)'}
          </button>
        </div>
      </div>
      {loading && tracks.length === 0 ? (
        <p className="text-sm text-zinc-500">Loading playlist...</p>
      ) : error ? (
        <p className="text-sm text-red-400">{error}</p>
      ) : (
        <div className="scrollbar-dark max-h-64 overflow-y-auto overflow-x-hidden pr-1">
          <div className="grid grid-cols-4 gap-3 sm:grid-cols-6 md:grid-cols-8">
            {tracks.map(({ track, played, refilled }) => (
              <div
                key={track.id}
                className="group relative flex flex-col items-center"
              >
                <div className="relative aspect-square w-full">
                  {track.album?.images?.[0]?.url ? (
                    <img
                      src={track.album.images[0].url}
                      alt={track.album.name}
                      className="aspect-square w-full rounded-lg object-cover"
                    />
                  ) : (
                    <div className="aspect-square w-full rounded-lg bg-white/10" />
                  )}
                  {played && (
                    <div
                      className="absolute inset-0 flex items-center justify-center rounded-lg bg-black/50 backdrop-blur-[2px]"
                      title="Played this session"
                    >
                      <span className="text-2xl font-medium text-[#1DB954]">▶</span>
                    </div>
                  )}
                  {refilled && !played && (
                    <div
                      className="absolute bottom-1 right-1 flex h-5 w-5 items-center justify-center rounded bg-black/50 backdrop-blur-sm"
                      title="Added as vibe fill"
                    >
                      <span className="text-xs font-bold text-[#1DB954]">=</span>
                    </div>
                  )}
                </div>
                <p
                  className="mt-1 w-full truncate text-center text-xs text-zinc-400"
                  title={track.name}
                >
                  {track.name}
                </p>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
