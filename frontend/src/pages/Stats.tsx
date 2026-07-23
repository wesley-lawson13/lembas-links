import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getStats, ApiError } from '../api'
import ClicksChart from '../components/ClicksChart'
import CopyButton from '../components/CopyButton'
import RecentClicksTable from '../components/RecentClicksTable'
import { formatDate, isExpired } from '../format'
import type { StatsResponse } from '../types'
import '../styles/stats.css'

// Stats page (route "/stats/:slug"): fetches one owned slug's detail on mount
// and renders the header, points-to card, metric cards, 7-day chart, and
// recent passages. A 404 (nonexistent, not owned, or deleted — the API keeps
// them indistinguishable) gets a themed not-found state.
function Stats() {
  const { slug } = useParams<{ slug: string }>()
  const [stats, setStats] = useState<StatsResponse | null>(null)
  const [error, setError] = useState<'notfound' | string | null>(null)

  useEffect(() => {
    if (!slug) return
    setStats(null)
    setError(null)
    getStats(slug)
      .then(setStats)
      .catch((err) => {
        if (err instanceof ApiError && err.status === 404) setError('notfound')
        else setError(err instanceof ApiError ? err.message : 'could not reach the archives')
      })
  }, [slug])

  const backLink = (
    <Link to="/" className="back-link">
      ← Return to dashboard
    </Link>
  )

  if (error === 'notfound') {
    return (
      <div>
        {backLink}
        <div className="state-panel">
          <p className="state-title">This passage is not recorded</p>
          <p className="state-body">
            The Red Book holds no account of «{slug}» — it may never have been forged, or its
            tale belongs to another.
          </p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div>
        {backLink}
        <div className="state-panel">
          <p className="state-title">The palantír is dark</p>
          <p className="state-body">{error}</p>
        </div>
      </div>
    )
  }

  if (!stats) {
    return (
      <div>
        {backLink}
        <div className="muted stats-loading">Consulting the archives…</div>
      </div>
    )
  }

  const expired = isExpired(stats.expires_at)

  return (
    <div>
      {backLink}

      <div className="stats-head">
        <div className="stats-head-main">
          <h1>{stats.slug}</h1>
          <div className="stats-url-row">
            <span className="mono stats-url">{stats.short_url}</span>
            <CopyButton text={stats.short_url} className="btn-outline btn-outline-sm" />
          </div>
        </div>
        <span className={`badge ${expired ? 'badge-expired' : 'badge-active'}`}>
          <span className="badge-dot" />
          {expired ? 'Expired' : 'Active'}
        </span>
      </div>

      <div className="card points-to">
        <div className="points-to-main">
          <div className="points-to-caption">Points to</div>
          <div className="truncate points-to-url">{stats.original}</div>
        </div>
        <a href={stats.original} target="_blank" rel="noreferrer" className="points-to-open">
          Open ↗
        </a>
      </div>

      <div className="metrics">
        <div className="metric">
          <div className="metric-value metric-value-big">
            {stats.click_count.toLocaleString('en-US')}
          </div>
          <div className="metric-label">Clicks</div>
        </div>
        <div className="metric">
          <div className="metric-value">{formatDate(stats.created_at)}</div>
          <div className="metric-label">Forged on</div>
        </div>
        <div className="metric">
          <div className="metric-value">{formatDate(stats.expires_at)}</div>
          <div className="metric-label">Expires at</div>
        </div>
      </div>

      <ClicksChart days={stats.daily_clicks} />

      <RecentClicksTable clicks={stats.recent_clicks} />
    </div>
  )
}

export default Stats
