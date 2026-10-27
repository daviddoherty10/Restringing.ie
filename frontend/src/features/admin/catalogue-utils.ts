import { useState } from 'react'
import { MarketplaceError } from '../../lib/marketplace-api'
import { errorMessage } from '../management/use-remote'
export const sports = ['tennis', 'badminton', 'squash'] as const
export const materials = ['multifilament', 'synthetic_gut', 'polyester', 'natural_gut', 'nylon', 'hybrid'] as const
export const ratings = ['comfort', 'control', 'power', 'spin', 'durability'] as const
export const label = (text: string) => text.replaceAll('_', ' ')
export function useCatalogueSave(creating: boolean, onBusy: (busy: boolean) => void, saved: () => void) {
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [uncertain, setUncertain] = useState(false)
  async function save(write: () => Promise<unknown>) {
    if (busy || uncertain) return
    setBusy(true); onBusy(true); setError('')
    try { await write(); saved() }
    catch (e) {
      const unknown = creating && (!(e instanceof MarketplaceError) || e.status >= 500)
      setUncertain(unknown)
      setError(unknown ? 'The save result is uncertain. Return to the results and search for the record before creating it again.' : errorMessage(e) + (e instanceof MarketplaceError && e.status === 409 ? ' Search for an existing record with this identity.' : ''))
    } finally { setBusy(false); onBusy(false) }
  }
  return { busy, error, uncertain, save }
}
