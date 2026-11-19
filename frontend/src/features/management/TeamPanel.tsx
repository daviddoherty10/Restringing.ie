import { useCallback, useState, type FormEvent } from 'react'
import { marketplaceAPI, type ManagementProfile } from '../../lib/marketplace-api'
import { Feedback, LoadMessage } from './dashboard-shared'
import { errorMessage, useRemote } from './use-remote'
export function StaffInvitations({ accepted }: { accepted: () => void }) {
 const [revision,setRevision]=useState(0),[busy,setBusy]=useState(false),[error,setError]=useState('')
 const load=useCallback(()=>marketplaceAPI.staffInvitations(),[]),r=useRemote(`invites:${revision}`,load)
 async function accept(id:string){if(busy)return;setBusy(true);setError('');try{await marketplaceAPI.acceptStaffInvitation(id);setRevision(n=>n+1);accepted()}catch(e){setError(errorMessage(e))}finally{setBusy(false)}}
 return <section className="manage-panel" aria-label="Staff invitations"><h2>Staff invitations</h2><LoadMessage {...r} retry={()=>setRevision(n=>n+1)}/>{r.data?.map(i=><div key={i.id}><p>{i.business_name} has invited you to join as staff. Expires {new Date(i.expires_at).toLocaleDateString('en-IE')}.</p><button disabled={busy} onClick={()=>accept(i.id)}>Accept invitation to {i.business_name}</button></div>)}{r.data?.length===0&&<p>No pending invitations for your verified phone number.</p>}<Feedback error={error} message=""/></section>
}
export default function TeamPanel({business}:{business:ManagementProfile}){
 const [revision,setRevision]=useState(0),[phone,setPhone]=useState(''),[busy,setBusy]=useState(false),[error,setError]=useState(''),[message,setMessage]=useState('')
 const load=useCallback(()=>marketplaceAPI.team(business.id),[business.id]),r=useRemote(`${business.id}:${revision}`,load)
 async function action(run:()=>Promise<unknown>,notice:string){if(busy)return;setBusy(true);setError('');setMessage('');try{await run();setRevision(n=>n+1);setMessage(notice)}catch(e){setError(errorMessage(e))}finally{setBusy(false)}}
 function invite(e:FormEvent){e.preventDefault();void action(()=>marketplaceAPI.inviteStaff(business.id,phone),'Invitation saved. Ask your colleague to sign in and open Team invitations or the Stringer dashboard from their Account menu to accept it. No invitation SMS was sent.')}
 return <section className="manage-panel"><h2>Team</h2><p>Invite staff using their international phone number. They must verify that number and accept before gaining access. Invitations expire after seven days.</p><form onSubmit={invite}><fieldset disabled={busy}><legend>Invite a colleague</legend><label>Phone number<input type="tel" required maxLength={40} placeholder="+353…" value={phone} onChange={e=>setPhone(e.target.value)}/></label><button>Invite staff</button></fieldset></form><LoadMessage {...r} retry={()=>setRevision(n=>n+1)}/>{r.data?.members.map(m=><div key={m.user_id}><p>{m.phone_number} · {m.role}</p>{m.role==='staff'&&<button disabled={busy} onClick={()=>{if(window.confirm('Remove this staff member’s access to this business?'))void action(()=>marketplaceAPI.removeStaff(business.id,m.user_id),'Staff access removed.')}}>Remove {m.phone_number}</button>}</div>)}{r.data?.invitations.map(i=><div key={i.id}><p>Pending: {i.phone_number}</p><button disabled={busy} onClick={()=>action(()=>marketplaceAPI.revokeStaffInvitation(business.id,i.id),'Invitation revoked.')}>Revoke invitation for {i.phone_number}</button></div>)}<Feedback error={error} message={message}/></section>
}
