import { MarketplaceHeader, MarketplaceFooter } from '../../components/layout/MarketplaceLayout'

type LegalKey = 'privacy' | 'terms' | 'cookies' | 'cancellations' | 'contact'
const pages: Record<LegalKey, { title: string; sections: { title: string; body: string }[] }> = {
  privacy: {
    title: 'Privacy information',
    sections: [
      { title: 'Data collected', body: 'The application uses telephone numbers for sign-in and maintains account, booking and order history, messages and stringer business records. Stringers can see customer contact details associated with their bookings. The exact collection and lawful bases must be verified before launch.' },
      { title: 'Service providers', body: 'The code supports an SMS verification provider and Stripe for stringer subscriptions. Hosting, data location, service-provider contracts and any international transfers require confirmation before this notice is final.' },
      { title: 'Your rights', body: 'Individuals may have rights to access, correct, erase, restrict or object to processing and to complain to the Data Protection Commission. Certain records may need to be retained for legal obligations or claims. Authenticated access, correction and erasure requests can be submitted through the account privacy section. Requests require proportionate identity review; retention and external processor handling remain subject to the reviewed policy.' },
      { title: 'Retention and security', body: 'The service uses session and verification controls. Final retention schedules for orders, conversations, verification data, backups and logs must be documented before launch.' },
    ],
  },
  terms: {
    title: 'Marketplace terms',
    sections: [
      { title: 'Marketplace model', body: 'Restringing.ie helps customers discover racket stringers and request bookings. The contracting trader and their identifying details must be clearly disclosed before each booking. The legal operator and contractual allocation of responsibility remain to be confirmed.' },
      { title: 'Bookings and payment', body: 'Bookings currently use cash payment directly to a stringer; the platform does not collect online payments from customers. The quoted total and booking details should be reviewed before placing an order.' },
      { title: 'Stringer subscriptions', body: 'Stringers can use a free allowance of five accepted bookings per Irish calendar month, or subscribe to a paid plan. Recurring amount, tax treatment and renewal/cancellation terms must be confirmed before enabling live billing.' },
      { title: 'Consumer rights', body: 'Nothing in these draft notes is intended to restrict mandatory consumer rights. A final contractual statement and complaints procedure must be approved before launch.' },
    ],
  },
  cookies: {
    title: 'Cookies and local storage',
    sections: [
      { title: 'Essential sign-in', body: 'The application uses a session cookie to authenticate accounts. The booking interface also uses browser session storage to preserve an in-progress draft across sign-in.' },
      { title: 'Optional tracking', body: 'The reviewed code contains no analytics or advertising scripts. Two homepage images are loaded from Unsplash, exposing request metadata to that provider; final notices and the image-hosting decision need review. Consent controls must be added before setting non-essential cookies where required.' },
      { title: 'Controls', body: 'You can manage cookies through your browser. Disabling essential storage may prevent sign-in or booking drafts from working.' },
    ],
  },
  cancellations: {
    title: 'Cancellations and refunds',
    sections: [
      { title: 'Before work begins', body: 'Ask the responsible stringer about a booking cancellation using the order conversation. The final notice must explain statutory withdrawal rights and any applicable exceptions before orders are accepted.' },
      { title: 'Early performance', body: 'If a service starts during a statutory withdrawal period, additional disclosures and an express customer request may be required. A versioned server-validated acknowledgement workflow has been implemented but remains disabled until final wording and contractual responsibilities are approved.' },
      { title: 'Service problems', body: 'Customers may have statutory remedies for services not performed with reasonable care and skill. A final complaints and remedies procedure must be published before launch.' },
    ],
  },
  contact: {
    title: 'Contact and complaints',
    sections: [
      { title: 'Get in touch', body: 'For a booking-specific issue, contact the stringer through the relevant order conversation. The platform must publish confirmed operator contact information and a workable complaints contact method before launch.' },
      { title: 'Data protection', body: 'Authenticated users can submit privacy-rights requests in their account. Confirmed operator contact details and support coverage are still required before public launch.' },
    ],
  },
}
export default function LegalPage({ page }: { page: LegalKey }) {
  const info = pages[page]
  return <div className="marketplace"><MarketplaceHeader /><main id="main-content" tabIndex={-1} className="marketplace-container" style={{maxWidth: '820px', paddingTop: '2rem', paddingBottom: '4rem'}}>
    <p className="marketplace-kicker">LEGAL INFORMATION — PRE-LAUNCH DRAFT</p>
    <h1>{info.title}</h1>
    <p role="status"><strong>Draft for review:</strong> This page is an engineering placeholder, not a final legal notice or agreement. Required operator details and legal terms are still being confirmed. Do not launch public bookings until approved notices and checkout disclosures are in place.</p>
    {info.sections.map(section => <section key={section.title} style={{marginTop:'1.5rem'}}><h2>{section.title}</h2><p>{section.body}</p></section>)}
    <p><a href="/account/privacy">Account privacy requests</a> · <a href="/account/complaints">Complaints and decision reviews</a> · <a href="/report-content">Report potentially unlawful content</a></p>
    <p>For general enquiries: <a href="mailto:hello@restringing.ie">hello@restringing.ie</a> (address and support arrangements to be verified).</p>
  </main><MarketplaceFooter /></div>
}
