import { useCallback, useEffect, useState } from 'react'
import { MarketplaceError } from '../../lib/marketplace-api'
export function errorMessage(error: unknown): string {
  return error instanceof MarketplaceError ? [error.message, ...Object.values(error.fields)].join(' ') : 'Unable to connect. Please try again.'
}
// The result belongs to one explicit resource key. Changing business, filter or
// page immediately hides previous data, even if an older request finishes later.
export function useRemote<T>(key: string, load: () => Promise<T>) {
  const [result, setResult] = useState<{ key: string; data?: T; error?: string }>({ key: '' })
  useEffect(() => {
    let active = true
    load().then(data => { if (active) setResult({ key, data }) }).catch(e => { if (active) setResult({ key, error: errorMessage(e) }) })
    return () => { active = false }
  }, [key, load])
  const updateData = useCallback((update: (data: T) => T) => {
    setResult(current => current.key === key && current.data !== undefined ? { ...current, data: update(current.data) } : current)
  }, [key])
  return { updateData, loading: result.key !== key, data: result.key === key ? result.data : undefined, error: result.key === key ? result.error : undefined }
}
