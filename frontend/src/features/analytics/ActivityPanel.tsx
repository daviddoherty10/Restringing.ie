import { useCallback, useState } from 'react'
import { marketplaceAPI } from '../../lib/marketplace-api'
import type { components } from '../../lib/marketplace-contract'
import { LoadMessage } from '../management/dashboard-shared'
import { useRemote } from '../management/use-remote'
import './ActivityPanel.css'

type Report = components['schemas']['ActivityReport']
type Usage = components['schemas']['StringUsage']
const monthLabel = (month: string, year = false) => new Intl.DateTimeFormat('en-IE', {
  month: 'short', ...(year && { year: 'numeric' }), timeZone: 'UTC',
}).format(new Date(month + '-01T00:00:00Z'))

function ActivityChart({ report, platform }: { report: Report; platform: boolean }) {
  const [period, setPeriod] = useState(6)
  const [series, setSeries] = useState(platform ? 'bookings' : 'restrings')
  const months = report.months.slice(-period)
  const products = new Map<string, Usage>()
  for (const month of report.months) {
    const inPeriod = month.month >= months[0].month
    for (const product of month.strings) {
      const prior = products.get(product.product_id)
      if (prior) prior.restrings += inPeriod ? product.restrings : 0
      else products.set(product.product_id, { ...product, restrings: inPeriod ? product.restrings : 0 })
    }
  }
  const ranked = [...products.values()].sort((a, b) => b.restrings - a.restrings || a.name.localeCompare(b.name))
  const selected = products.get(series)
  const used = ranked.filter(p => p.restrings > 0)
  const businesses = series === 'businesses'
  const bookings = series === 'bookings'
  const title = selected ? selected.name : businesses ? 'Stringers receiving bookings' : bookings ? 'Bookings received' : 'Completed restrings'
  const values = months.map(month => selected
    ? month.strings.find(p => p.product_id === series)?.restrings ?? 0
    : businesses ? month.booking_businesses : bookings ? month.bookings : month.restrings)
  const peak = Math.max(1, ...values)
  const total = values.reduce((sum, n) => sum + n, 0)
  return <>
    <div className="activity-controls">
      <label>Graph<select value={series} onChange={e => setSeries(e.target.value)}>
        <option value="restrings">Completed restrings</option>
        <option value="bookings">Bookings received</option>
        {platform && <option value="businesses">Stringers receiving bookings</option>}
        {ranked.length > 0 && <optgroup label="Restrings by string">{ranked.map(p => <option key={p.product_id} value={p.product_id}>{p.name}</option>)}</optgroup>}
      </select></label>
      <label>Period<select value={period} onChange={e => setPeriod(Number(e.target.value))}>
        {[3, 6, 12].map(n => <option key={n} value={n}>Last {n} months</option>)}
      </select></label>
    </div>
    <div className="activity-layout">
      <figure className="activity-chart">
        <figcaption><strong>{(businesses ? values.at(-1) ?? 0 : total).toLocaleString('en-IE')}</strong><span>{title}{businesses && ' this month'}{selected && ' · completed restrings'}</span></figcaption>
        <ol aria-label={title + ' by month'}>{months.map((month, i) =>
          <li key={month.month} aria-label={monthLabel(month.month, true) + ', ' + values[i]}>
            <div className="activity-track"><b>{values[i]}</b><i style={{ height: (values[i] / peak * 80) + '%' }} /></div>
            <span>{monthLabel(month.month)}</span>
          </li>)}</ol>
        {months.length > 0 && <small>{monthLabel(months[0].month, true)} – {monthLabel(months[months.length - 1].month, true)} · Current month to date</small>}
      </figure>
      <section className="activity-popular"><h3>Most used strings</h3><p>Completed rackets in this period</p>
        {used.length ? <ol>{used.slice(0, 5).map(p => <li key={p.product_id}>
          <button type="button" onClick={() => setSeries(p.product_id)} aria-pressed={series === p.product_id}>
            <span>{p.name}</span><strong>{p.restrings}</strong>
          </button></li>)}</ol> : <p>No catalogue strings recorded on completed rackets.</p>}
      </section>
    </div>
    <p className="manage-muted">Bookings count when placed. Restrings count when completed. Each catalogue model counts once per racket, including hybrids. Rackets with unidentified strings count towards the restring total.</p>
  </>
}

export default function ActivityPanel({ business, revision = 0 }: { business?: string; revision?: number }) {
  const [retry, setRetry] = useState(0)
  const load = useCallback(() => business ? marketplaceAPI.businessActivity(business) : marketplaceAPI.admin.activity(), [business])
  const report = useRemote(`${business ?? 'platform'}:${revision}:${retry}`, load)
  return <section className="manage-panel activity-panel" aria-label="Activity analytics">
    <h2>{business ? 'Restring activity' : 'Platform activity'}</h2>
    <LoadMessage {...report} retry={() => setRetry(n => n + 1)} />
    {report.data && <ActivityChart report={report.data} platform={!business} />}
  </section>
}
