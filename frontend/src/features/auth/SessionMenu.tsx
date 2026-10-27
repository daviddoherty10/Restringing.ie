import './auth.css'
import { useEffect, useState } from 'react'
import { marketplaceAPI, MarketplaceError } from '../../lib/marketplace-api'
import { clearBookingDrafts, signInURL } from '../../lib/auth-navigation'

export default function SessionMenu() {
  const [signedIn, setSignedIn] = useState(false)
  const [admin, setAdmin] = useState(false)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  useEffect(() => {
    let active = true
    marketplaceAPI.me().then(me => { if (active) { setSignedIn(true); setAdmin(me.admin) } }).catch(() => {}).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [])
  async function logout() {
    setBusy(true); setError('')
    try { await marketplaceAPI.auth.logout(); clearBookingDrafts(); window.location.assign('/') }
    catch (e) { setError(e instanceof MarketplaceError ? e.message : 'Could not sign out. Please retry.'); setBusy(false) }
  }
  if (loading) return <span className="session-menu" role="status">Checking account…</span>
  return <div className="session-menu">{signedIn ? <><a href="/orders">My orders</a><a href="/manage">Stringer portal</a>{admin && <a href="/admin/catalogue">Admin catalogue</a>}<button type="button" disabled={busy} onClick={logout}>{busy ? 'Signing out…' : 'Sign out'}</button></> : <a href={signInURL(window.location.pathname)}>Sign in ↗</a>}{error && <span role="alert">{error}</span>}</div>
}
