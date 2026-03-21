import { useEffect, useState } from 'react'

type Playlist = {
  id: string
  name: string
  uri: string
}

type PlaylistPickerProps = {
  isOpen: boolean
  onClose: () => void
  onSelect: (playlistId: string, playlistName: string, refillThreshold: number, refillCount: number) => void
}

function loadPlaylists(
  setPlaylists: (p: Playlist[]) => void,
  setError: (e: string | null) => void,
  setLoading: (l: boolean) => void
) {
  setLoading(true)
  setError(null)
  fetch('/api/playlists')
    .then(async (r) => {
      const data = await r.json().catch(() => ({}))
      if (!r.ok) {
        const msg = (data as { error?: string }).error ?? `HTTP ${r.status}`
        throw new Error(msg)
      }
      return data
    })
    .then((data) => {
      setPlaylists(data.playlists ?? [])
      setError(null)
    })
    .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load playlists'))
    .finally(() => setLoading(false))
}

export function PlaylistPicker({ isOpen, onClose, onSelect }: PlaylistPickerProps) {
  const [playlists, setPlaylists] = useState<Playlist[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [selectedPlaylist, setSelectedPlaylist] = useState<Playlist | null>(null)
  const [refillThresholdInput, setRefillThresholdInput] = useState('0')
  const [refillCountInput, setRefillCountInput] = useState('10')

  useEffect(() => {
    if (isOpen) {
      setSelectedPlaylist(null)
      setRefillThresholdInput('0')
      setRefillCountInput('10')
      loadPlaylists(setPlaylists, setError, setLoading)
    }
  }, [isOpen])

  const handleConfirm = () => {
    if (!selectedPlaylist) return
    const threshold = Math.max(0, parseInt(refillThresholdInput, 10) || 0)
    const count = Math.max(1, Math.min(10, parseInt(refillCountInput, 10) || 10))
    onSelect(selectedPlaylist.id, selectedPlaylist.name, threshold, count)
    onClose()
  }

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70">
      <div className="mx-4 w-full max-w-md rounded-2xl border border-white/10 bg-[#141416] p-6 shadow-2xl">
        <h2 className="mb-4 text-xl font-semibold text-white">Start voting session</h2>
        <p className="mb-4 text-sm text-zinc-400">Select a playlist to pull tracks from for voting.</p>

        {loading && <p className="mb-4 text-sm text-zinc-500">Loading playlists...</p>}
        {error && (
          <div className="mb-4">
            <p className="text-sm text-red-400">{error}</p>
            <p className="mt-1 text-xs text-zinc-500">
              Ensure the server is running and Spotify is connected. If you recently connected, try reconnecting to refresh permissions.
            </p>
            <button
              type="button"
              onClick={() => loadPlaylists(setPlaylists, setError, setLoading)}
              className="mt-2 text-sm text-[#1DB954] hover:underline"
            >
              Retry
            </button>
          </div>
        )}

        <div className="mb-4 max-h-48 space-y-2 overflow-y-auto">
          {!loading && playlists.length === 0 && !error && (
            <p className="py-4 text-center text-sm text-zinc-500">
              No playlists found. Only playlists you created are shown. Create one in Spotify first.
            </p>
          )}
          {!loading &&
            playlists.map((p) => (
              <button
                key={p.id}
                type="button"
                onClick={() => setSelectedPlaylist(p)}
                className={`w-full rounded-lg border px-4 py-3 text-left transition ${
                  selectedPlaylist?.id === p.id
                    ? 'border-[#1DB954]/60 bg-[#1DB954]/20 text-white'
                    : 'border-white/10 bg-[#1a1a1e] text-white hover:border-[#1DB954]/50 hover:bg-[#222228]'
                }`}
              >
                <span className="font-medium">{p.name}</span>
              </button>
            ))}
        </div>

        <div className="mb-4 space-y-4">
          <div>
            <label className="mb-2 block text-sm text-zinc-400">
              Refill when below (tracks): <span className="text-zinc-500">0 = disabled</span>
            </label>
            <p className="mb-1 text-xs text-zinc-500">
              When (playlist size − played) &lt; this, add similar tracks
            </p>
            <input
              type="text"
              inputMode="numeric"
              pattern="[0-9]*"
              value={refillThresholdInput}
              onChange={(e) => setRefillThresholdInput(e.target.value.replace(/\D/g, ''))}
              placeholder="0"
              className="w-full rounded-lg border border-white/10 bg-[#1a1a1e] px-4 py-2 text-white placeholder-zinc-500 focus:border-[#1DB954] focus:outline-none"
            />
          </div>
          <div>
            <label className="mb-2 block text-sm text-zinc-400">
              Tracks to add when refilling: <span className="text-zinc-500">(max 10)</span>
            </label>
            <p className="mb-1 text-xs text-zinc-500">
              Number of similar-vibe tracks added (removed when session ends)
            </p>
            <input
              type="text"
              inputMode="numeric"
              pattern="[0-9]*"
              value={refillCountInput}
              onChange={(e) => setRefillCountInput(e.target.value.replace(/\D/g, ''))}
              placeholder="10"
              className="w-full rounded-lg border border-white/10 bg-[#1a1a1e] px-4 py-2 text-white placeholder-zinc-500 focus:border-[#1DB954] focus:outline-none"
            />
          </div>
        </div>

        <div className="flex gap-3">
          <button
            type="button"
            onClick={onClose}
            className="flex-1 rounded-lg border border-white/10 bg-[#1a1a1e] px-4 py-2 text-zinc-400 transition hover:bg-[#222228] hover:text-white"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={handleConfirm}
            disabled={!selectedPlaylist}
            className="flex-1 rounded-lg bg-[#1DB954] px-4 py-2 font-medium text-white transition hover:bg-[#1ed760] disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Start session
          </button>
        </div>
      </div>
    </div>
  )
}
