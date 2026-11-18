import { useCallback, useState, type FormEvent } from 'react'
import { marketplaceAPI, type Stringer } from '../../lib/marketplace-api'
import { MarketplaceHeader, MarketplaceFooter } from '../../components/layout/MarketplaceLayout'
import { Feedback, LoadMessage, Pagination } from '../management/dashboard-shared'
import { errorMessage, useRemote } from '../management/use-remote'
function ReviewApplication({ business, reviewed }: { business: Stringer; reviewed: () => void }) {
  const [decision, setDecision] = useState<'approved' | 'rejected'>('approved'), [reason, setReason] = useState(''), [busy, setBusy] = useState(false), [error, setError] = useState('')
  async function submit(e: FormEvent) {
    e.preventDefault(); if (busy || !reason.trim()) return
    setBusy(true); setError('')
    try { await marketplaceAPI.admin.reviewStringer(business.id, decision, reason.trim()); reviewed() }
    catch (e) { setError(errorMessage(e)); setBusy(false) }
  }
  return <article><h3>{business.name}</h3><p>{business.area}, {business.county}</p><p>{business.description}</p><a href={`/manage?business=${encodeURIComponent(business.id)}`}>Inspect business</a><form onSubmit={submit}><fieldset disabled={busy}><legend>Review {business.name}</legend><label>Decision<select value={decision} onChange={e => setDecision(e.target.value as typeof decision)}><option value="approved">Approve</option><option value="rejected">Reject</option></select></label><label>Reason for decision<textarea required maxLength={1000} value={reason} onChange={e => setReason(e.target.value)} /></label><button>{busy ? 'Saving…' : 'Submit decision'}</button></fieldset></form><Feedback error={error} message="" /></article>
}
function Applications() {
  const [page, setPage] = useState(1), [revision, setRevision] = useState(0), [message, setMessage] = useState('')
  const load = useCallback(() => marketplaceAPI.admin.applications({ page, page_size: 10 }), [page])
  const r = useRemote(`${page}:${revision}`, load)
  return <section className="manage-panel"><h2>Pending applications</h2><LoadMessage {...r} retry={() => setRevision(n => n + 1)} />{message && <p role="status">{message}</p>}{r.data && <>{r.data.data.length === 0 ? <p>No pending applications.</p> : r.data.data.map(s => <ReviewApplication key={s.id} business={s} reviewed={() => { setRevision(n => n + 1); setMessage('Application decision recorded.') }} />)}<Pagination page={page} size={10} total={r.data.total} change={setPage} /></>}</section>
}
export default function AdminApplications() {
  return <div className="marketplace">
    <MarketplaceHeader />
    <main id="main-content" tabIndex={-1} className="marketplace-container manage-content">
      <h1>Business applications</h1>
      <Applications />
    </main>
    <MarketplaceFooter />
  </div>
}
