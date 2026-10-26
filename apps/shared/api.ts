function resolveApiUrl(): string {
  if (import.meta.env.VITE_API_URL) {
    return import.meta.env.VITE_API_URL
  }
  if (typeof window !== 'undefined') {
    // When hosted on Cloudflare (or any remote domain), use same origin so API goes to Cloudflare Worker
    if (window.location.hostname !== 'localhost' && window.location.hostname !== '127.0.0.1') {
      return window.location.origin
    }
    // When developing locally with Vite on port 5173, point to local Go server on port 8080
    if (window.location.port === '5173') {
      return 'http://localhost:8080'
    }
    return window.location.origin
  }
  return 'http://localhost:8080'
}

export const API_URL = resolveApiUrl()

export type LoginResult = {
  access_token: string
  refresh_token?: string
  device_id?: string
  user_id: string
  role: string
  expires_in: number
}
export type User = {
  id: string
  email?: string
  display_name: string
  handle?: string
  bio?: string
  role?: string
  status_message?: string
  avatar_color?: string
  avatar_url?: string
  birthday?: string
  birthday_today?: boolean
  is_favorite?: boolean
  is_hidden?: boolean
  is_blocked?: boolean
  nickname?: string
  official?: boolean
}
export type Chat = {
  id: string
  kind: string
  title: string
  last_sequence: number
  last_read_sequence?: number
  unread_count: number
  pinned?: boolean
  muted?: boolean
  hidden?: boolean
  notice?: string
  official?: boolean
  member_count?: number
  members?: User[]
  updated_at?: string
  last_message?: {
    id: string
    kind: string
    body: string
    unsent?: boolean
    sender_id: string
    created_at: string
  } | null
}
export type Message = {
  id: string
  conversation_id: string
  sender_id: string
  client_id: string
  sequence: number
  kind: string
  body: string
  reply_to?: string
  metadata?: Record<string, unknown>
  created_at: string
  unsent?: boolean
  read_count?: number
  reactions?: Record<string, number>
  my_reactions?: string[]
  reply_preview?: {
    sender_id: string
    kind: string
    body: string
    unsent?: boolean
  }
}
export async function request<T>(path: string, token: string | null = null, method = 'GET', body?: unknown): Promise<T> {
  const resp = await fetch(API_URL + path, {
    method,
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
    },
    ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
  })
  const payload = await resp.json().catch(() => ({ error: `HTTP ${resp.status}` }))
  if (!resp.ok) throw new Error(payload.error || `HTTP ${resp.status}`)
  return payload as T
}
export function newClientID(): string {
  return crypto.randomUUID()
}
export function wsURL(ticket: string): string {
  const base = API_URL.startsWith('http')
    ? API_URL.replace(/^http/, 'ws')
    : `${typeof window !== 'undefined' && window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${typeof window !== 'undefined' ? window.location.host : 'localhost:8080'}`
  return base + '/api/v1/ws?ticket=' + encodeURIComponent(ticket)
}
