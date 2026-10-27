import { useCallback, useState, type FormEvent } from 'react'
import { marketplaceAPI, type Product, type CatalogueVariant, type Racket } from '../../lib/marketplace-api'
import type { components } from '../../lib/marketplace-contract'
import { Feedback, LoadMessage } from '../management/dashboard-shared'
import { useRemote } from '../management/use-remote'
import { decimalUnits } from '../management/dashboard-utils'
import { sports, materials, ratings, label, useCatalogueSave } from './catalogue-utils'
type S = components['schemas']
type Actions = { close: () => void; saved: () => void; onBusy: (value: boolean) => void }
function ProductForm({ initial, ...actions }: Actions & { initial: Product | null }) {
  const [brand, setBrand] = useState(initial?.brand ?? ''), [model, setModel] = useState(initial?.model ?? ''), [sport, setSport] = useState<S['ProductInput']['sport']>(initial?.sport ?? 'tennis'), [material, setMaterial] = useState<S['ProductInput']['material']>(initial?.material ?? 'multifilament')
  const [description, setDescription] = useState(initial?.description ?? ''), [active, setActive] = useState(initial?.active ?? false), [prestretch, setPrestretch] = useState(String(initial?.max_prestretch_percent ?? 0))
  const [values, setValues] = useState<Record<typeof ratings[number], string>>(() => Object.fromEntries(ratings.map(k => [k, initial ? String(initial[k]) : ''])) as Record<typeof ratings[number], string>)
  const save = useCatalogueSave(!initial, actions.onBusy, actions.saved)
  function submit(e: FormEvent) {
    e.preventDefault()
    const product: S['ProductInput'] = { brand: brand.trim(), model: model.trim(), sport, material, description, active, max_prestretch_percent: Number(prestretch), comfort: Number(values.comfort), control: Number(values.control), power: Number(values.power), spin: Number(values.spin), durability: Number(values.durability) }
    void save.save(() => initial ? marketplaceAPI.admin.updateProduct(initial.id, product) : marketplaceAPI.admin.createProduct(product))
  }
  return <section className="manage-panel"><h2>{initial ? 'Edit string product' : 'Create string product'}</h2><p>Shared brand/model and editorial characteristics. Each stringer sets their own prices and stock. Existing order snapshots are retained when this record changes.</p><form onSubmit={submit}><fieldset disabled={save.busy || save.uncertain}><legend>String product details</legend><div className="manage-form-grid"><label>Brand<input required maxLength={100} value={brand} onChange={e => setBrand(e.target.value)} /></label><label>Model<input required maxLength={120} value={model} onChange={e => setModel(e.target.value)} /></label><label>Sport<select value={sport} onChange={e => setSport(e.target.value as typeof sport)}>{sports.map(s => <option key={s}>{s}</option>)}</select></label><label>Material<select value={material} onChange={e => setMaterial(e.target.value as typeof material)}>{materials.map(s => <option value={s} key={s}>{label(s)}</option>)}</select></label></div><label>Description<textarea maxLength={2000} rows={3} value={description} onChange={e => setDescription(e.target.value)} /></label><p>Editorial ratings from 1 to 5 describe catalogue preferences, not medical outcomes. Enter each rating deliberately.</p><div className="catalogue-ratings">{ratings.map(k => <label key={k}>{k.charAt(0).toUpperCase() + k.slice(1)}<input type="number" required min={1} max={5} step={1} value={values[k]} onChange={e => setValues(v => ({ ...v, [k]: e.target.value }))} /></label>)}</div><label>Maximum pre-stretch (%)<input type="number" required min={0} max={30} step={1} value={prestretch} onChange={e => setPrestretch(e.target.value)} /></label><p>Use sourced product information. Zero disables pre-stretch for this product.</p><label className="manage-check"><input type="checkbox" checked={active} onChange={e => setActive(e.target.checked)} />Product active in the public catalogue</label><button>{save.busy ? 'Saving…' : 'Save product'}</button></fieldset></form><Feedback error={save.error} message="" /><button className="manage-secondary" type="button" disabled={save.busy} onClick={actions.close}>Back to results</button></section>
}
export function ProductEditor({ id, ...actions }: Actions & { id: string }) {
  const [retry, setRetry] = useState(0)
  const load = useCallback(() => id === 'new' ? Promise.resolve(null) : marketplaceAPI.admin.product(id), [id])
  const r = useRemote(`${id}:${retry}`, load)
  return <><LoadMessage {...r} retry={() => setRetry(n => n + 1)} />{r.data !== undefined ? <ProductForm key={id} initial={r.data} {...actions} /> : <button type="button" onClick={actions.close}>Back to results</button>}</>
}
function VariantForm({ initial, ...actions }: Actions & { initial: CatalogueVariant }) {
  const [gauge, setGauge] = useState(initial.id ? String(initial.gauge_mm) : ''), [colour, setColour] = useState(initial.colour), [active, setActive] = useState(initial.active), [error, setError] = useState('')
  const save = useCatalogueSave(!initial.id, actions.onBusy, actions.saved)
  function submit(e: FormEvent) {
    e.preventDefault(); const units = decimalUnits(gauge, 2)
    if (units === null || units < 40 || units > 200) { setError('Gauge must be 0.40–2.00 mm with at most two decimal places.'); return }
    setError(''); const value: S['VariantInput'] = { product_id: initial.product_id, gauge_mm: units / 100, colour: colour.trim(), active }
    void save.save(() => initial.id ? marketplaceAPI.admin.updateVariant(initial.id, value) : marketplaceAPI.admin.createVariant(value))
  }
  return <section className="manage-panel"><h2>{initial.id ? 'Edit string variant' : 'Create string variant'}</h2><p><strong>{initial.brand} {initial.model}</strong> · {initial.sport} · {label(initial.material)}</p><p>The product relationship is fixed in this editor. Changing a gauge or colour affects every stringer's reference to this variant; saved orders keep their original setup.</p>{!initial.product_active && <p className="manage-feedback">The parent product is inactive. This variant remains hidden publicly until both product and variant are active.</p>}<form onSubmit={submit}><fieldset disabled={save.busy || save.uncertain}><legend>Variant details</legend><div className="manage-form-grid"><label>Gauge (mm)<input type="number" required min="0.40" max="2.00" step="0.01" value={gauge} onChange={e => setGauge(e.target.value)} /></label><label>Colour<input maxLength={60} value={colour} onChange={e => setColour(e.target.value)} /></label></div><label className="manage-check"><input type="checkbox" checked={active} onChange={e => setActive(e.target.checked)} />Variant active</label><button>{save.busy ? 'Saving…' : 'Save variant'}</button></fieldset></form><Feedback error={save.error || error} message="" /><button className="manage-secondary" type="button" disabled={save.busy} onClick={actions.close}>Back to results</button></section>
}
export function VariantEditor({ id, productID, ...actions }: Actions & { id: string; productID?: string }) {
  const [retry, setRetry] = useState(0)
  const load = useCallback(async () => {
    if (id !== 'new') return marketplaceAPI.admin.variant(id)
    const p = await marketplaceAPI.admin.product(productID!)
    return { id: '', product_id: p.id, brand: p.brand, model: p.model, sport: p.sport, material: p.material, product_active: p.active, gauge_mm: 0, colour: '', active: false } satisfies CatalogueVariant
  }, [id, productID])
  const r = useRemote(`${id}:${productID}:${retry}`, load)
  return <><LoadMessage {...r} retry={() => setRetry(n => n + 1)} />{r.data ? <VariantForm key={id} initial={r.data} {...actions} /> : <button type="button" onClick={actions.close}>Back to results</button>}</>
}
function RacketForm({ initial, ...actions }: Actions & { initial: Racket | null }) {
  const [brand, setBrand] = useState(initial?.brand ?? ''), [model, setModel] = useState(initial?.model ?? ''), [sport, setSport] = useState<S['RacketInput']['sport']>(initial?.sport ?? 'tennis')
  const [min, setMin] = useState(initial ? String(initial.min_tension_centi_kg / 100) : ''), [max, setMax] = useState(initial ? String(initial.max_tension_centi_kg / 100) : ''), [step, setStep] = useState(initial ? String(initial.step_centi_kg / 100) : ''), [difference, setDifference] = useState(initial ? String(initial.max_difference_centi_kg / 100) : ''), [length, setLength] = useState(initial ? String(initial.string_length_mm / 1000) : '')
  const [knots, setKnots] = useState(initial?.allowed_knots ?? []), [four, setFour] = useState(initial ? String(initial.different_tensions_require_four_knots) : ''), [source, setSource] = useState(initial?.technical_source ?? ''), [verified, setVerified] = useState(initial?.verified ?? false), [error, setError] = useState('')
  const save = useCatalogueSave(!initial, actions.onBusy, actions.saved)
  function submit(e: FormEvent) {
    e.preventDefault()
    const low = decimalUnits(min, 2), high = decimalUnits(max, 2), increment = decimalUnits(step, 2), diff = decimalUnits(difference, 2), mm = decimalUnits(length, 3)
    if (low === null || high === null || increment === null || diff === null || mm === null || low < 1 || high < low || high > 10000 || increment < 1 || increment > 100 || diff > 10000 || mm < 1000 || mm > 30000 || !knots.length || four === '' || !source.trim()) { setError('Check all sourced technical limits, select allowed knots and the separate-tension rule, and supply a technical source.'); return }
    setError(''); const value: S['RacketInput'] = { brand: brand.trim(), model: model.trim(), sport, min_tension_centi_kg: low, max_tension_centi_kg: high, step_centi_kg: increment, max_difference_centi_kg: diff, allowed_knots: knots, different_tensions_require_four_knots: four === 'true', string_length_mm: mm, technical_source: source.trim(), verified }
    void save.save(() => initial ? marketplaceAPI.admin.updateRacket(initial.id, value) : marketplaceAPI.admin.createRacket(value))
  }
  return <section className="manage-panel"><h2>{initial ? 'Edit racket model' : 'Create racket model'}</h2><p>Use model-specific manufacturer documentation or another reviewed technical source. Values below are data-entry units, not recommended stringing settings. Only verified models appear in customer booking.</p><form onSubmit={submit}><fieldset disabled={save.busy || save.uncertain}><legend>Model and technical constraints</legend><div className="manage-form-grid"><label>Brand<input required maxLength={100} value={brand} onChange={e => setBrand(e.target.value)} /></label><label>Model<input required maxLength={120} value={model} onChange={e => setModel(e.target.value)} /></label><label>Sport<select value={sport} onChange={e => setSport(e.target.value as typeof sport)}>{sports.map(s => <option key={s}>{s}</option>)}</select></label><label>Minimum tension (kg)<input type="number" required min="0.01" max="100" step="0.01" value={min} onChange={e => setMin(e.target.value)} /></label><label>Maximum tension (kg)<input type="number" required min="0.01" max="100" step="0.01" value={max} onChange={e => setMax(e.target.value)} /></label><label>Tension increment (kg)<input type="number" required min="0.01" max="1" step="0.01" value={step} onChange={e => setStep(e.target.value)} /></label><label>Maximum mains/crosses difference (kg)<input type="number" required min="0" max="100" step="0.01" value={difference} onChange={e => setDifference(e.target.value)} /></label><label>Required string length (m)<input type="number" required min="1" max="30" step="0.001" value={length} onChange={e => setLength(e.target.value)} /></label></div><fieldset><legend>Allowed knot patterns</legend>{[2, 4].map(k => <label key={k} className="manage-check"><input type="checkbox" checked={knots.includes(k)} onChange={e => setKnots(n => e.target.checked ? [...n, k].sort() : n.filter(v => v !== k))} />{k} knots</label>)}</fieldset><label>Different tensions require four knots<select required value={four} onChange={e => setFour(e.target.value)}><option value="">Choose the sourced rule</option><option value="true">Yes</option><option value="false">No</option></select></label><label>Technical source<textarea required maxLength={1000} rows={4} value={source} onChange={e => setSource(e.target.value)} placeholder="Document title, URL or reference, model/version and relevant section" /></label><label className="manage-check"><input type="checkbox" checked={verified} onChange={e => setVerified(e.target.checked)} />Verified: I have checked these model-specific constraints against the technical source</label><p>Leave verification off to save an unverified record. Removing verification prevents new bookings for this model and keeps existing order snapshots intact.</p><button>{save.busy ? 'Saving…' : 'Save racket model'}</button></fieldset></form><Feedback error={save.error || error} message="" /><button className="manage-secondary" type="button" disabled={save.busy} onClick={actions.close}>Back to results</button></section>
}
export function RacketEditor({ id, ...actions }: Actions & { id: string }) {
  const [retry, setRetry] = useState(0)
  const load = useCallback(() => id === 'new' ? Promise.resolve(null) : marketplaceAPI.admin.racket(id), [id])
  const r = useRemote(`${id}:${retry}`, load)
  return <><LoadMessage {...r} retry={() => setRetry(n => n + 1)} />{r.data !== undefined ? <RacketForm key={id} initial={r.data} {...actions} /> : <button type="button" onClick={actions.close}>Back to results</button>}</>
}
