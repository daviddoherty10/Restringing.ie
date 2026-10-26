import { useCallback, useState, type FormEvent } from 'react'
import { marketplaceAPI, type ManagementProfile, type Stringer } from '../../lib/marketplace-api'
import { MarketplaceHeader, MarketplaceFooter } from '../../components/layout/MarketplaceLayout'
import { signInURL } from '../../lib/auth-navigation'
import { Feedback, LoadMessage, Pagination } from './dashboard-shared'
import { errorMessage, useRemote } from './use-remote'
import { euro } from './dashboard-utils'
import OrdersPanel from './OrdersPanel'
import InventoryPanel from './InventoryPanel'
import ServicesPanel from './ServicesPanel'
import ProfilePanel from './ProfilePanel'
import './Management.css'
function Summary({ id, revision }: { id: string; revision: number }) {
  const [retry, setRetry] = useState(0)
  const load = useCallback(() => marketplaceAPI.orderSummary(id), [id])
  const r = useRemote(`${id}:${revision}:${retry}`, load)
  return <section aria-label="Business booking totals"><LoadMessage {...r} retry={() => setRetry(n => n + 1)} />{r.data && <><div className="manage-summary">{[['All bookings', String(r.data.total_orders)], ['Active orders', String(r.data.active_orders)], ['Completed orders', String(r.data.completed_orders)], ['Booking value', euro(r.data.booked_value_cents)]].map(([title, value]) => <div key={title}><span>{title}</span><strong>{value}</strong></div>)}</div><p className="manage-muted">All-time totals for this business. Booking value excludes cancelled orders and is not payment revenue. {r.data.unpaid_orders} non-cancelled orders are unpaid.</p></>}</section>
}
function Workspace({ id, refresh, profileChanged }: { id: string; refresh: number; profileChanged: (profile: Stringer) => void }) {
  const [tab, setTab] = useState('orders'), [revision, setRevision] = useState(0), [summaryRevision, setSummaryRevision] = useState(0), [notice, setNotice] = useState('')
  const load = useCallback(() => marketplaceAPI.managementProfile(id), [id])
  const r = useRemote(`${id}:${revision}:${refresh}`, load)
  return <><LoadMessage {...r} retry={() => setRevision(n => n + 1)} />{notice && <p role="status" className="manage-feedback">{notice}</p>}{r.data && <><div className="manage-business-heading"><div><h2>{r.data.name}</h2><p>{r.data.area}, {r.data.county} · Your role: <strong>{r.data.membership_role}</strong></p><div className="manage-badges"><span>{r.data.approval_status}</span><span>{r.data.accepting_orders ? 'Bookings enabled when approved' : 'New bookings paused'}</span></div></div>{r.data.approval_status === 'approved' && <a href={`/stringers/${id}`}>View public profile ↗</a>}</div>{r.data.approval_status !== 'approved' && <p className="manage-feedback">This business is {r.data.approval_status}. New bookings are blocked; existing orders and stock can still be managed.</p>}<Summary id={id} revision={summaryRevision + refresh} /><nav className="manage-tabs" aria-label="Business sections">{[['orders', 'Orders'], ['inventory', 'Inventory'], ['services', 'Services'], ['profile', 'Profile']].map(([key, title]) => <button type="button" key={key} aria-pressed={tab === key} onClick={() => { setTab(key); setNotice('') }}>{title}</button>)}</nav>{tab === 'orders' && <OrdersPanel business={r.data} changed={() => setSummaryRevision(n => n + 1)} />}{tab === 'inventory' && <InventoryPanel business={r.data} />}{tab === 'services' && <ServicesPanel business={r.data} />}{tab === 'profile' && <ProfilePanel business={r.data} changed={updated => { setNotice('Business profile saved.'); setRevision(n => n + 1); profileChanged(updated) }} />}</>}</>
}
function ApplicationForm({ saved }: { saved: (id: string) => void }) {
  const [name, setName] = useState(''), [area, setArea] = useState(''), [county, setCounty] = useState(''), [description, setDescription] = useState(''), [busy, setBusy] = useState(false), [error, setError] = useState(''), [message, setMessage] = useState('')
  async function submit(e: FormEvent) {
    e.preventDefault(); if (busy) return
    setBusy(true); setError(''); setMessage('')
    try { const shop = await marketplaceAPI.applyStringer({ name, area, county, description, accepting_orders: false }); setMessage('Application sent for review.'); setName(''); setArea(''); setCounty(''); setDescription(''); saved(shop.id) }
    catch (e) { setError(errorMessage(e)) } finally { setBusy(false) }
  }
  return <details className="manage-panel"><summary>Apply to become a stringer</summary><form onSubmit={submit}><fieldset disabled={busy}><legend>New business application</legend><label>New business name<input maxLength={120} required value={name} onChange={e => setName(e.target.value)} /></label><div className="manage-form-grid"><label>New business area<input maxLength={120} value={area} onChange={e => setArea(e.target.value)} /></label><label>New business county<input maxLength={120} value={county} onChange={e => setCounty(e.target.value)} /></label></div><label>New business description<textarea maxLength={2000} value={description} onChange={e => setDescription(e.target.value)} /></label><button>{busy ? 'Submitting…' : 'Submit application'}</button></fieldset></form><Feedback error={error} message={message} /></details>
}
function ReviewApplication({ business, reviewed, manage }: { business: Stringer; reviewed: () => void; manage: (id: string) => void }) {
  const [decision, setDecision] = useState<'approved' | 'rejected'>('approved'), [reason, setReason] = useState(''), [busy, setBusy] = useState(false), [error, setError] = useState('')
  async function submit(e: FormEvent) {
    e.preventDefault(); if (busy || !reason.trim()) return
    setBusy(true); setError('')
    try { await marketplaceAPI.admin.reviewStringer(business.id, decision, reason.trim()); reviewed() }
    catch (e) { setError(errorMessage(e)); setBusy(false) }
  }
  return <article><h3>{business.name}</h3><p>{business.area}, {business.county}</p><p>{business.description}</p><button type="button" className="manage-secondary" onClick={() => manage(business.id)}>Inspect business</button><form onSubmit={submit}><fieldset disabled={busy}><legend>Review {business.name}</legend><label>Decision<select value={decision} onChange={e => setDecision(e.target.value as typeof decision)}><option value="approved">Approve</option><option value="rejected">Reject</option></select></label><label>Reason for decision<textarea required maxLength={1000} value={reason} onChange={e => setReason(e.target.value)} /></label><button>{busy ? 'Saving…' : 'Submit decision'}</button></fieldset></form><Feedback error={error} message="" /></article>
}
function Applications({ reviewed, manage }: { reviewed: () => void; manage: (id: string) => void }) {
  const [page, setPage] = useState(1), [revision, setRevision] = useState(0), [message, setMessage] = useState('')
  const load = useCallback(() => marketplaceAPI.admin.applications({ page, page_size: 10 }), [page])
  const r = useRemote(`${page}:${revision}`, load)
  return <section className="manage-panel"><h2>Pending applications</h2><LoadMessage {...r} retry={() => setRevision(n => n + 1)} />{message && <p role="status">{message}</p>}{r.data && <>{r.data.data.length === 0 ? <p>No pending applications.</p> : r.data.data.map(s => <ReviewApplication key={s.id} business={s} manage={manage} reviewed={() => { setRevision(n => n + 1); setMessage('Application decision recorded.'); reviewed() }} />)}<Pagination page={page} size={10} total={r.data.total} change={setPage} /></>}</section>
}
export default function Management() {
  const [revision, setRevision] = useState(0), [refresh, setRefresh] = useState(0)
  const [selected, setSelected] = useState(() => new URLSearchParams(window.location.search).get('business') ?? '')
  const [adminView, setAdminView] = useState(false)
  const load = useCallback(async () => { const [me, businesses] = await Promise.all([marketplaceAPI.me(), marketplaceAPI.myStringers()]); return { me, businesses } }, [])
  const r = useRemote(`account:${revision}`, load)
  const id = selected || r.data?.businesses[0]?.id || ''
  function select(value: string) { setSelected(value); setAdminView(false); window.history.replaceState(null, '', value ? `/manage?business=${encodeURIComponent(value)}` : '/manage') }
  return <div className="marketplace"><MarketplaceHeader backHref="/" backLabel="Home" /><main className="marketplace-container manage-content"><p className="marketplace-kicker">BUSINESS PORTAL</p><h1>Stringer dashboard</h1><LoadMessage {...r} retry={() => setRevision(n => n + 1)} />{r.error && <a href={signInURL('/manage')}>Sign in to access your businesses</a>}{r.data && <><div className="manage-portal-bar"><label>Your business<select value={id} onChange={e => select(e.target.value)}><option value="">Choose business</option>{r.data.businesses.map((s: ManagementProfile) => <option key={s.id} value={s.id}>{s.name} · {s.membership_role} · {s.approval_status}</option>)}{id && !r.data.businesses.some(s => s.id === id) && <option value={id}>Selected business</option>}</select></label>{r.data.me.admin && <button type="button" aria-pressed={adminView} onClick={() => setAdminView(v => !v)}>{adminView ? 'Back to business' : 'Review applications'}</button>}</div>{adminView && r.data.me.admin ? <Applications manage={select} reviewed={() => setRefresh(n => n + 1)} /> : id ? <Workspace key={id} id={id} refresh={refresh} profileChanged={profile => r.updateData(account => ({ ...account, businesses: account.businesses.map(b => b.id === profile.id ? { ...b, ...profile } : b) }))} /> : <div className="manage-panel"><h2>No business memberships yet</h2><p>Apply below to register your stringing business.</p></div>}<ApplicationForm saved={newID => { setRevision(n => n + 1); select(newID) }} /></>}</main><MarketplaceFooter /></div>
}
