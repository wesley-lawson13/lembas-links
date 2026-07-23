import { Link } from 'react-router-dom'
import CopyButton from './CopyButton'
import { daysUntil, expiresLabel, isExpired } from '../format'
import type { LinkSummary } from '../types'

interface LinkRowProps {
  link: LinkSummary
  onDelete: (slug: string) => void
}

// One row of the fellowship list: short URL, click/expiry meta, and the row
// actions. Expired links render dimmed and struck through with only
// Stats/Delete available. Rendered by Dashboard for each listed link.
function LinkRow({ link, onDelete }: LinkRowProps) {
  const expired = isExpired(link.expires_at)
  const expiringSoon = !expired && daysUntil(link.expires_at) <= 3

  return (
    <div className={`link-row${expired ? ' link-row-expired' : ''}`}>
      <div className="link-row-main">
        <div className={`mono truncate link-row-url${expired ? ' link-row-url-expired' : ''}`}>
          {link.short_url}
        </div>
        <div className="link-row-meta muted">
          {link.click_count.toLocaleString('en-US')} {link.click_count === 1 ? 'click' : 'clicks'}
          {' · '}
          {expired ? (
            <span className="link-row-expired-label">expired</span>
          ) : (
            <span className={expiringSoon ? 'warn' : ''}>{expiresLabel(link.expires_at)}</span>
          )}
        </div>
      </div>
      <div className="link-row-actions">
        {!expired && <CopyButton text={link.short_url} className="row-btn" />}
        <Link to={`/stats/${encodeURIComponent(link.slug)}`} className="row-btn">
          Stats
        </Link>
        {!expired && (
          <a href={link.short_url} target="_blank" rel="noreferrer" className="row-btn">
            Open
          </a>
        )}
        <button
          type="button"
          className="row-btn row-btn-danger"
          onClick={() => onDelete(link.slug)}
        >
          Delete
        </button>
      </div>
    </div>
  )
}

export default LinkRow
