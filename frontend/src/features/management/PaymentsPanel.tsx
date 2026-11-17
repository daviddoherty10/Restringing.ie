import { useCallback, useState, type FormEvent } from 'react'
import { marketplaceAPI, type ManagementProfile } from '../../lib/marketplace-api'
import type { components } from '../../lib/marketplace-contract'
import { useRemote, errorMessage } from './use-remote'
import { Feedback, LoadMessage } from './dashboard-shared'
type Settings = components['schemas']['PaymentOptions']
function PaymentEditor({ business, value, refresh }: { business: ManagementProfile; value: Settings; refresh: () => void }) {
  const [cash, setCash] = useState(value.onsite_cash), [card, setCard] = useState(value.onsite_card), [online, setOnline] = useState(value.online_enabled)
  const [savedOnline, setSavedOnline] = useState(value.online_enabled)
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [message, setMessage] = useState('')
  const owner = business.permissions.edit_profile
  async function save(e: FormEvent) {
    e.preventDefault(); if (busy) return
    setBusy(true); setError(''); setMessage('')
    try { const saved = await marketplaceAPI.savePaymentSettings(business.id, { onsite_cash: cash, onsite_card: card, online_enabled: online }); setSavedOnline(saved.online_enabled); setMessage('Payment options saved. Existing bookings keep their agreed payment details.') }
    catch (e) { setError(errorMessage(e)) } finally { setBusy(false) }
  }
  async function setup() {
    if (busy) return
    setBusy(true); setError('')
    try { const link = await marketplaceAPI.paymentOnboarding(business.id); window.location.assign(link.url) }
    catch (e) { setError(errorMessage(e)); setBusy(false) }
  }
  return <><p>Choose how customers can pay for their racket. Online payments go directly to your Stripe account. Your business handles payment fees, refunds and formal payment disputes; Restringing.ie takes no transaction fee.</p><form onSubmit={save}><fieldset disabled={busy || !owner}><legend>Accepted payment options</legend><label className="manage-check"><input type="checkbox" checked={cash} onChange={e => setCash(e.target.checked)} /> Cash onsite</label><label className="manage-check"><input type="checkbox" checked={card} onChange={e => setCard(e.target.checked)} /> Card onsite using your own terminal</label><label className="manage-check"><input type="checkbox" checked={online} disabled={!value.online_ready && !online} onChange={e => setOnline(e.target.checked)} /> Pay online through Stripe</label>{owner && <button disabled={busy || (!cash && !card && !online)}>{busy ? 'Saving…' : 'Save payment options'}</button>}</fieldset></form>{!owner && <p>Only the business owner can change payment options or set up Stripe.</p>}<div className="manage-editor"><h3>Online payment setup</h3><p role="status">{!value.online_configured ? 'Online payments have not been configured for this app. You can offer onsite payments now.' : value.online_ready ? savedOnline ? 'Stripe is ready and online payments are enabled for new bookings.' : 'Stripe is ready to accept payments. Enable online payments above to offer them to customers.' : 'Complete Stripe setup, then refresh the status. Online payments stay unavailable until Stripe confirms readiness.'}</p><div className="manage-actions">{owner && value.online_configured && <button type="button" disabled={busy} onClick={setup}>Set up or update Stripe</button>}<button type="button" disabled={busy} onClick={refresh}>Refresh payment status</button></div>{value.online_ready && <p><a href="https://dashboard.stripe.com" target="_blank" rel="noopener noreferrer">Open Stripe Dashboard for payments, refunds and disputes</a></p>}<p>Customers can raise service issues through Restringing.ie. Keep booking messages and service details up to date to help resolve them.</p></div><Feedback error={error} message={message} /></>
}
export default function PaymentsPanel({ business }: { business: ManagementProfile }) {
  const [revision, setRevision] = useState(0)
  const load = useCallback(() => marketplaceAPI.paymentSettings(business.id), [business.id])
  const r = useRemote(`${business.id}:${revision}`, load)
  return <section className="manage-panel" aria-label="Racket payment settings"><h2>Racket payments</h2><LoadMessage {...r} retry={() => setRevision(n => n + 1)} />{r.data && <PaymentEditor key={`${business.id}:${revision}`} business={business} value={r.data} refresh={() => setRevision(n => n + 1)} />}</section>
}
