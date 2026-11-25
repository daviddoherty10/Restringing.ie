import { useEffect, useState, type FormEvent } from 'react'
import type { components } from '../../lib/marketplace-contract'
import { marketplaceAPI, MarketplaceError } from '../../lib/marketplace-api'
import { internationalPhone, profileSetupURL, safeReturnTo } from '../../lib/auth-navigation'
import { BrandLogo } from '../../components/ui/BrandLogo'
import { MarketplaceFooter } from '../../components/layout/MarketplaceLayout'
import { identifyUser } from '../../telemetry'
import './auth.css'

type Challenge = components['schemas']['AuthChallenge']
type Config = components['schemas']['AuthConfig']
const message = (e: unknown) => e instanceof MarketplaceError ? e.message : 'Unable to connect. Please try again.'
export default function SignIn() {
  const [config, setConfig] = useState<Config | null>(null)
  const [loading, setLoading] = useState(true)
  const [reload, setReload] = useState(0)
  const [phone, setPhone] = useState('')
  const [callingCode, setCallingCode] = useState('+353')
  const [challenge, setChallenge] = useState<Challenge | null>(null)
  const [sentTo, setSentTo] = useState('')
  const [code, setCode] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [resendAt, setResendAt] = useState(0)
  const [now, setNow] = useState(()=>Date.now())
  const returnTo = safeReturnTo(new URLSearchParams(window.location.search).get('returnTo'))
  useEffect(() => {
    let active = true
    marketplaceAPI.auth.config().then(c => { if (active) { setConfig(c); setCallingCode(c.allowed_calling_codes.includes('+353') ? '+353' : c.allowed_calling_codes[0] ?? '+353'); setError('') } }).catch(e => { if (active) setError(message(e)) }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [reload])
  useEffect(() => { const timer = window.setInterval(() => setNow(Date.now()), 1000); return () => clearInterval(timer) }, [])
  const remaining = Math.max(0, Math.ceil((resendAt - now) / 1000))
  const expired = !!challenge && Date.parse(challenge.expires_at) <= now
  async function send(e?: FormEvent) {
    e?.preventDefault(); if (busy || remaining > 0) return
    setBusy(true); setError('')
    const number = challenge ? sentTo : internationalPhone(phone, callingCode)
    try {
      const result = await marketplaceAPI.auth.requestCode(number)
      setChallenge(result); setSentTo(number); setCode(''); setNow(Date.now()); setResendAt(Date.now() + result.resend_after_seconds * 1000)
    } catch (e) {
      setError(message(e))
      if (e instanceof MarketplaceError && e.status === 429) setResendAt(Date.now() + e.retryAfterSeconds * 1000)
    } finally { setBusy(false) }
  }
  async function verify(e: FormEvent) {
    e.preventDefault(); if (!challenge || expired || busy) return
    setBusy(true); setError('')
    try { const session = await marketplaceAPI.auth.verifyCode(challenge.challenge_id, code); identifyUser(session.id); window.location.assign(session.needs_profile ? profileSetupURL(returnTo) : returnTo) }
    catch (e) { setError(message(e)); setBusy(false) }
  }
  return <div className="marketplace"><header className="marketplace-header"><div className="marketplace-container marketplace-nav"><BrandLogo variant="marketplace" /><a href={returnTo}>← Back</a></div></header><main id="main-content" tabIndex={-1} className="auth-content marketplace-container"><section className="auth-card auth-signin"><img className="auth-brand-mark" src="/favicon.svg" alt="" width="46" height="46" /><h1>{challenge ? 'Check your phone.' : 'Welcome to court.'}</h1><p>{challenge ? 'Enter the one-time code from your message.' : 'Use your mobile number to sign in or create an account.'}</p>
    {loading ? <p role="status">Checking SMS availability…</p> : !config ? <button onClick={() => { setLoading(true); setReload(n => n + 1) }}>Retry connection</button> : !config.available ? <p role="status">SMS sign-in is temporarily unavailable. Please try again later.</p> : !challenge ? <form onSubmit={send}><label>Country calling code<select value={callingCode} onChange={e => setCallingCode(e.target.value)}>{config.allowed_calling_codes.map(c => <option key={c} value={c}>{c === '+353' ? 'Ireland (+353)' : c === '+44' ? 'United Kingdom (+44)' : c}</option>)}</select></label><label>Mobile number<input type="tel" autoComplete="tel" required maxLength={40} value={phone} onChange={e => setPhone(e.target.value)} placeholder="087 123 4567" /></label><p className="auth-note">We’ll text you a one-time code to sign in. <a href="/privacy">Privacy</a> · <a href="/cookies">Cookies</a></p><button className="auth-primary" disabled={busy || remaining > 0}>{busy ? 'Sending…' : remaining ? `Try again in ${remaining}s` : 'Send sign-in code →'}</button></form> : <><p>Code sent to <strong>{sentTo}</strong>.</p><form onSubmit={verify}><label>One-time code<input type="text" inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{4,10}" minLength={4} maxLength={10} required autoFocus value={code} onChange={e => setCode(e.target.value.replace(/[^0-9]/g, ''))} /></label>{expired && <p role="alert">This code has expired. Request another code.</p>}<button className="auth-primary" disabled={busy || expired}>{busy ? 'Checking…' : 'Sign in →'}</button></form><div className="auth-actions"><button type="button" disabled={busy || remaining > 0} onClick={() => send()}>{remaining ? `Resend in ${remaining}s` : 'Resend code'}</button><button type="button" disabled={busy} onClick={() => { setChallenge(null); setCode(''); setError('') }}>Change number</button></div></>}
    {error && <p className="auth-error" role="alert">{error}</p>}
  </section></main><MarketplaceFooter /></div>
}
