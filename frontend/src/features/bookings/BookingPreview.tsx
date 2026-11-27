import StringPhoto from '../strings/StringPhoto'
import { useEffect, useRef, useState } from 'react'
import { marketplaceAPI, MarketplaceError, type Booking, type Quote, type Stringer, type Variant } from '../../lib/marketplace-api'
import type { components } from '../../lib/marketplace-contract'
import { MarketplaceHeader, MarketplaceFooter } from '../../components/layout/MarketplaceLayout'
import './BookingPreview.css'
import TensionInput from './TensionInput'
import CheckoutConsent from '../compliance/CheckoutConsent'
import GripPicker from '../grips/GripPicker'
import HybridPicker from '../hybrids/HybridPicker'
import GuidedStringFinder from '../recommendations/GuidedStringFinder'
import { profileSetupURL, signInURL } from '../../lib/auth-navigation'
import { loadBookingDraft, saveBookingDraft, removeBookingDraft, type BookingDraft } from '../../lib/booking-draft'
import { enteredTension, kilogramsToCentiKg, savedTension, tensionBandLabel, type TensionUnit } from '../../lib/marketplace-units'
type Sport=Variant['sport']
type Options=components['schemas']['BookingOptions']
const eur=(c:number)=>new Intl.NumberFormat('en-IE',{style:'currency',currency:'EUR'}).format(c/100)
function problem(e:unknown){return e instanceof MarketplaceError?[e.message,...Object.values(e.fields)].join(' '):'The service is temporarily unavailable. Please retry.'}
export default function BookingPreview({id}:{id:string}){
 const [initialDraft]=useState(()=>loadBookingDraft(id))
 const [compact] = useState(() => window.matchMedia('(max-width: 900px)').matches)
 const [stringer,setStringer]=useState<Stringer|null>(null)
 const guidedConfirmation=useRef<HTMLDivElement>(null)
 const [draftReady,setDraftReady]=useState(!initialDraft)
 const draft=useRef(initialDraft)
 const [gripProductID,setGripProductID]=useState(initialDraft?.gripProductID??'')
 const [gripLabel,setGripLabel]=useState(initialDraft?.gripLabel??'')
 const [gripPrice,setGripPrice]=useState<number|null>(initialDraft?.gripProductID?null:0)
 const [gripsOffered,setGripsOffered]=useState(false)
 const [setupNotice,setSetupNotice]=useState('')
 const [signedIn,setSignedIn]=useState(false)
 const [services,setServices]=useState<components['schemas']['Service'][]|null>(null)
 const [sport,setSport]=useState<Sport|''>(initialDraft?.sport??'')
 const [racketDetails,setRacketDetails]=useState(initialDraft?.racketDetails??'')
 const [tensionPending,setTensionPending]=useState(initialDraft?.tensionPending??!initialDraft?.racketID)
 const [tensionBand,setTensionBand]=useState<Booking['tension_band']>(initialDraft?.tensionBand)
 const [racketID,setRacketID]=useState(initialDraft?.racketID??'')
 const [paymentOptions,setPaymentOptions]=useState<components['schemas']['PaymentOptions']|null>(null)
 const [paymentMethod,setPaymentMethod]=useState<'cash'|'onsite'|'online'>(initialDraft?.paymentMethod??(initialDraft?.attempt?'cash':'onsite'))
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
 const [inventoryResult,setInventoryResult]=useState<{key:string;failed:boolean}|null>(null)
 const [mains,setMains]=useState('')
 const [crosses,setCrosses]=useState('')
 const [split,setSplit]=useState(false)
 const [knots,setKnots]=useState<2|4>(4)
 const [express,setExpress]=useState(false)
 const [stencil,setStencil]=useState(false)
 const [prestretch,setPrestretch]=useState(0)
 const [notes,setNotes]=useState('')
 const [acceptance,setAcceptance]=useState<components['schemas']['BookingAcceptance']|undefined>(initialDraft?.attempt?initialDraft.acceptance:undefined)
 const [legalSubmitLabel,setLegalSubmitLabel]=useState('Confirm booking')
 const [legalReady,setLegalReady]=useState(false)
 const [quote,setQuote]=useState<Quote|null>(null)
 const [quoteBusy,setQuoteBusy]=useState(false)
 const [orderBusy,setOrderBusy]=useState(false)
 const [orderID,setOrderID]=useState('')
 const [onlineOrder,setOnlineOrder]=useState(false)
 const [attempt,setAttempt]=useState<{quoteID:string;key:string}|null>(initialDraft?.attempt??null)
 const [error,setError]=useState('')
 const [fields,setFields]=useState<Record<string,string>>({})
 const [pendingPrice,setPendingPrice]=useState(false)
 const [tensionUnit,setTensionUnit]=useState<TensionUnit>(initialDraft?.exactPounds&&initialDraft.tensionUnit==='lb'?'lb':'kg')
 const [savedRackets,setSavedRackets]=useState<components['schemas']['SavedRacket'][]>([])
 const [setupRevision,setSetupRevision]=useState(0)
 const [savedReference,setSavedReference]=useState<Booking|null>(null)
 const [guided,setGuided]=useState(initialDraft?.guided??false)
 const [finding,setFinding]=useState(false)
 const [refresh,setRefresh]=useState(0)
 const quoteSeq=useRef(0),inventorySeq=useRef(0)
 const racket=config?.racket
 const offeredSport=services?.length===1?services[0].sport:undefined
 const bookingSport=racketID||(initialDraft?.attempt&&attempt)?sport:services&&(offeredSport||sport)
 const racketSport=bookingSport||config?.service.sport
 const service=config?.service
 const labour=service?.labour_cents??(services?.length?Math.min(...services.map(s=>s.labour_cents)):null)
 const area=[...new Set([stringer?.area,stringer?.county].filter(Boolean))].join(', ')
 const inventoryKey=JSON.stringify([id,racketSport,inventoryQuery,page,refresh])
 const inventoryLoading=inventoryResult?.key!==inventoryKey

 const hybridSupported=!!service?.hybrid&&!!config?.allowed_knots.includes(4)
 const prestretchOptions=(stringSource==='stock'||hybrid)&&variant?Object.keys(service?.prestretch_options??{}).filter(p=>Number(p)>0&&Number(p)<=variant.max_prestretch_percent&&(!hybrid||!!crossVariant&&Number(p)<=crossVariant.max_prestretch_percent)):[]
 const range=config
 const centi=(s:string)=>kilogramsToCentiKg(s)??NaN
 const tensionValid=(s:string)=>!!range&&enteredTension(s,tensionUnit,range.min_tension_centi_kg,range.max_tension_centi_kg,range.step_centi_kg)!==null
 const crossesActual=split?crosses:mains
 const onsiteTenders=[paymentOptions?.onsite_cash?'cash':'',paymentOptions?.onsite_card?'card':''].filter(Boolean).join(' or ')
 const stringPrice = stringSource === 'customer' ? service?.own_strings_cents ?? 0
  : hybrid ? variant && crossVariant ? Math.ceil((variant.material_price_cents + crossVariant.material_price_cents) / 2) : null
  : variant?.material_price_cents ?? null
 const splitPrice = !tensionPending && Number.isFinite(centi(mains)) && Number.isFinite(centi(crossesActual)) && centi(mains) !== centi(crossesActual)
 const estimate = draftReady && service && stringPrice !== null && (!gripProductID || gripPrice !== null)
  ? service.labour_cents + stringPrice + (gripProductID ? gripPrice ?? 0 : 0) +
    (knots === 2 ? service.two_knots_cents ?? 0 : 0) +
    (splitPrice ? service.different_tensions_cents ?? 0 : 0) +
    (stencil ? service.stencil_cents ?? 0 : 0) +
    (express ? service.express_fee_cents ?? 0 : 0) +
    (service.prestretch_options?.[prestretch] ?? 0)
  : null
 const paymentValid=!!paymentOptions&&(paymentMethod==='online'?paymentOptions.online_available:paymentMethod==='cash'?paymentOptions.onsite_cash:paymentOptions.onsite_cash||paymentOptions.onsite_card)
 function quoteHint() {
  if (!draftReady) return 'Restoring your setup…'
  if (services?.length === 0) return 'This stringer has no bookable services.'
  if (!racketSport && services?.length) return 'Choose a sport to continue.'
  if (!config) return 'Waiting for booking options…'
  if (hybrid && ((!variant && !ownMains.trim()) || (!crossVariant && !ownCrosses.trim()))) {
   return 'Choose both the mains and crosses.'
  }
  if (hybrid && ((variant && !mainsAvailable) || (crossVariant && !crossesAvailable))) {
   return 'Wait for the string availability check or choose different strings.'
  }
  if (!hybrid && stringSource === 'stock' && !variant) return 'Choose a string or use the guided finder.'
  if (!tensionPending && (!tensionValid(mains) || !tensionValid(crossesActual))) return 'Check your tension settings.'
  if (!paymentOptions) return 'Waiting for payment options…'
  if (!paymentValid) return 'Choose a payment method above.'
  return 'Check your setup and extras before requesting a quote.'
 }
 const valid=draftReady&&paymentValid&&(stringSource!=='customer'||!!service?.own_strings)&&!!racketSport&&!!config&&(tensionPending||tensionValid(mains)&&tensionValid(crossesActual)&&Math.abs(centi(mains)-centi(crossesActual))*(tensionUnit==='lb'?0.45359237:1)<=config.max_difference_centi_kg)&&(!split||tensionPending||!!service?.different_tensions)&&config.allowed_knots.includes(knots)&&(!split||centi(mains)===centi(crossesActual)||!(racket?.different_tensions_require_four_knots??true)||knots===4)&&(!stencil||!!service?.stencil)&&(!prestretch||(stringSource==='stock'||hybrid)&&Object.hasOwn(service?.prestretch_options??{},String(prestretch))&&!!variant&&prestretch<=variant.max_prestretch_percent)&&(hybrid?hybridSupported&&knots===4&&(variant?mainsAvailable:stringSource==='customer'&&ownMains.trim().length>0&&ownMains.length<=160)&&(crossVariant?crossesAvailable:stringSource==='customer'&&ownCrosses.trim().length>0&&ownCrosses.length<=160)&&(!prestretch||!!crossVariant&&prestretch<=crossVariant.max_prestretch_percent):(stringSource==='customer'?!!service?.own_strings:!!variant))&&(!express||!!service?.express)&&(!gripProductID||gripPrice!==null)&&notes.length<=500&&racketDetails.length<=160
 function invalidate(){setAcceptance(undefined);setLegalReady(false);quoteSeq.current++;setQuoteBusy(false);setQuote(null);setFields({});setPendingPrice(false);setAttempt(null);removeBookingDraft(id);setOrderID('');setError('')}
 function modify(fn:()=>void){if(attempt){setError('An order submission may still be pending. Retry it before changing the setup.');return}invalidate();fn()}
 useEffect(()=>{let active=true;marketplaceAPI.me().then(()=>{if(active)setSignedIn(true)}).catch(()=>{if(active)setSignedIn(false)});return()=>{active=false}},[])
 useEffect(()=>{
  if(!signedIn)return
  let active=true
  marketplaceAPI.savedRackets().then(r=>{if(active&&Array.isArray(r))setSavedRackets(r)}).catch(()=>{})
  return()=>{active=false}
 },[signedIn])
 useEffect(()=>{let active=true;marketplaceAPI.paymentOptions(id).then(p=>{if(active){setPaymentOptions(p);if(!initialDraft&& !p.onsite_cash&&!p.onsite_card&&p.online_available)setPaymentMethod('online')}}).catch(e=>{if(active)setError(problem(e))});return()=>{active=false}},[id,refresh,initialDraft])
 useEffect(()=>{
  let active=true
  marketplaceAPI.stringer(id).then(data=>{if(active)setStringer(data)}).catch(()=>{})
  return()=>{active=false}
 },[id])
 useEffect(()=>{let active=true;marketplaceAPI.services(id).then(r=>{if(active)setServices(r.filter(s=>s.active))}).catch(e=>{if(active)setError(problem(e))});return()=>{active=false}},[id,refresh])
 useEffect(()=>{
  let active=true
  if(!racketID&&!bookingSport)return
  async function load(){
   try{
    const c=await marketplaceAPI.bookingConfiguration(id,racketID,bookingSport||undefined)
    if(!active)return
    setConfig(c)
    const saved=draft.current;draft.current=null
    if(saved&&saved.racketID===racketID&&(!saved.sport||saved.sport===bookingSport)){
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
       const [m,cross]=await Promise.all([saved.variantID?(restoredSource==='customer'?marketplaceAPI.strings({variant_id:saved.variantID,page_size:1}):marketplaceAPI.hybridInventory(id,racketID,'mains',{sport:c.service.sport,variant_id:saved.variantID,page_size:1})):Promise.resolve({data:[] as Variant[]}),saved.crossesVariantID?(restoredSource==='customer'?marketplaceAPI.strings({variant_id:saved.crossesVariantID,page_size:1}):marketplaceAPI.hybridInventory(id,racketID,'crosses',{sport:c.service.sport,variant_id:saved.crossesVariantID,page_size:1})):Promise.resolve({data:[] as Variant[]})])
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
 },[id,racketID,bookingSport,setupRevision])
 useEffect(()=>{
  if(!racketSport)return
  const controller=new AbortController(),seq=++inventorySeq.current
  const timer=setTimeout(()=>{
   marketplaceAPI.inventory(id,{sport:racketSport,individual_only:true,q:inventoryQuery,page,page_size:10},controller.signal).then(result=>{
    if(seq!==inventorySeq.current||controller.signal.aborted)return
    setVariants(result.data)
    setTotal(result.total)
    setInventoryResult({key:inventoryKey,failed:false})
   }).catch(()=>{
    if(seq===inventorySeq.current&&!controller.signal.aborted)setInventoryResult({key:inventoryKey,failed:true})
   })
  },300)
  return()=>{clearTimeout(timer);controller.abort()}
 },[id,racketSport,inventoryQuery,page,inventoryKey])
 useEffect(()=>()=>{quoteSeq.current++},[])
 useEffect(()=>{if(!draftReady||!config||orderID||attempt)return;saveBookingDraft(id,{savedAt:Date.now(),guided,tensionUnit,exactPounds:true,racketID,sport:racketSport||undefined,racketDetails,tensionPending,tensionBand,paymentMethod,variantID:variant?.id??'',variantModel:variant?.model??'',stringSource,hybrid,customerMains:ownMains,customerCrosses:ownCrosses,crossesVariantID:crossVariant?.id??'',mains,crosses,split,knots,gripProductID,gripLabel,express,stencil,prestretch,notes,attempt})},[id,draftReady,config,guided,tensionUnit,racketID,racketSport,racketDetails,tensionPending,tensionBand,paymentMethod,variant,stringSource,hybrid,crossVariant,ownMains,ownCrosses,mains,crosses,split,knots,gripProductID,gripLabel,express,stencil,prestretch,notes,attempt,orderID])
 const selected=variant
 const booking:Booking={payment_method:paymentMethod,...(hybrid?{hybrid:true,crosses_variant_id:crossVariant?.id,...(stringSource==='customer'?{customer_mains_description:ownMains,customer_crosses_description:ownCrosses}:{})}:{}),...(racketID?{racket_model_id:racketID}:{}),sport:racketSport||undefined,racket_details:racketDetails,tension_pending:tensionPending,...(tensionPending?{tension_band:tensionBand??'estimate'}:{}),string_source:stringSource,mains_tension_centi_kg:tensionPending||tensionUnit==='lb'?0:centi(mains),crosses_tension_centi_kg:tensionPending||tensionUnit==='lb'?0:centi(crossesActual),...(!tensionPending&&tensionUnit==='lb'?{mains_tension_centi_lb:centi(mains),crosses_tension_centi_lb:centi(crossesActual)}:{}),knots,...(gripProductID?{grip_product_id:gripProductID}:{}),express,stencil,prestretch_percent:prestretch,instructions:notes,...((stringSource==='stock'||hybrid)&&selected?{variant_id:selected.id}:{})}
 function storeDraft(pending=attempt){if(pending&&initialDraft?.attempt)return saveBookingDraft(id,{...initialDraft,savedAt:Date.now(),attempt:pending});return saveBookingDraft(id,{savedAt:Date.now(),guided,tensionUnit,exactPounds:true,racketID,sport:racketSport||undefined,racketDetails,tensionPending,tensionBand,paymentMethod,variantID:variant?.id??'',variantModel:variant?.model??'',stringSource,hybrid,customerMains:ownMains,customerCrosses:ownCrosses,crossesVariantID:crossVariant?.id??'',mains,crosses,split,knots,gripProductID,gripLabel,express,stencil,prestretch,notes,acceptance,attempt:pending})}
 function signIn(){if(!storeDraft()){setError('Your browser cannot save this setup. Allow session storage before signing in to keep your choices.');return}window.location.assign(signInURL(`/stringers/${id}/book`))}
 async function review(){
  if(!valid)return;const seq=++quoteSeq.current;setQuoteBusy(true);setError('');setFields({})
  try{const q=await marketplaceAPI.quote(id,booking);if(seq!==quoteSeq.current)return;setAcceptance(undefined);setLegalReady(false);setQuote(q);setPendingPrice(false)}
  catch(e){if(seq!==quoteSeq.current)return;setError(problem(e));if(e instanceof MarketplaceError){setFields(e.fields);if(e.code==='grip_out_of_stock'||e.code==='hybrid_out_of_stock'||e.fields.grip_product_id)setRefresh(n=>n+1);}if(e instanceof MarketplaceError&&e.status===401)signIn()}
  finally{if(seq===quoteSeq.current)setQuoteBusy(false)}
 }
 async function openCheckout(order:string){
  setOrderBusy(true);setError('')
  try{
    const checkout=await marketplaceAPI.orderPaymentCheckout(order)
    window.location.assign(checkout.url)
  }catch(e){
    setError(problem(e))
  }finally{
    setOrderBusy(false)
  }
 }
 async function submit(){
  if((!quote&&!attempt)||pendingPrice||orderBusy)return
  if(!attempt&&quote&&Date.parse(quote.expires_at)<=Date.now()){setError('This quote expired. Request a fresh quotation.');setQuote(null);setAttempt(null);return}
  const current=attempt??{quoteID:quote!.id,key:crypto.randomUUID()}
  storeDraft(current)
  setAttempt(current);setOrderBusy(true);setError('')
  try{
    const order=await marketplaceAPI.createOrder(current.quoteID,current.key,acceptance,paymentMethod)
    const online=order.payment_method==='online'
    setOrderID(order.id);setOnlineOrder(online);setAttempt(null);removeBookingDraft(id)
    if(online)await openCheckout(order.id)
  }
  catch(e){
    if(e instanceof MarketplaceError&&e.code==='profile_name_required'){
      if(storeDraft(null)){
        window.location.assign(profileSetupURL(`/stringers/${id}/book`,true))
        return
      }
      setAttempt(null)
      setError('Your browser cannot save this setup. Add your name under Account → Profile before booking.')
      return
    }
    setError(problem(e));if(e instanceof MarketplaceError&&['grip_out_of_stock','hybrid_out_of_stock'].includes(e.code))setRefresh(n=>n+1);if(e instanceof MarketplaceError&&e.status===401)setSignedIn(false);if(e instanceof MarketplaceError&&[404,409,422].includes(e.status)){setPendingPrice(e.status===409&&e.code!=='booking_limit_reached');setQuote(null);setAttempt(null);removeBookingDraft(id);if(e.status!==409){setConfig(null);setServices(null);setRacketID('');setSport('');setTensionUnit('kg');setVariant(null);setVariants([]);setRefresh(n=>n+1)}}}
  finally{setOrderBusy(false)}
 }
 function reuseRacket(orderID:string){
  const b=savedRackets.find(r=>r.order_id===orderID)?.configuration
  if(!b?.sport||!services?.some(s=>s.sport===b.sport))return
  const tension=savedTension(b)
  const restored:BookingDraft={
   savedAt:Date.now(), racketID:'', sport:b.sport, racketDetails:b.racket_details,
   tensionUnit:tension.unit, exactPounds:true, tensionPending:!!b.tension_pending, tensionBand:b.tension_band,
   variantID:b.variant_id??'', variantModel:b.string_snapshot, stringSource:b.string_source,
   hybrid:b.hybrid, crossesVariantID:b.crosses_variant_id,
   customerMains:b.customer_mains_description, customerCrosses:b.customer_crosses_description,
   mains:String(tension.mains), crosses:String(tension.crosses), split:tension.mains!==tension.crosses,
   knots:b.knots as 2|4, stencil:!!b.stencil, prestretch:b.prestretch_percent??0,
   notes:b.instructions??'', express:!!b.express, gripProductID:b.grip_product_id, gripLabel:b.grip_snapshot, attempt:null,
  }
  modify(()=>{
   draft.current=restored
   setSavedReference(b)
   setDraftReady(false);setRacketID('');setSport(b.sport??'');setRacketDetails(b.racket_details??'')
   setTensionUnit(tension.unit);setTensionPending(!!b.tension_pending);setTensionBand(b.tension_band)
   setVariant(null);setCrossVariant(null);setMainsAvailable(false);setCrossesAvailable(false)
   setGripProductID(restored.gripProductID??'');setGripLabel(restored.gripLabel??'');setGripPrice(restored.gripProductID?null:0);setGuided(false);setFinding(false)
   setInventoryQuery('');setPage(1);setSetupNotice('')
   setSetupRevision(n=>n+1)
  })
 }
 const offeredRackets=savedRackets.filter(r=>services?.some(s=>s.sport===r.configuration.sport))
 function useSetup(rec:components['schemas']['Recommendation']){
  const setup=rec.setup;if(!setup||(setup.booking.racket_model_id??'')!==racketID||!!setup.booking.sport&&setup.booking.sport!==racketSport||attempt)return
  const b=setup.booking
  modify(()=>{setStringSource('stock');setVariant(rec.variant);setCrossVariant(setup.crosses_variant??null);setHybrid(!!b.hybrid);setMainsAvailable(false);setCrossesAvailable(false);setOwnMains('');setOwnCrosses('');setKnots(b.knots as 2|4);setPrestretch(b.prestretch_percent??0);setStencil(!!b.stencil);setExpress(!!b.express);setGripProductID('');setGripLabel('');setGripPrice(0);setGuided(true);setSetupNotice('')})
  requestAnimationFrame(()=>guidedConfirmation.current?.focus())
 }
 return <div className="booking-preview"><MarketplaceHeader backHref={`/stringers/${id}`} backLabel="Back to profile"/><main id="main-content" tabIndex={-1} className="marketplace-container booking-content">
  <div className="booking-heading"><h1>Choose your <em>setup.</em></h1></div>
  {setupNotice&&<p className="booking-banner" role="status">{setupNotice}</p>}
  {error&&<div className="booking-banner" role="alert">{error} <button type="button" onClick={()=>{setError('');setRefresh(n=>n+1)}}>Retry loading</button></div>}
  {Object.keys(fields).length>0&&<div role="alert" className="booking-banner">{Object.entries(fields).map(([k,v])=><p key={k}>{k}: {v}</p>)}</div>}
  <div className="booking-grid"><section className="booking-form">{!draftReady&&<p role="status">Restoring your saved setup…</p>}<fieldset className="booking-setup" aria-label="Racket configuration" disabled={!draftReady||!!attempt}>
   {(offeredRackets.length>0||services&&services.length>1)&&<div className="booking-step"><div><h2>Your racket</h2>{offeredRackets.length>0&&<label className="booking-field">Use a saved racket<select value="" onChange={e=>reuseRacket(e.target.value)}><option value="">Choose a previous setup</option>{offeredRackets.map(r=><option key={r.order_id} value={r.order_id}>{r.configuration.racket_details} ({r.configuration.sport})</option>)}</select></label>}{services&&services.length>1&&<label className="booking-field">Sport<select value={racketSport??''} onChange={e=>modify(()=>{draft.current=null;setFinding(false);setGuided(false);setRacketID('');setSport(e.target.value as Sport|'');setSavedReference(null);setTensionUnit('kg');setConfig(null);setVariant(null);setCrossVariant(null);setHybrid(false);setVariants([]);setMainsAvailable(false);setCrossesAvailable(false);setTensionPending(true);setTensionBand(undefined);setPage(1);setInventoryQuery('')})}><option value="">Choose your sport</option>{services.map(s=><option value={s.sport} key={s.sport}>{s.sport[0].toUpperCase()+s.sport.slice(1)}</option>)}</select></label>}</div></div>}
   {racketSport&&config&&<>
   <div className="booking-step"><div><h2>{stringSource==='customer'?'Your supplied strings':'Strings from this stringer'}</h2><GuidedStringFinder tensionBand={tensionPending?tensionBand??'estimate':undefined} key={`${id}:${racketID}:${racketSport}:${setupRevision}`} stringerID={id} racketID={racketID} sport={racketSport} disabled={!draftReady||!!attempt} onOpenChange={setFinding} choose={useSetup}/>{guided&&<div className="booking-guided-ready" ref={guidedConfirmation} tabIndex={-1}>
    <strong>Your setup is selected.</strong><p>Your stringer will confirm it before work starts.</p>
   </div>}<details className="booking-manual" open={!guided&&!finding&&(!compact||!!savedReference||!!initialDraft?.variantID||initialDraft?.stringSource==='customer')}><summary>Choose strings myself</summary>{service?.own_strings&&<label className="finder-check"><input type="checkbox" checked={stringSource==='customer'} onChange={e=>modify(()=>{setGuided(false);setStringSource(e.target.checked?'customer':'stock');setVariant(null);setCrossVariant(null);setOwnMains('');setOwnCrosses('');setMainsAvailable(false);setCrossesAvailable(false);setPrestretch(0)})}/> I'll provide my own strings</label>}
   {<><fieldset className="booking-fieldset"><legend>String setup</legend><label className="finder-check"><input type="radio" name="string-setup" checked={!hybrid} onChange={()=>modify(()=>{setGuided(false);setHybrid(false);setCrossVariant(null);setPrestretch(0)})}/> Full bed — one string throughout</label>{hybridSupported&&<label className="finder-check"><input type="radio" name="string-setup" checked={hybrid} disabled={!hybridSupported} onChange={()=>modify(()=>{setHybrid(true);setVariant(null);setCrossVariant(null);setMainsAvailable(false);setCrossesAvailable(false);setKnots(4);setPrestretch(0);setGuided(false)})}/> Hybrid — choose mains and crosses</label>}</fieldset>{hybrid&&<><p>Choose the mains and crosses below. Stock material uses half of each full-bed price; customer-supplied material has no charge. Pre-stretch, if selected, applies to both strings.</p><HybridPicker stringerID={id} racketID={racketID} component="mains" source={stringSource} sport={racketSport!} description={ownMains} describe={text=>modify(()=>{setOwnMains(text);setVariant(null);setPrestretch(0)})} selected={variant} locked={!!attempt} revision={refresh} choose={v=>modify(()=>{setVariant(v);setOwnMains('');if(variant?.id!==v?.id)setMainsAvailable(false);setPrestretch(0)})} onAvailability={setMainsAvailable}/><HybridPicker stringerID={id} racketID={racketID} component="crosses" mainsPriceCents={variant?.material_price_cents} source={stringSource} sport={racketSport!} description={ownCrosses} describe={text=>modify(()=>{setOwnCrosses(text);setCrossVariant(null);setPrestretch(0)})} selected={crossVariant} locked={!!attempt} revision={refresh} choose={v=>modify(()=>{setCrossVariant(v);setOwnCrosses('');if(crossVariant?.id!==v?.id)setCrossesAvailable(false);setPrestretch(0)})} onAvailability={setCrossesAvailable}/></>}{!hybrid&&stringSource==='stock'&&<><label className="booking-field">Search strings<input type="search" value={inventoryQuery} onChange={e=>{setInventoryQuery(e.target.value);setPage(1)}} placeholder="Brand, name, colour or gauge"/></label><div className="booking-choice-list">{!inventoryLoading&&!inventoryResult?.failed&&variants.map(v=><button key={v.id} type="button" className={selected?.id===v.id?'booking-option chosen':'booking-option'} aria-pressed={selected?.id===v.id} onClick={()=>modify(()=>{setGuided(false);setVariant(v);setPrestretch(0)})}><span className="string-choice-photo"><StringPhoto variant={v}/><span><strong>{v.brand} {v.model}</strong><small>{v.material.replaceAll('_',' ')} · {v.gauge_mm} mm · {v.colour}</small></span></span><span>{eur(v.material_price_cents)}</span></button>)}</div> {inventoryLoading?<p role="status">Loading strings…</p>:inventoryResult?.failed?<div role="alert"><p>Couldn’t load strings.</p><button type="button" onClick={()=>setRefresh(n=>n+1)}>Retry strings</button></div>:variants.length===0&&(inventoryQuery.trim()?<div role="status"><p>No strings match this search.</p><button type="button" onClick={()=>{setInventoryQuery('');setPage(1)}}>Clear search</button></div>:<p role="status">No strings are currently listed for this sport.</p>)}{selected&&<p className="selected-string">Selected: {selected.brand} {selected.model}</p>}{!inventoryLoading&&!inventoryResult?.failed&&total>10&&<div className="catalogue-pages"><button type="button" disabled={page===1} onClick={()=>setPage(p=>p-1)}>Previous</button><span>Page {page} / {Math.ceil(total/10)}</span><button type="button" disabled={page*10>=total} onClick={()=>setPage(p=>p+1)}>Next</button></div>}
   </>}</>}</details>{(guided||selected||stringSource==='customer')&&<a className="booking-review-link" href="#booking-summary-title">Review setup</a>}</div></div>

   <div className="booking-step"><div className="booking-instructions"><h2>Stringing instructions</h2><label className="booking-field booking-racket-details">Racket details (optional)<input maxLength={160} value={racketDetails} onChange={e=>modify(()=>setRacketDetails(e.target.value))} placeholder="e.g. Family junior racket or Wilson Blade"/></label><fieldset className="booking-fieldset"><legend>Tension preference</legend><label className="booking-field">Tension guide<select value={tensionPending?tensionBand??'estimate':''} onChange={e=>modify(()=>{setTensionPending(true);setTensionBand(e.target.value as Booking['tension_band']);setSplit(false)})}>{!tensionPending&&<option value="" hidden>Exact tension entered</option>}<option value="lower">Lower tension (softer feeling)</option><option value="middle">Mid range</option><option value="higher">Higher tension (firmer feeling)</option><option value="estimate">Please estimate my current tension</option></select></label>{tensionPending&&<p>Your stringer will check your racket and help choose the tension.</p>}</fieldset><label className="finder-check"><input type="checkbox" checked={!tensionPending} onChange={()=>modify(()=>{setTensionPending(!tensionPending);setTensionBand(undefined);if(tensionUnit==='kg'){if(!tensionValid(mains))setMains(String(config.min_tension_centi_kg/100));if(!tensionValid(crosses))setCrosses(String(config.min_tension_centi_kg/100))}})}/> I know my tension</label><details className="booking-technical" open={(!compact&&!guided&&!finding)||!tensionPending}><summary>Adjust stringing settings (optional)</summary><p>{config.notice}</p><p>Allowed tension: {config.min_tension_centi_kg/100}–{config.max_tension_centi_kg/100} kg in {config.step_centi_kg/100} kg increments.</p>{!tensionPending&&<><label className="booking-field">Tension unit<select value={tensionUnit} onChange={e=>modify(()=>{setTensionUnit(e.target.value as TensionUnit);setMains('');setCrosses('')})}><option value="kg">Kilograms (kg)</option><option value="lb">Pounds (lb)</option></select></label><TensionInput label="Mains tension" value={mains} unit={tensionUnit} min={config.min_tension_centi_kg} max={config.max_tension_centi_kg} step={config.step_centi_kg} change={v=>modify(()=>setMains(v))}/>
   {service?.different_tensions&&<label className="finder-check"><input type="checkbox" checked={split} onChange={e=>modify(()=>setSplit(e.target.checked))}/> Use separate crosses tension</label>}{split&&<TensionInput label="Crosses tension" value={crosses} unit={tensionUnit} min={config.min_tension_centi_kg} max={config.max_tension_centi_kg} step={config.step_centi_kg} change={v=>modify(()=>setCrosses(v))}/>}
   </>}{!hybrid&&<fieldset className="booking-fieldset"><legend>Knots</legend><div className="booking-choice-grid">{config.allowed_knots.filter(k=>k===2||k===4).map(k=><button type="button" key={k} className={knots===k?'booking-option chosen':'booking-option'} aria-pressed={knots===k} onClick={()=>modify(()=>setKnots(k as 2|4))}>{k} knots</button>)}</div>{service?.two_knots&&!config.allowed_knots.includes(2)&&<p>Four-knot stringing is the available method for this booking.</p>}<p>These are the stringer’s offered methods. They will confirm the pattern for your racket before work. Different tensions may require four knots.</p></fieldset>}
   {service?.stencil&&<label className="finder-check"><input type="checkbox" checked={stencil} onChange={e=>modify(()=>setStencil(e.target.checked))}/> Add stencil ({eur(service.stencil_cents)})</label>}
   {prestretchOptions.length>0&&<label className="booking-field">Pre-stretch<select value={prestretch} onChange={e=>modify(()=>setPrestretch(Number(e.target.value)))}><option value={0}>None</option>{prestretchOptions.map(p=><option value={p} key={p}>{p}%</option>)}</select></label>}
   </details>
   {service?.express&&<fieldset className="booking-fieldset booking-turnaround"><legend>Turnaround</legend><label className="finder-check"><input type="radio" name="turnaround" checked={!express} onChange={()=>modify(()=>setExpress(false))}/> Standard turnaround</label><label className="finder-check"><input type="radio" name="turnaround" checked={express} onChange={()=>modify(()=>setExpress(true))}/> Express turnaround (+{eur(service.express_fee_cents)})</label><p>{service.express_turnaround}</p><p>Your stringer confirms the ready estimate after accepting your booking. A requested collection date does not select express automatically.</p></fieldset>}

   <GripPicker onOffersChange={setGripsOffered} revision={refresh} stringerID={id} selectedID={gripProductID} selectedLabel={gripLabel} locked={!!attempt} onPrice={setGripPrice} choose={g=>modify(()=>{setGripProductID(g?.id??'');setGripLabel(g?`${g.brand} ${g.model} · ${g.kind.replaceAll('_',' ')} · ${g.colour}`:'');setGripPrice(g?.fitted_price_cents??0)})}/>
   <label className="booking-field">Additional instructions<textarea maxLength={500} rows={3} value={notes} onChange={e=>modify(()=>setNotes(e.target.value))}/></label></div></div></>}
  </fieldset></section>
  <aside className="booking-summary" aria-labelledby="booking-summary-title">
   <h2 id="booking-summary-title" tabIndex={-1}>Review your setup</h2>
   {stringer&&<p className="booking-provider"><strong>{stringer.name}</strong>{area&&<span>{area}</span>}</p>}
   {!quote&&!orderID&&<div aria-live="polite" aria-atomic="true">
    {estimate!==null?<div className="booking-total"><span>Estimated total</span><strong>{eur(estimate)}</strong></div>:labour!==null&&<p className="booking-price-note">Labour {service?'':'from '}<strong>{eur(labour)}</strong>. Strings and extras may cost more.</p>}
   </div>}
   {stringer&&!orderID&&<p className="booking-price-note">Confirm drop-off and collection with your stringer.</p>}
   <dl className="booking-summary-setup">
    <div className="booking-summary-line"><dt>Racket</dt><dd>{racketDetails||racketSport||'Not selected'}</dd></div>
    <div className="booking-summary-line"><dt>String</dt><dd>{hybrid?'Hybrid':stringSource==='customer'?'Own strings':selected?`${selected.brand} ${selected.model}`:'Not selected'}</dd></div>
    {hybrid&&<>
     <div className="booking-summary-line"><dt>Mains</dt><dd>{variant?`${variant.brand} ${variant.model}`:ownMains||'Choose mains'}</dd></div>
     <div className="booking-summary-line"><dt>Crosses</dt><dd>{crossVariant?`${crossVariant.brand} ${crossVariant.model}`:ownCrosses||'Choose crosses'}</dd></div>
    </>}
    {(gripsOffered||gripProductID)&&<div className="booking-summary-line"><dt>Grip</dt><dd>{gripProductID?gripLabel:'No new grip'}</dd></div>}
    {(service?.express||express)&&<div className="booking-summary-line"><dt>Turnaround</dt><dd>{express?'Express':'Standard'}</dd></div>}
    <div className="booking-summary-line"><dt>Tension</dt><dd>{tensionPending?(tensionBandLabel(tensionBand)||'To agree with your stringer'):mains&&crossesActual?`${mains} / ${crossesActual} ${tensionUnit}`:'Enter tension'}</dd></div>
   </dl>

   <fieldset className="booking-payment" disabled={!!attempt||!!orderID}>
    <legend>Payment method</legend>
    {!paymentOptions&&<p role="status">Loading payment options…</p>}
    {onsiteTenders&&(
     <label className="booking-payment-option">
      <input type="radio" name="payment" value="onsite" aria-labelledby="booking-pay-onsite" aria-describedby="booking-pay-onsite-detail"
       checked={paymentValid&&paymentMethod!=='online'} onChange={()=>modify(()=>setPaymentMethod('onsite'))}/>
      <span>
       <strong id="booking-pay-onsite">Pay onsite</strong>
       <small id="booking-pay-onsite-detail">Pay your stringer in person by {onsiteTenders}.</small>
      </span>
     </label>
    )}
    {paymentOptions?.online_available&&(
     <label className="booking-payment-option">
      <input type="radio" name="payment" value="online" aria-labelledby="booking-pay-online" aria-describedby="booking-pay-online-detail"
       checked={paymentMethod==='online'} onChange={()=>modify(()=>setPaymentMethod('online'))}/>
      <span>
       <strong id="booking-pay-online">Pay online</strong>
       <small id="booking-pay-online-detail">Continue directly to secure Stripe checkout.</small>
      </span>
     </label>
    )}
    {paymentOptions&&!paymentValid&&!attempt&&(
     <p role="alert">{onsiteTenders||paymentOptions.online_available?'Choose an available payment method.':'No payment methods are available right now.'}</p>
    )}
   </fieldset>

   {!valid&&!quote&&!attempt&&!orderID&&<p id="booking-quote-hint" role="status" className="booking-price-note">{quoteHint()}</p>}
   {!orderID&&(
    <button aria-describedby={!valid&&!quote&&!attempt?'booking-quote-hint':undefined} type="button" className={quote?'booking-secondary-button':'booking-review-button'} disabled={!valid||quoteBusy||!!attempt} onClick={review}>
     {quoteBusy?'Requesting quote…':quote?'Refresh quote':'Request quote'}
    </button>
   )}
   {!signedIn&&!orderID&&<button type="button" className="booking-secondary-button" disabled={!draftReady} onClick={signIn}>Sign in and keep this setup</button>}
   {quote&&!orderID&&<>
    <div className="booking-reviewed">
     <h3>Your quote</h3>
     {quote.configuration?.grip_snapshot&&<p className="booking-grip-quoted">Quoted grip: {quote.configuration.grip_snapshot} (fitting included)</p>}
     {quote.configuration?.express&&<p className="booking-express-quoted">Quoted express turnaround: {quote.configuration.express_turnaround_snapshot}</p>}
     {quote.breakdown.map(line=><div className="booking-summary-line" key={line.code}><span>{line.label}</span><strong>{eur(line.amount_cents)}</strong></div>)}
     <div className="booking-total"><span>Total</span><strong>{eur(quote.total_cents)}</strong></div>
     <p>Expires {new Date(quote.expires_at).toLocaleString('en-IE')}</p>
    </div>
    <CheckoutConsent key={quote.id} id={id} locked={!!attempt} changed={setAcceptance} ready={setLegalReady} labelChanged={setLegalSubmitLabel}/>
    <button type="button" className="booking-review-button" disabled={orderBusy||quoteBusy||pendingPrice||(!attempt&&!legalReady)} onClick={submit}>
     {orderBusy?'Submitting…':attempt?'Retry same order request':legalSubmitLabel}
    </button>
   </>}
   {!quote&&attempt&&<button type="button" className="booking-review-button" disabled={orderBusy} onClick={submit}>{orderBusy?'Checking…':'Retry pending booking'}</button>}
   {pendingPrice&&<p role="alert">The price or setup changed. Request a new quote before booking.</p>}
   {orderID&&onlineOrder&&<div className="booking-reviewed">
    <p role="status">{orderBusy?'Opening secure checkout…':'Payment is still needed for this booking.'}</p>
    {!orderBusy&&<button type="button" className="booking-review-button" onClick={()=>void openCheckout(orderID)}>Continue to payment</button>}
   </div>}
   {orderID&&!onlineOrder&&<div role="status" className="booking-reviewed">
    <strong>Booking created — pay onsite to your stringer.</strong>
    <p>Payment status: unpaid</p>
    <a href={`/orders/${orderID}`}>View order</a>
    <p><a href={`/orders/${orderID}/confirmation`}>View and download booking confirmation</a></p>
   </div>}
  </aside></div></main><MarketplaceFooter/></div>
}
