import { useCallback, useState, type FormEvent } from 'react'
import { marketplaceAPI, MarketplaceError, type ManagementProfile, type StockLot, type Variant } from '../../lib/marketplace-api'
import { Feedback, LoadMessage, Pagination } from './dashboard-shared'
import { errorMessage, useRemote } from './use-remote'
import { decimalUnits, euro, signedUnits } from './dashboard-utils'
const label = (v: { brand: string; model: string; gauge_mm: number; colour: string }) => `${v.brand} ${v.model} · ${v.gauge_mm} mm · ${v.colour}`
function OfferingEditor({ businessID, variant, saved, close }: { businessID: string; variant: Variant; saved: () => void; close: () => void }) {
  const [price, setPrice] = useState(variant.inventory_id ? String(variant.material_price_cents / 100) : '')
  const [available, setAvailable] = useState(variant.inventory_id ? variant.available : true)
  const [offering, setOffering] = useState(variant.inventory_id ?? '')
  const [kind, setKind] = useState<'set' | 'reel'>('set')
  const [quantity, setQuantity] = useState('')
  const [length, setLength] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const [uncertain, setUncertain] = useState(false)
  async function savePrice(e: FormEvent) {
    e.preventDefault(); if(busy)return; const cents = decimalUnits(price, 2)
    if (cents === null || cents > 1000000) { setError('Enter a price from €0 to €10,000, with at most two decimal places.'); return }
    setBusy(true); setError(''); setMessage('')
    try { const r = await marketplaceAPI.saveInventory(businessID, variant.id, { material_price_cents: cents, available }); setOffering(r.id); setMessage('Price and availability saved. Receive physical stock below if needed.'); saved() }
    catch (e) { setError(errorMessage(e)) } finally { setBusy(false) }
  }
  async function receive(e: FormEvent) {
    e.preventDefault(); if(busy||uncertain)return
    const units = kind === 'set' ? (/^[0-9]+$/.test(quantity) ? Number(quantity) : null) : decimalUnits(quantity, 3)
    const mm = kind === 'set' ? decimalUnits(length, 3) : 0
    if (units === null || !Number.isSafeInteger(units) || units < 1 || units > 100000000 || mm === null || (kind === 'set' && (mm < 1000 || mm > 30000))) { setError('Enter whole sets with a length of 1–30 metres, or a positive reel length in metres (up to three decimal places).'); return }
    setBusy(true); setError(''); setMessage('')
    try { await marketplaceAPI.receiveStock(businessID, variant.id, { kind, quantity: units, set_length_mm: mm }); setQuantity(''); setLength(''); setMessage('Stock received and recorded.'); saved() }
    catch (e) { const uncertain = !(e instanceof MarketplaceError) || e.status >= 500; setUncertain(uncertain); setError(errorMessage(e) + (uncertain ? ' Check Stock lots before repeating a receipt; it may already have been recorded.' : '')) } finally { setBusy(false) }
  }
  return <section className="manage-editor" aria-label="String settings"><div className="manage-section-heading"><h3>{label(variant)}</h3><button type="button" className="manage-secondary" disabled={busy} onClick={close}>Close string settings</button></div><form onSubmit={savePrice}><fieldset disabled={busy}><legend>Offering and material price</legend><label>Material price (€)<input inputMode="decimal" required value={price} onChange={e => setPrice(e.target.value)} /></label><label className="manage-check"><input type="checkbox" checked={available} onChange={e => setAvailable(e.target.checked)} />Available for booking</label><p>Labour and optional services are configured separately. A string also needs sufficient physical stock to be bookable.</p><button>{busy ? 'Saving…' : 'Save string offering'}</button></fieldset></form>{offering && <form onSubmit={receive}><fieldset disabled={busy || uncertain}><legend>Receive physical stock</legend><label>Stock type<select value={kind} onChange={e => { setKind(e.target.value as typeof kind); setQuantity(''); setLength('') }}><option value="set">Individual sets</option><option value="reel">Reel</option></select></label><label>{kind === 'set' ? 'Number of sets' : 'Reel length (metres)'}<input inputMode={kind === 'set' ? 'numeric' : 'decimal'} required value={quantity} onChange={e => setQuantity(e.target.value)} /></label>{kind === 'set' && <label>Length per set (metres)<input inputMode="decimal" required value={length} onChange={e => setLength(e.target.value)} /></label>}<p>Use the measured or labelled package length. Receiving stock creates a new lot and an audit entry.</p><button>{busy ? 'Saving…' : 'Receive stock'}</button></fieldset></form>}<Feedback error={error} message={message} /></section>
}
function Adjustment({ businessID, lot, saved, close }: { businessID: string; lot: StockLot; saved: () => void; close: () => void }) {
  const [delta, setDelta] = useState(''), [reason, setReason] = useState(''), [error, setError] = useState(''), [busy, setBusy] = useState(false), [uncertain, setUncertain] = useState(false)
  async function adjust(e: FormEvent) {
    e.preventDefault(); if(busy||uncertain)return; const n = lot.kind === 'set' ? (/^-?[0-9]+$/.test(delta) ? Number(delta) : null) : signedUnits(delta, 3)
    if (n === null || !Number.isSafeInteger(n) || n === 0 || Math.abs(n) > 100000000 || !reason.trim()) { setError('Enter a nonzero stock correction and a reason.'); return }
    setBusy(true); setError('')
    try { await marketplaceAPI.adjustStock(businessID, lot.id, { quantity_delta: n, reason: reason.trim() }); saved(); close() }
    catch (e) { const uncertain = !(e instanceof MarketplaceError) || e.status >= 500; setUncertain(uncertain); setError(errorMessage(e) + (uncertain ? ' Check the current stock balance before repeating this correction.' : '')) } finally { setBusy(false) }
  }
  return <form className="manage-editor" onSubmit={adjust}><fieldset disabled={busy || uncertain}><legend>Correct stock balance</legend><p>{label(lot)} · lot {lot.id.slice(0, 8)}. Reserved stock cannot be removed.</p><label>{lot.kind === 'set' ? 'Correction (sets, use a minus sign to remove)' : 'Correction (metres, use a minus sign to remove)'}<input required inputMode="decimal" value={delta} onChange={e => setDelta(e.target.value)} /></label><label>Reason for correction<textarea required maxLength={200} value={reason} onChange={e => setReason(e.target.value)} /></label><div className="manage-actions"><button>{busy ? 'Saving…' : 'Record stock correction'}</button><button type="button" className="manage-secondary" onClick={close}>Cancel correction</button></div></fieldset>{uncertain && <button type="button" onClick={() => { saved(); close() }}>Refresh stock balances</button>}<Feedback error={error} message="" /></form>
}
function LotsPanel({ business }: { business: ManagementProfile }) {
  const [query, setQuery] = useState(''), [sport, setSport] = useState(''), [page, setPage] = useState(1), [revision, setRevision] = useState(0)
  const [editing, setEditing] = useState<StockLot | null>(null)
  const load = useCallback(() => marketplaceAPI.lots(business.id, { q: query, sport: sport as Variant['sport'] || undefined, page, page_size: 10 }), [business.id, query, sport, page])
  const r = useRemote(`${business.id}:${query}:${sport}:${page}:${revision}`, load)
  const units = (lot: StockLot, n: number) => lot.kind === 'set' ? `${n} sets` : `${n / 1000} m`
  return <><div className="manage-filters"><label>Search stock lots<input type="search" value={query} onChange={e => { setQuery(e.target.value); setPage(1); setEditing(null) }} /></label><label>Stock sport<select value={sport} onChange={e => { setSport(e.target.value); setPage(1); setEditing(null) }}><option value="">All sports</option>{['tennis', 'badminton', 'squash'].map(s => <option key={s}>{s}</option>)}</select></label></div><p>Physical, reserved and free balances are shown separately. Reel balances use metres; set balances use individual sets.</p><LoadMessage {...r} retry={() => setRevision(n => n + 1)} />{r.data && <>{r.data.data.length === 0 ? <p>No stock lots match this filter.</p> : <div className="manage-table-wrap"><table><caption>Stock lots</caption><thead><tr><th>String / lot</th><th>Physical</th><th>Reserved</th><th>Free</th><th>Actions</th></tr></thead><tbody>{r.data.data.map(lot => <tr key={lot.id}><th scope="row">{label(lot)}<small>{lot.kind} · {lot.id.slice(0, 8)}{lot.kind === 'set' ? ` · ${lot.set_length_mm / 1000} m per set` : ''}</small></th><td>{units(lot, lot.quantity)}</td><td>{units(lot, lot.reserved)}</td><td>{units(lot, lot.quantity - lot.reserved)}</td><td><button type="button" disabled={!!editing} onClick={() => setEditing(lot)}>Correct stock</button></td></tr>)}</tbody></table></div>}<Pagination page={page} size={10} total={r.data.total} change={p => { setPage(p); setEditing(null) }} busy={!!editing} /></>}{editing && <Adjustment key={editing.id} businessID={business.id} lot={editing} saved={() => setRevision(n => n + 1)} close={() => setEditing(null)} />}</>
}
function OfferingsPanel({ business, catalogue }: { business: ManagementProfile; catalogue: boolean }) {
  const [query, setQuery] = useState(''), [sport, setSport] = useState(''), [material, setMaterial] = useState(''), [page, setPage] = useState(1), [revision, setRevision] = useState(0)
  const [editing, setEditing] = useState<Variant | null>(null)
  const [selecting, setSelecting] = useState(false), [selectionError, setSelectionError] = useState('')
  const load = useCallback(() => {
    const filters = { q: query, sport: sport as Variant['sport'] || undefined, material: material as Variant['material'] || undefined, page, page_size: 10 }
    return catalogue ? marketplaceAPI.strings(filters) : marketplaceAPI.manageInventory(business.id, filters)
  }, [business.id, catalogue, query, sport, material, page])
  const r = useRemote(`${business.id}:${catalogue}:${query}:${sport}:${material}:${page}:${revision}`, load)
  async function choose(v: Variant) {
    // Catalogue results are global: fetch the existing local offering before
    // editing so adding an already-stocked variant never resets its price.
    if (!catalogue) { setEditing(v); return }
    setSelecting(true); setSelectionError('')
    try { const owned = await marketplaceAPI.manageInventory(business.id, { variant_id: v.id, page_size: 1 }); setEditing(owned.data[0] ?? v) }
    catch (e) { setSelectionError(errorMessage(e)) } finally { setSelecting(false) }
  }
  return <><div className="manage-filters"><label>{catalogue ? 'Search catalogue' : 'Search stocked strings'}<input type="search" value={query} onChange={e => { setQuery(e.target.value); setPage(1) }} /></label><label>String sport<select value={sport} onChange={e => { setSport(e.target.value); setPage(1) }}><option value="">All sports</option>{['tennis', 'badminton', 'squash'].map(s => <option key={s}>{s}</option>)}</select></label><label>String material<select value={material} onChange={e => { setMaterial(e.target.value); setPage(1) }}><option value="">All materials</option>{['multifilament', 'synthetic_gut', 'polyester', 'natural_gut', 'nylon', 'hybrid'].map(s => <option key={s} value={s}>{s.replaceAll('_', ' ')}</option>)}</select></label></div><Feedback error={selectionError} message="" /><LoadMessage {...r} retry={() => setRevision(n => n + 1)} />{r.data && <>{r.data.data.length === 0 ? <p>No strings match this filter.</p> : <div className="manage-string-list">{r.data.data.map(v => <article key={v.id}><div><h3>{label(v)}</h3><p>{v.sport} · {v.material.replaceAll('_', ' ')}{!catalogue && <> · {euro(v.material_price_cents)} · {v.available ? 'Enabled' : 'Unavailable'}</>}</p></div><button type="button" disabled={!!editing || selecting} onClick={() => choose(v)}>{catalogue ? 'Select string' : 'Edit price / receive stock'}</button></article>)}</div>}<Pagination page={page} size={10} total={r.data.total} change={setPage} /></>}{editing && <OfferingEditor key={editing.id} businessID={business.id} variant={editing} close={() => setEditing(null)} saved={() => setRevision(n => n + 1)} />}</>
}
export default function InventoryPanel({ business }: { business: ManagementProfile }) {
  const [mode, setMode] = useState('offerings')
  if (!business.permissions.manage_inventory) return <p>You do not have permission to manage inventory.</p>
  return <section className="manage-panel"><h2>Inventory and pricing</h2><nav className="manage-tabs manage-subtabs" aria-label="Inventory sections">{[['offerings', 'Stocked strings'], ['catalogue', 'Add from catalogue'], ['lots', 'Stock lots']].map(([key, title]) => <button type="button" key={key} aria-pressed={mode === key} onClick={() => setMode(key)}>{title}</button>)}</nav>{mode === 'lots' ? <LotsPanel business={business} /> : <OfferingsPanel key={mode} business={business} catalogue={mode === 'catalogue'} />}</section>
}
