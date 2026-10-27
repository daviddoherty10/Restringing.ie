import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import { marketplaceAPI, MarketplaceError } from '../../lib/marketplace-api'
import { useLive } from './use-live'
import { dateTime } from './tracking-utils'

export default function Conversation({ orderID }: { orderID: string }) {
  const [page, setPage] = useState(1),
    [body, setBody] = useState(''),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(''),
    [sent, setSent] = useState(false)
  const attempt = useRef<{ key: string; body: string } | null>(null)
  const [uncertain, setUncertain] = useState(false)
  const load = useCallback(
    () => marketplaceAPI.messages(orderID, { page, page_size: 20 }),
    [orderID, page],
  )
  const live = useLive(load)
  const newestID = live.data?.data[0]?.id
  const [readError, setReadError] = useState('')
  const [readRevision, setReadRevision] = useState(0)
  useEffect(() => {
    if (
      !newestID ||
      page !== 1 ||
      !live.data?.unread_count ||
      document.visibilityState === 'hidden'
    )
      return
    let active = true
    marketplaceAPI
      .readMessages(orderID, newestID)
      .then(() => {
        if (active) setReadError('')
      })
      .catch(() => {
        if (active) setReadError('Could not mark these messages as read.')
      })
    return () => {
      active = false
    }
  }, [orderID, newestID, page, live.data?.unread_count, readRevision])
  async function send(e: FormEvent) {
    e.preventDefault()
    if (busy || !body.trim()) return
    if (!attempt.current)
      attempt.current = { key: crypto.randomUUID(), body: body.trim() }
    setBusy(true)
    setError('')
    setSent(false)
    try {
      await marketplaceAPI.sendMessage(
        orderID,
        attempt.current.body,
        attempt.current.key,
      )
      attempt.current = null
      setUncertain(false)
      setBody('')
      setSent(true)
      setPage(1)
      live.refresh()
    } catch (e) {
      const ambiguous = !(e instanceof MarketplaceError) || e.status >= 500
      setUncertain(ambiguous)
      if (!ambiguous) attempt.current = null
      setError(
        ambiguous
          ? 'We could not confirm whether your message was sent. Retry to safely check the same message.'
          : e instanceof Error
            ? e.message
            : 'Message could not be sent.',
      )
    } finally {
      setBusy(false)
    }
  }
  return (
    <section
      className="tracking-card conversation"
      aria-labelledby={`conversation-${orderID}`}
    >
      <div className="tracking-section-heading">
        <div>
          <p className="marketplace-kicker">STAY IN TOUCH</p>
          <h2 id={`conversation-${orderID}`}>Order conversation</h2>
        </div>
        <button type="button" onClick={live.refresh}>
          Refresh messages
        </button>
      </div>
      <p>
        Ask a question or discuss a delay here. Messages stay with this order. A
        message does not change the agreed price.
      </p>
      {live.error && (
        <p role="alert">
          {live.error} <button onClick={live.refresh}>Retry messages</button>
        </p>
      )}
      {!live.data && !live.error && <p role="status">Loading messages…</p>}
      {live.data && (
        <>
          {live.data.data.length ? (
            <ol className="message-list">
              {[...live.data.data].reverse().map((m) => (
                <li className={m.own ? 'message-own' : ''} key={m.id}>
                  <div>
                    <strong>
                      {m.own
                        ? 'You'
                        : m.sender_role === 'stringer'
                          ? 'Stringer team'
                          : m.sender_role === 'support'
                            ? 'Platform support'
                            : 'Customer'}
                    </strong>
                    <time dateTime={m.created_at}>
                      {dateTime(m.created_at)}
                    </time>
                  </div>
                  <p>{m.body}</p>
                </li>
              ))}
            </ol>
          ) : (
            <p className="tracking-muted">
              No messages yet. Start the conversation below.
            </p>
          )}
          <div className="tracking-pages">
            <button
              disabled={page * 20 >= live.data.total}
              onClick={() => setPage((p) => p + 1)}
            >
              Older messages
            </button>
            <span>Page {page}</span>
            <button disabled={page === 1} onClick={() => setPage((p) => p - 1)}>
              Newer messages
            </button>
          </div>
        </>
      )}
      {readError && (
        <p role="alert">
          {readError}{' '}
          <button onClick={() => setReadRevision((n) => n + 1)}>
            Retry marking read
          </button>
        </p>
      )}
      <form onSubmit={send}>
        <label htmlFor={`message-${orderID}`}>Your message</label>
        <textarea
          id={`message-${orderID}`}
          maxLength={2000}
          required
          rows={3}
          value={body}
          disabled={busy || uncertain}
          onChange={(e) => {
            setBody(e.target.value)
            setSent(false)
          }}
          placeholder="Ask about your racket, collection or a delay…"
        />
        <small>
          {body.length}/2000 characters · Times shown in Ireland time
        </small>
        {error && <p role="alert">{error}</p>}
        {sent && <p role="status">Message sent.</p>}
        <button disabled={busy || !body.trim()} type="submit">
          {busy
            ? 'Sending…'
            : uncertain
              ? 'Retry same message'
              : 'Send message'}
        </button>
      </form>
    </section>
  )
}
