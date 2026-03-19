import { useEffect, useState } from 'react'

/**
 * Returns progress in ms that ticks up every second when playing.
 * Syncs to serverProgressMs when it changes (e.g. on poll).
 */
export function useProgressTick(
  serverProgressMs: number,
  durationMs: number,
  isPlaying: boolean
): number {
  const [progressMs, setProgressMs] = useState(serverProgressMs)

  useEffect(() => {
    setProgressMs(serverProgressMs)
  }, [serverProgressMs])

  useEffect(() => {
    if (!isPlaying || durationMs <= 0) return
    const id = setInterval(() => {
      setProgressMs((prev) => Math.min(durationMs, prev + 1000))
    }, 1000)
    return () => clearInterval(id)
  }, [isPlaying, durationMs])

  return progressMs
}
