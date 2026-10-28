import { useCallback, useState } from 'react'
import { marketplaceAPI, type ManagementProfile, type Order, type OrderStatus } from '../../lib/marketplace-api'
import { Feedback, LoadMessage, Pagination } from './dashboard-shared'
import { errorMessage, useRemote } from './use-remote'
import { cancellable, euro, nextOrderStatus, orderStatuses, statusLabel } from './dashboard-utils'
function OrderDetails({ order }: { order: Order }) {
  const b = order.configuration
  return <><dl className="manage-details"><div><dt>Racket</dt><dd>{b.racket_snapshot}</dd></div><div><dt>Strings</dt><dd>{b.string_source === 'customer' ? 'Customer supplies strings' : b.string_snapshot}</dd></div><div><dt>Tensions</dt><dd>{b.mains_tension_centi_kg / 100} / {b.crosses_tension_centi_kg / 100} kg</dd></div><div><dt>Setup</dt><dd>{b.knots} knots · {b.stencil ? 'Stencil' : 'No stencil'} · {b.prestretch_percent}% pre-stretch</dd></div></dl>{b.instructions && <p className="manage-instructions">Customer instructions: {b.instructions}</p>}<p>{b.express ? `Express turnaround: ${b.express_turnaround_snapshot}` : 'Standard turnaround'}</p><p><strong>New grip:</strong> {b.grip_snapshot || 'No new grip'}</p><h4>Original price breakdown</h4>{order.breakdown.map(line => <p key={line.code}>{line.label}: {euro(line.amount_cents)}</p>)}<h4>Payment history</h4>{order.payment_events?.map((e,i)=><p key={i}>{e.reason} — {new Date(e.created_at).toLocaleString('en-IE')}</p>)}<h4>Status history</h4><ol>{order.events?.map((e, i) => <li key={i}>{statusLabel(e.to_status)} — {new Date(e.created_at).toLocaleString('en-IE')}</li>)}</ol></>
}
function OrderInspector({ businessID, id, revision }: { businessID: string; id: string; revision: number }) {
  const load = useCallback(() => marketplaceAPI.incomingOrder(businessID,id), [businessID,id])
  const [retry, setRetry] = useState(0)
  const r = useRemote(`${id}:${revision}:${retry}`, load)
  return <section className="manage-order-details" aria-label="Order details"><LoadMessage {...r} retry={() => setRetry(n => n + 1)} />{r.data && <><h4>Customer contact</h4><p>{[r.data.customer.first_name,r.data.customer.last_name].filter(Boolean).join(' ') || 'Customer'} · <a href={`tel:${r.data.customer.phone_number}`}>{r.data.customer.phone_number}</a></p><p><a href={`/orders/${id}`}>Open live tracking &amp; messages →</a></p><OrderDetails order={r.data} /></>}</section>
}
export default function OrdersPanel({ business, changed, canAccept }: { business: ManagementProfile; changed: () => void; canAccept?: boolean }) {
  const [status, setStatus] = useState<OrderStatus | ''>('')
  const [page, setPage] = useState(1)
  const [revision, setRevision] = useState(0)
  const [expanded, setExpanded] = useState('')
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const [cancelID, setCancelID] = useState(''), [cashID, setCashID] = useState('')
  const load = useCallback(() => marketplaceAPI.incomingOrders(business.id, { status: status || undefined, page, page_size: 10 }), [business.id, status, page])
  const r = useRemote(`${business.id}:${status}:${page}:${revision}`, load)
  async function update(order: Order, to: OrderStatus) {
    if (busy) return
    setBusy(order.id); setError(''); setMessage('')
    try { await marketplaceAPI.updateOrderStatus(order.id, to); setRevision(n => n + 1); setCancelID(''); setMessage(`Order updated to ${statusLabel(to)}.`); changed() }
    catch (e) { setError(errorMessage(e)); setRevision(n => n + 1) }
    finally { setBusy('') }
  }
  async function cash(order: Order) {
    if(busy)return
    setBusy(order.id);setError('');setMessage('')
    try { await marketplaceAPI.confirmCashReceipt(order.id); setCashID('');setMessage('Cash receipt recorded.');changed() }
    catch(e){setError(errorMessage(e))}
    finally{setBusy('');setRevision(n=>n+1)}
  }
  return <section className="manage-panel" aria-labelledby="incoming-title"><div className="manage-section-heading"><div><h2 id="incoming-title">Incoming orders</h2><p>Fulfilment and payment are separate. Confirm cash only after you have received it.</p></div><button type="button" disabled={!!busy} onClick={() => { setRevision(n => n + 1); changed() }}>Refresh orders</button></div><label className="manage-filter">Order status<select disabled={!!busy} value={status} onChange={e => { setStatus(e.target.value as OrderStatus | ''); setPage(1); setExpanded(''); setCancelID('') }}><option value="">All statuses</option>{orderStatuses.map(s => <option key={s} value={s}>{statusLabel(s)}</option>)}</select></label><Feedback error={error} message={message} /><LoadMessage {...r} retry={() => setRevision(n => n + 1)} />{r.data && <>{r.data.data.length === 0 ? <p>No orders match this filter.</p> : r.data.data.map(order => {
    const next = nextOrderStatus(order.status)
    return <article className="manage-order" key={order.id}><div className="manage-section-heading"><div><h3>Order {order.id.slice(0, 8)}</h3><p>{new Date(order.created_at).toLocaleString('en-IE')} · <strong>{euro(order.total_cents)}</strong></p></div><div className="manage-badges">{order.configuration.grip_product_id&&<span>Grip add-on</span>}{order.configuration.express&&<span>Express</span>}<span>{statusLabel(order.status)}</span><span>{order.payment_method==='cash'?(order.payment_status==='paid'?'Cash received':'Cash due'):`Payment: ${statusLabel(order.payment_status)}`}</span></div></div><p>{order.configuration.racket_snapshot} · {order.configuration.string_source === 'customer' ? 'Own strings' : order.configuration.string_snapshot}</p><div className="manage-actions"><button type="button" onClick={() => setExpanded(expanded === order.id ? '' : order.id)} aria-expanded={expanded === order.id}>{expanded === order.id ? 'Hide details' : 'View details'}</button>{business.permissions.manage_orders && next && <button type="button" disabled={!!busy || next==='accepted' && canAccept===false} onClick={() => update(order, next)}>{busy === order.id ? 'Saving…' : `Mark ${statusLabel(next)}`}</button>}{business.permissions.manage_orders && cancellable(order.status) && <button type="button" className="manage-secondary" disabled={!!busy} onClick={() => setCancelID(order.id)}>Cancel order</button>}{business.permissions.manage_orders && order.payment_method==='cash' && order.payment_status==='unpaid' && order.status!=='placed' && order.status!=='cancelled' && <button type="button" disabled={!!busy} onClick={()=>setCashID(order.id)}>Record cash received</button>}</div>{cashID===order.id && <div className="manage-confirm"><p>Confirm you received {euro(order.total_cents)} in cash. This records payment; no online charge is made.</p><button type="button" disabled={!!busy} onClick={()=>cash(order)}>Confirm cash received</button> <button type="button" disabled={!!busy} onClick={()=>setCashID('')}>Back</button></div>}{cancelID === order.id && <div className="manage-confirm"><p>Cancel this order and release its reserved stock? {order.payment_status==='paid'&&'Cash was recorded as received. Arrange any refund directly with the customer; cancellation does not record a refund.'}</p><button type="button" disabled={!!busy} onClick={() => update(order, 'cancelled')}>Confirm cancellation</button> <button type="button" disabled={!!busy} onClick={() => setCancelID('')}>Keep order</button></div>}{expanded === order.id && <OrderInspector businessID={business.id} id={order.id} revision={revision} />}</article>
  })}<Pagination page={page} size={10} total={r.data.total} change={setPage} busy={!!busy} /></>}</section>
}
