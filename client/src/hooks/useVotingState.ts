import { useCallback, useEffect, useState } from 'react'

export type Track = {
  id: string
  name: string
  uri: string
  artists: { id: string; name: string }[]
  album: { id: string; name: string; images: { url: string; width: number; height: number }[] }
  duration_ms: number
}

export type VotingState = {
  session: {
    id: number
    playlist_id: string
    playlist_name: string
    refill_threshold: number
    status: string
  } | null
  now_playing: {
    playing: boolean
    progress_ms: number
    item: Track | null
  } | null
  candidates: Track[]
  votes: Record<string, number>
  time_remaining_sec: number
}

const POLL_INTERVAL_MS = 2500

export function useVotingState(enabled: boolean) {
  const [state, setState] = useState<VotingState | null>(null)
  const [error, setError] = useState<string | null>(null)

  const fetchState = useCallback(async () => {
    try {
      const res = await fetch('/api/voting/state')
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}`)
      }
      const data = await res.json()
      setState({
        session: data.session,
        now_playing: data.now_playing,
        candidates: data.candidates ?? [],
        votes: data.votes ?? {},
        time_remaining_sec: data.time_remaining_sec ?? 0,
      })
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to fetch state')
    }
  }, [])

  useEffect(() => {
    if (!enabled) return
    fetchState()
    const id = setInterval(fetchState, POLL_INTERVAL_MS)
    return () => clearInterval(id)
  }, [enabled, fetchState])

  return { state, error, refetch: fetchState }
}
