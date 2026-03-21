import type { Track } from '../hooks/useVotingState'
import { useProgressTick } from '../hooks/useProgressTick'
import { formatMs } from '../utils/formatTime'

type NowPlayingProps = {
  item: Track | null
  progressMs?: number
  isPlaying?: boolean
}

export function NowPlaying({ item, progressMs = 0, isPlaying = false }: NowPlayingProps) {
  if (!item) {
    return (
      <div className="glass-panel p-6">
        <p className="text-zinc-500">Nothing playing</p>
      </div>
    )
  }

  const displayProgress = useProgressTick(progressMs, item.duration_ms ?? 0, isPlaying)
  const imageUrl = item.album?.images?.[0]?.url ?? item.album?.images?.[1]?.url
  const artists = item.artists?.map((a) => a.name).join(', ') ?? ''
  const progressPct =
    item.duration_ms > 0 ? (displayProgress / item.duration_ms) * 100 : 0
  const remainingMs = Math.max(0, (item.duration_ms ?? 0) - displayProgress)
  const spotifyUrl = `https://open.spotify.com/track/${item.id}`

  return (
    <div className="glass-panel p-6">
      <div className="flex items-center gap-4">
        {imageUrl && (
          <img
            src={imageUrl}
            alt={item.album?.name ?? ''}
            className="aspect-square w-40 rounded-lg object-cover shadow-lg sm:w-52 md:w-64"
          />
        )}
        <div className="min-w-0 flex-1">
          <p className="truncate text-lg font-medium text-white">{item.name}</p>
          <p className="truncate text-sm text-zinc-400">{artists}</p>
          <div className="mt-1 flex flex-wrap items-center gap-2">
            {isPlaying && (
              <span className="text-xs text-green-400">Now playing</span>
            )}
            <a
              href={spotifyUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="text-xs text-[#1DB954] hover:text-[#1ed760] hover:underline"
            >
              Open in Spotify
            </a>
          </div>
        </div>
      </div>
      {item.duration_ms > 0 && (
        <div className="mt-4">
          <div className="mb-1 flex justify-between text-xs text-zinc-500">
            <span>{formatMs(displayProgress)}</span>
            <span>{formatMs(remainingMs)} left</span>
          </div>
          <div className="h-1 w-full overflow-hidden rounded-full bg-white/[0.12] backdrop-blur-sm">
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
