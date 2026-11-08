# Security and operations review — 8 November 2026

Engineering review, not a launch approval or security/accessibility certification.
Review together with `compliance-implementation.md` and the draft launch checklist.

## Reviewed application controls

- Phone OTP verification remains provider-backed. No OTP is stored in PostgreSQL.
  Opaque random sessions are hashed in the database; cookie is HttpOnly, SameSite=Lax,
  Secure for HTTPS origins. Only loopback development permits HTTP.
- Cookie-authenticated writes require the exact configured frontend Origin. Bearer
  clients cannot fall back to ambient cookies. CORS has one explicit origin, never
  a credentialed wildcard. New routes use the existing strict 16 KiB JSON decoder.
- OTP and message rate limits remain enforced. Privacy requests cap open requests;
  notices cap hourly intake. Review edge limits, alerts and support fallback before
  exposing public notice intake. No email-address ownership is inferred from a notice.
- Private rights requests, trader documents, cases and financial facts use DB-backed
  customer/owner/admin authorisation. Staff do not gain owner verification or finance
  access. Financial facts and booking confirmations are append-only. Review admin
  grants and audit access regularly; do not share administrator accounts.
- Database queries are parameterised. Runtime connections must use a restricted
  application role; migrations use a separate credential. Protect DB/backups from
  public ingress. Local compose DB has no host port. Use TLS with certificate
  verification for external database connections; private Docker networking alone
  is not encryption for remote or managed DB traffic.
- API responses use no-store, nosniff, no-referrer, anti-framing and restrictive CSP.
  Nginx adds a frontend CSP with scripts restricted to self, no framing/objects and
  a narrowly enumerated image host. Inline style support remains for existing React
  styles; inline scripts are not allowed. Vite development is not a public server.
- Logs report request IDs/error types, not request bodies, tokens, phone numbers or
  webhook payloads. Keep proxy access logs free of sensitive query strings. Financial
  references and free-text admin reasons must never contain credentials or tax IDs.
- New reviewed wording uses plain text, not HTML injection. Internal legal-review
  references are excluded from public disclosures and confirmation snapshots.

## Deployment checklist (operator must supply evidence)

1. Keep preview separate from production. Bind web/landing ports to loopback behind
   the TLS edge; never publish PostgreSQL or the API directly. Restrict the preview
   hostname with edge access control. The included deployment still serves a public
   coming-soon page; it does not constitute launch approval.
2. Confirm DNS, certificates, HTTPS redirects and HSTS at the actual TLS terminator.
   Start HSTS only after HTTPS is verified; assess subdomains before includeSubDomains
   or preload. Verify headers on HTML, API errors and downloads through the real edge.
3. Set the exact HTTPS FRONTEND_ORIGIN. Trust only the actual reverse proxy CIDRs;
   the edge must replace untrusted forwarded headers. Test OTP limits using real
   proxy topology without sending messages to unconsenting recipients.
4. Store secret values in the deployment secret manager, never VITE variables,
   source, screenshots, shell history or logs. Rotate keys after exposure. Use
   scoped provider credentials and a separate restricted migration DB role.
5. Keep BOOKING_LEGAL_ENFORCEMENT and TRADER_VERIFICATION_ENFORCEMENT false until
   identities, reviewed notices, contractual timing, withdrawal treatment and durable
   confirmation delivery have been approved and exercised in staging. Publishing a
   version is not activation. Test migrated existing businesses before enabling.
6. Keep live billing/SMS disabled until their separate integration reviews. Cash
   customer payments and accepted-booking quota remain unchanged. Reconcile actual
   subscription invoices before recording financial fee facts.
7. Scan built container images and base OS packages in the deployment pipeline.
   Source scans below do not cover image layers. Review updates, pin approved image
   digests and stage them; this environment cannot run Docker/container scanners.

## Dependency verification

The initial symbol-level govulncheck scan found GO-2026-6629 and GO-2026-5970 through
x/text v0.29.0. Upgraded to v0.41.0 (x/sync v0.22.0 required transitively), aligned the
Go container build with tested Go 1.27.3, and reran govulncheck v1.9.0. Official reports:
https://pkg.go.dev/vuln/GO-2026-6629 and https://pkg.go.dev/vuln/GO-2026-5970.

The initial npm audit found development-tool advisories, including Vite and Zod.
Updated within declared compatible dependency ranges. See the PR validation record
for final scan results and any override; no audit finding is silently waived.
Use `npm audit`, `npm audit --omit=dev` and `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...`
from their respective module directories. Repeat on dependency changes and before
release. A clean scan is a point-in-time result, not proof of absence of defects.

## Accessibility review

WCAG 2.2 AA is the practical testing target; applicable legal scope/exemptions require
review. Shared skip navigation targets focusable main content. Focus styling has a
darker outline, reduced-motion preferences are respected, and mobile menus scroll
without covering the entire viewport. New forms use native labels, fieldsets,
checkboxes, status/error feedback and controls with usable touch sizes. Long IDs,
JSON previews and financial details wrap on narrow screens.

Automated axe checks and keyboard/mobile tests cover the new privacy/reporting entry
pages. Existing booking, account and fulfilment regression tests run on desktop and
mobile. Before launch, manually test VoiceOver/NVDA, keyboard-only booking/sign-in,
200%/400% zoom, real devices, contrast in all states, error recovery and provider
redirects. Automated tests do not establish full WCAG conformance.

## Processor and browser-storage inventory

| Item | Purpose/data | Current review action |
| --- | --- | --- |
| Hosting / DB / backups | Application records, logs, snapshots | Operator must identify vendor, region, DPA, subprocessors, transfers, access and deletion controls |
| Twilio Verify (when enabled) | Phone number and OTP delivery/verification metadata | Confirm agreement, geography, retention and lawful transfer mechanism; do not copy OTPs into logs |
| Stripe subscriptions (when enabled) | Business customer/subscription IDs, billing and webhook metadata | Confirm controller/processor roles, terms, retention and invoice reconciliation; no customer online payment here |
| Unsplash image CDN | Two homepage image requests with IP/browser metadata | Existing external images remain; assess licence, request disclosure and self-hosting decision before launch |
| session_token cookie | Strictly necessary account session | HttpOnly, SameSite=Lax, Secure on HTTPS, path /; current server lifetime is defined by auth.SessionLifetime |
| restringing:booking:* sessionStorage | Requested booking draft and pending-order retry metadata | Thirty-minute validity, tab-scoped, cleared on sign-out; may contain instructions; advise against sensitive details |
| Other tracking | None found in reviewed source | No analytics/advertising scripts or tracking banner added; re-audit any new SDK/widget |

The coming-soon page has a first-party year-label script, no tracker/storage. These
observations do not replace a deployed network/cookie audit. New non-essential
tracking requires a reviewed prior-consent design where applicable, with real
blocking, choices, withdrawal and records. Never add a cosmetic banner.

## Retention, backup and recovery runbook

Schedules need an owner-approved legal basis, duration, holds, deletion method and
processor propagation. Do not invent statutory retention periods or enable automatic
purges from this document. Inventory: user contacts; expired/revoked sessions; OTP
challenge/provider metadata; order/configuration/message/notification data; contracts;
private trader submissions/history; financial facts; privacy exports/cases; logs;
Stripe references; backups. Store policy approval and review date.

- Monitor overdue rights requests and retention review dates daily. Use the privacy
  review workflow for identity, dependency checks, partial-erasure reasons and expiry.
  Orders/messages/immutable contracts require a lawful retained scope or a separate
  reviewed redaction/purge process; do not mark complete deletion when data remains.
- Privacy exports include own messages, session timestamps (never token hashes),
  memberships and notifications in addition to basic account/order facts. Review
  business/processor and third-party information separately before fulfilment.
- Back up PostgreSQL to encrypted, access-restricted storage with documented expiry,
  key custody, offsite copies and alerting. Choose and approve recovery-point and
  recovery-time objectives. Never treat a Docker volume as a backup.
- Rehearse restoration in a disposable isolated environment with integrations disabled:
  restore a known backup, run migration/version checks and representative row/count,
  order/stock/financial reconciliation, auth boundaries and confirmation access tests.
  Reapply account closures, deletion/redaction tombstones and revoked sessions before
  permitting traffic. Verify backup-expiry behaviour and document duration/results.
- Test restore before launch and after material schema/backup changes. No live backup
  or restoration was performed in this coding environment; attach operational evidence.

## Incident and breach-response runbook

Appoint a named incident lead and backup, restricted contact channel and processor
contacts before launch. On an incident: contain access without destroying evidence;
record UTC discovery time, systems, categories and affected scope; preserve restricted
logs and timeline; rotate compromised keys/revoke sessions; assess whether personal
data was involved and risks to individuals with the privacy lead/counsel. Consider
supervisory-authority notification within the applicable GDPR 72-hour period after
awareness, document the assessment/delays and notify affected people when required.
Do not paste personal data into public issues. Coordinate processors, test recovery,
monitor recurrence and record a post-incident review with assigned actions.
Primary guidance: https://www.dataprotection.ie/en/organisations/know-your-obligations/breach-notification

## Release approval record (still required)

Named business/legal review of operator identity, notices, trader classification,
DSA/P2B applicability and exemptions; accountant DAC7/MRDP assessment; rights/complaint
queue coverage and independent reviewer; retention policy; processor agreements;
TLS/proxy/headers evidence; backup-restore rehearsal; manual accessibility audit;
built-image scan; confirmation-delivery review; secrets and billing/SMS staging checks.
Passing source tests does not fill these approvals. No public deployment is authorised
or performed by this PR series.

Final source scan results on this branch: govulncheck v1.9.0 found no reachable
vulnerabilities; npm audit (all dependencies and production-only) reported zero.
The remaining minimatch glob-amplification advisory required a reviewed override to
11.0.1, rather than npm's suggested forced ESLint downgrade. Lint/build and browser
regressions validate the toolchain with that override. Vite is now 8.3.6 and Zod
4.6.7 in the lockfile. These results do not cover deployed base-image packages.

Privacy corrections explicitly select which name/email fields to replace; unselected
values are preserved. Closing accounts serializes the last-administrator handover
check, revokes all active sessions, and requires a reviewed retention date for the
remaining rights-request accountability record. Migration 19 makes decision histories
append-only and exposes them only through authenticated administrator review screens.
The final automated accessibility pass also covers homepage, search and sign-in;
contrast and visible-label/name issues found there were corrected. Manual assistive
technology and production accessibility review are still required.
