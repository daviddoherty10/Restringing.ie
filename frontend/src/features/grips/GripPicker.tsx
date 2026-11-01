import { useCallback, useEffect, useState } from 'react'
import {
  marketplaceAPI,
  type GripOffering,
  type GripFilters,
} from '../../lib/marketplace-api'
import { errorMessage, useRemote } from '../management/use-remote'
import { LoadMessage, Pagination } from '../management/dashboard-shared'
import { euro } from '../management/dashboard-utils'
const label = (g: GripOffering) =>
  `${g.brand} ${g.model} · ${g.kind.replaceAll('_', ' ')} · ${g.colour || 'No colour label'}`
export default function GripPicker({
  stringerID,
  selectedID,
  selectedLabel,
  choose,
  onAvailability,
  locked,
  revision,
}: {
  stringerID: string
  selectedID: string
  selectedLabel: string
  choose: (grip: GripOffering | null) => void
  onAvailability: (available: boolean) => void
  locked: boolean
  revision: number
}) {
  const [query, setQuery] = useState(''),
    [kind, setKind] = useState<GripFilters['kind']>(),
    [page, setPage] = useState(1),
    [retry, setRetry] = useState(0)
  const [selection, setSelection] = useState<{
    id: string
    grip?: GripOffering
    error?: string
  }>({ id: '' })
  const load = useCallback(
    () =>
      marketplaceAPI.offeredGrips(stringerID, {
        q: query,
        kind,
        page,
        page_size: 5,
      }),
    [stringerID, query, kind, page],
  )
  const r = useRemote(
    `${stringerID}:${query}:${kind}:${page}:${retry}:${revision}`,
    load,
  )
  useEffect(() => {
    let active = true
    const request = selectedID
      ? marketplaceAPI.offeredGrips(stringerID, {
          grip_product_id: selectedID,
          page_size: 1,
        })
      : Promise.resolve({ data: [] as GripOffering[] })
    request
      .then((result) => {
        if (active) {
          const grip = result.data[0]
          setSelection({ id: selectedID, grip })
          onAvailability(!selectedID || !!grip)
        }
      })
      .catch((e) => {
        if (active) {
          setSelection({ id: selectedID, error: errorMessage(e) })
          onAvailability(false)
        }
      })
    return () => {
      active = false
    }
  }, [stringerID, selectedID, retry, revision, onAvailability])
  const checked = selection.id === selectedID
  return (
    <div className="grip-picker">
      <h3>Fresh grip?</h3>
      <p>
        Add one overgrip or replacement grip. The displayed price includes
        fitting.
      </p>
      <label className="finder-check">
        <input
          type="radio"
          name="grip"
          checked={!selectedID}
          disabled={locked}
          onChange={() => choose(null)}
        />
        No new grip
      </label>
      <div className="grip-picker-filters">
        <label className="booking-field">
          Search offered grips
          <input
            type="search"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value)
              setPage(1)
            }}
          />
        </label>
        <label className="booking-field">
          Grip type
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
      </div>
      <LoadMessage {...r} retry={() => setRetry((n) => n + 1)} />
      {r.data && (
        <>
          <div className="booking-choice-list">
            {r.data.data.map((g) => (
              <button
                type="button"
                key={g.id}
                disabled={locked}
                aria-pressed={selectedID === g.id}
                className={
                  selectedID === g.id
                    ? 'booking-option chosen'
                    : 'booking-option'
                }
                onClick={() => choose(g)}
              >
                <span>
                  <strong>
                    {g.brand} {g.model}
                  </strong>
                  <small>
                    {g.kind.replaceAll('_', ' ')} ·{' '}
                    {g.colour || 'No colour label'}
                  </small>
                </span>
                <span>{euro(g.fitted_price_cents)} fitted</span>
              </button>
            ))}
          </div>
          {r.data.total === 0 && (
            <p>
              No available grips match these filters. You can continue without a
              new grip.
            </p>
          )}
          <Pagination
            page={page}
            size={5}
            total={r.data.total}
            change={setPage}
          />
        </>
      )}
      {selectedID && (
        <p className="selected-string">
          Selected grip:{' '}
          {checked && selection.grip ? label(selection.grip) : selectedLabel}
        </p>
      )}
      {selectedID && checked && !selection.grip && (
        <p className="booking-banner" role="alert">
          {selection.error ||
            'Your selected grip is no longer available. Choose another grip or continue without one.'}{' '}
          <button type="button" onClick={() => setRetry((n) => n + 1)}>
            Recheck grips
          </button>
        </p>
      )}
    </div>
  )
}
