# Compliance engineering — review series

These changes prepare an engineering workflow for review. They do not establish
that Restringing.ie is legally compliant. No production deployment, legal-policy
activation, live payment change or sensitive tax transmission is authorised by
this series. PR #30 remains a draft and has not been merged by this task.

## Review sequence

1. Booking disclosures and immutable contractual records — `feat/compliant-checkout`.
2. Authenticated privacy requests, reviewed export and account closure.
3. Private trader verification and the approval gate.
4. Complaints, notices, decisions and review/appeal records.
5. Cash facts and tax-reporting readiness, without assumed DAC7 applicability.
6. Security/accessibility checks and operational launch documentation.

The first branch starts from current main (`fa64eef`) plus the unmerged draft-page
foundation (#30). Follow-up PRs are stacked in this order, for focused review.
Do not merge draft documents as operative terms or enable public bookings until
all applicable legal and operational prerequisites have been approved.

## Booking records

Migration 14 adds immutable versioned documents and per-order confirmations.
An administrator supplies the trader's public contracting identity, complete
approved terms, cancellation notice, tax/price notice, turnaround description,
three distinct checkbox labels, booking-button wording, reviewed withdrawal
period and review reference. `/admin/legal` has blank fields; no final wording or
business identity is invented or seeded. A new version deactivates the old one;
old text and order confirmations cannot be edited/deleted through the application.

`BOOKING_LEGAL_ENFORCEMENT` defaults to false for existing pre-launch previews.
When false, the UI explicitly identifies the contractual workflow as inactive;
it never treats the draft pages as accepted terms. Publishing text alone does
not turn it on. Before setting it true, obtain approval of the operator/trader
relationship, applicable statutory rights, price/tax treatment, wording, contract
formation point and confirmation delivery. The trader-verification prerequisite
is added later in this series. A deployment with enforcement on fails closed if
approved disclosures are missing.

With enforcement enabled, a quote binds an approved document version. Ordering
requires acceptance of that version. Early performance is optional, with separate
unchecked express-request and consequences-acknowledgement controls. Either both
are chosen or neither. Without early authorisation, starting work is blocked
until the complete recorded withdrawal period ends in Europe/Dublin. No general
customisation exemption or automatic loss of rights on starting work is assumed.
A changed version invalidates an uncommitted quote. Already committed exact
idempotent retries return the original order/record even after quote expiry or
later terms changes; changed acknowledgements conflict. A failure rolls back the
order, confirmation, stock allocations, usage effects and notifications together.

`GET /orders/{id}/confirmation` is access-controlled by the existing order
boundary; `/orders/{id}/confirmation` in React supports a downloadable JSON copy.
It includes the immutable final order, price breakdown, cash payment method,
disclosures, server acknowledgement timestamp and earliest work date. This is
not a receipt that cash was received. A download facility is not a claim that a
legally sufficient durable-medium delivery process has been completed: approve
and operationally verify delivery before public launch. No email/SMS was sent.

### Sources reviewed 8 November 2026

- [Consumer Rights Act 2022, distance-contract confirmation and cancellation](https://www.irishstatutebook.ie/eli/2022/act/37/enacted/en/print.html).
- [CCPC service obligations](https://www.ccpc.ie/information-for-businesses/selling-goods-and-services/selling-services).
- [DPC rights-response times](https://www.dataprotection.ie/en/individuals/exercising-your-rights/how-long-will-it-take).
- [DPC erasure and exceptions](https://www.dataprotection.ie/en/individuals/know-your-rights/right-erasure-articles-17-19-gdpr).
- [DSA text, including Article 29 small/micro exclusions and Articles 30–32](https://eur-lex.europa.eu/eli/reg/2022/2065).
- [CCPC DSA remit and exemptions](https://www.ccpc.ie/enforcement-and-regulation/digital/the-digital-services-act).
- [Revenue DAC7/MRDP guidance](https://www.revenue.ie/en/companies-and-charities/documents/aeoi/dac7-reg-faqs.pdf).

Applicability to this business, exemptions, retention periods and approved wording
remain professional-review decisions. Source summaries must not be turned into
unqualified legal conclusions about this platform.

## Account rights (migration 15)

`/account/privacy` supports authenticated access/correction/erasure requests and their
response deadlines; `/admin/privacy` is the private review queue. The original
response target is one calendar month. One explained extension can be recorded
within the original period, to three calendar months from submission. The account
page is the delivery channel: no SMS/email dispatch is enabled. Monitor the queue
daily and contact the person through the reviewed support process where necessary.

Identity confirmation must be proportionate, without routinely requesting ID copies.
Released exports are minimized, authenticated JSON snapshots. Review the preview,
supplementary orders/messages, business records, billing references and processors;
explain exclusions and supply necessary supplementary data securely before marking
fulfilled. The built-in export is not represented as a complete Article 15 response.
Phone correction requires verified ownership, not a manual database reassignment.

Account closure blocks unresolved memberships and live orders, revokes sessions,
removes provider challenge metadata and notifications, and pseudonymises the user.
Existing order/message/contract and business/Stripe references are not cascaded away.
Where order records remain, the administrator must record the reviewed retention
basis/categories, expiry and partial-erasure explanation. Resolve external processor
requests manually. No automatic retention duration is asserted as legally approved.

Retention decisions to approve before launch: expire auth challenge metadata after
its operational/fraud-review window; revoke then purge expired sessions; define
order/message/financial/contract schedules and applicable holds; restrict access to
privacy requests/exports and purge them after their accountability period. Logs must
exclude bodies and credentials. Backup expiry and restore-time reapplication of
closures/deletions must follow the reviewed schedule. A retention date here is a
review task, not an automated purge or permission to retain indefinitely.

## Trader verification (migration 16)

Owners submit private legal/business contact information and a separate public
identity at `/manage/trader`. Administrators use `/admin/traders`, review evidence
outside this form using a restricted reference, and check identity, contact/address
and applicable registration. Decisions and submissions have an audit history.
Public profiles expose only the verified public projection. No tax IDs, bank data
or identity document uploads are requested. Changes invalidate the verification and
current contractual disclosures; stale decisions are rejected by version checks.

`TRADER_VERIFICATION_ENFORCEMENT` defaults to false. When separately approved and
enabled, public discovery, fresh quote/order creation and business approval require
verified records. Existing accepted orders remain accessible for fulfilment.
Reviewed booking identities must match the verified public identity when enabled.

Assess DSA scope and Article 29's micro/small-enterprise exclusions for Section 4
before deciding which Article 30–32 duties apply. This is not a blanket exemption
from other law or all DSA obligations. Record the operator's classification, trader
categories and review date with counsel. The current workflow supports business
verification without asserting every participant must provide tax or ID documents.

## Transparency and complaints (migration 17)

Alphabetical listing order remains explained on search; there is no paid ranking
or verified-review claim. `/account/complaints` supports private listing/order
reports and one independent internal review. `/report-content` accepts public
unlawful-content notices without requiring an account: precise location, grounds,
contact and a good-faith declaration. No attachments, URL fetching or automatic
listing removal occurs. Public notice references are receipts, not public access
tokens. Administrators manually follow up on public contacts; no outbound email is
enabled. Monitor `/admin/complaints` daily before opening public intake.

Reporters see their own reports. Business owners see only outcome projections,
not reporter identities or allegations. Decision writers must exclude personal
reporter data from shared reasons. Private case updates are available in the account
page; owner decisions have their own view. A different administrator must handle
an appeal; assign a second reviewer before launch. Listing restrictions still use
the existing audited approval controls and must carry reasoned, human decisions.

Public intake has a database-backed hourly budget and authenticated per-user limit.
Deploy reviewed edge abuse controls and an alternative contact path; this is not a
complete anti-abuse service. Assess DSA and P2B scope, exemptions, response duties,
statement-of-reasons requirements, retention, external redress and escalation with
counsel. This engineering workflow does not assert statutory completeness.
