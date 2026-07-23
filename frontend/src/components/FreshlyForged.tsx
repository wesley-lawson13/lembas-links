import { Link } from 'react-router-dom'
import CopyButton from './CopyButton'
import type { CreateLinkResponse } from '../types'

interface FreshlyForgedProps {
  fresh: CreateLinkResponse
}

// The result card shown right after forging a link: the new short URL with a
// copy button, the original it points to, and a link into its stats page.
// Rendered by Dashboard above the fellowship list.
function FreshlyForged({ fresh }: FreshlyForgedProps) {
  return (
    <div className="card fresh-card">
      <div className="caption">Freshly forged</div>
      <div className="fresh-url-row">
        <span className="mono fresh-url">{fresh.short_url}</span>
        <CopyButton text={fresh.short_url} />
      </div>
      <div className="fresh-meta">
        <span className="truncate muted">→ {fresh.original}</span>
        <Link to={`/stats/${encodeURIComponent(fresh.slug)}`} className="fresh-stats-link">
          View stats →
        </Link>
      </div>
    </div>
  )
}

export default FreshlyForged
