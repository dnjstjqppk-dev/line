export const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'
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
}
export type Chat = {
  id: string
  kind: string
  title: string
  last_sequence: number
  last_read_sequence?: number
  unread_count: number
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
  return API_URL.replace(/^http/, 'ws') + '/api/v1/ws?ticket=' + encodeURIComponent(ticket)
}
