import { useCallback, useEffect, useState, type FormEvent } from 'react'
import {
  marketplaceAPI,
  type GripProduct,
  type GripStock,
  type GripFilters,
  type ManagementProfile,
} from '../../lib/marketplace-api'
import type { components } from '../../lib/marketplace-contract'
import {
  Feedback,
  LoadMessage,
  Pagination,
} from '../management/dashboard-shared'
import { errorMessage, useRemote } from '../management/use-remote'
import { decimalUnits, euro } from '../management/dashboard-utils'
import { useKeyedWrite } from './use-keyed-write'
const label = (p: GripProduct) =>
  `${p.brand} ${p.model} · ${p.kind.replaceAll('_', ' ')} · ${p.colour || 'No colour label'}`
function History({
  businessID,
  productID,
  revision,
}: {
  businessID: string
  productID: string
  revision: number
}) {
  const [page, setPage] = useState(1),
    [retry, setRetry] = useState(0)
  const load = useCallback(
    () =>
      marketplaceAPI.gripMovements(businessID, productID, {
        page,
        page_size: 10,
      }),
    [businessID, productID, page],
  )
  const r = useRemote(
    `${businessID}:${productID}:${page}:${revision}:${retry}`,
    load,
  )
  return (
    <section className="grip-history" aria-label="Grip stock history">
      <div className="manage-section-heading">
        <h4>Grip stock history</h4>
        <button type="button" onClick={() => setRetry((n) => n + 1)}>
          Refresh grip history
        </button>
      </div>
      <LoadMessage {...r} retry={() => setRetry((n) => n + 1)} />
      {r.data && (
        <>
          <ol>
            {r.data.data.map((m) => (
              <li key={m.id}>
                <strong>
                  {m.quantity_delta > 0 ? '+' : ''}
                  {m.quantity_delta} physical ·{' '}
                  {m.reserved_delta > 0 ? '+' : ''}
                  {m.reserved_delta} reserved
                </strong>
                <p>{m.reason}</p>
                <small>
                  {new Date(m.created_at).toLocaleString('en-IE', {
                    timeZone: 'Europe/Dublin',
                  })}{' '}
                  ·{' '}
                  {m.order_id
                    ? `Order ${m.order_id.slice(0, 8)}`
                    : 'Manual stock change'}
                </small>
              </li>
            ))}
          </ol>
          {r.data.total === 0 && <p>No grip stock changes yet.</p>}
          <Pagination
            page={page}
            size={10}
            total={r.data.total}
            change={setPage}
          />
        </>
      )}
    </section>
  )
}
function Settings({
  businessID,
  product,
  initial,
  saved,
  close,
  onBusy,
}: {
  businessID: string
  product: GripProduct
  initial?: GripStock
  saved: () => void
  close: () => void
  onBusy: (busy: boolean) => void
}) {
  const [stock, setStock] = useState(initial),
    [price, setPrice] = useState(
      initial ? String(initial.fitted_price_cents / 100) : '',
    ),
    [available, setAvailable] = useState(initial?.available ?? true)
  const [delta, setDelta] = useState(''),
    [reason, setReason] = useState(''),
    [message, setMessage] = useState(''),
    [error, setError] = useState(''),
    [revision, setRevision] = useState(0),
    [refreshing, setRefreshing] = useState(false)
  const offering = useKeyedWrite(
    (p: components['schemas']['GripOfferingInput']) =>
      marketplaceAPI.saveGripOffering(businessID, product.id, p),
  )
  const correction = useKeyedWrite(
    (p: components['schemas']['GripStockInput'], key: string) =>
      marketplaceAPI.adjustGripStock(businessID, product.id, p, key),
  )
  const locked = offering.locked || correction.locked || refreshing
  useEffect(() => {
    onBusy(locked)
    return () => onBusy(false)
  }, [onBusy, locked])
  async function save(e: FormEvent) {
    e.preventDefault()
    setError('')
    setMessage('')
    const cents = decimalUnits(price, 2)
    if (cents === null || cents > 1000000) {
      setError(
        'Enter a fitted price from €0 to €10,000 with at most two decimal places.',
      )
      return
    }
    const result = await offering.run({ fitted_price_cents: cents, available })
    if (result) {
      setStock(result)
      setMessage(
        'Grip offering saved. Receive physical stock below before it can be booked.',
      )
      saved()
    }
  }
  async function refreshStock() {
    setRefreshing(true)
    try {
      const result = await marketplaceAPI.manageGrips(businessID, {
        grip_product_id: product.id,
        page_size: 1,
      })
      setStock(result.data[0])
      setRevision((n) => n + 1)
      setError('')
    } catch (e) {
      setError('Could not refresh the grip balance. ' + errorMessage(e))
    } finally {
      setRefreshing(false)
    }
  }
  async function adjust(e: FormEvent) {
    e.preventDefault()
    setError('')
    setMessage('')
    const n = /^-?[0-9]+$/.test(delta) ? Number(delta) : NaN
    if (
      !Number.isSafeInteger(n) ||
      n === 0 ||
      Math.abs(n) > 1000000 ||
      !reason.trim()
    ) {
      setError('Enter a nonzero whole-unit receipt/correction and a reason.')
      return
    }
    if (await correction.run({ quantity_delta: n, reason: reason.trim() })) {
      setDelta('')
      setReason('')
      setMessage('Grip stock change recorded.')
      saved()
      await refreshStock()
    }
  }
  return (
    <section className="manage-editor" aria-label="Grip offering settings">
      <div className="manage-section-heading">
        <h3>{label(product)}</h3>
        <button type="button" disabled={locked} onClick={close}>
          Close grip settings
        </button>
      </div>
      {!product.active && (
        <p>
          This grip is inactive in the shared catalogue. You can manage existing
          stock; customers cannot book it.
        </p>
      )}
      <form onSubmit={save}>
        <fieldset disabled={locked}>
          <legend>Fitted price and availability</legend>
          <label>
            Fitted grip price (€)
            <input
              required
              inputMode="decimal"
              value={price}
              onChange={(e) => setPrice(e.target.value)}
            />
          </label>
          <label className="manage-check">
            <input
              type="checkbox"
              checked={available}
              onChange={(e) => setAvailable(e.target.checked)}
            />
            Offer this grip to customers
          </label>
          <p>
            This price includes the grip and fitting. Saving a price does not
            receive stock.
          </p>
        </fieldset>
        <Feedback error={offering.error} message="" />
        <button disabled={offering.busy || correction.locked || refreshing}>
          {offering.busy
            ? 'Saving…'
            : offering.uncertain
              ? 'Retry same offering save'
              : 'Save grip offering'}
        </button>
      </form>
      {stock && (
        <>
          <p className="grip-balance">
            <strong>
              {stock.quantity} physical · {stock.reserved} reserved ·{' '}
              {stock.quantity - stock.reserved} free
            </strong>
          </p>
          <button type="button" disabled={locked} onClick={refreshStock}>
            Refresh grip balance
          </button>
          <form onSubmit={adjust}>
            <fieldset disabled={locked}>
              <legend>Receive or correct grip stock</legend>
              <label>
                Grip units to add or remove
                <input
                  required
                  inputMode="numeric"
                  value={delta}
                  onChange={(e) => setDelta(e.target.value)}
                  placeholder="e.g. 10 or -2"
                />
              </label>
              <label>
                Grip stock reason
                <textarea
                  required
                  maxLength={500}
                  rows={2}
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                />
              </label>
              <p>
                Use whole individual grips. A negative correction cannot remove
                units reserved for orders.
              </p>
            </fieldset>
            <Feedback error={correction.error} message="" />
            <button disabled={correction.busy || offering.locked || refreshing}>
              {correction.busy
                ? 'Saving…'
                : correction.uncertain
                  ? 'Retry same stock change'
                  : 'Record grip stock change'}
            </button>
          </form>
          <History
            businessID={businessID}
            productID={product.id}
            revision={revision}
          />
        </>
      )}
      <Feedback error={error} message={message} />
    </section>
  )
}
function Editor({
  businessID,
  product,
  saved,
  close,
  onBusy,
}: {
  businessID: string
  product: GripProduct
  saved: () => void
  close: () => void
  onBusy: (busy: boolean) => void
}) {
  const [retry, setRetry] = useState(0)
  const load = useCallback(
    () =>
      marketplaceAPI.manageGrips(businessID, {
        grip_product_id: product.id,
        page_size: 1,
      }),
    [businessID, product.id],
  )
  const r = useRemote(`${businessID}:${product.id}:${retry}`, load)
  return (
    <>
      <LoadMessage {...r} retry={() => setRetry((n) => n + 1)} />
      {!r.data && (
        <button type="button" onClick={close}>
          Close grip settings
        </button>
      )}
      {r.data && (
        <Settings
          key={`${businessID}:${product.id}`}
          businessID={businessID}
          product={r.data.data[0] ?? product}
          initial={r.data.data[0]}
          saved={saved}
          close={close}
          onBusy={onBusy}
        />
      )}
    </>
  )
}
export default function GripsPanel({
  business,
}: {
  business: ManagementProfile
}) {
  const [mode, setMode] = useState('stock'),
    [query, setQuery] = useState(''),
    [kind, setKind] = useState<GripFilters['kind']>(),
    [page, setPage] = useState(1),
    [revision, setRevision] = useState(0),
    [edit, setEdit] = useState<GripProduct | null>(null),
    [busy, setBusy] = useState(false)
  const load = useCallback(() => {
    const f = { q: query, kind, page, page_size: 10 }
    return mode === 'catalogue'
      ? marketplaceAPI.grips(f)
      : marketplaceAPI.manageGrips(business.id, f)
  }, [mode, query, kind, page, business.id])
  const r = useRemote<
    | components['schemas']['GripProductPage']
    | components['schemas']['GripStockPage']
  >(`${business.id}:${mode}:${query}:${kind}:${page}:${revision}`, load)
  if (!business.permissions.manage_inventory)
    return <p>You do not have permission to manage grip stock.</p>
  return (
    <section className="manage-panel">
      <h2>Grips and fitted prices</h2>
      <p>
        Stock overgrips or replacement grips from the shared catalogue. Every
        price includes fitting.
      </p>
      <nav className="manage-tabs" aria-label="Grip sections">
        <button
          type="button"
          disabled={busy || !!edit}
          aria-pressed={mode === 'stock'}
          onClick={() => {
            setMode('stock')
            setPage(1)
          }}
        >
          Stocked grips
        </button>
        <button
          type="button"
          disabled={busy || !!edit}
          aria-pressed={mode === 'catalogue'}
          onClick={() => {
            setMode('catalogue')
            setPage(1)
          }}
        >
          Add grip from catalogue
        </button>
      </nav>
      <div className="manage-filters">
        <label>
          Search grips
          <input
            disabled={busy || !!edit}
            type="search"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value)
              setPage(1)
            }}
          />
        </label>
        <label>
          Stock grip type
          <select
            disabled={busy || !!edit}
            value={kind ?? ''}
            onChange={(e) => {
              setKind((e.target.value as GripFilters['kind']) || undefined)
              setPage(1)
            }}
          >
            <option value="">All types</option>
            <option value="overgrip">Overgrip</option>
            <option value="replacement_grip">Replacement grip</option>
          </select>
        </label>
      </div>
      <LoadMessage {...r} retry={() => setRevision((n) => n + 1)} />
      {r.data && (
        <>
          <div className="manage-string-list">
            {r.data.data.length ? (
              r.data.data.map((g) => (
                <article key={g.id}>
                  <div>
                    <h3>{label(g)}</h3>
                    {'quantity' in g && (
                      <p>
                        {euro(g.fitted_price_cents)} fitted · {g.quantity}{' '}
                        physical · {g.reserved} reserved ·{' '}
                        {g.quantity - g.reserved} free ·{' '}
                        {g.available && g.active ? 'Enabled' : 'Unavailable'}
                      </p>
                    )}
                    <p>{g.description}</p>
                  </div>
                  <button
                    type="button"
                    disabled={!!edit || busy}
                    onClick={() => setEdit(g)}
                  >
                    {mode === 'catalogue' ? 'Select grip' : 'Edit grip / stock'}
                  </button>
                </article>
              ))
            ) : (
              <p>No grips match these filters.</p>
            )}
          </div>
          <Pagination
            page={page}
            size={10}
            total={r.data.total}
            change={setPage}
            busy={busy || !!edit}
          />
        </>
      )}
      {edit && (
        <Editor
          key={`${business.id}:${edit.id}`}
          businessID={business.id}
          product={edit}
          close={() => setEdit(null)}
          saved={() => setRevision((n) => n + 1)}
          onBusy={setBusy}
        />
      )}
    </section>
  )
}
