import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, api, getToken, setToken } from './api'

function response(status: number, body: unknown = { error: 'unauthorized' }) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

beforeEach(() => {
  localStorage.clear()
  history.replaceState({}, '', '/')
  vi.stubGlobal('fetch', vi.fn())
  vi.spyOn(window, 'alert').mockImplementation(() => {})
})

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('API session expiry handling', () => {
  it('clears an expired token, alerts once, and navigates to login', async () => {
    setToken('expired-token')
    vi.mocked(fetch).mockImplementation(async () => response(401))

    await expect(api('/api/channels')).rejects.toBeInstanceOf(ApiError)

    expect(getToken()).toBe('')
    expect(window.alert).toHaveBeenCalledWith('登录状态已过期，请重新登录')
    expect(window.location.pathname).toBe('/login')
  })

  it('does not treat login endpoint 401 as an expired session', async () => {
    setToken('stale-token')
    vi.mocked(fetch).mockResolvedValue(response(401, { error: 'invalid credentials' }))

    await expect(api('http://localhost/api/auth/login?source=test')).rejects.toMatchObject({ status: 401, message: 'invalid credentials' })

    expect(getToken()).toBe('stale-token')
    expect(window.alert).not.toHaveBeenCalled()
    expect(window.location.pathname).toBe('/')
  })

  it('ignores tokenless 401 responses', async () => {
    vi.mocked(fetch).mockResolvedValue(response(401))

    await expect(api('/api/channels')).rejects.toBeInstanceOf(ApiError)

    expect(window.alert).not.toHaveBeenCalled()
    expect(window.location.pathname).toBe('/')
  })

  it('suppresses duplicate expiry notifications from concurrent failures', async () => {
    setToken('expired-token')
    vi.mocked(fetch).mockResolvedValue(response(401))

    await Promise.allSettled([api('/api/channels'), api('/api/keys')])

    expect(window.alert).toHaveBeenCalledTimes(1)
    expect(window.location.pathname).toBe('/login')
  })

  it('allows a later expiry notification after a new login token is set', async () => {
    vi.mocked(fetch).mockImplementation(async () => response(401))

    setToken('first-token')
    await expect(api('/api/channels')).rejects.toBeInstanceOf(ApiError)
    history.replaceState({}, '', '/')
    setToken('second-token')
    await expect(api('/api/keys')).rejects.toBeInstanceOf(ApiError)

    expect(window.alert).toHaveBeenCalledTimes(2)
    expect(window.location.pathname).toBe('/login')
  })

  it('clears the token without alerting again when already on login page', async () => {
    history.replaceState({}, '', '/login')
    setToken('expired-token')
    vi.mocked(fetch).mockResolvedValue(response(401))

    await expect(api('/api/channels')).rejects.toBeInstanceOf(ApiError)

    expect(getToken()).toBe('')
    expect(window.alert).not.toHaveBeenCalled()
    expect(window.location.pathname).toBe('/login')
  })
})
