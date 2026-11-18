import { useCallback, useState, type FormEvent } from 'react'
import { marketplaceAPI } from '../../lib/marketplace-api'
import { Feedback, LoadMessage } from '../management/dashboard-shared'
import { errorMessage, useRemote } from '../management/use-remote'
import './reviews.css'

export default function StringerReviews({ business, canReport = false }: { business: string; canReport?: boolean }) {
  const [retry, setRetry] = useState(0)
  const load = useCallback(() => marketplaceAPI.publicReviews(business), [business])
  const result = useRemote(`${business}:${retry}`, load)
  const data = result.data
  return <section className="stringer-reviews" aria-label="Customer reviews">
    <h2>Customer reviews</h2>
    <LoadMessage {...result} retry={() => setRetry(n => n + 1)} />
    {data && (data.count === 0 ? <p>Not enough reviews yet.</p> : <>
      <p><strong>{data.average.toFixed(1)} out of 5</strong> · {data.count} {data.count === 1 ? 'review' : 'reviews'}</p>
      <ul className="stringer-review-list">
        {data.reviews.map(review => <li key={review.id}>
          <p><strong>{review.rating} out of 5</strong> · Verified booking</p>
          {review.comment && <p className="review-comment">{review.comment}</p>}
          {canReport && <ReportForm business={business} review={review.id} />}
        </li>)}
      </ul>
    </>)}
  </section>
}

function ReportForm({ business, review }: { business: string; review: string }) {
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [sent, setSent] = useState(false)
  async function report(event: FormEvent) {
    event.preventDefault()
    if (busy) return
    setBusy(true)
    setError('')
    try {
      await marketplaceAPI.reportBookingReview(business, review, reason.trim())
      setSent(true)
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }
  if (sent) return <p role="status">Report sent to Restringing.ie.</p>
  return <details>
    <summary>Report review</summary>
    <form onSubmit={report}>
      <label>Reason for reporting<textarea rows={2} required maxLength={500} value={reason} onChange={event => setReason(event.target.value)} /></label>
      <button type="submit" disabled={busy || !reason.trim()}>{busy ? 'Sending…' : 'Send report'}</button>
      <Feedback error={error} message="" />
    </form>
  </details>
}
