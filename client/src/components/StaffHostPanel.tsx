import { useState } from 'react'
import type { useStaffAuth } from '../hooks/useStaffAuth'

type StaffAuth = ReturnType<typeof useStaffAuth>

type StaffHostPanelProps = {
  auth: StaffAuth
}

export function StaffHostPanel({ auth }: StaffHostPanelProps) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [venueName, setVenueName] = useState('')
  const [lat, setLat] = useState('')
  const [lng, setLng] = useState('')

  const useBrowserLocation = () => {
    if (!navigator.geolocation) {
      return
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setLat(String(pos.coords.latitude))
        setLng(String(pos.coords.longitude))
      },
      () => {
        /* user denied or unavailable */
      },
      { enableHighAccuracy: true, timeout: 10000 }
    )
  }

  if (!auth.user) {
    return (
      <div className="glass-panel p-6">
        <h3 className="mb-4 text-lg font-medium text-white">Staff account</h3>
        <p className="mb-4 text-sm text-white/70">
          Register or sign in with email to manage a venue location and publish a joinable session.
        </p>
        <div className="flex flex-col gap-3">
          <input
            type="email"
            placeholder="Email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="rounded-lg border border-white/10 bg-black/30 px-3 py-2 text-sm text-white"
          />
          <input
            type="password"
            placeholder="Password (min 8 characters)"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="rounded-lg border border-white/10 bg-black/30 px-3 py-2 text-sm text-white"
          />
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              disabled={auth.loading}
              onClick={() => void auth.login(email, password)}
              className="rounded-lg bg-emerald-500/20 px-4 py-2 text-sm text-emerald-300 hover:bg-emerald-500/30 disabled:opacity-50"
            >
              Sign in
            </button>
            <button
              type="button"
              disabled={auth.loading}
              onClick={() => void auth.register(email, password)}
              className="rounded-lg border border-white/15 px-4 py-2 text-sm text-white/90 hover:bg-white/5 disabled:opacity-50"
            >
              Register staff
            </button>
          </div>
          {auth.error && <p className="text-sm text-red-400">{auth.error}</p>}
        </div>
      </div>
    )
  }

  return (
    <div className="glass-panel p-6">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-lg font-medium text-white">Venue (staff)</h3>
        <button
          type="button"
          onClick={auth.logout}
          className="text-xs text-white/60 hover:text-white"
        >
          Sign out ({auth.user.email})
        </button>
      </div>

      {auth.venues.length === 0 ? (
        <div className="flex flex-col gap-3">
          <p className="text-sm text-white/70">Create a venue with a map location for patron discovery.</p>
          <input
            placeholder="Venue name"
            value={venueName}
            onChange={(e) => setVenueName(e.target.value)}
            className="rounded-lg border border-white/10 bg-black/30 px-3 py-2 text-sm text-white"
          />
          <div className="flex flex-wrap gap-2">
            <input
              placeholder="Latitude"
              value={lat}
              onChange={(e) => setLat(e.target.value)}
              className="min-w-[8rem] flex-1 rounded-lg border border-white/10 bg-black/30 px-3 py-2 text-sm text-white"
            />
            <input
              placeholder="Longitude"
              value={lng}
              onChange={(e) => setLng(e.target.value)}
              className="min-w-[8rem] flex-1 rounded-lg border border-white/10 bg-black/30 px-3 py-2 text-sm text-white"
            />
            <button
              type="button"
              onClick={useBrowserLocation}
              className="rounded-lg border border-white/15 px-3 py-2 text-xs text-white/80"
            >
              Use my location
            </button>
          </div>
          <button
            type="button"
            disabled={auth.loading}
            onClick={() =>
              void auth.createVenue(venueName, parseFloat(lat), parseFloat(lng))
            }
            className="rounded-lg bg-emerald-500/20 px-4 py-2 text-sm text-emerald-300 disabled:opacity-50"
          >
            Save venue
          </button>
        </div>
      ) : (
        <div className="flex flex-col gap-3">
          <label className="text-sm text-white/70">
            Active venue
            <select
              value={auth.selectedVenueId ?? ''}
              onChange={(e) => auth.setSelectedVenueId(Number(e.target.value))}
              className="mt-1 w-full rounded-lg border border-white/10 bg-black/30 px-3 py-2 text-sm text-white"
            >
              {auth.venues.map((v) => (
                <option key={v.id} value={v.id}>
                  {v.name} ({v.latitude.toFixed(4)}, {v.longitude.toFixed(4)})
                </option>
              ))}
            </select>
          </label>
          <label className="text-sm text-white/70">
            Optional join password (leave blank for open sessions)
            <input
              type="password"
              value={auth.joinPassword}
              onChange={(e) => auth.setJoinPassword(e.target.value)}
              className="mt-1 w-full rounded-lg border border-white/10 bg-black/30 px-3 py-2 text-sm text-white"
              placeholder="Patrons enter this in the mobile app"
            />
          </label>
        </div>
      )}
      {auth.error && <p className="mt-3 text-sm text-red-400">{auth.error}</p>}
    </div>
  )
}
