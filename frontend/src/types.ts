// TypeScript mirrors of the Go API's JSON contract (api/handlers/types.go).
// All timestamps stay RFC3339 strings at this boundary; format.ts owns Date conversion.

export interface SessionResponse {
  api_key: string
  expires_at: string
}

export interface CreateLinkResponse {
  slug: string
  short_url: string
  original: string
}

export interface LinkSummary {
  slug: string
  short_url: string
  original: string
  click_count: number
  created_at: string
  expires_at: string
}

export interface ListLinksResponse {
  links: LinkSummary[]
}

export interface ClickEntry {
  id: string
  slug: string
  clicked_at: string
  referrer: string
  user_agent: string
  ip_address: string
}

export interface DailyClick {
  date: string // "2026-07-23" (UTC bucket)
  count: number
}

export interface StatsResponse {
  slug: string
  short_url: string
  original: string
  click_count: number
  created_at: string
  expires_at: string
  is_active: boolean
  recent_clicks: ClickEntry[]
  daily_clicks: DailyClick[]
}
