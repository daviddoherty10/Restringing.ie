import type { Stringer } from '../../lib/marketplace-api'

export default function ListingContact({ stringer }: { stringer: Stringer }) {
  const contact = stringer.public_identity
  const area = [stringer.area, stringer.county].filter(Boolean).join(', ')
  if (!contact) return <div className="listing-contact">
    {area && <span>{area}</span>}
    {area && <a href={'https://www.google.com/maps/search/?api=1&query=' + encodeURIComponent(area)}>View area on map</a>}
    <small>Street address and contact details haven’t been published.</small>
  </div>

  const address = contact.address.trim()
  const phone = contact.phone.trim()
  const email = contact.email.trim()
  return <address className="listing-contact">
    <span>{address || area}</span>
    {address && <a href={'https://www.google.com/maps/dir/?api=1&destination=' + encodeURIComponent(address)}>Get directions</a>}
    {phone && (/^\+?[\d\s().-]+$/.test(phone)
      ? <a href={'tel:' + phone.replace(/[\s().-]/g, '')}>Call {phone}</a>
      : <span>{phone}</span>)}
    {email && (/^[^\s@?#&%]+@[^\s@?#&%]+\.[^\s@?#&%]+$/.test(email)
      ? <a href={'mailto:' + email}>{email}</a>
      : <span>{email}</span>)}
  </address>
}
