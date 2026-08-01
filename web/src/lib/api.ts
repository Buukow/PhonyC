const TOKEN_KEY = 'phonyg_token'
const LOGIN_PATH = '/api/auth/login'
let expiryHandled = false

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(t: string) {
  localStorage.setItem(TOKEN_KEY, t)
  expiryHandled = false
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

function requestPath(path: string) {
  try {
    return new URL(path, window.location.origin).pathname
  } catch {
    return path.split('?')[0]
  }
}

function handleExpiredSession(path: string, requestToken: string) {
  if (!requestToken || requestPath(path) === LOGIN_PATH) return
  clearToken()
  if (window.location.pathname === '/login' || expiryHandled) return
  expiryHandled = true
  window.alert('登录状态已过期，请重新登录')
  window.history.pushState({}, '', '/login')
  window.dispatchEvent(new PopStateEvent('popstate'))
}

export async function api<T = any>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers || {})
  if (!headers.has('Content-Type') && init.body) {
    headers.set('Content-Type', 'application/json')
  }
  const tok = getToken()
  if (tok) headers.set('Authorization', `Bearer ${tok}`)
  const res = await fetch(path, { ...init, headers })
  const text = await res.text()
  let data: any = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    data = text
  }
  if (!res.ok) {
    if (res.status === 401) handleExpiredSession(path, tok)
    const msg = (data && (data.error || data.message)) || res.statusText
    throw new ApiError(res.status, typeof msg === 'string' ? msg : JSON.stringify(msg))
  }
  return data as T
}
