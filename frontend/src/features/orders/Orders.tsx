import { useCallback, useState } from 'react'
import {
  marketplaceAPI,
  MarketplaceError,
  type Order,
} from '../../lib/marketplace-api'
import {
  MarketplaceHeader,
  MarketplaceFooter,
} from '../../components/layout/MarketplaceLayout'
import { signInURL } from '../../lib/auth-navigation'
import OrderWorkspace from '../tracking/OrderWorkspace'
import { useLive } from '../tracking/use-live'
import { dateTime, stages } from '../tracking/tracking-utils'
import './Orders.css'
const euro = (c: number) =>
  new Intl.NumberFormat('en-IE', { style: 'currency', currency: 'EUR' }).format(
    c / 100,
  )
function LoadError({
  error,
  retry,
  path,
}: {
  error: string
  retry: () => void
  path: string
}) {
  return (
    <div className="marketplace-empty" role="alert">
      <p>{error}</p>
      <a href={signInURL(path)}>Sign in</a>{' '}
      <button onClick={retry}>Retry</button>
    </div>
  )
}
function BookingDetails({ order }: { order: Order }) {
  const b = order.configuration
  return (
    <section className="tracking-card" aria-labelledby="booking-details">
      <p className="marketplace-kicker">YOUR SETUP</p>
      <h2 id="booking-details">Booking &amp; payment</h2>
      {b.express && (
        <p className="tracking-express">
          Express turnaround · {b.express_turnaround_snapshot}
        </p>
      )}
      <dl className="tracking-details">
        <div>
          <dt>Racket</dt>
          <dd>{b.racket_snapshot}</dd>
        </div>
        <div>
          <dt>Strings</dt>
          <dd>
            {b.string_source === 'customer'
              ? 'Customer supplies strings'
              : b.string_snapshot}
          </dd>
        </div>
        <div>
          <dt>Mains / crosses</dt>
          <dd>
            {b.mains_tension_centi_kg / 100} /{' '}
            {b.crosses_tension_centi_kg / 100} kg
          </dd>
        </div>
        <div>
          <dt>Setup</dt>
          <dd>
            {b.knots} knots · {b.stencil ? 'Stencil' : 'No stencil'} ·{' '}
            {b.prestretch_percent}% pre-stretch
          </dd>
        </div>
      </dl>
      {b.instructions && (
        <p className="tracking-instructions">
          Your instructions: {b.instructions}
        </p>
      )}
      <h3>Price breakdown</h3>
      {order.breakdown.map((line) => (
        <p key={line.code}>
          {line.label}: {euro(line.amount_cents)}
        </p>
      ))}
      <p>
        <strong>Total: {euro(order.total_cents)}</strong>
      </p>
      <p>
        <strong>Payment: </strong>
        {order.payment_method === 'cash'
          ? order.payment_status === 'paid'
            ? 'Cash received — confirmed by the stringer'
            : order.status === 'cancelled'
              ? 'Cancelled booking — contact the stringer about any payment'
              : 'Cash due directly to the stringer'
          : order.payment_status}
      </p>
      <p>Booking progress and payment status are recorded separately.</p>
      {order.payment_events?.map((event, i) => (
        <p key={i}>
          {event.reason} — {dateTime(event.created_at)}
        </p>
      ))}
    </section>
  )
}
function OrderDetail({ id }: { id: string }) {
  const load = useCallback(async () => {
    const [order, me] = await Promise.all([
      marketplaceAPI.order(id),
      marketplaceAPI.me(),
    ])
    return { order, me }
  }, [id])
  const live = useLive(load)
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(''),
    [confirm, setConfirm] = useState(false)
  async function cancel() {
    if (busy) return
    setBusy(true)
    setError('')
    try {
      await marketplaceAPI.updateOrderStatus(id, 'cancelled')
      setConfirm(false)
    } catch (e) {
      setError(
        e instanceof MarketplaceError
          ? e.message
          : 'Could not confirm cancellation. Refresh the order before retrying.',
      )
    } finally {
      setBusy(false)
      live.refresh()
    }
  }
  const order = live.data?.order
  return (
    <>
      {live.error && (
        <LoadError
          error={live.error}
          retry={live.refresh}
          path={`/orders/${id}`}
        />
      )}
      {!order && !live.error && <p role="status">Loading your order…</p>}
      {order && (
        <>
          <div className="orders-tracking-toolbar">
            <a href="/orders">← All my orders</a>
            <button onClick={live.refresh}>Refresh order</button>
          </div>
          {live.error && (
            <p role="status">The order below may be out of date.</p>
          )}
          <OrderWorkspace order={order} refreshOrder={live.refresh} />
          <BookingDetails order={order} />
          {live.data?.me.id === order.customer_id &&
            order.status === 'placed' && (
              <section className="tracking-card">
                <h2>Change of plans?</h2>
                <p>
                  You can cancel while your booking is waiting to be accepted.
                </p>
                {error && <p role="alert">{error}</p>}
                {confirm ? (
                  <>
                    <p>Cancel this booking and release its reserved stock?</p>
                    <button disabled={busy} onClick={cancel}>
                      {busy ? 'Cancelling…' : 'Confirm cancellation'}
                    </button>{' '}
                    <button disabled={busy} onClick={() => setConfirm(false)}>
                      Keep booking
                    </button>
                  </>
                ) : (
                  <button onClick={() => setConfirm(true)}>
                    Cancel placed order
                  </button>
                )}
              </section>
            )}
        </>
      )}
    </>
  )
}
function OrderList() {
  const [page, setPage] = useState(1)
  const load = useCallback(
    () => marketplaceAPI.orders({ page, page_size: 10 }),
    [page],
  )
  const live = useLive(load)
  return (
    <>
      <p className="marketplace-kicker">CUSTOMER ACCOUNT</p>
      <h1>Your orders</h1>
      <p>Follow your racket from booking to collection.</p>
      {live.error && (
        <LoadError error={live.error} retry={live.refresh} path="/orders" />
      )}
      {!live.data && !live.error && <p role="status">Loading orders…</p>}
      {live.data && (
        <>
          {live.data.data.length ? (
            <div className="orders-list">
              {live.data.data.map((o) => (
                <a href={`/orders/${o.id}`} className="orders-panel" key={o.id}>
                  <p className="marketplace-kicker">
                    ORDER {o.id.slice(0, 8).toUpperCase()}
                  </p>
                  <h2>
                    {stages.find((s) => s.status === o.status)?.title ??
                      'Order cancelled'}
                  </h2>
                  <p>{o.configuration.racket_snapshot}</p>
                  <p>
                    {o.configuration.string_source === 'customer'
                      ? 'Your own strings'
                      : o.configuration.string_snapshot}
                  </p>
                  <strong>{euro(o.total_cents)}</strong>
                  <span className="order-track-link">Track your racket →</span>
                </a>
              ))}
            </div>
          ) : (
            <div className="marketplace-empty">
              <h3>No orders yet</h3>
              <p>Your bookings will appear here after you place an order.</p>
              <a href="/search">Find a stringer →</a>
            </div>
          )}
          {live.data.total > 10 && (
            <div className="catalogue-pages">
              <button
                disabled={page === 1}
                onClick={() => setPage((p) => p - 1)}
              >
                Previous
              </button>
              <span>
                Page {page} of {Math.ceil(live.data.total / 10)}
              </span>
              <button
                disabled={page * 10 >= live.data.total}
                onClick={() => setPage((p) => p + 1)}
              >
                Next
              </button>
            </div>
          )}
        </>
      )}
    </>
  )
}
export default function Orders({ id }: { id?: string }) {
  return (
    <div className="marketplace orders-screen">
      <MarketplaceHeader backHref="/search" backLabel="Find a stringer" />
      <main className="marketplace-container orders-content">
        {id ? <OrderDetail key={id} id={id} /> : <OrderList />}
      </main>
      <MarketplaceFooter />
    </div>
  )
}
