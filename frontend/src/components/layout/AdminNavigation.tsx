const sections = [
  ['/admin', 'Applications'],
  ['/admin/catalogue', 'Catalogue'],
  ['/admin/complaints', 'Complaints'],
  ['/admin/reviews', 'Reviews'],
  ['/admin/traders', 'Verification'],
  ['/admin/privacy', 'Privacy'],
  ['/admin/financial', 'Financial reports'],
  ['/admin/legal', 'Booking wording'],
]

export function AdminNavigation() {
  const path = window.location.pathname.replace(/\/$/, '')
  return <nav className="marketplace-container admin-navigation" aria-label="Admin sections">
    {sections.map(([href, label]) => <a key={href} href={href} aria-current={path === href ? 'page' : undefined}>{label}</a>)}
  </nav>
}
