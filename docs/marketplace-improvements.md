# Marketplace improvements: delivery plan

This document records the requested next features and their proposed behaviour.
Shared navigation, order tracking, order conversations, in-app notifications and
post-booking collection requests/estimates and express turnaround are now implemented. Grip add-ons are also implemented; see [the grip guide](grip-addons.md). See
[the order-tracking guide](order-tracking.md) for the current API and behaviour.
True hybrid setups are implemented in [the hybrid guide](hybrid-setups.md).
The proposals below also describe future extensions. Outbound delivery and media
remain unimplemented; develop those on separate branches against current `main`
and update/test their contracts.

## Implemented: shared sticky navigation

The homepage and application pages now use one header. It stays at the top during
scrolling. Mobile/tablet widths use one expandable navigation panel: public links
first, then customer orders, the stringer portal, administrator catalogue (admins
only), and sign out. Desktop uses an Account dropdown to keep the header compact.

Navigation supports Escape with focus return, outside click/focus dismissal,
closed-menu link hiding, resize handling, safe sign-in returns and a scrollable
panel on short screens. Contextual back links remain below the header. Anchor
scrolling accounts for the sticky header. Account navigation does not change
server permissions or grant access to any dashboard.

## Current gaps

- Business profiles and string listings have no image-upload/media service.
- True hybrid setups now select separate mains/crosses variants. Packaged
  `material: hybrid` products remain distinct. Measured cut-piece tracking and automatic stock conversion remain future work.
- Customers now have a polling tracker, an order conversation and an in-app
  notification inbox. Current business members receive in-app new-job/reply alerts.
  Outbound order-event SMS/email and closed-browser push delivery remain absent.
- The current SMS integration handles sign-in OTP only. It is not a delivery
  mechanism for job-ready/new-order messages.
- Stringers can already see customer phone details on their own incoming orders.
  Customers do not have a separately configured business contact number.
- Express-service pricing and fitted grip add-ons are implemented.
  Collection date requests and ready estimates are captured after booking through
  the tracking API; they are not quoted appointments or door-to-door pickups.

## 1. Order conversations and notifications — implemented foundation

The [tracking guide](order-tracking.md) is authoritative for the implemented
in-app thread, read cursors, inbox and transaction boundaries. The following
original proposal includes future outbound delivery, business phone shortcuts
and richer issue handling; those extensions are not implemented.

Use an order-linked thread for questions, delays and issues. Offer a phone-call
shortcut as an additional contact channel. Keeping the thread attached to the
order gives both sides a record of what was agreed and what still needs a reply.
A generic unrestricted social-chat system is unnecessary for the initial release.

Proposed data:

- One conversation per order, with plain-text messages, sender user, timestamp and
  a client-generated idempotency key for each send.
- Per-user read position/unread state. An issue can be marked as requiring a reply
  and resolved without changing the payment or fulfilment state.
- Access for the customer owning the order and current members of its business;
  platform support access follows the existing admin order-access boundary.
- A separately configured business contact number, shared intentionally by the
  business. Do not expose all owner/staff personal login numbers to customers.

Do not modify saved prices or promise pickup dates merely because someone writes
a message. Price changes require a server quote and explicit customer acceptance;
messages are communication, not authority to change a charge.

Create notifications for:

| Event | Recipient |
| --- | --- |
| New booking | Current stringer owner/staff responsible for orders |
| Booking accepted or cancelled | Customer |
| New message / issue requiring a reply | Other side of the order |
| Confirmed timing change / delay | Customer, and business where relevant |
| Order ready for collection | Customer |

Write the order change, in-app notification and outbound delivery request in the
same PostgreSQL transaction. Use a unique event/recipient/channel key so order or
message retries do not duplicate alerts. A worker retries delivery separately;
a provider outage must not prevent the stringer marking a racket ready. Record
provider delivery state rather than claiming an SMS was delivered when an API
merely accepted it. Ambiguous provider responses need safe reconciliation; the
application must not promise exactly-once external delivery.

Start with a persistent in-app notification inbox, unread badge and bounded polling.
Add outbound SMS when the notification sender/provider is configured. Phone-only
sign-up does not provide an email address: email notifications would need an
optional verified notification address. Keep outbound SMS generic with a signed-in
order link, and provide preferences for nonessential updates. Notification SMS is
a separate integration from OTP; unresolved SMS credentials must not be masked by
fake delivery confirmations. No real messages are sent as part of development.

## 2. Collection requests and express service — implemented foundation

Post-booking requested dates, confirmed ready estimates, collection instructions,
issue flags and change history are implemented. Optional express fees and booked
turnaround snapshots are implemented in [the express guide](express-turnaround.md).
Integrating collection requests into the initial booking form remains proposed.

Assumption: pickup means the customer collecting the finished racket from the
stringer. Collection from the customer would be a separate pickup/delivery service
with its own address, availability and fee rules.

Customers can request a collection date, optionally a time window. Label it
**requested collection**, not a guaranteed booking slot. Store the customer's
request separately from the stringer's confirmed ready/collection time. Use Irish
local dates and `Europe/Dublin` for display and validation. Reject past requests.

Let the stringer confirm or propose another date when accepting the order, and
notify the customer. Initially this is a request/confirmation workflow; it must
not invent opening hours or promise capacity before availability scheduling exists.

Each sport service can offer an optional express turnaround, with a clear timing
statement, fee in integer cents and an availability toggle. Show standard and
express choices before quotation. The server validates that express is offered
and adds its price line. A requested early date must not silently select a paid
service. If express cannot meet the requested date, require another choice or
manual confirmation rather than manufacturing a guarantee.

Snapshot the requested date, selected turnaround description, confirmed timing and
price in the order. Changing service prices later must not rewrite that snapshot.
A delay notification communicates a revised proposal; it does not silently move a
customer's confirmed collection commitment.

## 3. Grips and accessories — implemented foundation

[Grip add-ons](grip-addons.md) now provide the shared catalogue, fitted prices,
unit stock and atomic multi-item reservation flow. Broader accessories and media
remain future work. The original design notes below also inform hybrid support.

Model grips separately from racket strings. Offer overgrips and replacement grips
with a few stringer-selected options (brand/model/type/colour), a final fitted price
and stock availability. Do not squeeze grips into the string gauge/material schema.

An optional grip is selected in booking and quoted by the server. Reserve its unit
alongside the string materials when the order is created. Release it on eligible
cancellation and consume it once when fitted/work starts under the defined workflow.
The customer sees separate material/service lines and an immutable final total.

The multi-item reservation work should be designed together with hybrid support.
It must be one transaction: an unavailable grip or cross string cannot leave a
half-created order or stranded mains reservation. A grip is an add-on, not another
restringing for the business's monthly plan allowance.

## 4. True hybrid string setups — implemented

See [the hybrid guide](hybrid-setups.md) for the revised implementation:

- One full-bed material price per string, with hybrid halves derived automatically.
- Availability-only by default; optional advanced quantity tracking. No mandatory stock receipt or hybrid length entry.
- Both stocked and customer-supplied hybrids, with separate mains/crosses identities.
- Shared-catalogue selection or bounded descriptions for supplied strings.
- Atomic purchased-item allocations, immutable pricing, four-knot and technical checks.
- Mixed stock/customer components and separate component pre-stretch remain future work.

## 5. Profile, banner and catalogue photos — proposed independent PR

Allow business owners/admins to set a profile image and banner. Allow authorised
inventory managers to set a photo for their own string offering. A shared catalogue
image can be the default so every stringer does not have to upload the same SKU;
a business image is an override, not a write to the shared global catalogue.

Use object storage for image data and PostgreSQL for asset identity/ownership,
attachment, processing state and display metadata. Keep the storage provider
configurable and backend credentials private. The upload/attach flow should:

1. Authenticate ownership/permissions before issuing a bounded upload operation.
2. Accept only supported raster formats with byte/dimension limits, verify actual
   decoded content and re-encode to strip metadata before publication.
3. Produce thumbnail/profile/banner sizes and preserve aspect ratio with a useful
   crop/focal point; use existing initials/placeholder styling when absent.
4. Attach only validated assets belonging to the requesting business/admin. Keep
   staging uploads private; publish only assets attached to eligible public records.
5. Replace/remove attachments safely and clean up orphaned uploads after a grace
   period. Avoid deleting an image that another active attachment still uses.

Do not ship an unrestricted public upload endpoint or treat an arbitrary image URL
as a completed upload feature. Grip photos can use the same asset infrastructure.
This needs a selected storage environment and configuration before a live rollout.

## Delivery and verification

Suggested sequence after tracking: outbound notifications when configured;
collection requests and express options; shared multi-item reservation design for
grips/hybrids; then implement those booking additions. Media can be developed as
an independent feature once its storage environment is available. Outbound SMS
follows provider resolution and sandbox delivery testing.

For each booking change, update Go DTOs, SQL migrations where needed, quotation
fingerprints, order snapshots, access checks, generated OpenAPI/TypeScript, preview
fixtures and React controls together. Test stock races, complete rollback, retries,
plan usage, permissions and customer history against real PostgreSQL. Notification
workers additionally need duplicate/retry/out-of-order delivery tests. UI checks
should include mobile booking and management, unread state and explicit requested
versus confirmed timing. Existing cash and subscription behaviour must remain valid.
