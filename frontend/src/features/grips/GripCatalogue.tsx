import { useCallback, useEffect, useState, type FormEvent } from 'react'
import {
  marketplaceAPI,
  type GripProduct,
  type GripFilters,
} from '../../lib/marketplace-api'
import type { components } from '../../lib/marketplace-contract'
import {
  Feedback,
  LoadMessage,
  Pagination,
} from '../management/dashboard-shared'
import { useRemote } from '../management/use-remote'
import { useKeyedWrite } from './use-keyed-write'
const label = (p: GripProduct) =>
  `${p.brand} ${p.model} · ${p.kind.replaceAll('_', ' ')} · ${p.colour || 'No colour label'}`
function Editor({
  product,
  close,
  saved,
  onBusy,
}: {
  product: GripProduct | null
  close: () => void
  saved: () => void
  onBusy: (busy: boolean) => void
}) {
  const [input, setInput] = useState<components['schemas']['GripInput']>(() =>
    product
      ? {
          brand: product.brand,
          model: product.model,
          kind: product.kind,
          colour: product.colour,
          description: product.description,
          active: product.active,
        }
      : {
          brand: '',
          model: '',
          kind: 'overgrip',
          colour: '',
          description: '',
          active: false,
        },
  )
  const write = useKeyedWrite((value: typeof input, key: string) =>
    product
      ? marketplaceAPI.admin.updateGrip(product.id, value)
      : marketplaceAPI.admin.createGrip(value, key),
  )
  useEffect(() => {
    onBusy(write.locked)
    return () => onBusy(false)
  }, [onBusy, write.locked])
  async function submit(e: FormEvent) {
    e.preventDefault()
    if (await write.run(input)) saved()
  }
  return (
    <section className="manage-panel">
      <div className="manage-section-heading">
        <h2>{product ? 'Edit grip product' : 'Create grip product'}</h2>
        <button type="button" disabled={write.locked} onClick={close}>
          Back to grips
        </button>
      </div>
      <form onSubmit={submit}>
        <fieldset disabled={write.locked}>
          <legend>Shared grip details</legend>
          <div className="manage-form-grid">
            <label>
              Grip brand
              <input
                required
                maxLength={100}
                value={input.brand}
                onChange={(e) =>
                  setInput((v) => ({ ...v, brand: e.target.value }))
                }
              />
            </label>
            <label>
              Grip model
              <input
                required
                maxLength={120}
                value={input.model}
                onChange={(e) =>
                  setInput((v) => ({ ...v, model: e.target.value }))
                }
              />
            </label>
            <label>
              Grip type
              <select
                value={input.kind}
                onChange={(e) =>
                  setInput((v) => ({
                    ...v,
                    kind: e.target.value as typeof v.kind,
                  }))
                }
              >
                <option value="overgrip">Overgrip</option>
                <option value="replacement_grip">Replacement grip</option>
              </select>
            </label>
            <label>
              Grip colour
              <input
                maxLength={80}
                value={input.colour}
                onChange={(e) =>
                  setInput((v) => ({ ...v, colour: e.target.value }))
                }
              />
            </label>
          </div>
          <label>
            Grip description
            <textarea
              rows={3}
              maxLength={1000}
              value={input.description}
              onChange={(e) =>
                setInput((v) => ({ ...v, description: e.target.value }))
              }
            />
          </label>
          <label className="manage-check">
            <input
              type="checkbox"
              checked={input.active}
              onChange={(e) =>
                setInput((v) => ({ ...v, active: e.target.checked }))
              }
            />
            Active in the shared grip catalogue
          </label>
          <p>
            Stringers set their own fitted prices and physical stock.
            Deactivation preserves historical orders.
          </p>
        </fieldset>
        <Feedback error={write.error} message="" />
        <button disabled={write.busy}>
          {write.busy
            ? 'Saving…'
            : write.uncertain
              ? 'Retry same grip save'
              : 'Save grip product'}
        </button>
      </form>
    </section>
  )
}
export default function GripCatalogue({
  onBusy,
}: {
  onBusy: (busy: boolean) => void
}) {
  const [query, setQuery] = useState(''),
    [kind, setKind] = useState<GripFilters['kind']>(),
    [active, setActive] = useState(''),
    [page, setPage] = useState(1),
    [revision, setRevision] = useState(0),
    [edit, setEdit] = useState<GripProduct | 'new' | null>(null),
    [message, setMessage] = useState('')
  const load = useCallback(
    () =>
      marketplaceAPI.admin.grips({
        q: query,
        kind,
        active: active === '' ? undefined : active === 'true',
        page,
        page_size: 10,
      }),
    [query, kind, active, page],
  )
  const r = useRemote(`${query}:${kind}:${active}:${page}:${revision}`, load)
  if (edit)
    return (
      <Editor
        key={edit === 'new' ? 'new' : edit.id}
        product={edit === 'new' ? null : edit}
        onBusy={onBusy}
        close={() => setEdit(null)}
        saved={() => {
          setEdit(null)
          setRevision((n) => n + 1)
          setMessage('Grip product saved.')
        }}
      />
    )
  return (
    <section className="manage-panel">
      <div className="manage-section-heading">
        <div>
          <h2>Grip catalogue</h2>
          <p>
            Shared overgrips and replacement grips. Each colour/type has its own
            product.
          </p>
        </div>
        <button onClick={() => setEdit('new')}>Create grip</button>
      </div>
      <div className="manage-filters">
        <label>
          Search grip products
          <input
            type="search"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value)
              setPage(1)
            }}
          />
        </label>
        <label>
          Catalogue grip type
          <select
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
        <label>
          Grip state
          <select
            value={active}
            onChange={(e) => {
              setActive(e.target.value)
              setPage(1)
            }}
          >
            <option value="">All states</option>
            <option value="true">Active</option>
            <option value="false">Inactive</option>
          </select>
        </label>
      </div>
      <Feedback error="" message={message} />
      <LoadMessage {...r} retry={() => setRevision((n) => n + 1)} />
      {r.data && (
        <>
          <div className="catalogue-list">
            {r.data.data.length ? (
              r.data.data.map((p) => (
                <article key={p.id}>
                  <div>
                    <h3>{label(p)}</h3>
                    <p>{p.active ? 'Active' : 'Inactive'}</p>
                    <p>{p.description}</p>
                  </div>
                  <button
                    aria-label={`Edit grip ${label(p)}`}
                    onClick={() => setEdit(p)}
                  >
                    Edit
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
          />
        </>
      )}
    </section>
  )
}
