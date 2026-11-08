import { useEffect, useRef, useState } from 'react'
import { marketplaceAPI, MarketplaceError, type Booking, type Quote, type Variant } from '../../lib/marketplace-api'
import type { components } from '../../lib/marketplace-contract'
import { MarketplaceHeader, MarketplaceFooter } from '../../components/layout/MarketplaceLayout'
import './BookingPreview.css'
import TensionInput from './TensionInput'
import CheckoutConsent from '../compliance/CheckoutConsent'
import GripPicker from '../grips/GripPicker'
import HybridPicker from '../hybrids/HybridPicker'
import GuidedStringFinder from '../recommendations/GuidedStringFinder'
import { signInURL } from '../../lib/auth-navigation'
import { loadBookingDraft, saveBookingDraft, removeBookingDraft } from '../../lib/booking-draft'
import { inTensionRange, kilogramsToCentiKg, tensionDisplay, type TensionUnit } from '../../lib/marketplace-units'
type Racket=components['schemas']['Racket']
type Options=components['schemas']['BookingOptions']
const eur=(c:number)=>new Intl.NumberFormat('en-IE',{style:'currency',currency:'EUR'}).format(c/100)
function problem(e:unknown){return e instanceof MarketplaceError?[e.message,...Object.values(e.fields)].join(' '):'The service is temporarily unavailable. Please retry.'}
export default function BookingPreview({id}:{id:string}){
 const [initialDraft]=useState(()=>loadBookingDraft(id))
 const [draftReady,setDraftReady]=useState(!initialDraft)
 const draft=useRef(initialDraft)
 const [gripProductID,setGripProductID]=useState(initialDraft?.gripProductID??'')
 const [gripLabel,setGripLabel]=useState(initialDraft?.gripLabel??'')
 const [gripValid,setGripValid]=useState(!initialDraft?.gripProductID)
 const [gripsOffered,setGripsOffered]=useState(false)
 const [setupNotice,setSetupNotice]=useState('')
 const [signedIn,setSignedIn]=useState(false)
 const [rackets,setRackets]=useState<Racket[]>([])
 const [racketID,setRacketID]=useState(initialDraft?.racketID??'')
 const [racketSearch,setRacketSearch]=useState('')
 const [config,setConfig]=useState<Options|null>(null)
 const [variants,setVariants]=useState<Variant[]>([])
 const [variant,setVariant]=useState<Variant|null>(null)
 const [hybrid,setHybrid]=useState(false)
 const [ownMains,setOwnMains]=useState(initialDraft?.customerMains??''),[ownCrosses,setOwnCrosses]=useState(initialDraft?.customerCrosses??'')
 const [crossVariant,setCrossVariant]=useState<Variant|null>(null)
 const [mainsAvailable,setMainsAvailable]=useState(false),[crossesAvailable,setCrossesAvailable]=useState(false)
 const [stringSource,setStringSource]=useState<'stock'|'customer'>('stock')
 const [inventoryQuery,setInventoryQuery]=useState('')
 const [page,setPage]=useState(1)
 const [total,setTotal]=useState(0)
 const [mains,setMains]=useState('')
 const [crosses,setCrosses]=useState('')
 const [split,setSplit]=useState(false)
 const [knots,setKnots]=useState<2|4>(4)
 const [express,setExpress]=useState(false)
 const [stencil,setStencil]=useState(false)
 const [prestretch,setPrestretch]=useState(0)
 const [notes,setNotes]=useState('')
 const [acceptance,setAcceptance]=useState<components['schemas']['BookingAcceptance']|undefined>(initialDraft?.attempt?initialDraft.acceptance:undefined)
 const [legalSubmitLabel,setLegalSubmitLabel]=useState('Book — pay cash to stringer')
 const [legalReady,setLegalReady]=useState(false)
 const [quote,setQuote]=useState<Quote|null>(null)
 const [quoteBusy,setQuoteBusy]=useState(false)
 const [orderBusy,setOrderBusy]=useState(false)
 const [orderID,setOrderID]=useState('')
 const [attempt,setAttempt]=useState<{quoteID:string;key:string}|null>(initialDraft?.attempt??null)
 const [error,setError]=useState('')
 const [fields,setFields]=useState<Record<string,string>>({})
 const [pendingPrice,setPendingPrice]=useState(false)
 const [tensionUnit,setTensionUnit]=useState<TensionUnit>(initialDraft?.tensionUnit??'kg')
 const [guided,setGuided]=useState(initialDraft?.guided??false)
 const [finding,setFinding]=useState(false)
 const [refresh,setRefresh]=useState(0)
 const quoteSeq=useRef(0),inventorySeq=useRef(0)
 const racket=rackets.find(r=>r.id===racketID)
 const racketSport=racket?.sport
 const service=config?.service
 const hybridSupported=!!service?.hybrid&&!!config?.allowed_knots.includes(4)
 const prestretchOptions=(stringSource==='stock'||hybrid)&&variant?Object.keys(service?.prestretch_options??{}).filter(p=>Number(p)>0&&Number(p)<=variant.max_prestretch_percent&&(!hybrid||!!crossVariant&&Number(p)<=crossVariant.max_prestretch_percent)):[]
 const range=config
 const centi=(s:string)=>kilogramsToCentiKg(s)??NaN
 const tensionValid=(s:string)=>!!range&&inTensionRange(s,range.min_tension_centi_kg,range.max_tension_centi_kg,range.step_centi_kg)
 const crossesActual=split?crosses:mains
 const valid=draftReady&&(stringSource!=='customer'||!!service?.own_strings)&&!!racket&&!!config&&tensionValid(mains)&&tensionValid(crossesActual)&&Math.abs(centi(mains)-centi(crossesActual))<=config.max_difference_centi_kg&&(!split||!!service?.different_tensions)&&config.allowed_knots.includes(knots)&&(!split||centi(mains)===centi(crossesActual)||!racket.different_tensions_require_four_knots||knots===4)&&(!stencil||!!service?.stencil)&&(!prestretch||(stringSource==='stock'||hybrid)&&Object.hasOwn(service?.prestretch_options??{},String(prestretch))&&!!variant&&prestretch<=variant.max_prestretch_percent)&&(hybrid?hybridSupported&&knots===4&&(variant?mainsAvailable:stringSource==='customer'&&ownMains.trim().length>0&&ownMains.length<=160)&&(crossVariant?crossesAvailable:stringSource==='customer'&&ownCrosses.trim().length>0&&ownCrosses.length<=160)&&(!prestretch||!!crossVariant&&prestretch<=crossVariant.max_prestretch_percent):(stringSource==='customer'?!!service?.own_strings:!!variant))&&(!express||!!service?.express)&&(!gripProductID||gripValid)&&notes.length<=500
 function invalidate(){setAcceptance(undefined);setLegalReady(false);quoteSeq.current++;setQuoteBusy(false);setQuote(null);setFields({});setPendingPrice(false);setAttempt(null);removeBookingDraft(id);setOrderID('');setError('')}
 function modify(fn:()=>void){if(attempt){setError('An order submission may still be pending. Retry it before changing the setup.');return}invalidate();fn()}
 useEffect(()=>{let active=true;marketplaceAPI.me().then(()=>{if(active)setSignedIn(true)}).catch(()=>{if(active)setSignedIn(false)});return()=>{active=false}},[])
 useEffect(()=>{let active=true;marketplaceAPI.rackets({page_size:50}).then(r=>{if(active)setRackets(r.data)}).catch(e=>{if(active)setError(problem(e))});return()=>{active=false}},[id,refresh])
 useEffect(()=>{
  let active=true
  if(!racketID)return
  async function load(){
   try{
    const c=await marketplaceAPI.bookingConfiguration(id,racketID)
    if(!active)return
    setConfig(c)
    const saved=draft.current;draft.current=null
    if(saved&&saved.racketID===racketID){
     // A possibly committed order keeps its original setup and retry identity.
     // Editable drafts drop services that have since been withdrawn.
     const pending=!!saved.attempt
     const restoredSource=pending||saved.stringSource!=='customer'||c.service.own_strings?saved.stringSource:'stock'
     const restoredHybrid=!!saved.hybrid&&(pending||!!c.service.hybrid&&c.allowed_knots.includes(4))
     const restoredSplit=saved.split&&(pending||c.service.different_tensions)
     const restoredExpress=!!saved.express&&(pending||!!c.service.express)
     const restoredStencil=saved.stencil&&(pending||c.service.stencil)
     const restoredKnots=pending?saved.knots:restoredHybrid?4:c.allowed_knots.includes(saved.knots)?saved.knots:c.allowed_knots.includes(4)?4:2
     let restoredMains:Variant|null=null,restoredCrosses:Variant|null=null
     setMains(saved.mains);setCrosses(saved.crosses);setKnots(restoredKnots);setSplit(restoredSplit);setExpress(restoredExpress);setStencil(restoredStencil);setNotes(saved.notes);setStringSource(restoredSource);setHybrid(restoredHybrid);setOwnMains(saved.customerMains??'');setOwnCrosses(saved.customerCrosses??'')
     if(restoredSource===saved.stringSource&&restoredHybrid===!!saved.hybrid){
      if(restoredHybrid){
       const [m,cross]=await Promise.all([saved.variantID?(restoredSource==='customer'?marketplaceAPI.strings({variant_id:saved.variantID,page_size:1}):marketplaceAPI.hybridInventory(id,racketID,'mains',{variant_id:saved.variantID,page_size:1})):Promise.resolve({data:[] as Variant[]}),saved.crossesVariantID?(restoredSource==='customer'?marketplaceAPI.strings({variant_id:saved.crossesVariantID,page_size:1}):marketplaceAPI.hybridInventory(id,racketID,'crosses',{variant_id:saved.crossesVariantID,page_size:1})):Promise.resolve({data:[] as Variant[]})])
       restoredMains=m.data[0]??null;restoredCrosses=cross.data[0]??null
       if(active&&((saved.variantID&&!restoredMains)||(saved.crossesVariantID&&!restoredCrosses)))setError('A selected hybrid string is no longer available. Please choose both components again.')
      }else if(saved.variantID&&restoredSource==='stock'){
       const result=await marketplaceAPI.inventory(id,{variant_id:saved.variantID,page_size:1})
       restoredMains=result.data.find(v=>v.id===saved.variantID)??null
       if(active&&!restoredMains)setError('Your selected string is no longer available. Please choose another.')
      }
     }
     if(!active)return
     setVariant(restoredMains);setCrossVariant(restoredCrosses)
     const restoredPrestretch=pending||!!restoredMains&&Object.hasOwn(c.service.prestretch_options??{},String(saved.prestretch))&&saved.prestretch<=restoredMains.max_prestretch_percent&&(!restoredHybrid||!!restoredCrosses&&saved.prestretch<=restoredCrosses.max_prestretch_percent)?saved.prestretch:0
     setPrestretch(restoredPrestretch)
     if(!pending&&(restoredSource!==saved.stringSource||restoredHybrid!==!!saved.hybrid||restoredSplit!==saved.split||restoredExpress!==!!saved.express||restoredStencil!==saved.stencil||restoredKnots!==saved.knots||restoredPrestretch!==saved.prestretch))setSetupNotice('Some saved options are no longer available for this setup and have been removed. Review your choices and request a new quote.')
    }else{setMains(String(c.min_tension_centi_kg/100));setCrosses(String(c.min_tension_centi_kg/100));setKnots(c.allowed_knots.includes(4)?4:2);setSplit(false);setHybrid(false);setCrossVariant(null);setExpress(false);setStencil(false);setPrestretch(0);if(!c.service.own_strings)setStringSource('stock')}

   }catch(e){if(active)setError(problem(e))}finally{if(active)setDraftReady(true)}
  }
  void load()
  return()=>{active=false}
 },[id,racketID])
 useEffect(()=>{if(!racketSport)return;const controller=new AbortController(),seq=++inventorySeq.current;const timer=setTimeout(()=>marketplaceAPI.inventory(id,{sport:racketSport,q:inventoryQuery,page,page_size:10},controller.signal).then(r=>{if(seq===inventorySeq.current){setVariants(r.data);setTotal(r.total)}}).catch(e=>{if(seq===inventorySeq.current&&!controller.signal.aborted)setError(problem(e))}),300);return()=>{clearTimeout(timer);controller.abort()}},[id,racketSport,inventoryQuery,page,refresh])
 useEffect(()=>()=>{quoteSeq.current++},[])
 useEffect(()=>{if(!draftReady||!config||orderID||attempt)return;saveBookingDraft(id,{savedAt:Date.now(),guided,tensionUnit,racketID,variantID:variant?.id??'',variantModel:variant?.model??'',stringSource,hybrid,customerMains:ownMains,customerCrosses:ownCrosses,crossesVariantID:crossVariant?.id??'',mains,crosses,split,knots,gripProductID,gripLabel,express,stencil,prestretch,notes,attempt})},[id,draftReady,config,guided,tensionUnit,racketID,variant,stringSource,hybrid,crossVariant,ownMains,ownCrosses,mains,crosses,split,knots,gripProductID,gripLabel,express,stencil,prestretch,notes,attempt,orderID])
 const selected=variant
 const booking:Booking={...(hybrid?{hybrid:true,crosses_variant_id:crossVariant?.id,...(stringSource==='customer'?{customer_mains_description:ownMains,customer_crosses_description:ownCrosses}:{})}:{}),racket_model_id:racketID,string_source:stringSource,mains_tension_centi_kg:centi(mains),crosses_tension_centi_kg:centi(crossesActual),knots,...(gripProductID?{grip_product_id:gripProductID}:{}),express,stencil,prestretch_percent:prestretch,instructions:notes,...((stringSource==='stock'||hybrid)&&selected?{variant_id:selected.id}:{})}
 function storeDraft(pending=attempt){if(pending&&initialDraft?.attempt)return saveBookingDraft(id,{...initialDraft,savedAt:Date.now(),attempt:pending});return saveBookingDraft(id,{savedAt:Date.now(),guided,tensionUnit,racketID,variantID:variant?.id??'',variantModel:variant?.model??'',stringSource,hybrid,customerMains:ownMains,customerCrosses:ownCrosses,crossesVariantID:crossVariant?.id??'',mains,crosses,split,knots,gripProductID,gripLabel,express,stencil,prestretch,notes,acceptance,attempt:pending})}
 function signIn(){if(!storeDraft()){setError('Your browser cannot save this setup. Allow session storage before signing in to keep your choices.');return}window.location.assign(signInURL(`/stringers/${id}/book`))}
 async function review(){
  if(!valid)return;const seq=++quoteSeq.current;setQuoteBusy(true);setError('');setFields({})
  try{const q=await marketplaceAPI.quote(id,booking);if(seq!==quoteSeq.current)return;setAcceptance(undefined);setLegalReady(false);setQuote(q);setPendingPrice(false)}
  catch(e){if(seq!==quoteSeq.current)return;setError(problem(e));if(e instanceof MarketplaceError){setFields(e.fields);if(e.code==='grip_out_of_stock'||e.code==='hybrid_out_of_stock'||e.fields.grip_product_id)setRefresh(n=>n+1);}if(e instanceof MarketplaceError&&e.status===401)signIn()}
  finally{if(seq===quoteSeq.current)setQuoteBusy(false)}
 }
 async function submit(){
  if((!quote&&!attempt)||pendingPrice||orderBusy)return
  if(!attempt&&quote&&Date.parse(quote.expires_at)<=Date.now()){setError('This quote expired. Request a fresh quotation.');setQuote(null);setAttempt(null);return}
  const current=attempt??{quoteID:quote!.id,key:crypto.randomUUID()}
  storeDraft(current)
  setAttempt(current);setOrderBusy(true);setError('')
  try{const order=await marketplaceAPI.createOrder(current.quoteID,current.key,acceptance);setOrderID(order.id);setAttempt(null);removeBookingDraft(id)}
  catch(e){setError(problem(e));if(e instanceof MarketplaceError&&['grip_out_of_stock','hybrid_out_of_stock'].includes(e.code))setRefresh(n=>n+1);if(e instanceof MarketplaceError&&e.status===401)setSignedIn(false);if(e instanceof MarketplaceError&&[404,409,422].includes(e.status)){setPendingPrice(e.status===409&&e.code!=='booking_limit_reached');setQuote(null);setAttempt(null);removeBookingDraft(id);if(e.status!==409){setConfig(null);setRacketID('');setVariant(null);setVariants([]);setRefresh(n=>n+1)}}}
  finally{setOrderBusy(false)}
 }
 function useSetup(rec:components['schemas']['Recommendation']){
  const setup=rec.setup;if(!setup||setup.booking.racket_model_id!==racketID||attempt)return
  const b=setup.booking
  modify(()=>{setStringSource('stock');setVariant(rec.variant);setCrossVariant(setup.crosses_variant??null);setHybrid(!!b.hybrid);setMainsAvailable(false);setCrossesAvailable(false);setOwnMains('');setOwnCrosses('');setMains(String(b.mains_tension_centi_kg/100));setCrosses(String(b.crosses_tension_centi_kg/100));setSplit(b.mains_tension_centi_kg!==b.crosses_tension_centi_kg);setKnots(b.knots as 2|4);setPrestretch(b.prestretch_percent??0);setStencil(!!b.stencil);setExpress(!!b.express);setGripProductID('');setGripLabel('');setGripValid(true);setGuided(true);setSetupNotice('')})
 }
 return <div className="booking-preview"><MarketplaceHeader backHref={`/stringers/${id}`} backLabel="Back to profile"/><main className="marketplace-container booking-content">
  <div className="booking-heading"><p className="marketplace-kicker">BUILD YOUR SETUP</p><h1>Make it <em>your game.</em></h1><p>Choose a verified racket and the services this stringer supports. Prices are supplied by the backend only.</p></div>
  {setupNotice&&<p className="booking-banner" role="status">{setupNotice}</p>}
  {error&&<div className="booking-banner" role="alert">{error} <button type="button" onClick={()=>{setError('');setRefresh(n=>n+1)}}>Retry loading</button></div>}
  {Object.keys(fields).length>0&&<div role="alert" className="booking-banner">{Object.entries(fields).map(([k,v])=><p key={k}>{k}: {v}</p>)}</div>}
  <div className="booking-grid"><section className="booking-form">{!draftReady&&<p role="status">Restoring your saved setup…</p>}<fieldset className="booking-setup" aria-label="Racket configuration" disabled={!draftReady||!!attempt}>
   <div className="booking-step"><span>01</span><div><h2>Verified racket model</h2><p>Only verified models can be booked. If yours is missing, ask the stringer for manual review.</p><label className="booking-field">Find your racket<input type="search" value={racketSearch} onChange={e=>setRacketSearch(e.target.value)} placeholder="Brand or model"/></label><label className="booking-field">Racket model<select value={racketID} onChange={e=>modify(()=>{setFinding(false);setGuided(false);setRacketID(e.target.value);setConfig(null);setVariant(null);setCrossVariant(null);setHybrid(false);setVariants([]);setPage(1);setInventoryQuery('')})}><option value="">Choose verified model</option>{rackets.filter(r=>r.verified&&[r.brand,r.model].join(' ').toLowerCase().includes(racketSearch.toLowerCase())).map(r=><option value={r.id} key={r.id}>{r.brand} {r.model} · {r.sport}</option>)}</select></label>{!racketID&&<p role="status">No model selected. Unverified rackets require manual review; technical limits cannot be guessed.</p>}</div></div>
   {racketID&&config&&<>
   <div className="booking-step"><span>02</span><div><h2>{stringSource==='customer'?'Your supplied strings':'Strings from this stringer'}</h2><p>{stringSource==='customer'?'Tell the stringer which strings you will bring. Hybrid mains and crosses are identified separately.':'Only actual offered and available variants are shown; search and paginate without losing your selection.'}</p><GuidedStringFinder key={`${id}:${racketID}`} stringerID={id} racketID={racketID} disabled={!draftReady||!!attempt} onOpenChange={setFinding} choose={useSetup}/>{guided&&<div className="booking-guided-ready" role="status"><strong>Your setup is ready.</strong><p>Strings, tension and stringing method are selected. Review the summary, add any extras you want, then request your final quote. The stringer confirms suitability before work.</p></div>}<details className="booking-manual" open={!guided&&!finding}><summary>Choose strings myself (optional)</summary>{service?.own_strings&&<label className="finder-check"><input type="checkbox" checked={stringSource==='customer'} onChange={e=>modify(()=>{setGuided(false);setStringSource(e.target.checked?'customer':'stock');setVariant(null);setCrossVariant(null);setOwnMains('');setOwnCrosses('');setMainsAvailable(false);setCrossesAvailable(false);setPrestretch(0)})}/> I'll provide my own strings</label>}
   {<><fieldset className="booking-fieldset"><legend>String setup</legend><label className="finder-check"><input type="radio" name="string-setup" checked={!hybrid} onChange={()=>modify(()=>{setGuided(false);setHybrid(false);setCrossVariant(null);setPrestretch(0)})}/> Full bed — one string throughout</label>{hybridSupported&&<label className="finder-check"><input type="radio" name="string-setup" checked={hybrid} disabled={!hybridSupported} onChange={()=>modify(()=>{setHybrid(true);setVariant(null);setCrossVariant(null);setMainsAvailable(false);setCrossesAvailable(false);setKnots(4);setPrestretch(0);setGuided(false)})}/> Hybrid — choose mains and crosses</label>}</fieldset>{hybrid&&<><p>Choose the mains and crosses below. Stock material uses half of each full-bed price; customer-supplied material has no charge. Pre-stretch, if selected, applies to both strings.</p><HybridPicker stringerID={id} racketID={racketID} component="mains" source={stringSource} sport={racketSport!} description={ownMains} describe={text=>modify(()=>{setOwnMains(text);setVariant(null);setPrestretch(0)})} selected={variant} locked={!!attempt} revision={refresh} choose={v=>modify(()=>{setVariant(v);setOwnMains('');if(variant?.id!==v?.id)setMainsAvailable(false);setPrestretch(0)})} onAvailability={setMainsAvailable}/><HybridPicker stringerID={id} racketID={racketID} component="crosses" mainsPriceCents={variant?.material_price_cents} source={stringSource} sport={racketSport!} description={ownCrosses} describe={text=>modify(()=>{setOwnCrosses(text);setCrossVariant(null);setPrestretch(0)})} selected={crossVariant} locked={!!attempt} revision={refresh} choose={v=>modify(()=>{setCrossVariant(v);setOwnCrosses('');if(crossVariant?.id!==v?.id)setCrossesAvailable(false);setPrestretch(0)})} onAvailability={setCrossesAvailable}/></>}{!hybrid&&stringSource==='stock'&&<><label className="booking-field">Search strings<input type="search" value={inventoryQuery} onChange={e=>{setInventoryQuery(e.target.value);setPage(1)}} placeholder="Brand, name, gauge"/></label><div className="booking-choice-list">{variants.map(v=><button key={v.id} type="button" className={selected?.id===v.id?'booking-option chosen':'booking-option'} aria-pressed={selected?.id===v.id} onClick={()=>modify(()=>{setGuided(false);setVariant(v);setPrestretch(0)})}><span><strong>{v.brand} {v.model}</strong><small>{v.material.replaceAll('_',' ')} · {v.gauge_mm} mm · {v.colour}</small></span><span>{eur(v.material_price_cents)}</span></button>)}</div>{variants.length===0&&<p>No offered strings match this search.</p>}{selected&&<p className="selected-string">Selected: {selected.brand} {selected.model}</p>}{total>10&&<div className="catalogue-pages"><button type="button" disabled={page===1} onClick={()=>setPage(p=>p-1)}>Previous</button><span>Page {page} / {Math.ceil(total/10)}</span><button type="button" disabled={page*10>=total} onClick={()=>setPage(p=>p+1)}>Next</button></div>}
   </>}</>}</details></div></div>

   <div className="booking-step"><span>03</span><div><h2>Stringing instructions</h2><details className="booking-technical" open={!guided&&!finding}><summary>Adjust stringing settings (optional)</summary><p>{config.notice} Allowed tension: {config.min_tension_centi_kg/100}–{config.max_tension_centi_kg/100} kg in {config.step_centi_kg/100} kg increments.</p><label className="booking-field">Tension unit<select value={tensionUnit} onChange={e=>setTensionUnit(e.target.value as TensionUnit)}><option value="kg">Kilograms (kg)</option><option value="lb">Pounds (lb)</option></select></label><TensionInput label="Mains tension" value={mains} unit={tensionUnit} min={config.min_tension_centi_kg} max={config.max_tension_centi_kg} step={config.step_centi_kg} change={v=>modify(()=>setMains(v))}/>
   {service?.different_tensions&&<label className="finder-check"><input type="checkbox" checked={split} onChange={e=>modify(()=>setSplit(e.target.checked))}/> Use separate crosses tension</label>}{split&&<TensionInput label="Crosses tension" value={crosses} unit={tensionUnit} min={config.min_tension_centi_kg} max={config.max_tension_centi_kg} step={config.step_centi_kg} change={v=>modify(()=>setCrosses(v))}/>}
   {!hybrid&&<fieldset className="booking-fieldset"><legend>Knots</legend><div className="booking-choice-grid">{config.allowed_knots.filter(k=>k===2||k===4).map(k=><button type="button" key={k} className={knots===k?'booking-option chosen':'booking-option'} aria-pressed={knots===k} onClick={()=>modify(()=>setKnots(k as 2|4))}>{k} knots</button>)}</div>{service?.two_knots&&!config.allowed_knots.includes(2)&&<p>Two-knot stringing is offered by this stringer, but this racket’s verified pattern allows four knots only.</p>}<p>Knot choices must be supported by both this racket and the stringer. Different tensions may require four knots.</p></fieldset>}
   {service?.stencil&&<label className="finder-check"><input type="checkbox" checked={stencil} onChange={e=>modify(()=>setStencil(e.target.checked))}/> Add stencil ({eur(service.stencil_cents)})</label>}
   {prestretchOptions.length>0&&<label className="booking-field">Pre-stretch<select value={prestretch} onChange={e=>modify(()=>setPrestretch(Number(e.target.value)))}><option value={0}>None</option>{prestretchOptions.map(p=><option value={p} key={p}>{p}%</option>)}</select></label>}
   </details>
   {service?.express&&<fieldset className="booking-fieldset booking-turnaround"><legend>Turnaround</legend><label className="finder-check"><input type="radio" name="turnaround" checked={!express} onChange={()=>modify(()=>setExpress(false))}/> Standard turnaround</label><label className="finder-check"><input type="radio" name="turnaround" checked={express} onChange={()=>modify(()=>setExpress(true))}/> Express turnaround (+{eur(service.express_fee_cents)})</label><p>{service.express_turnaround}</p><p>Your stringer confirms the ready estimate after accepting your booking. A requested collection date does not select express automatically.</p></fieldset>}

   <GripPicker onOffersChange={setGripsOffered} revision={refresh} stringerID={id} selectedID={gripProductID} selectedLabel={gripLabel} locked={!!attempt} onAvailability={setGripValid} choose={g=>modify(()=>{setGripProductID(g?.id??'');setGripLabel(g?`${g.brand} ${g.model} · ${g.kind.replaceAll('_',' ')} · ${g.colour}`:'');setGripValid(true)})}/>
   <label className="booking-field">Additional instructions<textarea maxLength={500} rows={3} value={notes} onChange={e=>modify(()=>setNotes(e.target.value))}/></label></div></div></>}
  </fieldset></section><aside className="booking-summary"><p className="marketplace-kicker">SERVER QUOTATION</p><h2>Review your setup</h2><p>Payment is made directly to the selected stringer. The legal identity of the contracting trader must be confirmed before public launch. Before booking, review our <a href="/terms" target="_blank" rel="noopener noreferrer">draft marketplace terms</a> and <a href="/cancellations" target="_blank" rel="noopener noreferrer">draft cancellation information</a>. These documents and the required contracting-trader details are pending legal review before public launch.</p><fieldset className="booking-fieldset"><legend>Payment method</legend><label><input type="radio" name="payment" value="cash" checked readOnly /> Pay cash directly to the stringer</label><p>Payment is due to the stringer. No money is collected online.</p></fieldset>{!signedIn&&<><p className="booking-stringer">Sign in to request a server quote and place your booking.</p><button type="button" disabled={!draftReady} onClick={signIn}>Sign in and keep this setup</button></>}<div className="booking-summary-line"><span>Racket</span><strong>{racket?`${racket.brand} ${racket.model}`:'Not selected'}</strong></div><div className="booking-summary-line"><span>String</span><strong>{hybrid?'Hybrid — mains and crosses':stringSource==='customer'?'Own strings':selected?`${selected.brand} ${selected.model}`:'Not selected'}</strong></div>{hybrid&&<><div className="booking-summary-line"><span>Mains</span><strong>{variant?`${variant.brand} ${variant.model}`:ownMains||'Choose mains'}</strong></div><div className="booking-summary-line"><span>Crosses</span><strong>{crossVariant?`${crossVariant.brand} ${crossVariant.model}`:ownCrosses||'Choose crosses'}</strong></div></>}{(gripsOffered||gripProductID)&&<div className="booking-summary-line"><span>Grip</span><strong>{gripProductID?gripLabel:'No new grip'}</strong></div>}{(service?.express||express)&&<div className="booking-summary-line"><span>Turnaround</span><strong>{express?'Express':'Standard'}</strong></div>}<div className="booking-summary-line"><span>Tensions</span><strong>{tensionUnit==='lb'?`${tensionDisplay(centi(mains),'lb')} / ${tensionDisplay(centi(crossesActual),'lb')} lb (${mains} / ${crossesActual} kg)`:`${mains} / ${crossesActual} kg`}</strong></div><button type="button" className="booking-review-button" disabled={!valid||quoteBusy||!!attempt} onClick={review}>{quoteBusy?'Requesting…':'Request server quote →'}</button>
  {quote&&<><CheckoutConsent key={quote.id} id={id} locked={!!attempt} changed={setAcceptance} ready={setLegalReady} labelChanged={setLegalSubmitLabel}/><div className="booking-reviewed">{quote.configuration?.grip_snapshot&&<p className="booking-grip-quoted">Quoted grip: {quote.configuration.grip_snapshot} (fitting included)</p>}{quote.configuration?.express&&<p className="booking-express-quoted">Quoted express turnaround: {quote.configuration.express_turnaround_snapshot}</p>}{quote.breakdown.map(line=><div className="booking-summary-line" key={line.code}><span>{line.label}</span><strong>{eur(line.amount_cents)}</strong></div>)}<div className="booking-total"><span>Total</span><strong>{eur(quote.total_cents)}</strong></div><p>Expires {new Date(quote.expires_at).toLocaleString('en-IE')}</p></div>{<button type="button" className="booking-review-button" disabled={orderBusy||pendingPrice||(!attempt&&!legalReady)} onClick={submit}>{orderBusy?'Submitting…':attempt?'Retry same order request':legalSubmitLabel}</button>}</>}
  {!quote&&attempt&&<button type="button" className="booking-review-button" disabled={orderBusy} onClick={submit}>{orderBusy?'Checking…':'Retry pending booking'}</button>}{pendingPrice&&<p role="alert">The price, stock or configuration changed. Request and confirm a new quotation before booking.</p>}{orderID&&<div role="status" className="booking-reviewed"><strong>Booking created — pay cash to your stringer.</strong><p>Payment status: unpaid</p><a href={`/orders/${orderID}`}>View order</a><p><a href={`/orders/${orderID}/confirmation`}>View and download booking confirmation</a></p></div>}
  </aside></div></main><MarketplaceFooter/></div>
}
