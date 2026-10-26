import { useCallback, useEffect, useState } from 'react'
import { marketplaceAPI, type ManagementProfile } from '../../lib/marketplace-api'
import { Feedback, LoadMessage } from './dashboard-shared'
import { errorMessage, useRemote } from './use-remote'
import { euro } from './dashboard-utils'
export default function BillingPanel({ business, revision, onUsage }: { business: ManagementProfile; revision: number; onUsage: (allowed: boolean) => void }) {
  const [retry, setRetry] = useState(0), [busy, setBusy] = useState(false), [error, setError] = useState('')
  const load = useCallback(() => marketplaceAPI.businessUsage(business.id), [business.id])
  const r = useRemote(`${business.id}:${revision}:${retry}`, load)
  useEffect(() => { if (r.data) onUsage(r.data.can_accept) }, [r.data, onUsage])
  async function billing(portal: boolean) {
    if (busy) return
    setBusy(true); setError('')
    try {
      const result = await (portal ? marketplaceAPI.subscriptionPortal(business.id) : marketplaceAPI.subscriptionCheckout(business.id))
      const url = new URL(result.url)
      if (url.protocol !== 'https:' || url.username || url.password || !['checkout.stripe.com', 'billing.stripe.com'].includes(url.host)) throw new Error('Invalid billing destination')
      window.location.assign(url.href)
    } catch (e) { setError(errorMessage(e)); setBusy(false) }
  }
  return <section className="manage-panel" aria-label="Plan and billing"><div className="manage-section-heading"><h2>Plan & billing</h2><button type="button" disabled={busy} onClick={() => setRetry(n => n + 1)}>Refresh usage</button></div><LoadMessage {...r} retry={() => setRetry(n => n + 1)} /><Feedback error={error} message="" />{r.data && <>
    <p><strong>{r.data.plan === 'paid' ? 'Paid plan · unlimited bookings' : 'Free plan · 5 accepted bookings per month'}</strong></p>
    <p>{r.data.accepted_bookings} bookings accepted this month{r.data.plan === 'free' ? ` · ${r.data.remaining} remaining` : ''}.</p>
    <p>Allowance resets {new Date(r.data.resets_at).toLocaleString('en-IE', { timeZone: 'Europe/Dublin' })} (Irish time). Shared across this business and its staff. Accepted bookings still count after cancellation.</p>
    {!r.data.can_accept && <p role="status" className="manage-feedback">Monthly allowance reached. Upgrade or wait for the reset to accept another booking. Existing orders and business tools remain available.</p>}
    <details><summary>Subscription details</summary>{business.approval_status!=='approved' && <p>Business approval is required before starting a subscription. Existing billing can still be managed.</p>}<p>Free: five accepted racket bookings per Irish calendar month. Paid: unlimited bookings.</p><p>Customer cash payments go directly to you. The subscription pays for your use of Restringing.ie.</p>
      {r.data.monthly_price_cents != null && <p>Paid plan: {euro(r.data.monthly_price_cents)} per month, tax inclusive. Confirm the recurring charge in secure checkout.</p>}
      {r.data.subscription_status !== 'none' && <p>Subscription status: {r.data.subscription_status.replaceAll('_', ' ')}.</p>}
      {r.data.paid_until && <p>Paid access through {new Date(r.data.paid_until).toLocaleString('en-IE', { timeZone: 'Europe/Dublin' })}.{r.data.cancel_at_period_end ? ' Cancellation is scheduled; the free allowance applies when paid access ends.' : ''}</p>}
      {!r.data.billing_available && <p>Subscription checkout is currently unavailable. Free-plan tools and existing orders remain available.</p>}
      {business.permissions.manage_billing ? <div className="manage-actions">{r.data.billing_available && r.data.plan === 'free' && !r.data.portal_available && <button type="button" disabled={busy || business.approval_status!=='approved'} onClick={() => billing(false)}>Upgrade to paid</button>}{r.data.portal_available && <><button type="button" disabled={busy} onClick={() => billing(true)}>Manage subscription</button>{r.data.billing_available && ['none', 'canceled', 'incomplete_expired'].includes(r.data.subscription_status) && <button type="button" disabled={busy || business.approval_status!=='approved'} onClick={() => billing(false)}>Start paid subscription</button>}</>}</div> : <p>Only a business owner or administrator can manage the subscription.</p>}
      <p>Returning from checkout does not activate paid access. Refresh usage after billing confirmation.</p>
    </details>
  </>}</section>
}
