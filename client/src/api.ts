function base() { return localStorage.getItem('server') || 'http://127.0.0.1:8080' }

export async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Content-Type', 'application/json')
  const token = localStorage.getItem('token')
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const response = await fetch(`${base()}${path}`, { ...options, headers })
  if (!response.ok) { const body = await response.json().catch(() => ({})); throw new Error(body.error || `请求失败：${response.status}`) }
  if (response.status === 204) return undefined as T
  return response.json()
}

export function deviceId() {
  let id = localStorage.getItem('deviceId')
  if (!id) { id = crypto.randomUUID(); localStorage.setItem('deviceId', id) }
  return id
}
