import { useEffect, useRef, useState } from 'react'
import {
  marketplaceAPI,
  type RecommendationRequest,
} from '../../lib/marketplace-api'
import type { components } from '../../lib/marketplace-contract'
import { errorMessage } from '../management/use-remote'
import { euro } from '../management/dashboard-utils'
import './GuidedStringFinder.css'
type Recommendation = components['schemas']['Recommendation']
const goals = {
  balanced: {
    label: 'A good all-round setup — I’m not sure',
    weights: { comfort: 4, control: 3, power: 3, spin: 2, durability: 3 },
  },
  comfort: {
    label: 'A softer, more forgiving feel',
    weights: { comfort: 5, control: 2, power: 3, spin: 1, durability: 2 },
  },
  power: {
    label: 'Get the ball deeper with less effort',
    weights: { comfort: 4, control: 2, power: 5, spin: 1, durability: 2 },
  },
  control: {
    label: 'Keep my shots inside the court',
    weights: { comfort: 3, control: 5, power: 1, spin: 3, durability: 3 },
  },
  spin: {
    label: 'Help the ball dip into the court with spin',
    weights: { comfort: 3, control: 4, power: 2, spin: 5, durability: 3 },
  },
  durability: {
    label: 'Make my strings last longer between breaks',
    weights: { comfort: 3, control: 3, power: 2, spin: 2, durability: 5 },
  },
} as const
export default function GuidedStringFinder({
  stringerID,
  racketID,
  disabled,
  onOpenChange,
  choose,
}: {
  stringerID: string
  racketID: string
  disabled: boolean
  onOpenChange: (open: boolean) => void
  choose: (r: Recommendation) => void
}) {
  const [open, setOpen] = useState(false)
  const [experience, setExperience] =
    useState<RecommendationRequest['experience']>('new')
  const [frequency, setFrequency] = useState(1)
  const [goal, setGoal] = useState<keyof typeof goals>('balanced')
  const [comfort, setComfort] = useState(false)
  const [result, setResult] = useState<
    components['schemas']['Recommendations'] | null
  >(null)
  const [busy, setBusy] = useState(false),
    [error, setError] = useState('')
  const seq = useRef(0),
    controller = useRef<AbortController | null>(null)
  useEffect(
    () => () => {
      seq.current++
      controller.current?.abort()
    },
    [],
  )
  function changed() {
    seq.current++
    controller.current?.abort()
    setResult(null)
    setError('')
    setBusy(false)
  }
  async function find() {
    controller.current?.abort()
    const abort = new AbortController()
    controller.current = abort
    const request = ++seq.current
    setBusy(true)
    setError('')
    setResult(null)
    try {
      const next = await marketplaceAPI.recommend(
        stringerID,
        {
          racket_model_id: racketID,
          experience,
          sessions_per_week: frequency,
          preferences: { ...goals[goal].weights },
          prioritise_arm_comfort: comfort,
        },
        abort.signal,
      )
      if (seq.current === request) setResult(next)
    } catch (e) {
      if (seq.current === request && !abort.signal.aborted)
        setError(errorMessage(e))
    } finally {
      if (seq.current === request) setBusy(false)
    }
  }
  const complete = result?.results.filter((r) => !!r.setup) ?? []
  function card(r: Recommendation, i: number) {
    const setup = r.setup!
    return (
      <article
        className="guided-setup-card"
        key={`${r.variant.id}:${setup.booking.crosses_variant_id ?? 'full'}`}
        aria-label={`Suggested setup ${i + 1}`}
      >
        <p className="marketplace-kicker">
          {i === 0 ? 'OUR STARTING SUGGESTION' : 'ANOTHER OPTION'}
        </p>
        <h4>
          {r.variant.brand} {r.variant.model}
          {setup.crosses_variant &&
            ` + ${setup.crosses_variant.brand} ${setup.crosses_variant.model}`}
        </h4>
        <p>{setup.summary}</p>
        <p>
          {r.variant.gauge_mm} mm · {r.variant.colour}
          {setup.crosses_variant &&
            ` / ${setup.crosses_variant.gauge_mm} mm · ${setup.crosses_variant.colour}`}
        </p>
        <p>
          <strong>
            {setup.booking.mains_tension_centi_kg / 100} kg in both directions
          </strong>{' '}
          ·{' '}
          {setup.booking.hybrid
            ? 'Two strings combined'
            : 'One string throughout'}
        </p>
        <p className="guided-setup-price">
          {euro(setup.estimated_total_cents)} estimated, including labour
        </p>
        <ul>
          {setup.explanations.map((reason) => (
            <li key={reason}>{reason}</li>
          ))}
        </ul>
        <details>
          <summary>How this suggestion was chosen</summary>
          <p>
            {setup.booking.hybrid ? 'Mains match score' : 'String match score'}:{' '}
            {r.score}. Scores compare editorial catalogue attributes; they
            aren’t a measured performance rating.
          </p>
          <ul>
            {r.reasons.map((reason) => (
              <li key={reason}>{reason}</li>
            ))}
          </ul>
          <p>Estimated price breakdown:</p>
          <ul>
            {setup.breakdown.map((line) => (
              <li key={line.code}>
                {line.label}: {euro(line.amount_cents)}
              </li>
            ))}
          </ul>
        </details>
        <button
          type="button"
          className="finder-submit"
          disabled={disabled}
          onClick={() => {
            choose(r)
            setOpen(false)
            onOpenChange(false)
          }}
        >
          Use this setup
        </button>
      </article>
    )
  }
  return (
    <section className="guided-finder" aria-label="Guided setup finder">
      <button
        type="button"
        className="finder-launch"
        aria-expanded={open}
        onClick={() => {
          setOpen(!open)
          onOpenChange(!open)
        }}
        disabled={disabled}
      >
        Help me choose a complete setup{' '}
        <span aria-hidden="true">{open ? '−' : '+'}</span>
      </button>
      {open && (
        <div className="finder">
          <h3>No stringing knowledge needed.</h3>
          <p>
            Tell us how you play. We’ll choose strings, a starting tension and
            the stringing method. No need to decide on knots or extra
            treatments.
          </p>
          <label className="booking-field">
            How would you describe your play?
            <select
              value={experience}
              onChange={(e) => {
                changed()
                setExperience(e.target.value as typeof experience)
              }}
            >
              <option value="new">I’m new or still learning</option>
              <option value="club">
                I play regularly and can rally comfortably
              </option>
              <option value="advanced">
                I’m experienced and play competitive matches
              </option>
            </select>
          </label>
          <label className="booking-field">
            How often do you usually play?
            <select
              value={frequency}
              onChange={(e) => {
                changed()
                setFrequency(Number(e.target.value))
              }}
            >
              <option value={0}>Less than once a week</option>
              <option value={1}>About once a week</option>
              <option value={2}>About twice a week</option>
              <option value={4}>Three or more times a week</option>
            </select>
          </label>
          <label className="booking-field">
            What would help you most?
            <select
              value={goal}
              onChange={(e) => {
                changed()
                setGoal(e.target.value as keyof typeof goals)
              }}
            >
              {Object.entries(goals).map(([key, g]) => (
                <option value={key} key={key}>
                  {g.label}
                </option>
              ))}
            </select>
          </label>
          <label className="finder-check">
            <input
              type="checkbox"
              checked={comfort}
              onChange={(e) => {
                changed()
                setComfort(e.target.checked)
              }}
            />
            Please put a softer feel first (optional)
          </label>
          <p>
            We don’t need injury details. A softer-feel preference is not a
            medical assessment.
          </p>
          <button
            type="button"
            className="finder-submit"
            disabled={busy || disabled}
            onClick={find}
          >
            {busy ? 'Choosing your setup…' : 'Find my complete setup'}
          </button>
          {error && (
            <div role="alert">
              <p>{error}</p>
              <button type="button" onClick={find}>
                Retry finder
              </button>
            </div>
          )}
          {result && (
            <div className="finder-results">
              <p>{result.notice}</p>
              {complete.length === 0 && (
                <p role="status">
                  No complete starting setup is available. Ask this stringer for
                  help or try another stringer.
                </p>
              )}
              {complete[0] && card(complete[0], 0)}
              {complete.length > 1 && (
                <details className="guided-alternatives">
                  <summary>See {complete.length - 1} other setups</summary>
                  {complete.slice(1).map((r, i) => card(r, i + 1))}
                </details>
              )}
            </div>
          )}
        </div>
      )}
    </section>
  )
}
