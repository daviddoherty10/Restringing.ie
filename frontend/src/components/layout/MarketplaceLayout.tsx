import SessionMenu from '../../features/auth/SessionMenu'
import { BrandLogo } from '../ui/BrandLogo'

export function MarketplaceHeader({ backHref = '/', backLabel = 'Back to home' }: { backHref?: string; backLabel?: string }) {
  return <header className="marketplace-header"><div className="marketplace-container marketplace-nav"><BrandLogo variant="marketplace" /><a href={backHref} className="marketplace-home">← {backLabel}</a><SessionMenu /></div></header>
}

export function MarketplaceFooter() {
  return <footer className="marketplace-footer"><div className="marketplace-container"><span>Restringing.ie</span><span>Made for players. Powered by stringers.</span></div></footer>
}
