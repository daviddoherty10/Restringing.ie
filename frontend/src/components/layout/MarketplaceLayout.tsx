import SiteHeader from './SiteHeader'

export function MarketplaceHeader({ backHref = '/', backLabel = 'Back to home' }: { backHref?: string; backLabel?: string }) {
  return <><SiteHeader variant="marketplace" /><div className="marketplace-container site-back-link"><a href={backHref}>← {backLabel}</a></div></>
}

export function LegalLinks() {
  return <nav aria-label="Legal information" style={{display:"flex",gap:"1rem",flexWrap:"wrap"}}><a href="/privacy">Privacy</a><a href="/terms">Terms</a><a href="/cookies">Cookies</a><a href="/cancellations">Cancellations</a><a href="/contact">Contact</a></nav>
}

export function MarketplaceFooter() {
  return <footer className="marketplace-footer"><div className="marketplace-container"><span>Restringing.ie</span><span>Made for players. Powered by stringers.</span><LegalLinks /></div></footer>
}
