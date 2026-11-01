# Simple string offerings and hybrid setups

A stringer selects a catalogue variant, enters **one full-bed material price**,
marks it **In stock / Unavailable**, and saves. New offerings default to manual
availability and need no stock receipt, quantity or string-length entry. Labour
remains a separate sport-service setting, charged once per racket.

## Pricing

A full bed uses the offering's full-bed material price. For a stocked hybrid:

`material cents = (mains full-bed cents + crosses full-bed cents + 1) / 2`

This is integer division: round the combined material once, half-up to the nearest
cent. For example, €14 mains material and €10 crosses material produce €12 material
plus labour/selected services. Swapping strings does not change the total; the
same string twice costs exactly its full-bed material price, including odd cents.

The server itemises mains and crosses. Mains gets its half rounded up; crosses
gets the remaining material total. If both full-bed prices have odd cents, this
assigns the rounding cent to mains. A zero-priced component remains zero. The UI
shows **Half of €X full bed** rather than displaying two individually rounded
half prices that might not sum to the final material charge.

There are no separate half-racket/mains/crosses price controls or price endpoint.
The earlier unmerged PR's price columns remain unused for migration compatibility;
current full-bed prices are authoritative. Quotation fingerprints include both
full-bed prices and tracking modes; changes require a fresh quote before ordering.
Existing order prices, component labels and successful idempotent retries remain
unchanged. No client price or stock quantity is accepted in a booking.

## Optional stock tracking

**Advanced stock settings (optional)** has two independent choices:

- **Track full-bed stock quantities** (`track_stock`) checks measured set/reel stock
  for full-bed orders. It does not restrict which strings can be used in hybrids.
- **Track hybrid portion quantities** (`track_hybrid_stock`) explicitly enables
  exact portion checks/reservations for hybrid components. Leave it off to use the
  offering's shared availability for either mains or crosses, with no extra stock
  entry or separate half-bed offering.

An ordinary offering with full-bed stock can therefore appear in both selectors,
even if it has no `half_racket` lot. Existing explicit hybrid portion tracking is
preserved. The shared catalogue identity, full-bed material price and availability
flag apply to both full-bed and hybrid configurations.

Save the offering before receiving stock in advanced receipts. Tracked hybrids
use one whole **half-racket portion** per component; the same variant twice needs
two portions and may use separate portion lots. No hybrid lengths are required.
When a mode is off, no string quantity is reserved/consumed for that mode: manage
its availability manually and reconcile counts after untracked work. Record
physical material once; there is no automatic conversion between portions and
sets/reels or claim that a mains cut equals half a physical string length.

A hybrid may combine a tracked offering with an availability-only offering. Only
the tracked component is reserved/consumed. Purchased grips always retain their
existing unit tracking. Creating an order, all purchased-item reservations,
ledgers and notifications is one transaction; any failure rolls everything back.
Eligible cancellation releases reserved items; starting work consumes them once.

Before enabling tracking, count current stock, including material used while
tracking was off. Tracking cannot change while that offering has reserved stock
(`409 stock_tracking_in_use`); complete/cancel those reservations first. Omitting either tracking
field preserves that setting. Enabling tracking after an
availability-only order never allocates stock retroactively: fulfilment uses the
allocations created with the order. Public inventory and the full-bed finder
include manually available offerings without requiring a lot.

Migration 12 retains tracking on all existing offerings to preserve their balances
and reservations, then makes new offerings default to manual availability. It
does not reset counts, change historical allocations or convert stock. Migration
13 separates hybrid tracking: old set/reel-only offerings immediately become
selectable for hybrids through shared availability, while existing portion-based
tracking and pending hybrid allocations remain tracked. Neither migration resets
stock or removes reservations.

## Customer-supplied hybrids

Enable **Hybrid stringing**, **Four-knot stringing** and **Accept customer-supplied
strings** under Services. Customers choose **I'll provide my own strings → Hybrid**
and identify mains and crosses independently from the shared active catalogue,
even if the stringer does not stock them. Alternatively, **My mains/crosses string
is not in the catalogue** accepts a 1–160 character identifying description.

Customers bringing both strings pay labour, configured own-string handling once,
selected services and any purchased grip. There is no string material charge,
reservation or string-stock deduction. Original order details distinguish the
supplied mains and crosses. Bring both labelled; the stringer confirms suitability
before starting work.

A single pre-stretch percentage applies to both components and must be supported
by the service and both catalogue products. If either component is described
rather than a known catalogue variant, pre-stretch is disabled. Verified racket,
sport, tension, knot and other service constraints remain in force; hybrids
require four knots. Packaged `material: hybrid` catalogue products are not
individual component strings. Mixed purchased/customer-supplied components and
separate component pre-stretch remain future work.

## API

Paths use `/api/v1`. Generated OpenAPI/TypeScript describe exact shapes.

| Endpoint / field | Behaviour |
| --- | --- |
| `PUT /stringers/{id}/inventory/{variantID}` | Current business member/admin saves `{material_price_cents, available, track_stock?, track_hybrid_stock?}`. Price 0–1,000,000 integer cents. New default: availability only; omitted tracking preserves existing mode. |
| Variant responses | `material_price_cents` is full-bed material; optional `track_stock` and `track_hybrid_stock` identify independent full-bed/hybrid quantity modes (absent/false means manual availability for that mode). |
| `GET /stringers/{id}/hybrid-inventory` | Requires `racket_model_id` and `component=mains\|crosses`; search/material/brand/gauge/exact-variant filters and pagination. Active available offerings; explicitly hybrid-tracked offers additionally need a free portion. No private balances. Quote checks combined tracked demand. |
| `GET /strings?individual_only=true` | Global active single-string catalogue for customer-supplied components, independent of stringer inventory. |
| Existing quote/order booking | `hybrid: true`; `variant_id` identifies mains and `crosses_variant_id` identifies crosses. Both are `string_source: stock` or both `customer`. Supplied components may instead use `customer_mains_description` / `customer_crosses_description` (one ID or description per component). |
| Existing order configuration | Immutable `mains_snapshot` / `crosses_snapshot`, with customer-supplied labels where applicable. Legacy length snapshots remain readable; new hybrids do not use them. |
| Existing lots / adjustments | Optional advanced receipts and corrections. `set`: whole sets plus package length; `reel`: millimetres; `half_racket`: whole portions, `set_length_mm: 0`. |

Customer choices and pending purchase keys survive sign-in/reload; slow restoration
cannot overwrite the saved setup. One racket uses one accepted booking allowance
unit, regardless of materials or grip count. Cash/payment and communication flows
remain unchanged.

## Deployment, preview and validation

Apply migrations through 13 and deploy matching backend/frontend with a controlled
restart. Earlier published migrations are retained for databases that already
applied them. Their historical fields remain unused; migration 11's rollback
intentionally refuses to delete portion lot/ledger/order history. Prefer a forward
fix or a verified backup. No production database is migrated by this change.

Fresh previews include the Baseline hybrid service and tracked sample stock to
exercise historical orders. New offerings use the simple default. Existing set/reel-only offerings gain hybrid selection after migration 13 without editing each offering. Existing marked
preview databases preserve edits on reseed. To try the simple flow on an existing
offering, turn tracking off in Advanced settings once its reservations are cleared.
Preview data is fictional and must not become live technical guidance.

Checks cover no-receipt full-bed/hybrid booking and recommendations, odd-cent and
same-string pricing, availability changes, tracking-mode/default/legacy behaviour,
tracked/manual hybrid combinations with grips, atomic failure/race/retry handling,
stock release/consumption, supplied-string catalogue/descriptions/pricing, role
boundaries, sign-in/restoration and mobile layouts.
