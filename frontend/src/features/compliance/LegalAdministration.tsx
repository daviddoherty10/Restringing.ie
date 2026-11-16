import { useState, type FormEvent } from 'react'
import {
  MarketplaceHeader,
  MarketplaceFooter,
} from '../../components/layout/MarketplaceLayout'
import { marketplaceAPI } from '../../lib/marketplace-api'
import type { components } from '../../lib/marketplace-contract'
import { errorMessage } from '../management/use-remote'
import '../management/Management.css'
export default function LegalAdministration() {
  const [business, setBusiness] = useState(''),
    [fields, setFields] = useState<Record<string, string>>({}),
    [reviewed, setReviewed] = useState(false),
    [days, setDays] = useState(''),
    [message, setMessage] = useState(''),
    [busy, setBusy] = useState(false)
  async function submit(e: FormEvent) {
    e.preventDefault()
    if (busy) return
    setBusy(true)
    setMessage('')
    try {
      const me = await marketplaceAPI.me()
      if (!me.admin) throw Error('Administrator access is required.')
      const document = {
        ...fields,
        trader: {
          name: fields.name,
          address: fields.address,
          email: fields.email,
          phone: fields.phone,
          trader_status: fields.trader_status,
        },
        withdrawal_days: Number(days),
        reviewed,
      } as components['schemas']['LegalDocument']
      for (const key of ['name', 'address', 'email', 'phone', 'trader_status'])
        delete (document as unknown as Record<string, unknown>)[key]
      await marketplaceAPI.publishLegal(business, document)
      setMessage(
        'Reviewed version recorded. Publishing does not activate the legal workflow; deployment activation remains a separate reviewed step.',
      )
    } catch (err) {
      setMessage(
        err instanceof Error &&
          err.message === 'Administrator access is required.'
          ? err.message
          : errorMessage(err),
      )
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="marketplace">
      <MarketplaceHeader />
      <main id="main-content" tabIndex={-1} className="marketplace-container manage-page manage-content compliance-page">
        <h1>Reviewed booking wording</h1>
        <p>
          Administrator only. Supply wording approved for this trader; no legal
          text is generated here. Do not activate public bookings until operator
          identity, withdrawal treatment, tax display and confirmation delivery
          have been reviewed.
        </p>
        <form onSubmit={submit}>
          <fieldset disabled={busy}>
            <legend>New immutable version</legend>
            <label>
              Stringer ID
              <input
                required
                value={business}
                onChange={(e) => setBusiness(e.target.value)}
              />
            </label>
            {[
              'version',
              'name',
              'address',
              'email',
              'phone',
              'trader_status',
              'terms',
              'cancellation_notice',
              'price_notice',
              'turnaround',
              'acceptance_label',
              'early_request_label',
              'early_acknowledgement_label',
              'submit_label',
              'review_reference',
            ].map((key) => (
              <label key={key}>
                {key.replaceAll('_', ' ')}
                <textarea
                  required
                  maxLength={4000}
                  value={fields[key] ?? ''}
                  onChange={(e) =>
                    setFields((v) => ({ ...v, [key]: e.target.value }))
                  }
                />
              </label>
            ))}
            <label>
              Reviewed withdrawal period in days
              <input
                type="number"
                min={1}
                max={60}
                required
                value={days}
                onChange={(e) => setDays(e.target.value)}
              />
            </label>
            <label>
              <input
                type="checkbox"
                required
                checked={reviewed}
                onChange={(e) => setReviewed(e.target.checked)}
              />
              I confirm this supplied wording and the public trader details were
              approved for use; the review reference identifies that approval.
            </label>
            <button>Record reviewed version</button>
          </fieldset>
        </form>
        {message && <p role="status">{message}</p>}
      </main>
      <MarketplaceFooter />
    </div>
  )
}
