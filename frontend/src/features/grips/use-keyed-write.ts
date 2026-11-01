import { useRef, useState } from 'react'
import { MarketplaceError } from '../../lib/marketplace-api'
import { errorMessage } from '../management/use-remote'
// Retain the exact payload and key across ambiguous retries while mounted.
export function useKeyedWrite<T, R>(
  send: (input: T, key: string) => Promise<R>,
) {
  const attempt = useRef<{ input: T; key: string } | null>(null)
  const [busy, setBusy] = useState(false),
    [uncertain, setUncertain] = useState(false),
    [error, setError] = useState('')
  async function run(input: T): Promise<R | undefined> {
    if (busy) return
    const current = attempt.current ?? { input, key: crypto.randomUUID() }
    attempt.current = current
    setBusy(true)
    setError('')
    try {
      const out = await send(current.input, current.key)
      attempt.current = null
      setUncertain(false)
      return out
    } catch (e) {
      const ambiguous = !(e instanceof MarketplaceError) || e.status >= 500
      setUncertain(ambiguous)
      if (!ambiguous) attempt.current = null
      setError(
        errorMessage(e) +
          (ambiguous
            ? ' The result is uncertain. Retry the same request to check safely.'
            : ''),
      )
    } finally {
      setBusy(false)
    }
  }
  return { run, busy, uncertain, error, locked: busy || uncertain }
}
