import { useEffect, useState } from 'react'

/**
 * Returns a countdown value that decrements every second.
 * Syncs to serverValue when it changes (e.g. on poll).
 */
export function useCountdown(serverValue: number): number {
  const [displayValue, setDisplayValue] = useState(serverValue)

  useEffect(() => {
    setDisplayValue(serverValue)
  }, [serverValue])

  useEffect(() => {
    const id = setInterval(() => {
      setDisplayValue((prev) => Math.max(0, prev - 1))
    }, 1000)
    return () => clearInterval(id)
  }, [])

  return displayValue
}
