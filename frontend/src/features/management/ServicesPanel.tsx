import { useCallback, useState, type FormEvent } from 'react'
import { marketplaceAPI, type ManagementProfile, type Service } from '../../lib/marketplace-api'
import type { components } from '../../lib/marketplace-contract'
import { Feedback, LoadMessage } from './dashboard-shared'
import { errorMessage, useRemote } from './use-remote'
import { decimalUnits, euro } from './dashboard-utils'
type Input = components['schemas']['ServiceInput']
const priceFields = [['labour_cents', 'Labour price'], ['own_strings_cents', 'Customer-supplied strings surcharge'], ['stencil_cents', 'Stencilling surcharge'], ['two_knots_cents', 'Two-knot surcharge'], ['four_knots_cents', 'Four-knot surcharge'], ['different_tensions_cents', 'Separate tensions surcharge']] as const
const capabilities = [['own_strings', 'Accept customer-supplied strings'], ['stencil', 'Stencilling'], ['two_knots', 'Two-knot stringing'], ['four_knots', 'Four-knot stringing'], ['different_tensions', 'Different mains and crosses tensions']] as const
function ServiceEditor({ business, sport, existing, saved }: { business: ManagementProfile; sport: string; existing?: Service; saved: () => void }) {
  const [active, setActive] = useState(existing?.active ?? false)
  const [offered, setOffered] = useState<Record<string, boolean>>(() => Object.fromEntries(capabilities.map(([key]) => [key, existing?.[key] ?? key === 'four_knots'])))
  const [prices, setPrices] = useState<Record<string, string>>(() => Object.fromEntries(priceFields.map(([key]) => [key, String((existing?.[key] ?? 0) / 100)])))
  const [min, setMin] = useState(existing ? String(existing.min_tension_centi_kg / 100) : '')
  const [max, setMax] = useState(existing ? String(existing.max_tension_centi_kg / 100) : '')
  const [step, setStep] = useState(existing ? String(existing.step_centi_kg / 100) : '')
  const [difference, setDifference] = useState(existing ? String(existing.max_difference_centi_kg / 100) : '')
  const [prestretch, setPrestretch] = useState(() => Object.entries(existing?.prestretch_options ?? {}).sort(([a], [b]) => Number(a) - Number(b)).map(([percent, cents]) => ({ percent, price: String(cents / 100) })))
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [message, setMessage] = useState('')
  async function submit(e: FormEvent) {
    e.preventDefault(); if (busy) return
    const money = Object.fromEntries(priceFields.map(([key]) => [key, decimalUnits(prices[key], 2)]))
    const tensions = { min_tension_centi_kg: decimalUnits(min, 2), max_tension_centi_kg: decimalUnits(max, 2), step_centi_kg: decimalUnits(step, 2), max_difference_centi_kg: decimalUnits(difference, 2) }
    const percentages: Record<string, number> = {}
    let valid = Object.values(money).every(n => n !== null && n <= 1000000) && Object.values(tensions).every(n => n !== null)
    for (const row of prestretch) {
      const price = decimalUnits(row.price, 2), percent = Number(row.percent)
      if (!/^[0-9]+$/.test(row.percent) || percent < 1 || percent > 30 || percentages[percent] !== undefined || price === null || price > 1000000) { valid = false; break }
      percentages[percent] = price
    }
    if (!valid) { setError('Use valid prices with at most two decimal places, technical values in kg and unique pre-stretch percentages from 1 to 30.'); return }
    if (!offered.two_knots && !offered.four_knots) { setError('Offer at least one knot pattern.'); return }
    setBusy(true); setError(''); setMessage('')
    try { await marketplaceAPI.saveService(business.id, { sport, active, ...offered, ...money, ...tensions, prestretch_options: percentages } as Input); setMessage('Service settings saved. Future quotes use these prices and capabilities.'); saved() }
    catch (e) { setError(errorMessage(e)) } finally { setBusy(false) }
  }
  return <form onSubmit={submit}><fieldset disabled={busy || !business.permissions.edit_services}><legend>{sport} service settings</legend><label className="manage-check"><input type="checkbox" checked={active} onChange={e => setActive(e.target.checked)} />Enable {sport} bookings</label><div className="manage-form-grid">{capabilities.map(([key, title]) => <label className="manage-check" key={key}><input type="checkbox" checked={offered[key]} onChange={e => setOffered(v => ({ ...v, [key]: e.target.checked }))} />{title}</label>)}</div><h3>Prices</h3><div className="manage-form-grid">{priceFields.map(([key, title]) => <label key={key}>{title} (€)<input inputMode="decimal" required value={prices[key]} onChange={e => setPrices(v => ({ ...v, [key]: e.target.value }))} /></label>)}</div><h3>Technical service limits</h3><p>Enter limits supported by your equipment and process. Each booking also checks its verified racket and string constraints.</p><div className="manage-form-grid"><label>Minimum tension (kg)<input required inputMode="decimal" value={min} onChange={e => setMin(e.target.value)} /></label><label>Maximum tension (kg)<input required inputMode="decimal" value={max} onChange={e => setMax(e.target.value)} /></label><label>Tension increment (kg)<input required inputMode="decimal" value={step} onChange={e => setStep(e.target.value)} /></label><label>Maximum mains/crosses difference (kg)<input required inputMode="decimal" value={difference} onChange={e => setDifference(e.target.value)} /></label></div><h3>Pre-stretch options</h3><p>Add only percentages you offer. Product limits still apply to each booking.</p>{prestretch.map((row, index) => <div className="manage-prestretch" key={index}><label>Percentage {index + 1}<input inputMode="numeric" required value={row.percent} onChange={e => setPrestretch(v => v.map((r, i) => i === index ? { ...r, percent: e.target.value } : r))} /></label><label>Pre-stretch price {index + 1} (€)<input inputMode="decimal" required value={row.price} onChange={e => setPrestretch(v => v.map((r, i) => i === index ? { ...r, price: e.target.value } : r))} /></label><button type="button" className="manage-secondary" onClick={() => setPrestretch(v => v.filter((_, i) => i !== index))}>Remove percentage {index + 1}</button></div>)}<div className="manage-actions"><button type="button" className="manage-secondary" disabled={prestretch.length >= 30} onClick={() => setPrestretch(v => [...v, { percent: '', price: '' }])}>Add pre-stretch option</button><button>{busy ? 'Saving…' : 'Save service settings'}</button></div></fieldset>{!business.permissions.edit_services && <p>Owners and administrators can edit service settings. Staff can view them.</p>}<Feedback error={error} message={message} /></form>
}
export default function ServicesPanel({ business }: { business: ManagementProfile }) {
  const [sport, setSport] = useState('tennis'), [revision, setRevision] = useState(0), [message, setMessage] = useState('')
  const load = useCallback(() => marketplaceAPI.manageServices(business.id), [business.id])
  const r = useRemote(`${business.id}:${revision}`, load)
  return <section className="manage-panel"><h2>Services and prices</h2><label className="manage-filter">Service sport<select value={sport} onChange={e => setSport(e.target.value)}>{['tennis', 'badminton', 'squash'].map(s => <option key={s}>{s}</option>)}</select></label><Feedback error="" message={message} /><LoadMessage {...r} retry={() => setRevision(n => n + 1)} />{r.data && <>{r.data.length > 0 && <p className="manage-muted">Configured: {r.data.map(s => `${s.sport} (${s.active ? 'enabled' : 'inactive'}, labour ${euro(s.labour_cents)})`).join(' · ')}</p>}<ServiceEditor key={sport} business={business} sport={sport} existing={r.data.find(s => s.sport === sport)} saved={() => { setMessage('Service settings saved.'); setRevision(n => n + 1) }} /></>}</section>
}
