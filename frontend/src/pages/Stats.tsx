import { Link } from 'react-router-dom'

// Stats page: per-slug metrics, 7-day chart, and recent passages.
// Fleshed out in the stats commit — this scaffold renders the shell only.
function Stats() {
  return (
    <div>
      <Link to="/" className="back-link">
        ← Return to dashboard
      </Link>
    </div>
  )
}

export default Stats
