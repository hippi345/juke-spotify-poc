import { useCallback, useEffect, useState } from 'react'

const TOKEN_KEY = 'juke_staff_token'

export type StaffUser = {
  id: number
  email: string
  role: string
}

export type Venue = {
  id: number
  name: string
  latitude: number
  longitude: number
}

function authHeaders(token: string | null): HeadersInit {
  const h: HeadersInit = { 'Content-Type': 'application/json' }
  if (token) {
    h.Authorization = `Bearer ${token}`
  }
  return h
}

export function useStaffAuth() {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem(TOKEN_KEY))
  const [user, setUser] = useState<StaffUser | null>(null)
  const [venues, setVenues] = useState<Venue[]>([])
  const [selectedVenueId, setSelectedVenueId] = useState<number | null>(null)
  const [joinPassword, setJoinPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const persistToken = useCallback((t: string | null) => {
    setToken(t)
    if (t) {
      localStorage.setItem(TOKEN_KEY, t)
    } else {
      localStorage.removeItem(TOKEN_KEY)
    }
  }, [])

  const refreshMe = useCallback(async () => {
    if (!token) {
      setUser(null)
      return
    }
    const res = await fetch('/api/auth/me', { headers: authHeaders(token) })
    if (!res.ok) {
      persistToken(null)
      setUser(null)
      return
    }
    const data = await res.json()
    setUser(data.user as StaffUser)
  }, [token, persistToken])

  const refreshVenues = useCallback(async () => {
    if (!token) {
      setVenues([])
      return
    }
    const res = await fetch('/api/venues/mine', { headers: authHeaders(token) })
    if (!res.ok) {
      setVenues([])
      return
    }
    const data = await res.json()
    const list = (data.venues ?? []) as Venue[]
    setVenues(list)
    if (list.length > 0 && selectedVenueId == null) {
      setSelectedVenueId(list[0].id)
    }
  }, [token, selectedVenueId])

  useEffect(() => {
    void refreshMe()
  }, [refreshMe])

  useEffect(() => {
    void refreshVenues()
  }, [refreshVenues])

  const register = async (email: string, password: string) => {
    setLoading(true)
    setError(null)
    try {
      const res = await fetch('/api/auth/register', {
        method: 'POST',
        headers: authHeaders(null),
        body: JSON.stringify({ email, password, role: 'staff' }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        throw new Error((data as { error?: string }).error || `Error ${res.status}`)
      }
      persistToken((data as { token: string }).token)
      setUser((data as { user: StaffUser }).user)
      await refreshVenues()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Registration failed')
    } finally {
      setLoading(false)
    }
  }

  const login = async (email: string, password: string) => {
    setLoading(true)
    setError(null)
    try {
      const res = await fetch('/api/auth/login', {
        method: 'POST',
        headers: authHeaders(null),
        body: JSON.stringify({ email, password }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        throw new Error((data as { error?: string }).error || `Error ${res.status}`)
      }
      persistToken((data as { token: string }).token)
      setUser((data as { user: StaffUser }).user)
      await refreshVenues()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Login failed')
    } finally {
      setLoading(false)
    }
  }

  const logout = () => {
    persistToken(null)
    setUser(null)
    setVenues([])
    setSelectedVenueId(null)
  }

  const createVenue = async (name: string, latitude: number, longitude: number) => {
    if (!token) return
    setLoading(true)
    setError(null)
    try {
      const res = await fetch('/api/venues', {
        method: 'POST',
        headers: authHeaders(token),
        body: JSON.stringify({ name, latitude, longitude }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        throw new Error((data as { error?: string }).error || `Error ${res.status}`)
      }
      await refreshVenues()
      setSelectedVenueId((data as Venue).id)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not create venue')
    } finally {
      setLoading(false)
    }
  }

  return {
    token,
    user,
    venues,
    selectedVenueId,
    setSelectedVenueId,
    joinPassword,
    setJoinPassword,
    error,
    loading,
    register,
    login,
    logout,
    createVenue,
    authHeaders: () => authHeaders(token),
  }
}
