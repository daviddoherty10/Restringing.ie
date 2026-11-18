import { useCallback, useState, type FormEvent, useId } from 'react'
import { marketplaceAPI, MarketplaceError } from '../../lib/marketplace-api'
import { Feedback, LoadMessage } from '../management/dashboard-shared'
import { errorMessage, useRemote } from '../management/use-remote'

export default function OrderReview({ order }: { order: string }) {
  const [retry, setRetry] = useState(0)
  const load = useCallback(async () => {
    try { return await marketplaceAPI.ownBookingReview(order) }
    catch (error) {
      if (error instanceof MarketplaceError && error.code === 'review_unavailable') return null
      throw error
    }
  }, [order])
  const result = useRemote(`${order}:${retry}`, load)
  if (result.data === null) return null
  return <section className="tracking-card" aria-label="Rate your restring">
    <h2>Rate your restring</h2>
    <LoadMessage {...result} retry={() => setRetry(n => n + 1)} />
    {result.data && <ReviewForm order={order} initial={result.data} />}
  </section>
}

function ReviewForm({ order, initial }: {
  order: string
  initial: Awaited<ReturnType<typeof marketplaceAPI.ownBookingReview>>
}) {
  const ratingID = useId()
  const [rating, setRating] = useState(initial.review?.rating ?? 0)
  const [comment, setComment] = useState(initial.review?.comment ?? '')
  const [saved, setSaved] = useState(initial)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')

  async function change(remove: boolean, event?: FormEvent) {
    event?.preventDefault()
    if (busy) return
    setBusy(true)
    setError('')
    setMessage('')
    try {
      if (remove) {
        await marketplaceAPI.withdrawBookingReview(order)
        setSaved({ review: null, hidden: false })
        setRating(0)
        setComment('')
        setMessage('Review removed.')
      } else {
        const result = await marketplaceAPI.saveBookingReview(order, { rating, comment })
        setSaved(result)
        setMessage('Review saved.')
      }
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  return <form onSubmit={event => void change(false, event)}>
    <p>Reviews appear publicly once five different customers have eligible reviews. Keep names, contact details and order information out of your comment.</p>
    {saved.hidden && <p>Your review is hidden from the public profile following moderation.</p>}
    <fieldset disabled={busy}>
      <legend>Your review</legend>
      <label htmlFor={ratingID}>Rating</label>
      <select id={ratingID} required value={rating || ''} onChange={event => setRating(Number(event.target.value))}>
        <option value="">Choose a rating</option>
        {[1, 2, 3, 4, 5].map(value => <option value={value} key={value}>{value} {value === 1 ? 'star' : 'stars'}</option>)}
      </select>
      <label>Comment (optional)<textarea rows={3} maxLength={500} value={comment} onChange={event => setComment(event.target.value)} /></label>
      <div className="manage-actions">
        <button type="submit">{busy ? 'Saving…' : saved.review ? 'Update review' : 'Save review'}</button>
        {saved.review && <button type="button" onClick={() => void change(true)}>Remove review</button>}
      </div>
    </fieldset>
    <Feedback error={error} message={message} />
    <p><a href="/account/complaints">Get help with a booking issue</a></p>
  </form>
}
