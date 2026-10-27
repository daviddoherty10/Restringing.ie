import { useCallback, useState } from 'react'
import { marketplaceAPI, MarketplaceError } from '../../lib/marketplace-api'
import {
  MarketplaceHeader,
  MarketplaceFooter,
} from '../../components/layout/MarketplaceLayout'
import { signInURL } from '../../lib/auth-navigation'
import { useLive } from './use-live'
import { dateTime } from './tracking-utils'
import './Tracking.css'
import '../orders/Orders.css'

export function NotificationLink() {
  const load = useCallback(
    () => marketplaceAPI.notifications({ page_size: 1 }),
    [],
  )
  const live = useLive(load, 30000)
  const count = live.data?.unread_count ?? 0
  return (
    <a
      href="/notifications"
      aria-current={
        window.location.pathname === '/notifications' ? 'page' : undefined
      }
    >
      Notifications
      {count > 0 && (
        <span className="notification-count" aria-label={`${count} unread`}>
          {count > 99 ? '99+' : count}
        </span>
      )}
    </a>
  )
}
export default function Notifications() {
  const [page, setPage] = useState(1),
    [busy, setBusy] = useState(''),
    [error, setError] = useState('')
  const load = useCallback(
    () => marketplaceAPI.notifications({ page, page_size: 20 }),
    [page],
  )
  const live = useLive(load)
  async function read(id: string) {
    if (busy) return
    setBusy(id)
    setError('')
    try {
      await marketplaceAPI.readNotification(id)
      live.refresh()
    } catch (e) {
      setError(
        e instanceof MarketplaceError
          ? e.message
          : 'Could not mark this notification read. Retry safely.',
      )
    } finally {
      setBusy('')
    }
  }
  return (
    <div className="marketplace orders-screen">
      <MarketplaceHeader backHref="/orders" backLabel="My orders" />
      <main className="marketplace-container orders-content">
        <p className="marketplace-kicker">YOUR ACCOUNT</p>
        <h1>Notifications</h1>
        <p>
          Booking updates and messages, all in one place. Updates refresh while
          this page is open.
        </p>
        <button onClick={live.refresh}>Refresh notifications</button>
        {live.error && (
          <div role="alert">
            <p>{live.error}</p>
            <a href={signInURL('/notifications')}>Sign in</a>{' '}
            <button onClick={live.refresh}>Retry</button>
          </div>
        )}
        {error && <p role="alert">{error}</p>}
        {!live.data && !live.error && (
          <p role="status">Loading notifications…</p>
        )}
        {live.data && (
          <>
            <p>{live.data.unread_count} unread</p>
            <div className="notification-list">
              {live.data.data.length ? (
                live.data.data.map((n) => (
                  <article
                    className={`notification-item${n.read_at ? '' : ' notification-item-unread'}`}
                    key={n.id}
                  >
                    <a href={`/orders/${n.order_id}`}>{n.title} →</a>
                    <p>
                      Order {n.order_id.slice(0, 8)} ·{' '}
                      {n.for_stringer ? 'Stringer team' : 'Your booking'}
                    </p>
                    <time dateTime={n.created_at}>
                      {dateTime(n.created_at)}
                    </time>
                    {!n.read_at && (
                      <button disabled={!!busy} onClick={() => read(n.id)}>
                        {busy === n.id ? 'Saving…' : 'Mark read'}
                      </button>
                    )}
                  </article>
                ))
              ) : (
                <p>
                  You’re all caught up. New orders, replies and status updates
                  will appear here.
                </p>
              )}
            </div>
            {live.data.total > 20 && (
              <div className="tracking-pages">
                <button
                  disabled={page === 1}
                  onClick={() => setPage((p) => p - 1)}
                >
                  Previous
                </button>
                <span>Page {page}</span>
                <button
                  disabled={page * 20 >= live.data.total}
                  onClick={() => setPage((p) => p + 1)}
                >
                  Next
                </button>
              </div>
            )}
          </>
        )}
      </main>
      <MarketplaceFooter />
    </div>
  )
}
