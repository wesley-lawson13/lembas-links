import { useEffect, useRef, useState } from 'react'

interface CopyButtonProps {
  text: string
  className?: string
}

// Writes `text` to the clipboard and briefly flips its label to "Copied".
// Used by the freshly-forged card, the stats header (className "btn-outline")
// and list rows (className "row-btn").
function CopyButton({ text, className = 'btn-outline' }: CopyButtonProps) {
  const [copied, setCopied] = useState(false)
  const timer = useRef<number | undefined>(undefined)

  useEffect(() => () => window.clearTimeout(timer.current), [])

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      window.clearTimeout(timer.current)
      timer.current = window.setTimeout(() => setCopied(false), 1500)
    } catch {
      // clipboard unavailable (e.g. insecure context) — leave the label as-is
    }
  }

  return (
    <button type="button" className={className} onClick={copy}>
      {copied ? 'Copied' : 'Copy'}
    </button>
  )
}

export default CopyButton
