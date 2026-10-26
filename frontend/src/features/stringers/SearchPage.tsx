import { useMemo, useState, type FormEvent } from 'react'
import './SearchPage.css'

type Stringer = {
  id: number
  name: string
  area: string
  county: string
  experience: string
  fromPrice: number
  turnaround: string
  racketTypes: string[]
  initials: string
}

const examples: Stringer[] = [
  { id: 1, name: 'Baseline Stringing', area: 'Dún Laoghaire', county: 'Dublin', experience: 'Tennis specialist', fromPrice: 20, turnaround: '2–3 days', racketTypes: ['Tennis'], initials: 'BS' },
  { id: 2, name: 'The Racket Workshop', area: 'Blackrock', county: 'Dublin', experience: 'Tennis & badminton', fromPrice: 22, turnaround: '1–2 days', racketTypes: ['Tennis', 'Badminton'], initials: 'RW' },
  { id: 3, name: 'Court Craft', area: 'Sandyford', county: 'Dublin', experience: 'All racket sports', fromPrice: 18, turnaround: '3–4 days', racketTypes: ['Tennis', 'Squash', 'Badminton'], initials: 'CC' },
  { id: 4, name: 'Match Point Strings', area: 'Galway City', county: 'Galway', experience: 'Tennis specialist', fromPrice: 24, turnaround: '2–3 days', racketTypes: ['Tennis'], initials: 'MP' },
  { id: 5, name: 'String Lab', area: 'Cork City', county: 'Cork', experience: 'Tennis & squash', fromPrice: 21, turnaround: '1–2 days', racketTypes: ['Tennis', 'Squash'], initials: 'SL' },
  { id: 6, name: 'Rally Ready', area: 'Limerick City', county: 'Limerick', experience: 'Tennis & badminton', fromPrice: 19, turnaround: '2–4 days', racketTypes: ['Tennis', 'Badminton'], initials: 'RR' },
]

type Sport = 'All rackets' | 'Tennis' | 'Badminton' | 'Squash'
type Sort = 'recommended' | 'price-low' | 'price-high'

export default function SearchPage() {
  const initialLocation = new URLSearchParams(window.location.search).get('location') ?? ''
  const [locationInput, setLocationInput] = useState(initialLocation)
  const [location, setLocation] = useState(initialLocation)
  const [sport, setSport] = useState<Sport>('All rackets')
  const [sort, setSort] = useState<Sort>('recommended')

  const matches = useMemo(() => {
    const term = location.trim().toLocaleLowerCase('en-IE')
    const result = examples.filter((stringer) => {
      const matchesLocation = !term || [stringer.area, stringer.county].some((value) => value.toLocaleLowerCase('en-IE').includes(term))
      const matchesSport = sport === 'All rackets' || stringer.racketTypes.includes(sport)
      return matchesLocation && matchesSport
    })
    if (sort === 'price-low') result.sort((a, b) => a.fromPrice - b.fromPrice)
    if (sort === 'price-high') result.sort((a, b) => b.fromPrice - a.fromPrice)
    return result
  }, [location, sport, sort])

  function submitSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setLocation(locationInput)
    const url = new URL(window.location.href)
    if (locationInput.trim()) url.searchParams.set('location', locationInput.trim())
    else url.searchParams.delete('location')
    window.history.replaceState(null, '', url)
  }

  return <div className="marketplace">
    <header className="marketplace-header">
      <div className="marketplace-container marketplace-nav">
        <a className="marketplace-brand" href="/"><span className="marketplace-brand-icon">R<span>.</span></span>restringing<span className="marketplace-brand-domain">.ie</span></a>
        <a href="/" className="marketplace-home">← Back to home</a>
      </div>
    </header>
    <main>
      <section className="marketplace-intro">
        <div className="marketplace-container">
          <p className="marketplace-kicker">THE RESTRINGING MARKETPLACE</p>
          <h1>Find your <em>perfect stringer.</em></h1>
          <p className="marketplace-subtitle">Discover local racket restringing services and find the right setup for your game.</p>
          <div className="marketplace-demo" role="note"><strong>Preview only</strong> — these are fictional sample listings. Prices, locations and turnaround times are illustrative; bookings aren't open yet.</div>
          <form className="marketplace-search" onSubmit={submitSearch}>
            <label htmlFor="marketplace-location">SEARCH BY LOCATION</label>
            <div className="marketplace-search-row">
              <input id="marketplace-location" type="search" placeholder="Town or county, e.g. Dublin" value={locationInput} onChange={(event) => setLocationInput(event.target.value)} />
              <button type="submit">Search <span aria-hidden="true">→</span></button>
            </div>
          </form>
        </div>
      </section>
      <section className="marketplace-results">
        <div className="marketplace-container">
          <div className="marketplace-toolbar">
            <div>
              <span className="marketplace-results-count">{matches.length} sample {matches.length === 1 ? 'listing' : 'listings'}</span>
              <h2>Explore stringers</h2>
            </div>
            <div className="marketplace-sort">
              <label htmlFor="sort-listings">Sort by</label>
              <select id="sort-listings" value={sort} onChange={(event) => setSort(event.target.value as Sort)}>
                <option value="recommended">Recommended</option>
                <option value="price-low">Price: low to high</option>
                <option value="price-high">Price: high to low</option>
              </select>
            </div>
          </div>
          <div className="marketplace-filters" aria-label="Filter by racket sport">
            {(['All rackets', 'Tennis', 'Badminton', 'Squash'] as Sport[]).map((name) => <button key={name} type="button" className={name === sport ? 'marketplace-filter active' : 'marketplace-filter'} aria-pressed={name === sport} onClick={() => setSport(name)}>{name}</button>)}
          </div>
          {matches.length ? <div className="marketplace-grid">
            {matches.map((stringer) => <article className="marketplace-card" key={stringer.id}>
              <div className="marketplace-card-banner"><span className="marketplace-card-monogram">{stringer.initials}</span><span className="marketplace-card-label">SAMPLE LISTING</span></div>
              <div className="marketplace-card-body">
                <div className="marketplace-card-heading"><div><h3>{stringer.name}</h3><p>⌖ {stringer.area}, {stringer.county}</p></div><span className="marketplace-from">From <strong>€{stringer.fromPrice}</strong></span></div>
                <div className="marketplace-tags">{stringer.racketTypes.map((type) => <span key={type}>{type}</span>)}</div>
                <div className="marketplace-card-meta"><span>{stringer.experience}</span><span>Typical turnaround: {stringer.turnaround}</span></div>
                <button type="button" disabled aria-label={`Booking for ${stringer.name} coming soon`}>Booking coming soon <span aria-hidden="true">↗</span></button>
              </div>
            </article>)}
          </div> : <div className="marketplace-empty"><h3>No sample listings found</h3><p>Try searching another town or county, or choose a different racket sport. These are demonstration results, not actual service availability.</p><button type="button" onClick={() => {setLocation('');setLocationInput('');setSport('All rackets');const url = new URL(window.location.href);url.searchParams.delete('location');window.history.replaceState(null, '', url)}}>Clear filters</button></div>}
        </div>
      </section>
    </main>
    <footer className="marketplace-footer"><div className="marketplace-container"><span>Restringing.ie</span><span>Made for players. Powered by stringers.</span></div></footer>
  </div>
}
