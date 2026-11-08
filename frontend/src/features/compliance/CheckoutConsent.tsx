import { useCallback, useEffect, useState } from 'react'
import { marketplaceAPI } from '../../lib/marketplace-api'
import type { components } from '../../lib/marketplace-contract'
import { useRemote } from '../management/use-remote'
import { LoadMessage } from '../management/dashboard-shared'
type Acceptance = components['schemas']['BookingAcceptance']
export default function CheckoutConsent({
  id,
  locked,
  changed,
  ready,
  labelChanged,
}: {
  id: string
  locked: boolean
  changed: (a: Acceptance | undefined) => void
  ready: (v: boolean) => void
  labelChanged: (label: string) => void
}) {
  const [retry, setRetry] = useState(0),
    [accepted, setAccepted] = useState(false),
    [early, setEarly] = useState(false),
    [ack, setAck] = useState(false)
  const load = useCallback(() => marketplaceAPI.checkoutDisclosure(id), [id])
  const r = useRemote(`${id}:${retry}`, load)
  useEffect(() => {
    const d = r.data
    if (!d) {
      ready(false)
      return
    }
    if (!d.enabled) {
      labelChanged('Book — pay cash to stringer')
      changed(undefined)
      ready(true)
      return
    }
    labelChanged(d.document!.submit_label)
    changed({
      version_id: d.version_id!,
      accepted,
      early_request: early,
      early_acknowledgement: ack,
    })
    ready(accepted && early === ack)
  }, [r.data, accepted, early, ack, changed, ready,labelChanged])
  return (
    <section className="checkout-consent" aria-label="Booking disclosures">
      <LoadMessage {...r} retry={() => setRetry((n) => n + 1)} />
      {r.data && !r.data.enabled && (
        <p>
          Pre-launch preview: draft legal pages are not an operative agreement.
          The reviewed contractual workflow has not been activated.
        </p>
      )}
      {r.data?.enabled && r.data.document && (
        <>
          <h3>Contracting stringer</h3>
          <address>
            {r.data.document.trader.name}
            <br />
            {r.data.document.trader.address}
            <br />
            {r.data.document.trader.email}
            <br />
            {r.data.document.trader.phone}
          </address>
          <p>{r.data.document.trader.trader_status}</p>
          <p>{r.data.document.price_notice}</p>
          <p>{r.data.document.turnaround}</p>
          <p>
            Payment: cash directly to this stringer. Review the exact quote and
            racket configuration above.
          </p>
          <details>
            <summary>Contractual terms — {r.data.document.version}</summary>
            <p style={{ whiteSpace: 'pre-wrap' }}>{r.data.document.terms}</p>
          </details>
          <details>
            <summary>Cancellation and withdrawal notice</summary>
            <p style={{ whiteSpace: 'pre-wrap' }}>
              {r.data.document.cancellation_notice}
            </p>
          </details>
          <fieldset disabled={locked}>
            <legend>Your choices</legend>
            <label>
              <input
                type="checkbox"
                checked={accepted}
                onChange={(e) => setAccepted(e.target.checked)}
              />
              {r.data.document.acceptance_label}
            </label>
            <label>
              <input
                type="checkbox"
                checked={early}
                onChange={(e) => setEarly(e.target.checked)}
              />
              {r.data.document.early_request_label}
            </label>
            <label>
              <input
                type="checkbox"
                checked={ack}
                onChange={(e) => setAck(e.target.checked)}
              />
              {r.data.document.early_acknowledgement_label}
            </label>
            <p>
              Leave both early-performance choices unselected to wait until the
              disclosed withdrawal period has ended before work starts.
            </p>
          </fieldset>
        </>
      )}
    </section>
  )
}
