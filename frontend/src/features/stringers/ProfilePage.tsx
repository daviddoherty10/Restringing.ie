import { examples } from './sampleStringers'
import './ProfilePage.css'

export default function ProfilePage({ id }: { id: number }) {
  const stringer = examples.find((item) => item.id === id)
  if (!stringer) {
    return <main className="profile-missing"><h1>Stringer not found</h1><p>That sample listing doesn't exist.</p><a href="/search">← Return to search</a></main>
  }

  return <div className="profile-page">
    <header className="marketplace-header">
      <div className="marketplace-container marketplace-nav">
        <a className="marketplace-brand" href="/"><span className="marketplace-brand-icon">R<span>.</span></span>restringing<span className="marketplace-brand-domain">.ie</span></a>
        <a href="/search" className="marketplace-home">← All stringers</a>
      </div>
    </header>
    <main className="marketplace-container">
      <nav className="profile-breadcrumb" aria-label="Breadcrumb"><a href="/">Home</a><span>/</span><a href="/search">Find a stringer</a><span>/</span><span aria-current="page">{stringer.name}</span></nav>
      <div className="profile-notice" role="note"><strong>Example profile.</strong> This stringer is fictional. All services, prices and turnaround times are illustrative. Booking is not yet available.</div>
      <div className="profile-hero">
        <div className="profile-banner"><span>{stringer.initials}</span><small>SAMPLE STRINGER</small></div>
        <div className="profile-overview">
          <p className="marketplace-kicker">RACKET RESTRINGING</p>
          <h1>{stringer.name}</h1>
          <p className="profile-location">⌖ {stringer.area}, {stringer.county}</p>
          <p className="profile-about">A preview of how a restringer can showcase their expertise, racket sports, available services and estimated turnaround time on Restringing.ie.</p>
          <div className="profile-sports">{stringer.racketTypes.map((sport) => <span key={sport}>{sport}</span>)}</div>
        </div>
      </div>
      <div className="profile-layout">
        <section className="profile-services">
          <p className="profile-section-label">THE SERVICES</p>
          <h2>Find your next setup.</h2>
          <p className="profile-section-intro">Service offerings will eventually come directly from each stringer's inventory and pricing dashboard.</p>
          <div className="profile-service-card">
            <div><h3>Racket restring</h3><p>Standard restringing service — sample price, not a live quotation.</p><span>{stringer.experience}</span></div>
            <strong>From €{stringer.fromPrice}</strong>
          </div>
          <div className="profile-service-card profile-service-muted">
            <div><h3>String selection</h3><p>Compare string brands, gauges and prices once inventory integration is available.</p><span>Product catalogue coming soon</span></div>
            <span aria-label="Unavailable">—</span>
          </div>
          <section className="profile-details"><h2>About this stringer</h2><p>Each real stringer will be able to describe their experience, stringing equipment, specialities and handover options here. This preview intentionally does not invent those details.</p></section>
        </section>
        <aside className="profile-booking" aria-label="Booking preview">
          <p className="profile-section-label">READY TO RESTRING?</p>
          <h2>Let's get you back on court.</h2>
          <div className="profile-price"><span>Indicative price from</span><strong>€{stringer.fromPrice}</strong></div>
          <div className="profile-turnaround"><span>Typical turnaround (sample)</span><strong>{stringer.turnaround}</strong></div>
          <button type="button" disabled>Booking coming soon ↗</button>
          <p>Real prices, availability and order tracking will appear when the marketplace backend is connected.</p>
        </aside>
      </div>
    </main>
    <footer className="marketplace-footer"><div className="marketplace-container"><span>Restringing.ie</span><span>Made for players. Powered by stringers.</span></div></footer>
  </div>
}
