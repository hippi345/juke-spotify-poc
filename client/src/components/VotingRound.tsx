import type { Track } from '../hooks/useVotingState'
import { useCountdown } from '../hooks/useCountdown'

type VotingRoundProps = {
  candidates: Track[]
  votes: Record<string, number>
  timeRemainingSec: number
  onVote: (trackId: string) => Promise<void>
}

export function VotingRound({ candidates, votes, timeRemainingSec, onVote }: VotingRoundProps) {
  const displayCountdown = useCountdown(timeRemainingSec)
  const votingEnded = displayCountdown <= 0
  if (candidates.length === 0) {
    return (
      <div className="glass-panel p-6">
        <p className="text-zinc-500">Loading next round...</p>
      </div>
    )
  }

  return (
    <div className="glass-panel p-6">
      <div className="mb-4 flex items-center justify-between">
        <h3 className="text-lg font-medium text-white">Vote for next song</h3>
        <span className="rounded-full bg-[#1DB954]/15 px-3 py-1 text-sm font-medium text-[#1DB954] backdrop-blur-sm">
          {votingEnded ? 'Voting ended' : `Voting ends in ${displayCountdown}s`}
        </span>
      </div>

      <div className="grid gap-4 sm:grid-cols-3">
        {candidates.map((track) => {
          const voteCount = votes[track.id] ?? 0
          return (
            <div
              key={track.id}
              className="rounded-xl border border-white/[0.08] bg-white/[0.03] p-4 backdrop-blur-sm transition hover:border-[#1DB954]/40 hover:bg-white/[0.05]"
            >
              <div className="mb-3">
                {track.album?.images?.[0]?.url && (
                  <img
                    src={track.album.images[0].url}
                    alt={track.album.name}
                    className="mb-2 aspect-square w-full rounded-lg object-cover"
                  />
                )}
                <p className="truncate font-medium text-white">{track.name}</p>
                <p className="truncate text-sm text-zinc-400">
                  {track.artists?.map((a) => a.name).join(', ') ?? ''}
                </p>
              </div>
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm text-zinc-500">{voteCount} votes</span>
                <button
                  type="button"
                  onClick={() => onVote(track.id)}
                  disabled={votingEnded}
                  className="rounded-lg bg-[#1DB954] px-4 py-2 text-sm font-medium text-white transition hover:bg-[#1ed760] disabled:cursor-not-allowed disabled:opacity-60"
                >
                  Vote
                </button>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
