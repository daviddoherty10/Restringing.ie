# Stringer dashboard

The `/manage` dashboard supports business owners, staff and platform administrators.
It uses the existing authenticated marketplace API and works with local preview
sessions while live Twilio credentials are unavailable. No SMS account or provider
connection is needed for this preview. The command below explicitly disables SMS
for the preview API process, even if your shell has partial Twilio credentials. Phone OTP remains a separate integration
that can be enabled later using `docs/phone-auth.md`.

## Preview it

From the repository root on the current application branch, run:

```bash
npm --prefix frontend ci
bash scripts/preview.sh seed
AUTH_SMS_PROVIDER= BILLING_PROVIDER= bash scripts/preview.sh api
```

In a second terminal:

```bash
bash scripts/preview.sh web
```

In a third terminal, choose a fictional account:

```bash
bash scripts/preview.sh session stringer
# Or: bash scripts/preview.sh session staff
# Or: bash scripts/preview.sh session admin
```

Open <http://127.0.0.1:5174/manage> and follow the session command's guarded browser
instructions. The stringer and staff fixtures belong to Preview — Baseline Workshop.
The administrator can open the [shared catalogue](admin-catalogue.md) at `/admin/catalogue` to curate products, variants and racket models, and use **Review applications** to inspect and decide the pending
fictional application. The cookie expires after an hour; issue another preview
session when necessary. These credentials never send SMS or change production data.

A business can be linked directly using a UUID:
<http://127.0.0.1:5174/manage?business=90000000-0000-4000-8000-000000000101>.
Selecting a business replaces the workspace and prevents late responses from the
previous business from appearing. A private endpoint verifies access before any
management data is shown; public profile data is never an authorization fallback.

Sample edits persist across seed reruns. If you want a fresh fictional fixture,
follow the explicit separate-volume reset procedure in `docs/preview-data.md`.

## What can be managed

| Screen | Behaviour |
| --- | --- |
| Booking totals | Business-wide all-time counts and non-cancelled booking value, independent of the current order page/filter |
| Plan & billing | Irish calendar-month usage, remaining free allowance and owner-only subscription checkout/management |
| Cash receipts | Confirm the saved amount received after acceptance; audit separately from fulfilment |
| Orders | Paginated incoming orders, status filter, immutable setup/pricing, customer instructions, private fulfilment contact and status history |
| Fulfilment | Advance placed → accepted → received → in progress → ready → completed; cancel only before work starts |
| Stocked strings | Search/filter/paginate local offerings, edit material prices and availability |
| Add from catalogue | Select shared catalogue variants; exact variant lookup preserves an existing local price |
| Receive stock | Receive individually counted sets with a package length, or reels measured in metres |
| Stock lots | Search/filter/paginate labelled lots, including inactive catalogue products; show physical, reserved and free quantities separately |
| Stock corrections | Signed quantity corrections with a required reason; backend refuses removal of reserved stock |
| Services | Configure each sport's labour, own strings, stencil, knot patterns, separate tensions and pre-stretch percentages/prices |
| Profile | Edit name, description, area/county and new-booking availability |
| Applications | Submit a business application; administrators review a paginated queue with a recorded reason |

Money inputs use euros but are converted to integer cents before submission.
Tension settings use kg and become integer centi-kg. Reel inputs use metres and
become integer millimetres. Set balances always count sets, not metres. A receipt
requires the labelled/measured package length; the UI does not invent racket limits.
A free pre-stretch option remains a valid offered option.

## Permissions

| Operation | Owner | Staff | Platform admin |
| --- | --- | --- | --- |
| Read associated business/orders/contact/stock/services | Yes | Yes | Yes, using a business link |
| Manage order status, stock and material prices | Yes | Yes | Yes |
| Edit profile and service capabilities/prices | Yes | No | Yes |
| Approve/reject business applications | No | No | Yes |

A user can be staff at one business and an owner at another. `GET /me/stringers`
now returns private `ManagementProfile` objects: the previous profile fields plus
`membership_role` and effective `permissions`. Public profiles still use `Stringer`
and expose no user role or membership. These UI flags describe permissions;
backend authorization remains mandatory on every request and reflects membership
revocation immediately. Business plans add migration 00006; no role assignment endpoint is introduced.

Pending/suspended businesses can manage existing data and orders, but the existing
backend approval gate blocks public listing/new bookings as applicable. Pausing
new bookings does not cancel existing orders. Application reasons are recorded in
the existing approval audit table; stock changes use the existing stock ledger.

## API additions and extensions

All paths are relative to `/api/v1` and require the existing verified session:

- `GET /stringers/{id}/manage/profile`: private profile, membership and permissions.
- `GET /stringers/{id}/manage/summary`: all-time counts and booking value in EUR.
- `GET /stringers/{id}/orders/{order_id}`: business-scoped order detail with the
  customer's current verified phone/name, for fulfilment. Ordinary customer order
  responses and order lists do not gain contact fields. A different business's
  order never returns contact details through this route.
- `GET /stringers/{id}/inventory/lots`: adds variant/product labels to each lot and
  supports `q`, `sport`, `variant_id`, `page`, and `page_size`. Physical lots remain
  visible even after a global catalogue item is deactivated.
- Catalogue and inventory searches also accept exact `variant_id` to look up a
  local offering without guessing which page contains it.

The dashboard uses the existing profile/service/inventory/stock/order mutation
endpoints. OpenAPI and the generated TypeScript declarations describe the current
responses. Money and quantity conversions are tested; prices and technical rules
are always validated again by the backend.

## Payment and retry boundaries

Booking value is not revenue and does not imply a payment was taken. The dashboard
shows payment status separately; associated staff can confirm cash received using the audited cash-receipt action.
This does not collect money online. See [business plans and cash](subscriptions-and-cash.md)
for the five-booking free allowance, paid subscriptions and payment boundaries.
Online customer checkout and its unpaid hold expiry remain future work. Booked value excludes cancelled orders and uses saved order prices,
so later changes to catalogue or service prices do not rewrite sales history.

Stock receipt/correction POSTs are not automatically retried. If a network or
server error leaves a write uncertain, the editor disables repeating that write
and asks the operator to check refreshed stock balances first. Price/service/profile
updates use the existing replacement PUT endpoints. Server-side idempotency for
manual stock receipt/correction is a future extension; order creation retains its
existing idempotency support.

## Validation

Real PostgreSQL integration tests cover owner/staff/admin metadata and writes,
revoked memberships, non-public profiles, per-business summaries/contact isolation,
filtered/paginated lots, exact variant lookup and inactive catalogue stock labels.
The existing race tests continue to cover reservations, consumption, cancellation
and order idempotency. Browser tests run on desktop and mobile with mocked API
responses, covering fulfilment, prices/stock units, reserved-stock rejection,
services, permissions, business switching, admin review, errors and uncertain writes.
No Twilio credentials, real customers or live messages are used by those tests.
