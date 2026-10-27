import SiteHeader from './SiteHeader'

export function MarketplaceHeader({ backHref = '/', backLabel = 'Back to home' }: { backHref?: string; backLabel?: string }) {
  return <><SiteHeader variant="marketplace" /><div className="marketplace-container site-back-link"><a href={backHref}>← {backLabel}</a></div></>
}

export function MarketplaceFooter() {
  return <footer className="marketplace-footer"><div className="marketplace-container"><span>Restringing.ie</span><span>Made for players. Powered by stringers.</span></div></footer>
}
