import { useState, type FormEvent } from 'react'
import './App.css'
import SignIn from './features/auth/SignIn'
import SessionMenu from './features/auth/SessionMenu'
import SearchPage from './features/stringers/SearchPage'
import ProfilePage from './features/stringers/ProfilePage'
import BookingPreview from './features/bookings/BookingPreview'
import Orders from './features/orders/Orders'
import Management from './features/management/Management'
import { BrandLogo } from './components/ui/BrandLogo'

const steps = [
  { number: '01', title: 'Find your stringer', description: 'Search for trusted racket stringers in your area.' },
  { number: '02', title: 'Choose your setup', description: 'Compare strings, prices and restringing services.' },
  { number: '03', title: 'Get back on court', description: 'Book your restring and follow your order.' },
]

function Arrow({ diagonal = false }: { diagonal?: boolean }) {
  return <span aria-hidden="true">{diagonal ? '↗' : '→'}</span>
}

function App() {
  const [menuOpen, setMenuOpen] = useState(false)
  const [location, setLocation] = useState('')
  const [searchMessage, setSearchMessage] = useState('')

  function handleSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!location.trim()) { setSearchMessage('Enter a town or county to start your search.'); return }
    window.location.assign('/search?location=' + encodeURIComponent(location.trim()))
  }

  const pathname = window.location.pathname.replace(/\/$/, '')
  if (pathname === '/signin') return <SignIn />
  if (pathname === '/search') return <SearchPage />
  if (pathname === '/orders') return <Orders />
  const orderMatch = pathname.match(/^\/orders\/([0-9a-fA-F-]{36})$/)
  if (orderMatch) return <Orders id={orderMatch[1]} />
  if (pathname === '/manage') return <Management />
  const bookingMatch = pathname.match(/^\/stringers\/([0-9a-fA-F-]{36})\/book$/)
  if (bookingMatch) return <BookingPreview id={bookingMatch[1]} />
  const profileMatch = pathname.match(/^\/stringers\/([0-9a-fA-F-]{36})$/)
  if (profileMatch) return <ProfilePage id={profileMatch[1]} />
  if (pathname !== '' && pathname !== '/') return <main className="not-found"><h1>Page not found</h1><p>We couldn't find that page.</p><a href="/">← Back to homepage</a></main>

  return (
    <>
      <header className="site-header">
        <div className="container nav-inner">
          <BrandLogo />
          <nav className={menuOpen ? 'navigation navigation-open' : 'navigation'} aria-label="Main navigation">
            <a href="#how-it-works" onClick={() => setMenuOpen(false)}>How it works</a>
            <a href="/search" onClick={() => setMenuOpen(false)}>Find a stringer</a>
            <a href="#why-restring" onClick={() => setMenuOpen(false)}>Why Restringing.ie</a>
            <a href="#for-stringers" onClick={() => setMenuOpen(false)}>For stringers</a>

          </nav>
          <SessionMenu />
          <button className="menu-toggle" type="button" aria-label={menuOpen ? 'Close menu' : 'Open menu'} aria-expanded={menuOpen} onClick={() => setMenuOpen(!menuOpen)}>
            <span /><span />
          </button>
        </div>
      </header>

      <main>
        <section className="hero-section">
          <div className="container hero-layout">
            <div className="hero-copy">
              <div className="eyebrow"><span className="eyebrow-line" /> A BETTER WAY TO RESTRING</div>
              <h1>Play better.<br /> <em>String smarter.</em></h1>
              <p className="hero-lead">Find trusted local racket stringers, choose the right strings, and get back to playing your best.</p>
              <form className="search-form" onSubmit={handleSearch}>
                <label className="search-label" htmlFor="location">FIND A RESTRINGER NEAR YOU</label>
                <div className="search-controls">
                  <span className="location-icon" aria-hidden="true">⌖</span>
                  <input id="location" type="search" placeholder="Town, city or Eircode" value={location} onChange={(event) => setLocation(event.target.value)} />
                  <button type="submit">Find a stringer <Arrow /></button>
                </div>
                {searchMessage && <p className="search-message" role="status">{searchMessage}</p>}
              </form>
              <div className="hero-note"><span className="note-bullet" /> Made for players. Powered by stringers.</div>
            </div>
            <div className="hero-visual">
              <div className="hero-photo" role="img" aria-label="Tennis racket and balls on a tennis court" />
              <div className="photo-accent" aria-hidden="true"><span>PRECISION</span><span>IN EVERY</span><span>STRING.</span></div>
              <div className="hero-image-label"><span>01 / 03</span><span>THE DIFFERENCE IS IN THE DETAILS</span></div>
            </div>
          </div>
        </section>

        <div className="ticker" aria-label="Restringing principles"><div className="ticker-inner"><span>YOUR RACKET. YOUR GAME.</span><span aria-hidden="true">✳</span><span>BETTER STRINGS. BETTER PLAY.</span><span aria-hidden="true">✳</span><span>LOCAL EXPERTS. REAL RESULTS.</span></div></div>

        <section id="how-it-works" className="section steps-section">
          <div className="container">
            <div className="section-heading">
              <div><p className="section-kicker">THE PROCESS</p><h2>Restringing,<br /><em>made simple.</em></h2></div>
              <p>No endless messages or guesswork. A simpler way to find the right person for your racket.</p>
            </div>
            <div className="steps-grid">
              {steps.map((step) => <article className="step-card" key={step.number}><span className="step-number">{step.number}</span><span className="step-symbol" aria-hidden="true"><Arrow diagonal /></span><h3>{step.title}</h3><p>{step.description}</p></article>)}
            </div>
          </div>
        </section>

        <section id="why-restring" className="feature-section">
          <div className="container feature-grid">
            <div className="feature-image" role="img" aria-label="Close-up of tennis racket strings" />
            <div className="feature-content">
              <p className="section-kicker">BUILT FOR THE GAME</p>
              <h2>Every detail<br /><em>matters.</em></h2>
              <p>Fresh strings change how your racket feels. We’re making quality restringing easier to discover, compare and book — without the hassle.</p>
              <div className="feature-points"><span>01 — Local expertise</span><span>02 — Choice of strings</span><span>03 — Simple booking</span></div>
              <a className="text-link" href="/search">Explore the marketplace <Arrow /></a>
            </div>
          </div>
        </section>

        <section id="for-stringers" className="stringers-section">
          <div className="container stringers-inner">
            <div><p className="section-kicker">FOR PROFESSIONAL STRINGERS</p><h2>Your craft.<br /><em>More customers.</em></h2></div>
            <div><p>Build your profile, showcase the strings you stock, set your prices and manage orders — all in one place.</p><a href="mailto:hello@restringing.ie?subject=Restringer%20interest" className="button-light">Register your interest <Arrow /></a></div>
          </div>
        </section>

        <section className="closing-section" id="coming-soon">
          <div className="container closing-inner"><span className="section-kicker">COMING SOON</span><h2>Your next great match<br />starts with <em>your strings.</em></h2><p>We’re building Ireland’s home for racket restringing.</p></div>
        </section>
      </main>

      <footer className="site-footer"><div className="container footer-inner"><BrandLogo /><span>Made for the love of the game.</span><span>© {new Date().getFullYear()} Restringing.ie</span></div></footer>
    </>
  )
}

export default App
