import StringPhoto from '../strings/StringPhoto'
import { useEffect, useRef, useState } from 'react'
import {
  marketplaceAPI,
  type RecommendationRequest,
} from '../../lib/marketplace-api'
import type { components } from '../../lib/marketplace-contract'
import { errorMessage } from '../management/use-remote'
import { savedTensionLabel, tensionBandLabel } from '../../lib/marketplace-units'
import { euro } from '../management/dashboard-utils'
import './GuidedStringFinder.css'
type Recommendation = components['schemas']['Recommendation']
const goals = {
  balanced: {
    title: 'Balanced',
    label: 'A mix of comfort, depth and spin',
    weights: { comfort: 4, control: 3, power: 3, spin: 2, durability: 3 },
  },
  comfort: {
    title: 'Comfort',
    label: 'A softer, more forgiving feel',
    weights: { comfort: 5, control: 2, power: 3, spin: 1, durability: 2 },
  },
  power: {
    title: 'Easy depth',
    label: 'Get the ball deeper with less effort',
    weights: { comfort: 4, control: 2, power: 5, spin: 1, durability: 2 },
  },
  spin: {
    title: 'Spin',
    label: 'Help the ball dip into the court with spin',
    weights: { comfort: 3, control: 4, power: 2, spin: 5, durability: 3 },
  },
} as const
// These sketches explain a preference, rather than predicting shot performance.
function GoalSketch({ goal }: { goal: keyof typeof goals }) {
  const paths = {
    balanced: ['M16 24H64M40 14V46M30 46H50', 'M23 24L15 38H31L23 24M57 24L49 38H65L57 24', 'M15 38Q23 48 31 38M49 38Q57 48 65 38'],
    comfort: ['M10 25Q25 8 40 25T70 25', 'M10 37Q25 20 40 37T70 37'],
    power: ['M10 44Q34 7 68 19', 'M57 12L68 19L59 28'],
    spin: ['M18 35C4 8 62 3 64 28C66 47 29 56 22 38', 'M14 41L22 38L28 46'],
  }
  return <svg className="goal-sketch" viewBox="0 0 80 60" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
    {paths[goal].map((d, i) => <path key={i} d={d} pathLength="1" />)}
  </svg>
}
function SetupSketch({ hybrid }: { hybrid: boolean }) {
  return <svg className={`setup-sketch${hybrid ? ' setup-sketch-hybrid' : ''}`} viewBox="0 0 120 160" aria-hidden="true" fill="none">
    <g stroke="currentColor" strokeWidth="2">
      {[36, 48, 60, 72, 84].map((x) => <path key={x} d={`M${x} ${x === 36 || x === 84 ? 26 : 17}V${x === 36 || x === 84 ? 94 : 103}`} />)}
    </g>
    <g className="setup-crosses" strokeWidth="2">
      {[30, 42, 54, 66, 78, 90].map((y) => <path key={y} d={`M${y === 30 || y === 90 ? 30 : 23} ${y}H${y === 30 || y === 90 ? 90 : 97}`} />)}
    </g>
    <ellipse cx="60" cy="60" rx="39" ry="47" stroke="currentColor" strokeWidth="5" />
    <path d="M42 104L56 129M78 104L64 129M56 129V153H64V129" stroke="currentColor" strokeWidth="5" strokeLinejoin="round" />
  </svg>
}
export default function GuidedStringFinder({
  savedSetup,
  tensionBand,
  savedRoundPoly,
  stringerID,
  racketID,
  sport,
  disabled,
  onOpenChange,
  choose,
}: {
  stringerID: string
  racketID: string
  sport: RecommendationRequest['sport']
  savedSetup?: components['schemas']['Booking']
  tensionBand?: RecommendationRequest['tension_band']
  savedRoundPoly: boolean
  disabled: boolean
  onOpenChange: (open: boolean) => void
  choose: (r: Recommendation) => void
}) {
  const [open, setOpen] = useState(false)
  const [level, setLevel] = useState<'new' | 'club' | 'advanced'>('new')
  const [goal, setGoal] = useState<keyof typeof goals>('balanced')
  const [ageGroup, setAgeGroup] = useState<NonNullable<RecommendationRequest['age_group']> | ''>('')
  const isJunior = sport === 'tennis' && ageGroup !== '' && ageGroup !== 'adult'
  const [breaksSyntheticGut, setBreaksSyntheticGut] = useState(false)
  const [roundPoly, setRoundPoly] = useState('')
  const [preferThicker, setPreferThicker] = useState(false)
  const [comfort, setComfort] = useState(false)
  const useSavedRoundPoly = savedRoundPoly && ageGroup !== '12_to_14'
  const grouped = level === 'advanced' ? 'class_1_4' : level === 'club' ? 'class_5_7' : 'beginner'
  const olderJunior = ageGroup === '12_to_14' || ageGroup === '15_to_17'
  const askAboutRoundPoly = isJunior && olderJunior && level === 'advanced' && goal === 'spin' && !comfort
  const askAboutBreakage = isJunior && olderJunior && level === 'advanced' && goal === 'balanced' && !comfort
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
          ...(racketID ? {racket_model_id: racketID} : {}),
          sport,
          experience: sport === 'tennis' && !isJunior ? grouped : level,
          goal,
          ...(tensionBand ? { tension_band: tensionBand } : {}),
          ...(preferThicker ? { prefer_thicker: true } : {}),
          ...(askAboutRoundPoly && (roundPoly === 'yes' || !roundPoly && useSavedRoundPoly) ? {
            round_poly_full_bed: true,
            ...(savedRoundPoly && savedSetup?.variant_id ? { current_variant_id: savedSetup.variant_id } : {}),
          } : {}),
          ...(!tensionBand ? { tension_help: true } : {}),
          ...(sport === 'tennis' && ageGroup ? { age_group: ageGroup } : {}),
          ...(askAboutBreakage && breaksSyntheticGut
            ? { breaks_synthetic_gut: true }
            : {}),
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
  const comparison = complete.length === 2 && complete.every(r => !!r.label)
  function card(r: Recommendation, i: number) {
    const setup = r.setup!
    return (
      <article
        className="guided-setup-card"
        key={`${r.variant.id}:${setup.booking.crosses_variant_id ?? 'full'}`}
        aria-label={r.label ?? `Suggested setup ${i + 1}`}
      >
        <p className="marketplace-kicker">
          {r.label ?? (i === 0 ? 'OUR STARTING SUGGESTION' : 'ANOTHER OPTION')}
        </p>
        <h4>
          {r.variant.brand} {r.variant.model}
          {setup.crosses_variant &&
            ` + ${setup.crosses_variant.brand} ${setup.crosses_variant.model}`}
        </h4>
        {(r.variant.has_photo || setup.crosses_variant?.has_photo) && <div className="string-photos">
          {[r.variant, setup.crosses_variant].map((variant, index) => variant?.has_photo && <figure key={index}>
            <StringPhoto variant={variant} />
            <figcaption>{setup.crosses_variant ? (index === 0 ? 'Mains · ' : 'Crosses · ') : ''}{variant.colour || 'String photo'}</figcaption>
          </figure>)}
        </div>}
        <div className="setup-explainer">
          <SetupSketch hybrid={!!setup.booking.hybrid} />
          <div>
            <strong>{setup.booking.hybrid ? 'Two strings, working together' : 'One string, in both directions'}</strong>
            <p>{setup.summary}</p>
            <p className="setup-key"><span className="setup-key-main" /> Mains (vertical): {r.variant.brand} {r.variant.model}</p>
            <p className={`setup-key${setup.booking.hybrid ? ' setup-key-cross' : ''}`}><span /> Crosses (horizontal): {setup.crosses_variant ? `${setup.crosses_variant.brand} ${setup.crosses_variant.model}` : 'the same string'}</p>
            <small>Diagram shows string directions, not product colours.</small>
          </div>
        </div>
        <p>
          {r.variant.gauge_mm} mm · {r.variant.colour}
          {setup.crosses_variant &&
            ` / ${setup.crosses_variant.gauge_mm} mm · ${setup.crosses_variant.colour}`}
        </p>
        <p>
          <strong>
            {setup.booking.tension_pending ? tensionBandLabel(setup.booking.tension_band) || 'Tension to agree with your stringer' : savedTensionLabel(setup.booking)}
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
            Tell us how you play. We’ll suggest strings and a stringing setup. Your stringer will help confirm tension. No need to decide on knots or extra
            treatments.
          </p>
          <ol className="finder-overview" aria-label="How the finder works">
            <li><span aria-hidden="true">1</span> Tell us how you play</li>
            <li><span aria-hidden="true">2</span> Compare a suggested setup</li>
            <li><span aria-hidden="true">3</span> Review before booking</li>
          </ol>
          {sport === 'tennis' && (
            <label className="booking-field">
              Player age group
              <select value={ageGroup} onChange={(e) => {
                changed()
                setRoundPoly(''); setBreaksSyntheticGut(false)
                setAgeGroup(e.target.value as typeof ageGroup)
              }}>
                <option value="" disabled>Choose age group</option>
                <option value="under_11_inclusive">11 and under</option>
                <option value="12_to_14">12–14</option>
                <option value="15_to_17">15–17</option>
                <option value="adult">18+</option>
              </select>
            </label>
          )}
          <label className="booking-field">
            How would you describe your play?
            <select
              value={level}
              onChange={(e) => {
                changed()
                setLevel(e.target.value as typeof level)
              }}
            >
              <option value="advanced">{sport === 'tennis' ? 'Class 1–4' : 'Advanced'}</option>
              <option value="club">{sport === 'tennis' ? 'Class 5–7' : 'Club'}</option>
              <option value="new">Beginner</option>
            </select>
          </label>
          <fieldset className="goal-options" disabled={disabled}>
            <legend>What would help you most?</legend>
            <p>Choose the priority that matters most to you.</p>
            <div className="goal-grid">
              {Object.entries(goals).map(([key, g]) => (
                <label className={`goal-option${goal === key ? ' goal-selected' : ''}`} key={key}>
                  <input type="radio" name={`setup-goal-${stringerID}`} value={key} checked={goal === key}
                    onChange={() => { changed(); setGoal(key as keyof typeof goals) }} />
                  <GoalSketch key={`${key}:${goal === key}`} goal={key as keyof typeof goals} />
                  <strong>{g.title}</strong>
                  <span>{g.label}</span>
                </label>
              ))}
            </div>
          </fieldset>
          {askAboutRoundPoly && (
            <label className="booking-field">
              {ageGroup === '12_to_14' ? 'Are they at least 13 and already using round polyester in the whole racket?' : 'Do they already use round polyester in the whole racket?'}
              <select value={roundPoly || (useSavedRoundPoly ? 'yes' : 'unsure')} onChange={e => { changed(); setRoundPoly(e.target.value) }}>
                <option value="unsure">I’m not sure</option>
                <option value="yes">Yes, in mains and crosses</option>
                <option value="no">No</option>
              </select>
            </label>
          )}
          {askAboutBreakage && (
            <label className="finder-check">
              <input type="checkbox" checked={breaksSyntheticGut} onChange={(e) => {
                changed()
                setBreaksSyntheticGut(e.target.checked)
              }} />
              {ageGroup === '12_to_14' ? 'They are at least 13 and regularly break synthetic gut' : 'They regularly break synthetic gut'}
            </label>
          )}
          <label className="finder-check">
            <input type="checkbox" checked={preferThicker} onChange={e => { changed(); setPreferThicker(e.target.checked) }} />
            Prefer a thicker string for durability
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
            disabled={busy || disabled || sport === 'tennis' && !ageGroup}
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
          <p className="finder-feedback" role="status">{busy ? 'Matching your preferences with this stringer’s available strings…' : comparison ? 'Two setups are ready to compare.' : result && complete.length > 0 ? 'Your suggestion is ready. Review the strings and estimated price below.' : ''}</p>
          {result && (
            <div className="finder-results">
              <p>{result.notice}</p>
              {complete.length === 0 && (
                <p role="status">
                  No complete starting setup is available. Ask this stringer for
                  help or try another stringer.
                </p>
              )}
              {comparison ? (
                <div className="guided-setup-comparison">{complete.map(card)}</div>
              ) : (
                <>
                  {complete[0] && card(complete[0], 0)}
                  {complete.length > 1 && (
                    <details className="guided-alternatives">
                      <summary>See {complete.length - 1} other setups</summary>
                      {complete.slice(1).map((r, i) => card(r, i + 1))}
                    </details>
                  )}
                </>
              )}
            </div>
          )}
        </div>
      )}
    </section>
  )
}
