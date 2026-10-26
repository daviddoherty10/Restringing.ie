import { useMemo, useState } from 'react'
import { examples } from '../stringers/sampleStringers'
import { MarketplaceHeader, MarketplaceFooter } from '../../components/layout/MarketplaceLayout'
import { catalogue, money, type StringMaterial } from './sampleCatalogue'
import StringFinder from './StringFinder'
import './BookingPreview.css'

const PAGE_SIZE = 8
const validKg = (v: string) => v.trim() !== '' && Number.isFinite(Number(v)) && Number(v) >= 15 && Number(v) <= 35 && Math.round(Number(v)*2) === Number(v)*2
export default function BookingPreview({ id }: { id: number }) {
  const stringer = examples.find((item)=>item.id===id)
  const [sport,setSport]=useState('')
  const [stringId,setStringId]=useState('own')
  const [query,setQuery]=useState('')
  const [material,setMaterial]=useState('All materials')
  const [page,setPage]=useState(1)
  const [finderOpen,setFinderOpen]=useState(false)
  const [different,setDifferent]=useState(false)
  const [mains,setMains]=useState('24')
  const [crosses,setCrosses]=useState('24')
  const [knots,setKnots]=useState<'2'|'4'>('2')
  const [stencil,setStencil]=useState(false)
  const [prestretch,setPrestretch]=useState<'0'|'5'|'10'>('0')
  const [notes,setNotes]=useState('')
  const [review,setReview]=useState(false)
  const selected=catalogue.find(item=>item.id===stringId)
  const filtered=useMemo(()=>catalogue.filter(item=>(material==='All materials'||item.material===material)&&[item.name,item.brand,item.gauge,item.material].some(s=>s.toLowerCase().includes(query.toLowerCase().trim()))),[material,query])
  if(!stringer)return <main className="profile-missing"><h1>Stringer not found</h1><a href="/search">← Back to search</a></main>
  const total=stringer.fromPrice+(selected?.supplement ?? 0)+(stencil?3:0)+(prestretch!=='0'?4:0)
  const valid=!!sport&&validKg(mains)&&(!different||validKg(crosses))
  function changeString(next: string){setStringId(next);setReview(false)}
  return <div className="booking-preview">
    <MarketplaceHeader backHref={`/stringers/${id}`} backLabel="Back to profile"/>
    <main className="marketplace-container booking-content">
      <div className="booking-heading"><p className="marketplace-kicker">CONFIGURE YOUR RESTRING</p><h1>Make it <em>your game.</em></h1><p>Choose your racket, strings and exact instructions, then review a sample price breakdown.</p></div>
      <div className="booking-banner" role="note"><strong>Interactive demo only.</strong> These products, services and prices are fictional. Nothing is transmitted, booked or saved, and these options do not imply this stringer's actual equipment or capabilities.</div>
      <div className="booking-grid"><section className="booking-form" aria-label="Racket configuration">
        <div className="booking-step"><span>01</span><div><h2>Your racket</h2><p>Select the sport for the racket you're restringing.</p><div className="booking-choice-grid">{stringer.racketTypes.map(type=><button key={type} type="button" className={sport===type?'booking-option chosen':'booking-option'} aria-pressed={sport===type} onClick={()=>{setSport(type);setReview(false)}}>{type}<span aria-hidden="true">{sport===type?'✓':'○'}</span></button>)}</div></div></div>
        <div className="booking-step"><span>02</span><div><h2>Choose your strings</h2><p>Search or filter a catalogue designed to scale beyond 20 strings. All items below are illustrative, not verified inventory.</p>
          <button type="button" className="finder-launch" onClick={()=>setFinderOpen(value=>!value)} aria-expanded={finderOpen}>Not sure? Help me find a setup <span aria-hidden="true">→</span></button>
          {finderOpen&&<StringFinder onClose={()=>setFinderOpen(false)} onChoose={item=>{changeString(item.id);setMaterial('All materials');setQuery('');setPage(1);setFinderOpen(false)}}/>}
          <label className="booking-field" htmlFor="string-search">Find a string<input id="string-search" type="search" placeholder="Search name, gauge or material" value={query} onChange={e=>{setQuery(e.target.value);setPage(1)}}/></label>
          <label className="booking-field" htmlFor="string-material">Filter by material<select id="string-material" value={material} onChange={e=>{setMaterial(e.target.value);setPage(1)}}>{['All materials','Multifilament','Synthetic gut','Polyester','Natural gut'].map(m=><option key={m}>{m}</option>)}</select></label>
          <p className="catalogue-count">{filtered.length} sample products found</p>
          <div className="booking-choice-list"><button type="button" className={stringId==='own'?'booking-option chosen':'booking-option'} aria-pressed={stringId==='own'} onClick={()=>changeString('own')}><span><strong>Bring my own strings</strong><small>Labour only</small></span><span>Included</span></button>{filtered.slice((page-1)*PAGE_SIZE,page*PAGE_SIZE).map(item=><button key={item.id} type="button" aria-pressed={stringId===item.id} className={stringId===item.id?'booking-option chosen':'booking-option'} onClick={()=>changeString(item.id)}><span><strong>{item.name}</strong><small>{item.material} · {item.gauge}</small></span><span>+{money(item.supplement)}</span></button>)}</div>
          {filtered.length>PAGE_SIZE&&<div className="catalogue-pages"><button type="button" disabled={page===1} onClick={()=>setPage(p=>p-1)}>Previous</button><span>Page {page} of {Math.ceil(filtered.length/PAGE_SIZE)}</span><button type="button" disabled={page===Math.ceil(filtered.length/PAGE_SIZE)} onClick={()=>setPage(p=>p+1)}>Next</button></div>}
          {filtered.length===0&&<p role="status">No sample strings match these filters. Try another search.</p>}
          {selected&&<p className="selected-string">Selected: {selected.name} ({selected.gauge})</p>}
        </div></div>
        <div className="booking-step"><span>03</span><div><h2>Tension & stringing details</h2><p>All values here are preferences to communicate to the stringer, not recommendations. Confirm the racket manufacturer's permitted range with your stringer.</p>
          <label className="finder-check"><input type="checkbox" checked={different} onChange={e=>{setDifferent(e.target.checked);setReview(false)}}/> Use different tensions for mains and crosses</label>
          <div className="tension-grid"><label className="booking-field">Mains tension (kg)<input type="number" min="15" max="35" step="0.5" value={mains} aria-invalid={!validKg(mains)} onChange={e=>{setMains(e.target.value);setReview(false)}}/>{!validKg(mains)&&<span role="alert">Enter 15–35 kg in 0.5 kg increments.</span>}</label>{different&&<label className="booking-field">Crosses tension (kg)<input type="number" min="15" max="35" step="0.5" value={crosses} aria-invalid={!validKg(crosses)} onChange={e=>{setCrosses(e.target.value);setReview(false)}}/>{!validKg(crosses)&&<span role="alert">Enter 15–35 kg in 0.5 kg increments.</span>}</label>}</div>
          <fieldset className="booking-fieldset"><legend>Knot pattern</legend><div className="booking-choice-grid">{(['2','4'] as const).map(value=><button key={value} type="button" className={knots===value?'booking-option chosen':'booking-option'} aria-pressed={knots===value} onClick={()=>{setKnots(value);setReview(false)}}>{value} knots</button>)}</div><small>Some string patterns or rackets require a specific knot layout. Your stringer must confirm compatibility.</small></fieldset>
          <label className="finder-check"><input type="checkbox" checked={stencil} onChange={e=>{setStencil(e.target.checked);setReview(false)}}/> Add stencil ink (+{money(3)} demo)</label>
          <label className="booking-field">Pre-stretch<select value={prestretch} onChange={e=>{setPrestretch(e.target.value as '0'|'5'|'10');setReview(false)}}><option value="0">No pre-stretch</option><option value="5">5% pre-stretch (+{money(4)} demo)</option><option value="10">10% pre-stretch (+{money(4)} demo)</option></select></label>
          <label className="booking-field">Additional instructions<textarea rows={3} maxLength={500} value={notes} placeholder="Racket model, preferred stencil, or details for the stringer" onChange={e=>{setNotes(e.target.value);setReview(false)}}/></label>
        </div></div>
      </section>
      <aside className="booking-summary"><p className="marketplace-kicker">YOUR SETUP</p><h2>Order preview</h2><p className="booking-stringer">{stringer.name} · {stringer.area}</p>
        <div className="booking-summary-line"><span>Racket</span><strong>{sport||'Not selected'}</strong></div>
        <div className="booking-summary-line"><span>Strings</span><strong>{selected?.name??'Own strings'}</strong></div>
        <div className="booking-summary-line"><span>Tension</span><strong>{validKg(mains)&&(!different||validKg(crosses))?`${mains} / ${different?crosses:mains} kg`:'Invalid tension'}</strong></div>
        <div className="booking-summary-line"><span>Knots</span><strong>{knots}</strong></div>
        <div className="booking-summary-line"><span>Pre-stretch</span><strong>{prestretch}%</strong></div>
        <div className="booking-summary-line"><span>Stencil</span><strong>{stencil?'Yes':'No'}</strong></div>
        <div className="booking-summary-line"><span>Example labour</span><strong>{money(stringer.fromPrice)}</strong></div>
        <div className="booking-summary-line"><span>Example strings</span><strong>{money(selected?.supplement??0)}</strong></div>
        <div className="booking-summary-line"><span>Example extras</span><strong>{money((stencil?3:0)+(prestretch==='0'?0:4))}</strong></div>
        <div className="booking-total"><span>Illustrative total</span><strong>{money(total)}</strong></div>
        <p className="booking-price-note">Not a quotation. String compatibility, real prices, stock, fees and availability must be confirmed by the backend and the stringer.</p>
        <button type="button" className="booking-review-button" disabled={!valid} onClick={()=>setReview(true)}>Review example setup →</button>
        {review&&<div className="booking-reviewed" role="status"><strong>Example setup reviewed.</strong><p>No order has been placed, saved or sent.</p>{notes&&<p>Instructions added: {notes}</p>}</div>}
        <button type="button" className="booking-disabled" disabled>Place order — coming soon</button>
      </aside></div>
    </main><MarketplaceFooter/>
  </div>
}
