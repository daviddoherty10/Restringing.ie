import { AdminAccessGranted } from '../../lib/admin-access'
import { useEffect, useState, type ReactNode } from 'react'
import { MarketplaceFooter, MarketplaceHeader } from '../../components/layout/MarketplaceLayout'
import { marketplaceAPI, MarketplaceError } from '../../lib/marketplace-api'
import { signInURL } from '../../lib/auth-navigation'

export function AdminAccess({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<'loading' | 'signed-out' | 'denied' | 'error' | 'allowed'>('loading')
  useEffect(() => {
    let active = true
    marketplaceAPI.me().then(me => {
      if (active) setStatus(me.admin ? 'allowed' : 'denied')
    }).catch(error => {
      if (active) setStatus(error instanceof MarketplaceError && error.status === 401 ? 'signed-out' : 'error')
    })
    return () => { active = false }
  }, [])
  if (status === 'allowed') return <AdminAccessGranted.Provider value={true}>{children}</AdminAccessGranted.Provider>
  return <div className="marketplace">
    <MarketplaceHeader />
    <main id="main-content" tabIndex={-1} className="marketplace-container manage-content">
      <h1>{status === 'denied' ? 'Administrator access required' : 'Admin'}</h1>
      {status === 'loading' && <p role="status">Checking admin access…</p>}
      {status === 'signed-out' && <a href={signInURL(window.location.pathname)}>Sign in to Admin</a>}
      {status === 'denied' && <p role="alert">This workspace is available to platform administrators.</p>}
      {status === 'error' && <p role="alert">Could not check admin access. <button type="button" onClick={() => window.location.reload()}>Try again</button></p>}
    </main>
    <MarketplaceFooter />
  </div>
}
