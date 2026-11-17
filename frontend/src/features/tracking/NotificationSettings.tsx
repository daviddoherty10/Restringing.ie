import { useCallback, useState } from 'react'
import { marketplaceAPI } from '../../lib/marketplace-api'
import { errorMessage, useRemote } from '../management/use-remote'
export default function NotificationSettings(){
 const [revision,setRevision]=useState(0),[busy,setBusy]=useState(false),[error,setError]=useState('')
 const load=useCallback(()=>marketplaceAPI.notificationPreferences(),[]),r=useRemote(`sms:${revision}`,load)
 async function change(enabled:boolean){if(busy||!r.data)return;const previous=r.data;setBusy(true);setError('');r.updateData(()=>({sms_enabled:enabled}));try{const saved=await marketplaceAPI.saveNotificationPreferences(enabled);r.updateData(()=>saved)}catch(e){r.updateData(()=>previous);setError(errorMessage(e))}finally{setBusy(false)}}
 return <section aria-label="SMS notification settings"><h2>SMS order alerts</h2><p>Receive order updates and new-message alerts at your verified phone number. Messages link to your signed-in order and contain no conversation text. You can turn these off here. Delivery is available only when the service is configured, for Irish and UK numbers.</p>{r.data&&<label><input type="checkbox" checked={r.data.sms_enabled} disabled={busy} onChange={e=>change(e.target.checked)}/>Send me SMS order alerts</label>}{!r.data&&!r.error&&<p role="status">Loading preferences…</p>}{(error||r.error)&&<p role="alert">{error||r.error}<button onClick={()=>setRevision(n=>n+1)}>Retry preferences</button></p>}</section>
}
