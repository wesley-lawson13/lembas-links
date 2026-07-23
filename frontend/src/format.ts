// Presentation helpers shared by the dashboard and stats pages. All functions
// take RFC3339 timestamp strings (or raw header strings) straight from the API.

// Whole days from now until the given timestamp, rounded up (a link expiring in
// 26.5 days shows "27"). Called by LinkRow for the expiry countdown.
export function daysUntil(iso: string): number {
  return Math.ceil((Date.parse(iso) - Date.now()) / 86_400_000)
}

// Whether the timestamp is in the past. Called by LinkRow (dimmed rows) and the
// stats page (Active/Expired badge).
export function isExpired(iso: string): boolean {
  return Date.parse(iso) < Date.now()
}

// Human expiry line for a link row: "expires in 26 days" / "expires in 1 day" /
// "expires today". Called by LinkRow for non-expired links.
export function expiresLabel(iso: string): string {
  const days = daysUntil(iso)
  if (days <= 0) return 'expires today'
  return `expires in ${days} ${days === 1 ? 'day' : 'days'}`
}

// "Apr 6, 2026"-style date. Called by the stats page metric cards.
export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

// "Jul 23 · 14:02"-style timestamp. Called by RecentClicksTable's "When" column.
export function formatClickTime(iso: string): string {
  const d = new Date(iso)
  const day = d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
  const time = d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false })
  return `${day} · ${time}`
}

// Weekday label ("Thu") for a backend "YYYY-MM-DD" UTC bucket. UTC is forced on
// both parse and format so labels can never drift off the bucket in other
// timezones. Called by ClicksChart.
export function weekdayLabel(date: string): string {
  return new Date(`${date}T00:00:00Z`).toLocaleDateString('en-US', {
    weekday: 'short',
    timeZone: 'UTC',
  })
}

// Referrer hostname for display ("twitter.com"), or null for direct visits /
// unparseable values worth showing raw. Called by RecentClicksTable.
export function referrerHost(referrer: string): string | null {
  if (!referrer) return null
  try {
    return new URL(referrer).hostname
  } catch {
    return referrer
  }
}

// Coarse "Chrome · macOS"-style summary of a raw User-Agent header — family
// names only, no versions; exotic agents fall back to "Unknown". Called by
// RecentClicksTable. Order matters in both ladders (e.g. Chrome UAs contain
// "Safari", iPad UAs contain "Mac OS X").
export function summarizeUserAgent(ua: string): string {
  let browser = 'Unknown'
  if (/Edg\//.test(ua)) browser = 'Edge'
  else if (/OPR\//.test(ua)) browser = 'Opera'
  else if (/Firefox\//.test(ua)) browser = 'Firefox'
  else if (/Chrome\//.test(ua)) browser = 'Chrome'
  else if (/Safari\//.test(ua)) browser = 'Safari'

  let os = 'Unknown'
  if (/Windows NT/.test(ua)) os = 'Windows'
  else if (/iPhone|iPad/.test(ua)) os = 'iOS'
  else if (/Mac OS X|Macintosh/.test(ua)) os = 'macOS'
  else if (/Android/.test(ua)) os = 'Android'
  else if (/Linux/.test(ua)) os = 'Linux'

  return `${browser} · ${os}`
}
