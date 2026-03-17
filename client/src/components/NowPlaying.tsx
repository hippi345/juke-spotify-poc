import type { Track } from '../hooks/useVotingState'

type NowPlayingProps = {
  item: Track | null
  progressMs?: number
  isPlaying?: boolean
}

export function NowPlaying({ item, progressMs = 0, isPlaying = false }: NowPlayingProps) {
  if (!item) {
    return (
      <div className="rounded-xl border border-white/5 bg-black/30 p-6">
        <p className="text-zinc-500">Nothing playing</p>
      </div>
    )
  }

  const imageUrl = item.album?.images?.[0]?.url ?? item.album?.images?.[1]?.url
  const artists = item.artists?.map((a) => a.name).join(', ') ?? ''
  const progressPct = item.duration_ms > 0 ? (progressMs / item.duration_ms) * 100 : 0

  return (
    <div className="rounded-xl border border-white/5 bg-black/30 p-6">
      <div className="flex items-center gap-4">
        {imageUrl && (
          <img
            src={imageUrl}
            alt={item.album?.name ?? ''}
            className="h-20 w-20 rounded-lg object-cover shadow-lg"
          />
        )}
        <div className="min-w-0 flex-1">
          <p className="truncate text-lg font-medium text-white">{item.name}</p>
          <p className="truncate text-sm text-zinc-400">{artists}</p>
          {isPlaying && (
            <p className="mt-1 text-xs text-green-400">Now playing</p>
          )}
        </div>
      </div>
      {item.duration_ms > 0 && (
        <div className="mt-4">
          <div className="h-1 w-full overflow-hidden rounded-full bg-white/10">
            <div
              className="h-full rounded-full bg-[#1DB954] transition-all duration-1000"
              style={{ width: `${Math.min(100, progressPct)}%` }}
            />
          </div>
        </div>
      )}
    </div>
  )
}
