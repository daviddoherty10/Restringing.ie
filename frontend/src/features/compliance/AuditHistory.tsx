import {useEffect,useState} from 'react'
import {marketplaceAPI} from '../../lib/marketplace-api'
import type {components} from '../../lib/marketplace-contract'
import {errorMessage} from '../management/use-remote'
export default function AuditHistory({id,kind}:{id:string;kind:'privacy_request'|'trader'|'case'}){
 const [rows,setRows]=useState<components['schemas']['ComplianceHistory']>([]),[page,setPage]=useState(1),[error,setError]=useState('')
 useEffect(()=>{let active=true;marketplaceAPI.complianceHistory(id,kind,page).then(v=>{if(active)setRows(v)}).catch(e=>{if(active)setError(errorMessage(e))});return()=>{active=false}},[id,kind,page])
 return <section><h3>Private decision history</h3>{error&&<p role="alert">{error}</p>}{rows.map(v=><article key={v.id}><p>{new Date(v.created_at).toLocaleString()} · {v.action} · Administrator {v.actor_id||'Public submission'}</p><p>{v.reason}</p><details><summary>Recorded review details</summary><pre>{JSON.stringify(v.details,null,2)}</pre></details></article>)}<button type="button" disabled={page===1} onClick={()=>setPage(page-1)}>Previous decisions</button><button type="button" disabled={rows.length<50} onClick={()=>setPage(page+1)}>More decisions</button></section>
}
