import { useEffect, useState, type FormEvent } from 'react'
import { MarketplaceFooter, MarketplaceHeader } from '../../components/layout/MarketplaceLayout'
import { safeReturnTo, signInURL } from '../../lib/auth-navigation'
import { marketplaceAPI, MarketplaceError } from '../../lib/marketplace-api'
import type { components } from '../../lib/marketplace-contract'
import './auth.css'

type Profile = components['schemas']['AccountProfile']

export default function AccountProfile() {
  const params = new URLSearchParams(window.location.search)
  const setup = params.has('returnTo')
  const required = params.get('required') === '1'
  const returnTo = safeReturnTo(params.get('returnTo'))
  const [profile, setProfile] = useState<Profile>()
  const [revision, setRevision] = useState(0)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [fields, setFields] = useState<Record<string, string>>({})
  const [saved, setSaved] = useState(false)
  const [signedOut, setSignedOut] = useState(false)

  useEffect(() => {
    let active = true
    marketplaceAPI.accountProfile().then(value => {
      if (active) setProfile(value)
    }).catch(e => {
      if (!active) return
      if (e instanceof MarketplaceError && e.status === 401) setSignedOut(true)
      else setError('Could not load your profile. Please try again.')
    })
    return () => { active = false }
  }, [revision])

  async function save(event: FormEvent) {
    event.preventDefault()
    if (!profile || busy) return
    setBusy(true)
    setError('')
    setFields({})
    setSaved(false)
    try {
      const updated = await marketplaceAPI.updateAccountProfile(profile)
      if (setup) window.location.assign(returnTo)
      else { setProfile(updated); setSaved(true) }
    } catch (e) {
      if (e instanceof MarketplaceError && e.status === 401) setSignedOut(true)
      else if (e instanceof MarketplaceError && Object.keys(e.fields).length > 0) setFields(e.fields)
      else setError('Could not save your profile. Please try again.')
    } finally {
      setBusy(false)
    }
  }

  function change(key: keyof Profile, value: string) {
    setProfile(current => current && { ...current, [key]: value })
    setFields(current => ({ ...current, [key]: '' }))
    setSaved(false)
  }

  return <div className="marketplace">
    <MarketplaceHeader />
    <main id="main-content" tabIndex={-1} className="auth-content marketplace-container">
      <section className="auth-card">
        <h1>{setup ? 'Finish setting up your account' : 'Your profile'}</h1>
        <p>{required ? 'Add the name your stringer should use before you place your booking.' : 'Add the name you’d like your stringer to use.'}</p>
        {signedOut ? <a href={signInURL(setup ? returnTo : '/account/profile')}>Sign in to continue</a>
          : profile ? <form onSubmit={save}>
            <fieldset disabled={busy}>
              <legend className="auth-visually-hidden">Personal details</legend>
              <label>First name<input autoComplete="given-name" required maxLength={100} value={profile.first_name} onChange={e => change('first_name', e.target.value)} aria-invalid={!!fields.first_name} aria-describedby={fields.first_name ? 'first-name-error' : undefined} /></label>
              {fields.first_name && <p className="auth-error" id="first-name-error">{fields.first_name}</p>}
              <label>Last name (optional)<input autoComplete="family-name" maxLength={100} value={profile.last_name} onChange={e => change('last_name', e.target.value)} aria-invalid={!!fields.last_name} aria-describedby={fields.last_name ? 'last-name-error' : undefined} /></label>
              {fields.last_name && <p className="auth-error" id="last-name-error">{fields.last_name}</p>}
              <label>Email address (optional)<input type="email" autoComplete="email" maxLength={254} value={profile.email} onChange={e => change('email', e.target.value)} aria-invalid={!!fields.email} aria-describedby={fields.email ? 'profile-email-help email-error' : 'profile-email-help'} /></label>
              {fields.email && <p className="auth-error" id="email-error">{fields.email}</p>}
              <p id="profile-email-help" className="auth-note">Save an email for order confirmations when they become available.</p>
              <button className="auth-primary" type="submit">{busy ? 'Saving…' : setup ? 'Save and continue' : 'Save profile'}</button>
            </fieldset>
            {Object.values(fields).some(Boolean) && <p role="alert" className="auth-visually-hidden">Check the highlighted fields.</p>}
            {saved && <p role="status">Profile saved.</p>}
          </form> : error ? <button type="button" onClick={() => { setError(''); setRevision(value => value + 1) }}>Retry</button> : <p role="status">Loading your profile…</p>}
        {error && <p className="auth-error" role="alert">{error}</p>}
        {setup && !required && !signedOut && <p className="auth-note"><a href={returnTo}>Skip for now</a></p>}
      </section>
    </main>
    <MarketplaceFooter />
  </div>
}
