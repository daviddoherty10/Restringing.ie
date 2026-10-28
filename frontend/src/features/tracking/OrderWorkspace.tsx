import { useCallback, useRef, useState, type FormEvent } from 'react'
import {
  marketplaceAPI,
  MarketplaceError,
  type Order,
} from '../../lib/marketplace-api'
import type { components } from '../../lib/marketplace-contract'
import { useLive } from './use-live'
import {
  collectionDate,
  dateTime,
  irishToday,
  localDateTime,
  stages,
} from './tracking-utils'
import Conversation from './Conversation'
import './Tracking.css'
type Tracking = components['schemas']['OrderTracking']
type Update = components['schemas']['TrackingUpdate']

function CollectionForm({
  order,
  tracking,
  updated,
}: {
  order: Order
  tracking: Tracking
  updated: (message: string) => void
}) {
  const [expectedVersion, setExpectedVersion] = useState(tracking.version)
  const [date, setDate] = useState(tracking.requested_collection_date ?? '')
  const [ready, setReady] = useState(localDateTime(tracking.confirmed_ready_at))
  const [instructions, setInstructions] = useState(
    tracking.collection_instructions,
  )
  const [issue, setIssue] = useState(tracking.issue_open),
    [note, setNote] = useState('')
  const [busy, setBusy] = useState(false),
    [uncertain, setUncertain] = useState(false),
    [error, setError] = useState('')
  const attempt = useRef<{ key: string; submit: () => Promise<Update> } | null>(
    null,
  )
  const [mode, setMode] = useState<'customer' | 'stringer'>(
    tracking.can_manage && !tracking.is_customer ? 'stringer' : 'customer',
  )
  const disabled = busy || uncertain
  function resetDraft(state: components['schemas']['TrackingState']) {
    setExpectedVersion(state.version)
    setDate(state.requested_collection_date ?? '')
    setReady(localDateTime(state.confirmed_ready_at))
    setInstructions(state.collection_instructions)
    setIssue(state.issue_open)
    setNote('')
    setError('')
  }
  async function save(e: FormEvent) {
    e.preventDefault()
    if (busy) return
    if (!attempt.current) {
      const key = crypto.randomUUID()
      if (mode === 'customer') {
        const input = { date, expected_version: expectedVersion }
        attempt.current = {
          key,
          submit: () => marketplaceAPI.requestCollection(order.id, input, key),
        }
      } else {
        const input = {
          confirmed_ready_at:
            ready === localDateTime(tracking.confirmed_ready_at)
              ? tracking.confirmed_ready_at
              : ready
                ? new Date(ready).toISOString()
                : null,
          collection_instructions: instructions,
          issue_open: issue,
          note: note.trim(),
          expected_version: expectedVersion,
        }
        attempt.current = {
          key,
          submit: () => marketplaceAPI.updateTracking(order.id, input, key),
        }
      }
    }
    setBusy(true)
    setError('')
    try {
      const result = await attempt.current.submit()
      resetDraft(result.state)
      attempt.current = null
      setUncertain(false)
      updated(
        mode === 'customer'
          ? 'Collection date requested. Your stringer will confirm what is possible.'
          : 'Collection update saved and an in-app notification created.',
      )
    } catch (e) {
      const ambiguous = !(e instanceof MarketplaceError) || e.status >= 500
      setUncertain(ambiguous)
      if (!ambiguous) attempt.current = null
      const message = ambiguous
        ? 'We could not confirm whether this was saved. Retry to safely check the same update.'
        : e instanceof Error
          ? e.message
          : 'Update failed.'
      setError(message)
      if (e instanceof MarketplaceError && e.code === 'tracking_changed')
        updated(message)
    } finally {
      setBusy(false)
    }
  }
  return (
    <form className="collection-form" onSubmit={save}>
      {tracking.can_manage && tracking.is_customer && (
        <label>
          Update as
          <select
            disabled={disabled}
            value={mode}
            onChange={(e) => setMode(e.target.value as 'customer' | 'stringer')}
          >
            <option value="customer">Customer</option>
            <option value="stringer">Stringer team</option>
          </select>
        </label>
      )}
      {mode === 'customer' ? (
        <>
          <h3>Request a collection date</h3>
          <p>
            Choose when you would like to collect your finished racket. This is
            a request for your stringer to agree.
          </p>
          <label htmlFor="collection-date">Requested collection date</label>
          <input
            id="collection-date"
            type="date"
            required
            min={irishToday()}
            value={date}
            disabled={disabled}
            onChange={(e) => setDate(e.target.value)}
          />
        </>
      ) : (
        <>
          <h3>Update collection details</h3>
          <p>
            Explain any change so your customer knows what to expect. Saving
            does not change the order status or price.
          </p>
          <label htmlFor="ready-estimate">Confirmed ready estimate</label>
          <input
            id="ready-estimate"
            type="datetime-local"
            value={ready}
            disabled={disabled}
            onChange={(e) => setReady(e.target.value)}
          />
          <small>
            Enter in your device timezone (
            {Intl.DateTimeFormat().resolvedOptions().timeZone}). Customers see
            Ireland time. Leave blank if an estimate is not agreed.
          </small>
          <label htmlFor="collection-instructions">
            Collection instructions
          </label>
          <textarea
            id="collection-instructions"
            rows={3}
            maxLength={1000}
            value={instructions}
            disabled={disabled}
            onChange={(e) => setInstructions(e.target.value)}
          />
          <label className="tracking-checkbox">
            <input
              type="checkbox"
              checked={issue}
              disabled={disabled}
              onChange={(e) => setIssue(e.target.checked)}
            />{' '}
            Flag an issue needing attention
          </label>
          <label htmlFor="tracking-note">Update for the customer</label>
          <textarea
            id="tracking-note"
            rows={3}
            maxLength={1000}
            required
            value={note}
            disabled={disabled}
            onChange={(e) => setNote(e.target.value)}
            placeholder="Explain the collection estimate, delay or issue…"
          />
        </>
      )}
      {error && <p role="alert">{error}</p>}
      {tracking.version !== expectedVersion && !uncertain && (
        <p role="status">
          Collection details changed while you were editing. Your draft has been
          kept.{' '}
          <button type="button" onClick={() => resetDraft(tracking)}>
            Load latest collection details
          </button>
        </p>
      )}
      <button
        type="submit"
        disabled={
          busy ||
          (!uncertain && tracking.version !== expectedVersion) ||
          (mode === 'stringer' && order.status === 'placed')
        }
      >
        {busy
          ? 'Saving…'
          : uncertain
            ? 'Retry same update'
            : mode === 'customer'
              ? 'Request collection date'
              : 'Save collection update'}
      </button>
      {mode === 'stringer' && order.status === 'placed' && (
        <small>
          Accept this booking in the stringer portal before confirming
          collection details.
        </small>
      )}
    </form>
  )
}

export default function OrderWorkspace({
  order,
  refreshOrder,
}: {
  order: Order
  refreshOrder: () => void
}) {
  const [updatePage, setUpdatePage] = useState(1),
    [feedback, setFeedback] = useState('')
  const load = useCallback(async () => {
    const [tracking, updates] = await Promise.all([
      marketplaceAPI.tracking(order.id),
      marketplaceAPI.trackingUpdates(order.id, {
        page: updatePage,
        page_size: 10,
      }),
    ])
    return { tracking, updates, checkedAt: Date.now() }
  }, [order.id, updatePage])
  const live = useLive(load)
  const tracking = live.data?.tracking
  const stage = stages.find((s) => s.status === order.status)
  const cancelled = order.status === 'cancelled'
  const reached = cancelled
    ? Math.max(
        0,
        ...(order.events ?? []).map((e) =>
          stages.findIndex((s) => s.status === e.to_status),
        ),
      )
    : stages.findIndex((s) => s.status === order.status)
  function updated(message: string) {
    setFeedback(message)
    setUpdatePage(1)
    live.refresh()
    refreshOrder()
  }
  return (
    <div className="order-workspace">
      <section
        className={`tracking-hero${cancelled ? ' tracking-cancelled' : ''}`}
        aria-labelledby="tracking-title"
      >
        <div className="tracking-section-heading">
          <p className="tracking-eyebrow">
            ORDER {order.id.slice(0, 8).toUpperCase()}
          </p>
          <span className="tracking-live">
            Updates every 15 seconds while open
          </span>
        </div>
        {order.configuration.express && (
          <div className="tracking-express">
            Express service · {order.configuration.express_turnaround_snapshot}
          </div>
        )}
        <h1 id="tracking-title">
          {cancelled
            ? 'Order cancelled'
            : (stage?.title ?? 'Track your racket')}
        </h1>
        <p>
          {cancelled
            ? 'This booking has been cancelled. You can still review its history and contact the stringer below.'
            : stage?.description}
        </p>
        {tracking && (
          <>
            <a
              className="tracking-business"
              href={`/stringers/${order.stringer_id}`}
            >
              {tracking.stringer_name} ↗
            </a>
            <p className="tracking-location">
              {[tracking.area, tracking.county].filter(Boolean).join(', ')}
            </p>
            {tracking.issue_open && !cancelled && (
              <div className="tracking-attention" role="status">
                <strong>Your order needs attention</strong>
                <p>
                  Read the latest update below and reply in the order
                  conversation.
                </p>
              </div>
            )}
            {!cancelled && order.status !== 'completed' && (
              <div className="tracking-estimate">
                <span>
                  {order.status === 'ready'
                    ? 'READY TO COLLECT'
                    : 'CONFIRMED READY ESTIMATE'}
                </span>
                <strong>
                  {order.status === 'ready'
                    ? 'Your racket is ready'
                    : tracking.confirmed_ready_at
                      ? dateTime(tracking.confirmed_ready_at)
                      : 'Awaiting your stringer’s estimate'}
                </strong>
                {tracking.confirmed_ready_at &&
                  order.status !== 'ready' &&
                  new Date(tracking.confirmed_ready_at).getTime() <
                    (live.data?.checkedAt ?? 0) && (
                    <p>
                      The estimate has passed. Ask your stringer for an update
                      below.
                    </p>
                  )}
                <small>
                  Times shown in Ireland time. Collection arrangements are
                  agreed with your stringer.
                </small>
              </div>
            )}
          </>
        )}
        {tracking?.latest_update && (
          <div className="tracking-latest">
            <span>LATEST COLLECTION UPDATE</span>
            <p>{tracking.latest_update.note}</p>
            <time dateTime={tracking.latest_update.created_at}>
              {dateTime(tracking.latest_update.created_at)}
            </time>
          </div>
        )}
      </section>
      <section className="tracking-card" aria-label="Order progress">
        <ol className="tracking-stages">
          {stages.map((s, i) => {
            const done = i < reached || order.status === 'completed',
              current = !cancelled && s.status === order.status
            const event = order.events?.find((e) => e.to_status === s.status)
            return (
              <li
                key={s.status}
                className={`${done ? 'stage-done ' : ''}${current ? 'stage-current' : ''}`}
                aria-current={current ? 'step' : undefined}
              >
                <span className="stage-marker" aria-hidden="true">
                  {done ? '✓' : String(i + 1).padStart(2, '0')}
                </span>
                <div>
                  <strong>{s.title}</strong>
                  {event && (
                    <time dateTime={event.created_at}>
                      {dateTime(event.created_at)}
                    </time>
                  )}
                  {cancelled && i > reached && <small>Not reached</small>}
                </div>
              </li>
            )
          })}
        </ol>
      </section>
      {live.error && (
        <div className="tracking-card" role="alert">
          <p>
            {live.error}{' '}
            {tracking && 'The collection details below may be out of date.'}
          </p>
          <button onClick={live.refresh}>Retry collection updates</button>
        </div>
      )}
      {!tracking && !live.error && (
        <p role="status">Loading collection details…</p>
      )}
      {tracking && (
        <div className="tracking-columns">
          <section className="tracking-card" aria-labelledby="collection-title">
            <p className="marketplace-kicker">PLAN YOUR COLLECTION</p>
            <h2 id="collection-title">Collection details</h2>
            {tracking.can_manage && (
              <p>
                <a href="/manage">Manage fulfilment in the stringer portal →</a>
              </p>
            )}
            <dl className="tracking-details">
              <div>
                <dt>Your requested date</dt>
                <dd>
                  {tracking.requested_collection_date
                    ? collectionDate(tracking.requested_collection_date)
                    : 'No date requested'}
                </dd>
              </div>
              <div>
                <dt>Stringer’s ready estimate</dt>
                <dd>
                  {tracking.confirmed_ready_at
                    ? dateTime(tracking.confirmed_ready_at)
                    : 'Not confirmed yet'}
                </dd>
              </div>
            </dl>
            {tracking.collection_instructions ? (
              <p className="tracking-instructions">
                {tracking.collection_instructions}
              </p>
            ) : (
              <p>
                Agree collection arrangements with your stringer in the
                conversation below.
              </p>
            )}
            {feedback && <p role="status">{feedback}</p>}
            {!cancelled &&
              order.status !== 'completed' &&
              (tracking.is_customer || tracking.can_manage) && (
                <CollectionForm
                  key={order.id}
                  order={order}
                  tracking={tracking}
                  updated={updated}
                />
              )}
          </section>
          <section className="tracking-card" aria-labelledby="updates-title">
            <p className="marketplace-kicker">FROM YOUR STRINGER & YOU</p>
            <h2 id="updates-title">Collection updates</h2>
            {live.data?.updates.data.length ? (
              <ol className="tracking-updates">
                {live.data.updates.data.map((u) => (
                  <li key={u.id}>
                    <strong>
                      {u.kind === 'collection_request'
                        ? 'Collection requested'
                        : u.state.issue_open
                          ? 'Attention needed'
                          : 'Stringer update'}
                    </strong>
                    <p>{u.note}</p>
                    <time dateTime={u.created_at}>
                      {dateTime(u.created_at)}
                    </time>
                  </li>
                ))}
              </ol>
            ) : (
              <p>
                Collection estimates, requests and issue updates will appear
                here.
              </p>
            )}
            {live.data && live.data.updates.total > 10 && (
              <div className="tracking-pages">
                <button
                  disabled={updatePage === 1}
                  onClick={() => setUpdatePage((p) => p - 1)}
                >
                  Newer updates
                </button>
                <span>Page {updatePage}</span>
                <button
                  disabled={updatePage * 10 >= live.data.updates.total}
                  onClick={() => setUpdatePage((p) => p + 1)}
                >
                  Older updates
                </button>
              </div>
            )}
          </section>
        </div>
      )}
      <Conversation key={order.id} orderID={order.id} />
    </div>
  )
}
