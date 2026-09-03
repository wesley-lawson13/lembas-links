// Typed client for the Lembas Links API. Owns the anonymous-key lifecycle:
// a key is silently minted via POST /session on first use, cached in
// localStorage, and transparently re-minted (with a single retry) on 401.

import type {
  CreateLinkResponse,
  ListLinksResponse,
  SessionResponse,
  StatsResponse,
} from './types'

const API_BASE = (import.meta.env.VITE_API_BASE_URL as string) ?? ''
const KEY_STORAGE = 'lembas_api_key'

// ApiError carries the HTTP status alongside the server's {"error": "..."} message
// so callers (pages) can branch on status (e.g. 404 → themed not-found state).
export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

// Extracts the server's {"error"} message from a failed response, falling back
// to the HTTP status text. Called by getOrCreateApiKey and request on non-ok responses.
async function parseError(res: Response): Promise<string> {
  try {
    const body = (await res.json()) as { error?: string }
    if (body.error) return body.error
  } catch {
    // non-JSON body — fall through to statusText
  }
  return res.statusText || `request failed with status ${res.status}`
}

// Returns the cached anonymous API key, minting a fresh one via POST /session
// (unauthenticated) when none is stored. Called by request() before every API call.
export async function getOrCreateApiKey(): Promise<string> {
  const cached = localStorage.getItem(KEY_STORAGE)
  if (cached) return cached

  const res = await fetch(`${API_BASE}/session`, { method: 'POST' })
  if (!res.ok) throw new ApiError(res.status, await parseError(res))

  const body = (await res.json()) as SessionResponse
  localStorage.setItem(KEY_STORAGE, body.api_key)
  return body.api_key
}

// The single fetch core every public API function goes through: attaches the
// Bearer key, and on a 401 (stale/expired key) discards it, mints a new one,
// and retries exactly once — a second 401 throws.
async function request<T>(path: string, init: RequestInit = {}, retried = false): Promise<T> {
  const key = await getOrCreateApiKey()

  const headers = new Headers(init.headers)
  headers.set('Authorization', `Bearer ${key}`)
  if (init.body !== undefined) headers.set('Content-Type', 'application/json')

  const res = await fetch(`${API_BASE}${path}`, { ...init, headers })

  if (res.status === 401 && !retried) {
    localStorage.removeItem(KEY_STORAGE)
    return request<T>(path, init, true)
  }
  if (res.status === 204) return undefined as T
  if (!res.ok) throw new ApiError(res.status, await parseError(res))

  return (await res.json()) as T
}

// Creates a short link for the given URL. Called from the dashboard's forge form.
export function createLink(url: string): Promise<CreateLinkResponse> {
  return request<CreateLinkResponse>('/links', {
    method: 'POST',
    body: JSON.stringify({ url }),
  })
}

// Lists the caller's links, newest first (server-sorted). Called on dashboard
// mount and after create/delete to refresh the fellowship list.
export function listLinks(): Promise<ListLinksResponse> {
  return request<ListLinksResponse>('/links')
}

// Fetches detail stats (metrics, daily click counts, recent clicks) for one
// owned slug. Called by the stats page on mount.
export function getStats(slug: string): Promise<StatsResponse> {
  return request<StatsResponse>(`/links/${encodeURIComponent(slug)}/stats`)
}

// Soft-deletes an owned link. Called from the dashboard's per-row Delete button.
export function deleteLink(slug: string): Promise<void> {
  return request<void>(`/links/${encodeURIComponent(slug)}`, { method: 'DELETE' })
}
