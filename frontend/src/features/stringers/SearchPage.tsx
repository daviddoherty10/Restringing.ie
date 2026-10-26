import { useEffect, useRef, useState, type FormEvent } from 'react'
import { marketplaceAPI, MarketplaceError, type Stringer } from '../../lib/marketplace-api'
import { MarketplaceHeader, MarketplaceFooter } from '../../components/layout/MarketplaceLayout'
import './SearchPage.css'
type Sport = '' | 'tennis' | 'badminton' | 'squash'
export default function SearchPage() {
  const initial = new URLSearchParams(window.location.search).get('location') ?? ''
  const [input,setInput] = useState(initial)
  const [q,setQ] = useState(initial)
  const [county,setCounty] = useState('')
  const [sport,setSport] = useState<Sport>('')
  const [page,setPage] = useState(1)
  const [items,setItems] = useState<Stringer[]>([])
  const [total,setTotal] = useState(0)
  const [loading,setLoading] = useState(true)
  const [error,setError] = useState('')
  const [retry,setRetry] = useState(0)
  const seq = useRef(0)
  useEffect(()=>{ const timer = window.setTimeout(()=>{setQ(input.trim());setPage(1)},350);return ()=>clearTimeout(timer)},[input])
  useEffect(()=>{
    const controller = new AbortController()
    const current = ++seq.current
    marketplaceAPI.stringers({q,county,sport:sport||undefined,page,page_size:12},controller.signal)
      .then(result=>{if(seq.current!==current)return;setItems(result.data);setTotal(result.total);setError('')})
      .catch(err=>{if(seq.current!==current||controller.signal.aborted)return;setError(err instanceof MarketplaceError?err.message:'Marketplace is currently unavailable.');setItems([])})
      .finally(()=>{if(seq.current===current)setLoading(false)})
    return ()=>{controller.abort()}
  },[q,county,sport,page,retry])
  function submit(e:FormEvent){e.preventDefault();setQ(input.trim());setPage(1)}
  return <div className="marketplace"><MarketplaceHeader/><main>
    <section className="marketplace-intro"><div className="marketplace-container"><p className="marketplace-kicker">THE RESTRINGING MARKETPLACE</p><h1>Find your <em>perfect stringer.</em></h1><p className="marketplace-subtitle">Explore approved racket stringers offering real services.</p>
    <form className="marketplace-search" onSubmit={submit}><label htmlFor="marketplace-location">SEARCH BUSINESSES</label><div className="marketplace-search-row"><input id="marketplace-location" type="search" placeholder="Business name or area" value={input} onChange={e=>{setLoading(true);setInput(e.target.value)}}/><button type="submit">Search →</button></div></form></div></section>
    <section className="marketplace-results"><div className="marketplace-container"><div className="marketplace-toolbar"><div><span className="marketplace-results-count">{!loading&&!error?`${total} businesses`:'Live marketplace'}</span><h2>Explore stringers</h2></div><label className="marketplace-sort">County <input aria-label="Filter by county" value={county} onChange={e=>{setCounty(e.target.value);setPage(1);setLoading(true)}} placeholder="e.g. Dublin"/></label></div>
    <div className="marketplace-filters" aria-label="Racket sport">{(['','tennis','badminton','squash'] as Sport[]).map(s=><button type="button" key={s} className={sport===s?'marketplace-filter active':'marketplace-filter'} aria-pressed={sport===s} onClick={()=>{setSport(s);setPage(1);setLoading(true)}}>{s||'All rackets'}</button>)}</div>
    {loading?<div role="status" className="marketplace-empty">Loading approved stringers…</div>:error?<div role="alert" className="marketplace-empty"><h3>Marketplace unavailable</h3><p>{error}</p><button onClick={()=>{setLoading(true);setRetry(n=>n+1)}}>Retry</button></div>:items.length===0?<div className="marketplace-empty"><h3>No stringers found</h3><p>There are no approved businesses matching these filters yet.</p></div>:<div className="marketplace-grid">{items.map(item=><article className="marketplace-card" key={item.id}><div className="marketplace-card-banner"><span className="marketplace-card-monogram">{item.name.slice(0,2).toUpperCase()}</span></div><div className="marketplace-card-body"><h3>{item.name}</h3><p>{[item.area,item.county].filter(Boolean).join(', ')}</p><p>{item.description}</p><div className="marketplace-tags"><span>{item.accepting_orders?'Accepting enquiries':'Not accepting orders'}</span></div><a className="marketplace-profile-link" href={`/stringers/${item.id}`}>View profile ↗</a></div></article>)}</div>}
    {!loading&&!error&&total>12&&<div className="catalogue-pages"><button disabled={page===1} onClick={()=>{setPage(p=>p-1);setLoading(true)}}>Previous</button><span>Page {page} of {Math.ceil(total/12)}</span><button disabled={page*12>=total} onClick={()=>{setPage(p=>p+1);setLoading(true)}}>Next</button></div>}
    </div></section></main><MarketplaceFooter/></div>
}
