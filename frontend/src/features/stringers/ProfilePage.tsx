import { useEffect, useState } from 'react'
import { marketplaceAPI, MarketplaceError, type Stringer, type Service } from '../../lib/marketplace-api'
import { MarketplaceHeader, MarketplaceFooter } from '../../components/layout/MarketplaceLayout'
import ListingContact from './ListingContact'
import StringerReviews from '../reviews/StringerReviews'
import './ProfilePage.css'
function ListingImage({id,slot,alt}:{id:string;slot:string;alt:string}){const [failed,setFailed]=useState(false);return failed?null:<img src={marketplaceAPI.businessImageURL(id,slot)} alt={alt} onError={()=>setFailed(true)} className={`profile-upload-${slot}`}/>}
const money=(c:number)=>new Intl.NumberFormat('en-IE',{style:'currency',currency:'EUR'}).format(c/100)
export default function ProfilePage({id}:{id:string}){
  const [data,setData]=useState<Stringer|null>(null),[services,setServices]=useState<Service[]>([])
  const [error,setError]=useState(''),[loading,setLoading]=useState(true),[retry,setRetry]=useState(0)
  useEffect(()=>{let active=true;Promise.all([marketplaceAPI.stringer(id),marketplaceAPI.services(id)]).then(([profile,offers])=>{if(active){setData(profile);setServices(offers);setError('')}}).catch(e=>{if(active)setError(e instanceof MarketplaceError?e.message:'Profile is unavailable.')}).finally(()=>{if(active)setLoading(false)});return()=>{active=false}},[id,retry])
  const activeServices = services.filter(service => service.active)
  const canBook = !!data?.booking_available && activeServices.length > 0

  return <div className="profile-page">
    <MarketplaceHeader backHref="/search" backLabel="All stringers"/>
    <main id="main-content" tabIndex={-1} className="marketplace-container">
      <nav className="profile-breadcrumb"><a href="/">Home</a> / <a href="/search">Stringers</a> / Profile</nav>
      {loading ? <p role="status">Loading profile…</p> : error ? (
        <div className="marketplace-empty" role="alert">
          <h2>Unable to load profile</h2>
          <p>{error}</p>
          <button type="button" onClick={()=>{setLoading(true);setRetry(n=>n+1)}}>Retry</button>
        </div>
      ) : data && <>
        <div className="profile-hero">
          <div className="profile-overview">
            <h1>{data.name}</h1>
            <p>{canBook ? 'Accepting bookings' : 'Currently not accepting bookings'}</p>
            {canBook && <a className="profile-booking-preview-link" href={`/stringers/${id}/book`}>Book a restring</a>}
            <ListingImage key={`${id}:profile`} id={id} slot="profile" alt={`${data.name} profile`}/>
            <h2>Location and contact</h2>
            <ListingContact stringer={data}/>
            {data.public_identity && <p>Contact the stringer to confirm drop-off and collection arrangements.</p>}
            <p className="profile-about">{data.description}</p>
          </div>
          <div className="profile-banner">
            <ListingImage key={`${id}:banner`} id={id} slot="banner" alt=""/>
            <span aria-hidden="true">{data.name.slice(0,2).toUpperCase()}</span>
          </div>
        </div>
        <div className="profile-layout">
          <section className="profile-services">
            <h2>Racket restringing</h2>
            {activeServices.length ? activeServices.map(service => (
              <div className="profile-service-card" key={service.id}>
                <div>
                  <h3>{service.sport.charAt(0).toUpperCase()+service.sport.slice(1)}</h3>
                  <p>Service labour price</p>
                  {service.express && <p className="profile-express">Express available: +{money(service.express_fee_cents)} · {service.express_turnaround}</p>}
                </div>
                <strong>{money(service.labour_cents)}</strong>
              </div>
            )) : <p>No active services are listed at present.</p>}
          </section>
          <p className="profile-listing-report"><a href={`/account/complaints?business=${id}`}>Report a listing issue</a></p>
        </div>
        {data.public_identity && <section aria-label="Trader identity">
          <h2>Trader identity</h2>
          <p>{data.public_identity.name} · {data.public_identity.trader_status}</p>
        </section>}
        <StringerReviews key={id} business={id}/>
      </>}
    </main>
    <MarketplaceFooter/>
  </div>
}
