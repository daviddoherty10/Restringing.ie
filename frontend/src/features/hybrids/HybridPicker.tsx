import { useCallback, useEffect, useState } from 'react'
import { marketplaceAPI, type Variant } from '../../lib/marketplace-api'
import { LoadMessage, Pagination } from '../management/dashboard-shared'
import { useRemote, errorMessage } from '../management/use-remote'
import { euro } from '../management/dashboard-utils'
export default function HybridPicker({
  stringerID,
  racketID,
  component,
  selected,
  choose,
  onAvailability,
  locked,
  revision,
}: {
  stringerID: string
  racketID: string
  component: 'mains' | 'crosses'
  selected: Variant | null
  choose: (v: Variant) => void
  onAvailability: (v: boolean) => void
  locked: boolean
  revision: number
}) {
  const [query, setQuery] = useState(''),
    [material, setMaterial] = useState(''),
    [page, setPage] = useState(1),
    [retry, setRetry] = useState(0)
  const [checked, setChecked] = useState({
    id: '',
    available: false,
    error: '',
  })
  const load = useCallback(
    () =>
      marketplaceAPI.hybridInventory(stringerID, racketID, component, {
        q: query,
        material: (material as Variant['material']) || undefined,
        page,
        page_size: 5,
      }),
    [stringerID, racketID, component, query, material, page],
  )
  const r = useRemote(
    `${stringerID}:${racketID}:${component}:${query}:${material}:${page}:${retry}:${revision}`,
    load,
  )
  const selectedID = selected?.id ?? ''
  useEffect(() => {
    let active = true
    const request = selectedID
      ? marketplaceAPI.hybridInventory(stringerID, racketID, component, {
          variant_id: selectedID,
          page_size: 1,
        })
      : Promise.resolve({ data: [] as Variant[] })
    request
      .then((result) => {
        if (active) {
          const available = !!result.data[0]
          setChecked({ id: selectedID, available, error: '' })
          onAvailability(available)
        }
      })
      .catch((e) => {
        if (active) {
          setChecked({
            id: selectedID,
            available: false,
            error: errorMessage(e),
          })
          onAvailability(false)
        }
      })
    return () => {
      active = false
    }
  }, [
    stringerID,
    racketID,
    component,
    selectedID,
    onAvailability,
    retry,
    revision,
  ])
  return (
    <section
      className="hybrid-picker"
      aria-label={`${component} string selection`}
    >
      <h3>{component === 'mains' ? 'Mains string' : 'Crosses string'}</h3>
      <div className="grip-picker-filters">
        <label className="booking-field">
          Search {component} strings
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
          {component === 'mains' ? 'Mains' : 'Crosses'} material
          <select
            value={material}
            onChange={(e) => {
              setMaterial(e.target.value)
              setPage(1)
            }}
          >
            <option value="">All materials</option>
            {[
              'multifilament',
              'synthetic_gut',
              'polyester',
              'natural_gut',
              'nylon',
            ].map((m) => (
              <option value={m} key={m}>
                {m.replaceAll('_', ' ')}
              </option>
            ))}
          </select>
        </label>
      </div>
      <LoadMessage {...r} retry={() => setRetry((n) => n + 1)} />
      {r.data && (
        <>
          <div className="booking-choice-list">
            {r.data.data.map((v) => (
              <button
                type="button"
                key={v.id}
                disabled={locked}
                aria-pressed={selectedID === v.id}
                className={
                  selectedID === v.id
                    ? 'booking-option chosen'
                    : 'booking-option'
                }
                onClick={() => choose(v)}
              >
                <span>
                  <strong>
                    {v.brand} {v.model}
                  </strong>
                  <small>
                    {v.gauge_mm} mm · {v.colour} ·{' '}
                    {v.material.replaceAll('_', ' ')}
                  </small>
                </span>
                <span>
                  {euro(
                    (component === 'mains'
                      ? v.mains_price_cents
                      : v.crosses_price_cents) ?? 0,
                  )}
                </span>
              </button>
            ))}
          </div>
          {r.data.total === 0 && (
            <p>No offered reel-cut strings match this selection.</p>
          )}
          <Pagination
            page={page}
            size={5}
            total={r.data.total}
            change={setPage}
          />
        </>
      )}
      {selected && (
        <p>
          Selected {component}: {selected.brand} {selected.model} ·{' '}
          {selected.gauge_mm} mm · {selected.colour}
        </p>
      )}
      {selectedID && checked.id === selectedID && !checked.available && (
        <p role="alert">
          {checked.error ||
            `The selected ${component} string is no longer available. Choose another string or a full-bed setup.`}
        </p>
      )}
      <p>
        Each component needs a sufficient reel. The server checks the combined
        stock requirement when you request a quote.
      </p>
    </section>
  )
}
