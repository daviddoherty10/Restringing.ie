import { useState, type FormEvent } from 'react'
import { marketplaceAPI, type ManagementProfile, type Stringer } from '../../lib/marketplace-api'
import { Feedback } from './dashboard-shared'
import { errorMessage } from './use-remote'
export default function ProfilePanel({ business, changed }: { business: ManagementProfile; changed: (profile: Stringer) => void }) {
  const [name, setName] = useState(business.name), [area, setArea] = useState(business.area), [county, setCounty] = useState(business.county), [description, setDescription] = useState(business.description), [accepting, setAccepting] = useState(business.accepting_orders)
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [message, setMessage] = useState('')
  async function submit(e: FormEvent) {
    e.preventDefault(); if (busy) return
    setBusy(true); setError(''); setMessage('')
    try { const updated = await marketplaceAPI.updateProfile(business.id, { name, area, county, description, accepting_orders: accepting }); setMessage('Business profile saved.'); changed(updated) }
    catch (e) { setError(errorMessage(e)) } finally { setBusy(false) }
  }
  return <section className="manage-panel"><h2>Business profile</h2><p>Approval: <strong>{business.approval_status}</strong>. Only approved businesses can appear publicly and receive new bookings.</p><form onSubmit={submit}><fieldset disabled={busy || !business.permissions.edit_profile}><legend>Profile details</legend><div className="manage-form-grid"><label>Business name<input required maxLength={120} value={name} onChange={e => setName(e.target.value)} /></label><label>Area<input maxLength={120} value={area} onChange={e => setArea(e.target.value)} /></label><label>County<input maxLength={120} value={county} onChange={e => setCounty(e.target.value)} /></label></div><label>Description<textarea maxLength={2000} rows={5} value={description} onChange={e => setDescription(e.target.value)} /></label><label className="manage-check"><input type="checkbox" checked={accepting} onChange={e => setAccepting(e.target.checked)} />Accept new bookings when approved</label><button>{busy ? 'Saving…' : 'Save business profile'}</button></fieldset></form>{!business.permissions.edit_profile && <p>Owners and administrators can edit this profile. Staff can view it.</p>}<Feedback error={error} message={message} /></section>
}
