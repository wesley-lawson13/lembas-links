import { useCallback, useEffect, useState } from 'react'
import { deleteLink, listLinks, ApiError } from '../api'
import CreateLinkForm from '../components/CreateLinkForm'
import FreshlyForged from '../components/FreshlyForged'
import LinkRow from '../components/LinkRow'
import type { CreateLinkResponse, LinkSummary } from '../types'
import '../styles/dashboard.css'

// Dashboard page (route "/"): the forge form, the freshly-forged result card,
// and the caller's fellowship of links, loaded on mount and refreshed after
// every create/delete.
function Dashboard() {
  const [links, setLinks] = useState<LinkSummary[] | null>(null)
  const [fresh, setFresh] = useState<CreateLinkResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  // Re-fetches the authoritative, server-sorted list; called on mount and
  // after create/delete rather than mutating local state optimistically.
  const refresh = useCallback(async () => {
    try {
      const resp = await listLinks()
      setLinks(resp.links)
      setError(null)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'could not reach the archives')
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const handleCreated = (created: CreateLinkResponse) => {
    setFresh(created)
    void refresh()
  }

  const handleDelete = async (slug: string) => {
    if (!window.confirm(`Cast «${slug}» into the fire?`)) return
    try {
      await deleteLink(slug)
    } catch (err) {
      // A 404 here means it's already gone (double-click race) — refreshing
      // resolves either way; other errors surface in the list error line.
      if (!(err instanceof ApiError && err.status === 404)) {
        setError(err instanceof ApiError ? err.message : 'could not delete the link')
      }
    }
    if (fresh?.slug === slug) setFresh(null)
    void refresh()
  }

  return (
    <>
      <header className="dash-header">
        <h1>Lembas Links</h1>
      </header>

      <CreateLinkForm onCreated={handleCreated} />

      {fresh && <FreshlyForged fresh={fresh} />}

      <section className="fellowship">
        <div className="fellowship-head">
          <h2>Your fellowship of links</h2>
          {links !== null && links.length > 0 && (
            <span className="muted fellowship-count">
              {links.length} forged
            </span>
          )}
        </div>

        {error && <div className="warn fellowship-error">{error}</div>}

        {links === null && !error && (
          <div className="muted fellowship-loading">Consulting the archives…</div>
        )}

        {links !== null && links.length === 0 && !fresh && (
          <div className="state-panel">
            <p className="state-title">The road goes ever on</p>
            <p className="state-body">No links yet forged. Paste a URL above to begin the journey.</p>
          </div>
        )}

        {links !== null &&
          links.map((link) => <LinkRow key={link.slug} link={link} onDelete={handleDelete} />)}
      </section>
    </>
  )
}

export default Dashboard
