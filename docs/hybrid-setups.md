# Hybrid string setups

A hybrid chooses mains and crosses separately. Each stocked string has **one fixed
half-racket material price**, used for either role regardless of racket model.
Customers can instead bring both strings, identifying each from the shared
catalogue or describing it if missing. No hybrid component lengths are entered.

## Stringer setup

1. Apply migrations 10 and 11 after migration 9 and deploy the matching backend
   and frontend with a controlled restart. Migration 11 is additive for databases
   that already applied the first version of PR26; migration 10 is not rewritten.
2. In **Stringer portal → Services**, enable hybrid stringing for the sport and
   four-knot stringing. To accept customer-supplied hybrids, also enable
   **Accept customer-supplied strings**. Existing racket verification, sport,
   tension, tension-difference, knot and pre-stretch constraints still apply.
3. In **Inventory**, save each offered string and enter **Half-racket material
   price (€)**. This single fixed price applies to mains or crosses. Blank/null
   disables the offering for stock hybrids; zero deliberately offers material free.
   It is never silently calculated by halving the full-bed price.
4. Choose stock type **Half-racket portions (hybrid stock)** and enter how many
   half rackets you can supply. No metre or racket-specific cut length is required.
   One mains or crosses component uses one portion; the same string in both roles
   needs two portions. Stock corrections and balances use whole portions.

For example, string A at €7 per half racket and string B at €5 produce €12 material
cost, with labour and selected services added once. Swapping A/B between mains
and crosses does not change those material prices.

Half-racket portions are an operational count of available component servings,
not a promise that every racket uses the same physical length. The stringer
manages that count and confirms material suitability before work. Record physical
material once: do not book the same material as both portions and measured
sets/reels. There is no automatic conversion, split, replenishment or inferred
remainder from existing measured stock. Existing full-bed set/reel stock remains
available for its original booking flow. Hybrid portions are not advertised as
full-bed set/reel stock.

An administrator no longer has to enter hybrid mains/crosses lengths to enable
booking. The existing full-bed racket length belongs to the earlier measured
full-bed inventory feature; it is not used to price or allocate half-racket stock.
The model must still be verified and allow four knots.

## Customer-supplied hybrids

Select **I'll provide my own strings**, then **Hybrid — choose mains and crosses**
(or select Hybrid first). Choose each string independently from the shared global
catalogue, regardless of whether the selected stringer stocks it. These choices
identify material the customer brings; they do not purchase the stringer's stock.

For a string missing from the catalogue, tick **My mains/crosses string is not in
the catalogue** and enter its brand/model/gauge or other identifying details in
1–160 characters. Either or both components can use this description option.
The confirmation and immutable order details identify which supplied string is
for mains and which is for crosses. Bring both labelled for the stringer.

The server charges labour, the configured customer-supplied handling fee once,
knot/other selected services, and any optional fitted grip. It adds **no string
material charge and reserves/consumes no stringer string stock** for supplied
hybrids. An optional purchased grip still reserves/releases/consumes normally.
One racket counts as one accepted booking toward the business allowance.

A single pre-stretch percentage applies to both strings. It must be offered by the
stringer and within both known catalogue limits. If either string is described
rather than identified in the catalogue, pre-stretch is disabled because its
technical limit is unknown. Racket/service technical checks remain in effect.
A catalogue product marked `material: hybrid` is a packaged combination, not an
individual mains/crosses string, and is excluded from component selection.

Both components must come from the same source in this release: both stringer
stock or both customer supplied. Mixing one purchased component with one supplied
component and separate per-component pre-stretch remain future extensions.

## Transactions and history

Quotes check availability but do not reserve. Order creation reprices and reserves
all purchased items in one existing business transaction. Stock hybrids reserve
one `mains` portion, one `crosses` portion and any grip; supplied hybrids reserve
only a purchased grip. Identical variants account for both portions before
allocation and can use separate portion lots. Any stock shortage or later failure
rolls back the complete order, allocations, ledgers and notifications.

Eligible cancellation releases every reserved item. Starting work consumes every
item exactly once under the existing status workflow. This records stock use,
not a measured physical cutting/fitting timestamp. Cash payment, conversations,
notifications and plan usage keep their existing boundaries.

The server fingerprints both selections/descriptions, prices, catalogue and
service/racket constraints. Changes require a fresh quote before a new purchase.
Existing orders retain their booked price and component snapshots. Exact order
retries return the original order, including after an offering is withdrawn or
stock sells out. Pending purchase identity and both customer choices persist
through reload/sign-in. Slow restoration cannot overwrite the saved draft.

## API

All paths use `/api/v1`; generated OpenAPI/TypeScript describe exact types.

| Endpoint / field | Behaviour |
| --- | --- |
| `GET /stringers/{stringerID}/hybrid-inventory` | Requires `racket_model_id`, `component=mains\|crosses`. Active offered strings with a fixed price and at least one free half-racket portion. Search/material/brand/gauge/exact-variant filters and pagination; no private balances. Final quote checks combined demand. |
| `PUT /stringers/{stringerID}/inventory/{variantID}/hybrid-prices` | Current business member/admin replaces `{half_racket_price_cents}`. Null disables; integer 0–1,000,000. Offering must exist. No stock/full-bed price mutation. |
| Existing variant responses | Optional `half_racket_price_cents`; the same price serves mains and crosses. |
| Existing stock receipt / lot | `kind: half_racket`, whole `quantity`, `set_length_mm: 0`. Existing audited corrections and lot listings show whole portions. |
| `GET /strings?individual_only=true` | Shared active single-string catalogue for supplied components; excludes packaged hybrids. Supports existing search/filter/pagination. No selected-stringer stock requirement. |
| Existing service replacement | `hybrid` capability plus `own_strings` for supplied hybrids; four knots required. |
| Existing booking | `hybrid: true`, `string_source: stock\|customer`. `variant_id` identifies mains, `crosses_variant_id` identifies crosses. Supplied components can instead use `customer_mains_description` / `customer_crosses_description`; use one identifier or description per component. |
| Existing order configuration | Immutable `mains_snapshot` and `crosses_snapshot`, with customer-supplied labelling where applicable. Legacy length snapshots remain readable for already-created orders; new bookings do not populate them. |

Public quotes never accept client material prices, labels or stock quantities.
Session/origin, current membership, admin and strict JSON checks remain enforced.

## Preview, migration and validation

Fresh preview seeds enable Baseline Workshop's tennis hybrid service, fixed €6.50
half-racket prices and fictional portion stock. Existing marked databases retain
edits on reseed; enable the feature and receive portions through the portal if the
preview predates it. Preview data is fictional, never live technical guidance.

Migration 11 preserves unused migration-10 columns and historical snapshots.
Where old mains/crosses prices were identical, it carries that value over exactly;
differing prices remain unset until a stringer chooses their fixed price. It does
not convert measured stock. Deploy the matching application together; an old
backend must not accept new bookings with the revised schema/contract. Rolling
migration 11 down intentionally fails while portion lots remain, rather than
deleting stock/ledger/order history. Prefer a forward fix or a verified backup.

Tests cover half-racket prices, no required hybrid lengths, zero/disabled values,
whole-portion receipt/corrections, same-variant demand across lots, last-cross
contention, atomic all-item failures, immutable retries, stock/supplied source and
role checks, global catalogue selections, descriptions, no material charge/stock
use for supplied strings, pre-stretch restrictions, cancellation, fulfilment,
plan usage, sign-in/reload and mobile layout.
