// Keep authentication redirects on known application routes, including when an
// attacker supplies encoded slashes, backslashes or an external URL.
export function safeReturnTo(raw: string | null): string {
  if (!raw || raw.length > 300) return '/orders'
  if (/^\/(orders(?:\/[0-9a-fA-F-]{36})?|manage|search|stringers\/[0-9a-fA-F-]{36}(?:\/book)?)$/.test(raw)) return raw
  return '/orders'
}
export function signInURL(returnTo: string): string {
  return `/signin?returnTo=${encodeURIComponent(safeReturnTo(returnTo))}`
}
export function internationalPhone(raw: string, callingCode: string): string {
  const value = raw.trim().replace(/[\s()-]/g, '')
  if (value.startsWith('+')) return value
  if (value.startsWith('00')) return `+${value.slice(2)}`
  return `${callingCode}${value.replace(/^0/, '')}`
}
export function clearBookingDrafts(): void {
  try {
    for (const key of Object.keys(sessionStorage)) if (key.startsWith('restringing:booking:')) sessionStorage.removeItem(key)
  } catch { /* Storage may be blocked by browser policy. */ }
}
