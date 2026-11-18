import { useCallback, useState } from 'react'
import { MarketplaceHeader, MarketplaceFooter } from '../../components/layout/MarketplaceLayout'
import { marketplaceAPI } from '../../lib/marketplace-api'
import { Feedback, LoadMessage, Pagination } from '../management/dashboard-shared'
import { errorMessage, useRemote } from '../management/use-remote'
import './reviews.css'

export default function AdminReviews() {
  const [page, setPage] = useState(1)
  const [revision, setRevision] = useState(0)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const load = useCallback(() => marketplaceAPI.reportedBookingReviews(page), [page])
  const result = useRemote(`${page}:${revision}`, load)
  function refresh() { setRevision(n => n + 1) }
  async function decide(id: string, version: number, hide: boolean) {
    if (busy) return
    setBusy(true)
    setError('')
    setMessage('')
    try {
      await marketplaceAPI.moderateBookingReview(id, { version, hide })
      if (result.data?.data.length === 1 && page > 1) setPage(n => n - 1)
      refresh()
      setMessage(hide ? 'Review hidden from the public profile.' : 'Report dismissed. Review kept.')
    } catch (e) {
      setError(errorMessage(e))
      refresh()
    } finally {
      setBusy(false)
    }
  }
  return <div className="marketplace">
    <MarketplaceHeader />
    <main id="main-content" tabIndex={-1} className="marketplace-container manage-content">
      <h1>Reported reviews</h1>
      <p>Review reports for abuse, personal information or irrelevant content. Keep feedback that describes the booking, including criticism.</p>
      <LoadMessage {...result} retry={refresh} />
      <Feedback error={error} message={message} />
      {result.data && <>
        {result.data.data.length === 0 && <p>No reported reviews.</p>}
        {result.data.data.map(review => <article className="manage-panel" key={review.id}>
          <h2>{review.business}</h2>
          <p>{review.rating} out of 5</p>
          {review.comment && <p className="review-comment">{review.comment}</p>}
          <p className="review-comment"><strong>Report reason</strong> {review.reason}</p>
          <div className="manage-actions">
            <button type="button" disabled={busy} onClick={() => void decide(review.id, review.version, false)}>Keep review</button>
            <button type="button" disabled={busy} onClick={() => void decide(review.id, review.version, true)}>Hide review</button>
          </div>
        </article>)}
        <Pagination page={page} size={result.data.page_size} total={result.data.total} change={setPage} busy={busy} />
      </>}
    </main>
    <MarketplaceFooter />
  </div>
}
