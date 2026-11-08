import { useCallback, useState } from 'react'
import { marketplaceAPI } from '../../lib/marketplace-api'
import {
  MarketplaceHeader,
  MarketplaceFooter,
} from '../../components/layout/MarketplaceLayout'
import { useRemote } from '../management/use-remote'
import { LoadMessage } from '../management/dashboard-shared'
export default function BookingConfirmationPage({ id }: { id: string }) {
  const [retry, setRetry] = useState(0),
    load = useCallback(() => marketplaceAPI.bookingConfirmation(id), [id]),
    r = useRemote(`${id}:${retry}`, load)
  function download() {
    if (!r.data) return
    const url = URL.createObjectURL(
      new Blob([JSON.stringify(r.data, null, 2)], { type: 'application/json' }),
    )
    const a = document.createElement('a')
    a.href = url
    a.download = `booking-confirmation-${id}.json`
    a.click()
    URL.revokeObjectURL(url)
  }
  return (
    <div className="marketplace">
      <MarketplaceHeader />
      <main className="marketplace-container" style={{ padding: '2rem 0' }}>
        <h1>Booking confirmation</h1>
        <LoadMessage {...r} retry={() => setRetry((n) => n + 1)} />
        {r.data && (
          <>
            <p>
              Recorded {new Date(r.data.recorded_at).toLocaleString('en-IE')}.
              Keep a copy of the exact booking and contractual information
              below. This record is separate from confirmation that cash has
              been paid.
            </p>
            <button onClick={download}>Download confirmation</button>
            <pre style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>
              {JSON.stringify(r.data.snapshot, null, 2)}
            </pre>
          </>
        )}
      </main>
      <MarketplaceFooter />
    </div>
  )
}
