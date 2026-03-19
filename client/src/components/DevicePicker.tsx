import { useCallback, useEffect, useState } from 'react'

type Device = {
  id: string
  name: string
  type: string
  is_active: boolean
  is_restricted: boolean
}

type DevicePickerProps = {
  activeDeviceId: string | null
  onDeviceSelect: (deviceId: string) => Promise<void>
}

export function DevicePicker({ activeDeviceId, onDeviceSelect }: DevicePickerProps) {
  const [devices, setDevices] = useState<Device[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const fetchDevices = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await fetch('/api/spotify/devices')
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        setError((data as { error?: string }).error || 'Failed to load devices')
        setDevices([])
        return
      }
      setDevices((data as { devices?: Device[] }).devices ?? [])
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load devices')
      setDevices([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchDevices()
    const interval = setInterval(fetchDevices, 10000) // Refresh every 10s
    return () => clearInterval(interval)
  }, [fetchDevices])

  const handleSelect = async (deviceId: string) => {
    if (deviceId === activeDeviceId) return
    setSaving(true)
    setError(null)
    try {
      await onDeviceSelect(deviceId)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to set device')
    } finally {
      setSaving(false)
    }
  }

  if (loading && devices.length === 0 && !error) {
    return (
      <p className="text-sm text-zinc-500">Loading devices...</p>
    )
  }

  if (devices.length === 0) {
    return (
      <div className="space-y-2">
        <p className="text-sm text-zinc-500">
          No devices found. Spotify only lists devices that have played something recently.
        </p>
        <p className="text-sm text-zinc-400">
          Play a song in your Spotify desktop app (or any Spotify app), then click Refresh.
        </p>
        {error && (
          <p className="text-sm text-red-400">{error}</p>
        )}
        <button
          type="button"
          onClick={() => fetchDevices()}
          disabled={loading}
          className="rounded-lg border border-white/15 bg-white/[0.06] px-3 py-1.5 text-sm text-zinc-300 backdrop-blur-sm hover:bg-white/10 hover:text-white disabled:opacity-50"
        >
          {loading ? 'Refreshing...' : 'Refresh'}
        </button>
      </div>
    )
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-2">
        <p className="text-sm text-zinc-500">Playback device:</p>
        <button
          type="button"
          onClick={() => fetchDevices()}
          disabled={loading}
          className="text-xs text-zinc-400 hover:text-zinc-300 disabled:opacity-50"
        >
          Refresh
        </button>
      </div>
      <div className="flex flex-wrap gap-2">
        {devices
          .filter((d) => !d.is_restricted)
          .map((d) => (
            <button
              key={d.id}
              type="button"
              onClick={() => handleSelect(d.id)}
              disabled={saving}
              className={`rounded-lg border px-3 py-1.5 text-sm transition disabled:opacity-50 ${
                d.id === activeDeviceId
                  ? 'border-[#1DB954]/60 bg-[#1DB954]/15 text-[#1DB954] backdrop-blur-sm'
                  : 'border-white/15 bg-white/[0.06] text-zinc-300 backdrop-blur-sm hover:bg-white/10 hover:text-white'
              }`}
            >
              {d.name}
              {d.is_active && (
                <span className="ml-1.5 text-xs text-green-400">●</span>
              )}
            </button>
          ))}
      </div>
      {error && <p className="text-sm text-red-400">{error}</p>}
    </div>
  )
}
