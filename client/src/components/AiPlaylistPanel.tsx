import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import { AiSparkleIcon } from './AiSparkleIcon'

export type AiPlaylistResult = {
  playlist_id: string
  playlist_url: string
  name: string
  tracks_added: number
  tracks_requested: number
  ai_suggestions?: number
  not_found?: { title: string; artist: string; reason?: string }[]
}

type JobAccepted = { job_id: string; status: string }
type JobPoll =
  | { status: 'pending' | 'running' }
  | { status: 'completed'; result: AiPlaylistResult }
  | { status: 'failed'; error: string }

type AiPlaylistPanelProps = {
  onPlaylistReady?: (result: AiPlaylistResult) => void
  onPlaylistFailed?: (message: string) => void
}

export function AiPlaylistPanel({ onPlaylistReady, onPlaylistFailed }: AiPlaylistPanelProps) {
  const [open, setOpen] = useState(false)
  const [playlistName, setPlaylistName] = useState('')
  const [trackCount, setTrackCount] = useState(15)
  const [description, setDescription] = useState('')
  const [artistInput, setArtistInput] = useState('')
  const [similarArtists, setSimilarArtists] = useState<string[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [jobRunning, setJobRunning] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const stopPoll = useCallback(() => {
    if (pollRef.current != null) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
  }, [])

  useEffect(() => {
    return () => stopPoll()
  }, [stopPoll])

  const resetVibeSenseForm = useCallback(() => {
    setOpen(false)
    setJobRunning(false)
    setPlaylistName('')
    setTrackCount(15)
    setDescription('')
    setArtistInput('')
    setSimilarArtists([])
    setError(null)
    setSubmitting(false)
  }, [])

  const addArtist = useCallback(() => {
    const s = artistInput.trim()
    if (!s) return
    setSimilarArtists((prev) => (prev.includes(s) ? prev : [...prev, s]))
    setArtistInput('')
  }, [artistInput])

  const removeArtist = useCallback((name: string) => {
    setSimilarArtists((prev) => prev.filter((a) => a !== name))
  }, [])

  const pollJob = useCallback(
    (jobId: string) => {
      stopPoll()
      pollRef.current = window.setInterval(async () => {
        try {
          const res = await fetch(`/api/ai-playlist/jobs/${encodeURIComponent(jobId)}`)
          const data = (await res.json().catch(() => ({}))) as JobPoll & { error?: string }
          if (!res.ok) {
            stopPoll()
            setJobRunning(false)
            setError(data.error || `Status check failed (${res.status})`)
            onPlaylistFailed?.(data.error || 'Status check failed')
            return
          }
          if (data.status === 'pending' || data.status === 'running') {
            return
          }
          stopPoll()
          setJobRunning(false)
          if (data.status === 'failed') {
            const msg = data.error || 'VibeSense could not create your playlist'
            setError(msg)
            onPlaylistFailed?.(msg)
            return
          }
          if (data.status === 'completed' && data.result) {
            resetVibeSenseForm()
            onPlaylistReady?.(data.result)
          }
        } catch (e) {
          stopPoll()
          setJobRunning(false)
          const msg = e instanceof Error ? e.message : 'Polling failed'
          setError(msg)
          onPlaylistFailed?.(msg)
        }
      }, 1500)
    },
    [onPlaylistFailed, onPlaylistReady, resetVibeSenseForm, stopPoll]
  )

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    setSubmitting(true)
    const name = playlistName.trim()
    if (!name) {
      setError('Enter a name for the new playlist.')
      setSubmitting(false)
      return
    }
    if ([...name].length > 100) {
      setError('Playlist name must be at most 100 characters.')
      setSubmitting(false)
      return
    }
    const desc = description.trim()
    if (!desc) {
      setError('Describe the playlist you want.')
      setSubmitting(false)
      return
    }
    if (trackCount < 1 || trackCount > 100) {
      setError('Number of tracks must be between 1 and 100.')
      setSubmitting(false)
      return
    }
    try {
      const res = await fetch('/api/ai-playlist/create', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          playlist_name: name,
          track_count: trackCount,
          description: desc,
          similar_artists: similarArtists,
        }),
      })
      const data = (await res.json().catch(() => ({}))) as JobAccepted & { error?: string }

      if (res.status === 202 && data.job_id) {
        setJobRunning(true)
        pollJob(data.job_id)
        return
      }

      if (!res.ok) {
        const msg = data.error || `Request failed (${res.status})`
        setError(msg)
        onPlaylistFailed?.(msg)
        return
      }

      const unexpected = `Unexpected response (${res.status})`
      setError(unexpected)
      onPlaylistFailed?.(unexpected)
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Request failed'
      setError(msg)
      onPlaylistFailed?.(msg)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="glass-panel p-6">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-start gap-4">
          <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl border border-violet-500/35 bg-gradient-to-br from-violet-500/20 to-indigo-600/15 shadow-inner shadow-violet-500/10">
            <AiSparkleIcon variant="lg" className="h-11 w-11" />
          </div>
          <div className="min-w-0">
            <h3 className="text-lg font-medium text-white">VibeSense</h3>
            <p className="mt-1 text-sm text-zinc-500">
              Describe a vibe; optional inspiration artists. VibeSense uses AI to suggest tracks, then finds them on
              Spotify and saves a new playlist to your library.
            </p>
          </div>
        </div>
        <button
          type="button"
          onClick={() => {
            setOpen((o) => !o)
            setError(null)
          }}
          className="shrink-0 rounded-lg border border-violet-500/40 bg-violet-500/15 px-4 py-2 text-sm font-medium text-violet-200 transition hover:bg-violet-500/25"
        >
          {open ? 'Close' : 'Create playlist with AI VibeSense'}
        </button>
      </div>

      {open && (
        <form onSubmit={handleSubmit} className="space-y-4 border-t border-white/10 pt-4">
          <div>
            <label htmlFor="ai-playlist-name" className="mb-1 block text-sm text-zinc-400">
              Playlist name <span className="text-red-400">*</span>
            </label>
            <input
              id="ai-playlist-name"
              type="text"
              required
              maxLength={100}
              value={playlistName}
              onChange={(e) => setPlaylistName(e.target.value)}
              placeholder="e.g. Midnight highway synth"
              className="w-full rounded-lg border border-white/15 bg-white/[0.06] px-3 py-2 text-white placeholder:text-zinc-600 focus:border-violet-500/50 focus:outline-none focus:ring-1 focus:ring-violet-500/40"
            />
          </div>

          <div>
            <label htmlFor="ai-track-count" className="mb-1 block text-sm text-zinc-400">
              Number of tracks <span className="text-red-400">*</span>
            </label>
            <input
              id="ai-track-count"
              type="number"
              min={1}
              max={100}
              required
              value={trackCount}
              onChange={(e) => setTrackCount(Number(e.target.value))}
              className="w-full max-w-xs rounded-lg border border-white/15 bg-white/[0.06] px-3 py-2 text-white placeholder:text-zinc-600 focus:border-violet-500/50 focus:outline-none focus:ring-1 focus:ring-violet-500/40"
            />
          </div>

          <div>
            <label htmlFor="ai-desc" className="mb-1 block text-sm text-zinc-400">
              Playlist description <span className="text-red-400">*</span>
            </label>
            <textarea
              id="ai-desc"
              required
              rows={4}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="e.g. Late-night driving synthwave, melancholic but hopeful, no vocals or sparse vocals"
              className="w-full rounded-lg border border-white/15 bg-white/[0.06] px-3 py-2 text-white placeholder:text-zinc-600 focus:border-violet-500/50 focus:outline-none focus:ring-1 focus:ring-violet-500/40"
            />
          </div>

          <div>
            <span className="mb-1 block text-sm text-zinc-400">Similar artists (optional)</span>
            <div className="flex flex-wrap gap-2">
              <input
                type="text"
                value={artistInput}
                onChange={(e) => setArtistInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault()
                    addArtist()
                  }
                }}
                placeholder="Artist name, then Add"
                className="min-w-[12rem] flex-1 rounded-lg border border-white/15 bg-white/[0.06] px-3 py-2 text-white placeholder:text-zinc-600 focus:border-violet-500/50 focus:outline-none focus:ring-1 focus:ring-violet-500/40"
              />
              <button
                type="button"
                onClick={addArtist}
                className="rounded-lg border border-white/15 bg-white/[0.06] px-3 py-2 text-sm text-zinc-300 hover:bg-white/10"
              >
                Add
              </button>
            </div>
            {similarArtists.length > 0 && (
              <ul className="mt-2 flex flex-wrap gap-2">
                {similarArtists.map((a) => (
                  <li
                    key={a}
                    className="inline-flex items-center gap-1 rounded-full border border-white/15 bg-white/[0.06] px-2 py-1 text-xs text-zinc-300"
                  >
                    {a}
                    <button
                      type="button"
                      onClick={() => removeArtist(a)}
                      className="rounded text-zinc-500 hover:text-white"
                      aria-label={`Remove ${a}`}
                    >
                      ×
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>

          {error && <p className="text-sm text-red-400">{error}</p>}

          {jobRunning && (
            <p className="text-sm text-violet-300/90">
              VibeSense is creating your playlist in the background… This can take a minute. You can keep using the app;
              we’ll notify you when it’s ready.
            </p>
          )}

          <div className="flex flex-wrap items-center gap-3">
            <button
              type="submit"
              disabled={submitting || jobRunning}
              className="rounded-lg bg-violet-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-violet-500 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {submitting ? 'Starting…' : jobRunning ? 'Working in background…' : 'Generate with VibeSense'}
            </button>
            <p className="text-xs text-zinc-500">
              VibeSense needs a Gemini API key on the server and a connected Spotify account.
            </p>
          </div>
        </form>
      )}
    </div>
  )
}
