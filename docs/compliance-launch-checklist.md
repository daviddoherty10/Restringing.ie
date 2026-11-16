# Compliance release checklist (draft — not legal advice)

This is a working engineering checklist, not proof of compliance. The site's legal information is deliberately marked as draft until the actual operator, contracts, disclosures, processors and retention periods are verified by an Irish professional.

## Before accepting public bookings
- [ ] Confirm legal operator name, registered address, business registration, VAT position, and support/privacy contact details.
- [ ] Confirm whether each restringing contract is between customer and independent stringer or with Restringing.ie. Make the actual contracting trader's identity, geographic address and contact details available before booking.
- [ ] Have Irish legal advice confirm cancellation/withdrawal information, when performance may begin within the withdrawal period, and the appropriate separate express request and acknowledgement flow. Distinguish statutory remedies for defective services.
- [ ] Implement versioned checkout disclosures and **server-side** acceptance/early-performance records; do not rely on UI-only checkboxes. Store confirmations with orders and send durable confirmations.
- [ ] Review and publish final privacy notice, terms, cancellation policy and cookie notice, replacing all draft warnings.
- [ ] Implement authenticated data-access and erasure-request handling; documented retention and deletion for accounts, order messages, OTP metadata, logs and backups, observing lawful retention exceptions.
- [ ] Document actual subprocessors, transfer safeguards, processing agreements, security/incident procedures and DPA contact.
- [ ] Assess DSA, P2B, DAC7, VAT, e-commerce accessibility and relevant micro/small-business exemptions with qualified advisers.
- [ ] Assess real trader verification obligations; keep private verification evidence away from public APIs.
- [ ] Confirm search ranking statement matches the backend sort order, and update if ranking is changed.
- [ ] Document Stripe subscription billing, VAT display and invoicing, renewals and cancellation.
- [ ] Review non-essential cookies/scripts before enabling analytics, and implement consent if applicable.
- [ ] Conduct accessibility and security checks before deployment.

## Engineering notes
Current customer bookings are cash-only; Stripe is used for stringer subscriptions. Do not describe customer bookings as prepaid. A public legal page linking to an incomplete draft is not a substitute for a completed privacy notice. Do not publish the draft as an operative legal agreement.

Subsequent PRs should implement (1) server-enforced checkout disclosures and consent snapshots, (2) privacy request/retention workflow, (3) private trader legal-identity records and verification workflow, and (4) tax-reporting readiness, subject to legal assessment.
