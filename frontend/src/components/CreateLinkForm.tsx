import { useState } from 'react'
import type { FormEvent } from 'react'
import { createLink, ApiError } from '../api'
import type { CreateLinkResponse } from '../types'

interface CreateLinkFormProps {
  onCreated: (fresh: CreateLinkResponse) => void
}

// The "Forge a short link" form: URL input + accent submit button, with API
// validation/rate-limit errors shown inline underneath. Rendered by Dashboard,
// which receives the created link via onCreated.
function CreateLinkForm({ onCreated }: CreateLinkFormProps) {
  const [url, setUrl] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const trimmed = url.trim()
    if (!trimmed || busy) return
    setBusy(true)
    setError(null)
    try {
      const fresh = await createLink(trimmed)
      onCreated(fresh)
      setUrl('')
    } catch (err) {
      // Keep the input so the user can correct it; surface the API's message.
      setError(err instanceof ApiError ? err.message : 'the forge is cold — try again')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="forge">
      <label className="section-label" htmlFor="forge-input">
        Forge a short link
      </label>
      <form className="forge-row" onSubmit={submit}>
        <input
          id="forge-input"
          type="text"
          placeholder="Paste link here"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
        />
        <button type="submit" className="btn-accent" disabled={busy}>
          Forge link
        </button>
      </form>
      {error && <div className="forge-error warn">{error}</div>}
    </section>
  )
}

export default CreateLinkForm
