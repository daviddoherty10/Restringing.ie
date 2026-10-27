import './auth.css'
import { useEffect, useRef, useState } from 'react'
import { marketplaceAPI, MarketplaceError } from '../../lib/marketplace-api'
import { clearBookingDrafts, signInURL } from '../../lib/auth-navigation'
export default function SessionMenu() {
  const [signedIn, setSignedIn] = useState(false), [admin, setAdmin] = useState(false), [loading, setLoading] = useState(true), [busy, setBusy] = useState(false), [error, setError] = useState(''), [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null), toggle = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    let active = true
    marketplaceAPI.me().then(me => { if (active) { setSignedIn(true); setAdmin(me.admin) } }).catch(() => {}).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [])
  useEffect(() => {
    const media = window.matchMedia('(max-width: 1050px)')
    const resize = () => setOpen(false)
    media.addEventListener('change', resize)
    return () => media.removeEventListener('change', resize)
  }, [])
  useEffect(() => {
    if (!open) return
    const outside = (e: PointerEvent) => { if (!root.current?.contains(e.target as Node)) setOpen(false) }
    const escape = (e: KeyboardEvent) => { if (e.key === 'Escape') { setOpen(false); toggle.current?.focus() } }
    window.addEventListener('pointerdown', outside); window.addEventListener('keydown', escape)
    return () => { window.removeEventListener('pointerdown', outside); window.removeEventListener('keydown', escape) }
  }, [open])
  async function logout() {
    if (busy) return
    setBusy(true); setError('')
    try { await marketplaceAPI.auth.logout(); clearBookingDrafts(); window.location.assign('/') }
    catch (e) { setError(e instanceof MarketplaceError ? e.message : 'Could not sign out. Please retry.'); setBusy(false) }
  }
  return <div ref={root} onBlur={e => { if (e.relatedTarget && !e.currentTarget.contains(e.relatedTarget as Node)) setOpen(false) }} className="shared-account">{loading ? <span className="shared-account-loading" role="status">Checking account…</span> : signedIn ? <><button ref={toggle} type="button" className="shared-account-toggle" aria-expanded={open} aria-controls="account-navigation" onClick={() => setOpen(v => !v)}>Account <span aria-hidden="true">⌄</span></button><div id="account-navigation" className={`shared-account-links${open ? ' shared-account-links-open' : ''}`}><span className="shared-account-title">Your account</span><a href="/orders" aria-current={window.location.pathname === '/orders' ? 'page' : undefined}>My orders</a><a href="/manage" aria-current={window.location.pathname === '/manage' ? 'page' : undefined}>Stringer portal</a>{admin && <a href="/admin/catalogue" aria-current={window.location.pathname === '/admin/catalogue' ? 'page' : undefined}>Admin catalogue</a>}<button type="button" disabled={busy} onClick={logout}>{busy ? 'Signing out…' : 'Sign out'}</button>{error && <span className="shared-account-error" role="alert">{error}</span>}</div></> : <><span className="shared-account-title">Your account</span><a className="shared-signin" href={signInURL(window.location.pathname)}>Sign in ↗</a></>}</div>
}
