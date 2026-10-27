import { useCallback, useEffect, useState } from 'react'
import { MarketplaceError } from '../../lib/marketplace-api'

export function useLive<T>(load: () => Promise<T>, interval = 15000) {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState('')
  const [revision, setRevision] = useState(0)
  const refresh = useCallback(() => setRevision((n) => n + 1), [])
  useEffect(() => {
    let active = true
    let timer: ReturnType<typeof setTimeout>
    let running = false
    async function run() {
      if (running || !active) return
      clearTimeout(timer)
      if (document.visibilityState !== 'hidden') {
        running = true
        try {
          const result = await load()
          if (active) {
            setData(result)
            setError('')
          }
        } catch (e) {
          if (active) {
            if (
              e instanceof MarketplaceError &&
              [401, 403, 404].includes(e.status)
            )
              setData(null)
            setError(
              e instanceof MarketplaceError
                ? e.message
                : 'Updates are temporarily unavailable. Please retry.',
            )
          }
        } finally {
          running = false
        }
      }
      if (active) timer = setTimeout(run, interval)
    }
    const visible = () => {
      if (document.visibilityState === 'visible') void run()
    }
    void run()
    document.addEventListener('visibilitychange', visible)
    return () => {
      active = false
      clearTimeout(timer)
      document.removeEventListener('visibilitychange', visible)
    }
  }, [load, interval, revision])
  return { data, error, refresh }
}
