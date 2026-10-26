type BrandLogoProps = { variant?: 'home' | 'marketplace' }

export function BrandLogo({ variant = 'home' }: BrandLogoProps) {
  if (variant === 'marketplace') {
    return <a className="marketplace-brand" href="/" aria-label="Restringing.ie home"><span className="marketplace-brand-icon" aria-hidden="true">R<span>.</span></span>restringing<span className="marketplace-brand-domain">.ie</span></a>
  }
  return <a className="logo" href="/" aria-label="Restringing.ie home"><span className="logo-mark" aria-hidden="true">R<span className="logo-dot">.</span></span><span>restringing<span className="logo-domain">.ie</span></span></a>
}
