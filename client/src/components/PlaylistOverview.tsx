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
}

export function PlaylistOverview({ sessionActive }: PlaylistOverviewProps) {
  const [tracks, setTracks] = useState<TrackWithMeta[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

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
    const id = setInterval(fetchTracks, 10000)
    return () => clearInterval(id)
  }, [sessionActive, fetchTracks])

  if (!sessionActive) return null

  return (
    <div className="glass-panel p-6">
      <div className="mb-4 flex items-center justify-between gap-2">
        <h3 className="text-lg font-medium text-white">Playlist</h3>
        <div className="flex flex-wrap gap-3 text-xs text-zinc-500">
          <span title="Songs played this session">
            <span className="text-[#1DB954]">▶</span> Played
          </span>
          <span title="Added when playlist ran low">
            <span className="text-[#1DB954]">=</span> Vibe Fill
          </span>
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
