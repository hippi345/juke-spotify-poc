import { useState } from 'react'
import { PlaylistPicker } from './PlaylistPicker'

type SessionControlsProps = {
  sessionActive: boolean
  onSessionStart: (playlistId: string, playlistName: string, refillThreshold: number) => Promise<void>
  onSessionEnd: () => Promise<void>
}

export function SessionControls({
  sessionActive,
  onSessionStart,
  onSessionEnd,
}: SessionControlsProps) {
  const [pickerOpen, setPickerOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSelect = async (playlistId: string, playlistName: string, refillThreshold: number) => {
    setLoading(true)
    setError(null)
    try {
      await onSessionStart(playlistId, playlistName, refillThreshold)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to start session')
    } finally {
      setLoading(false)
    }
  }

  const handleEnd = async () => {
    setLoading(true)
    setError(null)
    try {
      await onSessionEnd()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to end session')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="glass-panel p-6">
      <h3 className="mb-4 text-lg font-medium text-white">Session</h3>
      {sessionActive ? (
        <div>
          <p className="mb-4 text-sm text-green-400">Voting session active</p>
          <button
            type="button"
            onClick={handleEnd}
            disabled={loading}
            className="rounded-lg border border-red-500/40 bg-red-500/[0.08] px-4 py-2 text-sm font-medium text-red-400 backdrop-blur-sm transition hover:bg-red-500/15 disabled:opacity-50"
          >
            {loading ? 'Ending...' : 'End session'}
          </button>
        </div>
      ) : (
        <div>
          <button
            type="button"
            onClick={() => setPickerOpen(true)}
            disabled={loading}
            className="rounded-lg bg-[#1DB954] px-4 py-2 text-sm font-medium text-white transition hover:bg-[#1ed760] disabled:opacity-50"
          >
            {loading ? 'Starting...' : 'Start session'}
          </button>
        </div>
      )}
      <PlaylistPicker
        isOpen={pickerOpen}
        onClose={() => setPickerOpen(false)}
        onSelect={handleSelect}
      />
      {error && <p className="mt-4 text-sm text-red-400">{error}</p>}
    </div>
  )
}
