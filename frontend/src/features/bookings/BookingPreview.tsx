import { useState } from 'react'
import { examples } from '../stringers/sampleStringers'
import { MarketplaceHeader, MarketplaceFooter } from '../../components/layout/MarketplaceLayout'
import './BookingPreview.css'

type StringChoice = 'own' | 'synthetic' | 'poly'
const stringOptions: { value: StringChoice; label: string; description: string; supplement: number }[] = [
  { value: 'own', label: 'Bring my own strings', description: 'Labour only — supply your own set', supplement: 0 },
  { value: 'synthetic', label: 'Synthetic gut', description: 'Example all-round, comfortable setup', supplement: 8 },
  { value: 'poly', label: 'Polyester', description: 'Example control-focused setup', supplement: 13 },
]
const euro = (amount: number) => new Intl.NumberFormat('en-IE', {style:'currency',currency:'EUR'}).format(amount)

export default function BookingPreview({ id }: { id: number }) {
  const stringer = examples.find((item) => item.id === id)
  const [sport, setSport] = useState('')
  const [choice, setChoice] = useState<StringChoice>('own')
  const [tension, setTension] = useState('24')
  const [notes, setNotes] = useState('')
  const [review, setReview] = useState(false)
  if (!stringer) return <main className="profile-missing"><h1>Stringer not found</h1><a href="/search">← Back to search</a></main>
  const selected = stringOptions.find((option) => option.value === choice)!
  const amount = stringer.fromPrice + selected.supplement
  const validTension = Number(tension) >= 15 && Number(tension) <= 35 && Number.isFinite(Number(tension))
  const ready = sport !== '' && validTension
  return <div className="booking-preview">
    <MarketplaceHeader backHref={`/stringers/${id}`} backLabel="Back to profile" />
    <main className="marketplace-container booking-content">
      <div className="booking-heading"><p className="marketplace-kicker">CONFIGURE YOUR RESTRING</p><h1>Make it <em>your game.</em></h1><p>Preview your racket setup and see an illustrative price before booking becomes available.</p></div>
      <div className="booking-banner" role="note"><strong>Demonstration only.</strong> The stringer and prices are fictional. This form is local to your browser; it does not create, save or send an order.</div>
      <div className="booking-grid">
        <section className="booking-form" aria-label="Racket configuration">
          <div className="booking-step"><span>01</span><div><h2>Choose your racket sport</h2><p>Only sports in this example stringer's listing are shown.</p><div className="booking-choice-grid">{stringer.racketTypes.map((type) => <button key={type} className={sport === type ? 'booking-option chosen' : 'booking-option'} aria-pressed={sport === type} type="button" onClick={() => {setSport(type);setReview(false)}}>{type}<span aria-hidden="true">{sport === type ? '✓' : '○'}</span></button>)}</div></div></div>
          <div className="booking-step"><span>02</span><div><h2>Choose your strings</h2><p>Example string categories; actual stock will come from the stringer's inventory.</p><div className="booking-choice-list">{stringOptions.map((option) => <button key={option.value} type="button" aria-pressed={choice === option.value} className={choice === option.value ? 'booking-option chosen' : 'booking-option'} onClick={() => {setChoice(option.value);setReview(false)}}><span><strong>{option.label}</strong><small>{option.description}</small></span><span>{option.supplement ? `+${euro(option.supplement)}` : 'Included'}</span></button>)}</div></div></div>
          <div className="booking-step"><span>03</span><div><h2>Tension & notes</h2><p>Enter your preferred tension in kilograms (15–35 kg). This is a preview input, not a stringing recommendation.</p><label className="booking-field" htmlFor="booking-tension">String tension (kg)<input id="booking-tension" type="number" step="0.5" min="15" max="35" value={tension} onChange={(event)=>{setTension(event.target.value);setReview(false)}} aria-invalid={!validTension} />{!validTension && <span role="alert">Enter a tension from 15 to 35 kg.</span>}</label><label className="booking-field" htmlFor="booking-notes">Optional notes<textarea id="booking-notes" rows={3} maxLength={500} placeholder="Racket model, preferences, or questions" value={notes} onChange={(event)=>{setNotes(event.target.value);setReview(false)}} /></label></div></div>
        </section>
        <aside className="booking-summary"><p className="marketplace-kicker">YOUR SETUP</p><h2>Order preview</h2><p className="booking-stringer">{stringer.name} · {stringer.area}</p><div className="booking-summary-line"><span>Racket</span><strong>{sport || 'Choose a sport'}</strong></div><div className="booking-summary-line"><span>Strings</span><strong>{selected.label}</strong></div><div className="booking-summary-line"><span>Tension</span><strong>{validTension ? `${tension} kg` : 'Needs attention'}</strong></div><div className="booking-summary-line"><span>Example labour</span><strong>{euro(stringer.fromPrice)}</strong></div><div className="booking-summary-line"><span>Example string supplement</span><strong>{euro(selected.supplement)}</strong></div><div className="booking-total"><span>Illustrative total</span><strong>{euro(amount)}</strong></div><p className="booking-price-note">Not a quotation. Real prices, stock, availability and any taxes or fees will be confirmed at checkout when supported.</p><button type="button" className="booking-review-button" disabled={!ready} onClick={()=>setReview(true)}>Review example setup →</button>{review && <div className="booking-reviewed" role="status"><strong>Setup preview complete.</strong><p>Your choices are displayed above. No order has been placed or saved.</p></div>}<button type="button" className="booking-disabled" disabled>Place order — coming soon</button></aside>
      </div>
    </main>
    <MarketplaceFooter />
  </div>
}
