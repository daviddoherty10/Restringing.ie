import ActivityPanel from '../analytics/ActivityPanel'
import { useCallback, useState, type FormEvent, type ReactNode } from 'react'
import { marketplaceAPI, type Stringer } from '../../lib/marketplace-api'
import { MarketplaceHeader, MarketplaceFooter } from '../../components/layout/MarketplaceLayout'
import { accountAccessChanged, signInURL } from '../../lib/auth-navigation'
import { Feedback, LoadMessage } from './dashboard-shared'
import { errorMessage, useRemote } from './use-remote'
import { euro } from './dashboard-utils'
import TeamPanel, { StaffInvitations } from './TeamPanel'
import OrdersPanel from './OrdersPanel'
import InventoryPanel from './InventoryPanel'
import ServicesPanel from './ServicesPanel'
import GripsPanel from '../grips/GripsPanel'
import ProfilePanel from './ProfilePanel'
import BillingPanel from './BillingPanel'
import PaymentsPanel from './PaymentsPanel'
import StringerReviews from '../reviews/StringerReviews'
import './Management.css'
function Summary({ id, revision }: { id: string; revision: number }) {
  const [retry, setRetry] = useState(0)
  const load = useCallback(() => marketplaceAPI.orderSummary(id), [id])
  const r = useRemote(`${id}:${revision}:${retry}`, load)
  return <section aria-label="Business booking totals"><LoadMessage {...r} retry={() => setRetry(n => n + 1)} />{r.data && <><div className="manage-summary">{[['All bookings', String(r.data.total_orders)], ['Active orders', String(r.data.active_orders)], ['Completed orders', String(r.data.completed_orders)], ['Booking value', euro(r.data.booked_value_cents)]].map(([title, value]) => <div key={title}><span>{title}</span><strong>{value}</strong></div>)}</div><p className="manage-muted">All-time totals for this business. Booking value excludes cancelled orders and is not payment revenue. {r.data.unpaid_orders} non-cancelled orders are unpaid.</p></>}</section>
}
function Workspace({ id, application, profileChanged }: { id: string; application: ReactNode; profileChanged: (profile: Stringer) => void }) {
  const [canAccept, setCanAccept] = useState<boolean | undefined>(undefined)
  const [tab, setTab] = useState(() => {
    const params = new URLSearchParams(window.location.search)
    if (params.has('payments')) return 'payments'
    if (params.has('billing')) return 'billing'
    return 'orders'
  })
  const [revision, setRevision] = useState(0), [summaryRevision, setSummaryRevision] = useState(0), [notice, setNotice] = useState('')
  const load = useCallback(() => marketplaceAPI.managementProfile(id), [id])
  const r = useRemote(`${id}:${revision}`, load)
  const inventory = tab === 'inventory' || tab === 'grips'
  const settings = ['profile', 'payments', 'billing', 'team'].includes(tab)
  const changeTab = (key: string) => { setTab(key); setNotice('') }
  const tabs = (items: string[][], active: string) => items.map(([key, label]) =>
    <button key={key} type="button" aria-pressed={active === key} onClick={() => changeTab(key)}>{label}</button>
  )
  return <>
    <LoadMessage {...r} retry={() => setRevision(n => n + 1)} />
    {notice && <p role="status" className="manage-feedback">{notice}</p>}
    {r.data && <>
      <div className="manage-business-heading">
        <div><h2>{r.data.name}</h2><p>{r.data.area}, {r.data.county} · Your role: <strong>{r.data.membership_role}</strong></p>
          <div className="manage-badges"><span>{r.data.approval_status}</span><span>{r.data.accepting_orders ? 'Bookings enabled when approved' : 'New bookings paused'}</span></div>
        </div>
        {r.data.approval_status === 'approved' && <a href={`/stringers/${id}`}>View public profile ↗</a>}
      </div>
      {r.data.approval_status !== 'approved' && <p className="manage-feedback">This business is {r.data.approval_status}. New bookings are blocked; existing orders and stock can still be managed.</p>}
      <nav className="manage-tabs" aria-label="Business sections">
        {tabs([['orders', 'Orders'], ['inventory', 'Inventory'], ['services', 'Services'], ['profile', 'Settings']], settings ? 'profile' : inventory ? 'inventory' : tab)}
      </nav>
      {inventory && <nav className="manage-tabs workspace-subnav" aria-label="Inventory sections">
        {tabs([['inventory', 'Strings'], ['grips', 'Grips']], tab)}
      </nav>}
      {settings && <nav className="manage-tabs workspace-subnav" aria-label="Settings sections">
        {tabs([['profile', 'Profile'], ['payments', 'Payments'], ['billing', 'Plan & billing'], ...(r.data.permissions.edit_profile ? [['team', 'Team']] : [])], tab)}
      </nav>}
      <div hidden={tab !== 'billing'}><BillingPanel business={r.data} revision={summaryRevision} onUsage={setCanAccept} /></div>
      {tab === 'orders' && <>
        {canAccept === false && <p className="manage-feedback" role="status">Monthly allowance reached. <button type="button" onClick={() => changeTab('billing')}>View plan</button></p>}
        <ActivityPanel business={id} revision={summaryRevision} />
        <OrdersPanel business={r.data} canAccept={canAccept} changed={() => setSummaryRevision(n => n + 1)} />
        <details className="manage-panel"><summary>Booking totals</summary><Summary id={id} revision={summaryRevision} /></details>
      </>}
      {tab === 'inventory' && <InventoryPanel business={r.data} />}
      {tab === 'grips' && <GripsPanel business={r.data} />}
      {tab === 'services' && <ServicesPanel business={r.data} />}
      {tab === 'payments' && <PaymentsPanel business={r.data} />}
      {tab === 'team' && r.data.permissions.edit_profile && <TeamPanel business={r.data} />}
      {tab === 'profile' && <>
        <ProfilePanel business={r.data} changed={updated => { setNotice('Business profile saved.'); setRevision(n => n + 1); profileChanged(updated) }} />
        {r.data.permissions.edit_profile && <nav className="manage-actions" aria-label="Business records">
          <a href={`/manage/trader?business=${encodeURIComponent(id)}`}>Business verification</a>
          <a href={`/manage/financial?business=${encodeURIComponent(id)}`}>Payment records</a>
        </nav>}
        {r.data.approval_status === 'approved' && <StringerReviews business={id} canReport />}
        {application}
      </>}
    </>}
  </>
}
function ApplicationForm({ saved, first = false }: { first?: boolean; saved: (id: string) => void }) {
  const [name, setName] = useState(''), [area, setArea] = useState(''), [county, setCounty] = useState(''), [description, setDescription] = useState(''), [busy, setBusy] = useState(false), [error, setError] = useState(''), [message, setMessage] = useState('')
  async function submit(e: FormEvent) {
    e.preventDefault(); if (busy) return
    setBusy(true); setError(''); setMessage('')
    try { const shop = await marketplaceAPI.applyStringer({ name, area, county, description, accepting_orders: false }); setMessage('Application sent for review.'); setName(''); setArea(''); setCounty(''); setDescription(''); saved(shop.id) }
    catch (e) { setError(errorMessage(e)) } finally { setBusy(false) }
  }
  return <details className="manage-panel" open={first || undefined}><summary>{first ? 'Register your stringing business' : 'Register another business'}</summary><p>Submit your business for review. You can set up services and prices while you wait. If you’re joining an existing business, ask its owner to invite you.</p><form onSubmit={submit}><fieldset disabled={busy}><legend>New business application</legend><label>New business name<input maxLength={120} required value={name} onChange={e => setName(e.target.value)} /></label><div className="manage-form-grid"><label>New business area<input maxLength={120} value={area} onChange={e => setArea(e.target.value)} /></label><label>New business county<input maxLength={120} value={county} onChange={e => setCounty(e.target.value)} /></label></div><label>New business description<textarea maxLength={2000} value={description} onChange={e => setDescription(e.target.value)} /></label><button>{busy ? 'Submitting…' : 'Submit application'}</button></fieldset></form><Feedback error={error} message={message} /></details>
}
export default function Management() {
  const [revision, setRevision] = useState(0)
  const [selected, setSelected] = useState(() => new URLSearchParams(window.location.search).get('business') ?? '')
  const load = useCallback(() => marketplaceAPI.myStringers(), [])
  const r = useRemote(`account:${revision}`, load)
  const id = selected || r.data?.[0]?.id || ''
  function select(value: string) {
    window.history.replaceState(null, '', value ? `/manage?business=${encodeURIComponent(value)}` : '/manage')
    setSelected(value)
  }
  function refreshBusinesses() {
    setRevision(n => n + 1)
    window.dispatchEvent(new Event(accountAccessChanged))
  }
  function applied(newID: string) { refreshBusinesses(); select(newID) }
  return <div className="marketplace">
    <MarketplaceHeader backHref="/" backLabel="Home" />
    <main id="main-content" tabIndex={-1} className="marketplace-container manage-content">
      <h1>Stringer dashboard</h1>
      <LoadMessage {...r} retry={() => setRevision(n => n + 1)} />
      {r.error && <a href={signInURL('/manage')}>Sign in to access your businesses</a>}
      {r.data && <>
        <StaffInvitations accepted={refreshBusinesses} />
        {id && (r.data.length > 1 || !r.data.some(business => business.id === id)) && <div className="manage-portal-bar">
          <label>Your business<select value={id} onChange={e => select(e.target.value)}>
            <option value="">Choose business</option>
            {r.data.map(business => <option key={business.id} value={business.id}>{business.name} · {business.membership_role}</option>)}
            {!r.data.some(business => business.id === id) && <option value={id}>Selected business</option>}
          </select></label>
        </div>}
        {id ? <Workspace key={id} id={id}
          application={<ApplicationForm saved={applied} />}
          profileChanged={profile => r.updateData(businesses => businesses.map(business => business.id === profile.id ? { ...business, ...profile } : business))} />
          : <ApplicationForm first saved={applied} />}
      </>}
    </main>
    <MarketplaceFooter />
  </div>
}
