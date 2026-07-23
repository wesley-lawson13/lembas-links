import { formatClickTime, referrerHost, summarizeUserAgent } from '../format'
import type { ClickEntry } from '../types'

interface RecentClicksTableProps {
  clicks: ClickEntry[]
}

// The "Recent passages" table: When / Came from / User agent / Origin for the
// 10 most recent clicks, with an empty-state line when nothing has passed yet.
// Rendered by the Stats page.
function RecentClicksTable({ clicks }: RecentClicksTableProps) {
  return (
    <div className="passages">
      <div className="passages-title">Recent passages</div>

      {clicks.length === 0 ? (
        <div className="passages-empty muted">No travellers have passed this way yet.</div>
      ) : (
        <>
          <div className="passages-grid passages-head">
            <div>When</div>
            <div>Came from</div>
            <div>User agent</div>
            <div className="passages-right">Origin</div>
          </div>
          {clicks.map((click) => {
            const host = referrerHost(click.referrer)
            return (
              <div key={click.id} className="passages-grid passages-row">
                <div>{formatClickTime(click.clicked_at)}</div>
                {host ? (
                  <div className="truncate">{host}</div>
                ) : (
                  <div className="muted passages-direct">— direct —</div>
                )}
                <div className="muted truncate">{summarizeUserAgent(click.user_agent)}</div>
                <div className="mono muted passages-right passages-ip">{click.ip_address}</div>
              </div>
            )
          })}
          <div className="passages-foot muted">Showing the 10 most recent passages</div>
        </>
      )}
    </div>
  )
}

export default RecentClicksTable
