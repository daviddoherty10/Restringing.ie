# Hybrid string setups

A customer can choose separate mains and crosses variants offered by the selected
stringer. The original full-bed and whole customer-supplied string flows remain
available. A catalogue product whose material is `hybrid` is a packaged product;
it is not a selectable individual component in the hybrid builder.

## Enable a hybrid service

1. Apply goose migration `00010_hybrid_setups.sql`, after migration 9, and deploy
   the matching backend and frontend with a controlled restart.
2. In **Shared catalogue → Racket models**, an admin records separately sourced
   mains and crosses cut lengths, including working allowance, and their technical
   reference. The model must be verified and allow four knots. Empty component
   fields disable hybrid booking; there is no automatic 50/50 split. Lengths are
   integer millimetres, each 1,000–30,000 mm, with a combined maximum of 30,000 mm.
3. In **Stringer portal → Services**, an owner/admin enables hybrid stringing for
   the sport. Four-knot support is mandatory. Existing tension, tension-difference,
   knot and pre-stretch constraints still apply.
4. In **Inventory**, business inventory managers save each offered variant, set
   explicit mains and/or crosses material prices, and receive physical reel stock.
   Blank/null component price disables that component. Zero deliberately offers
   it free. Prices are for one racket component, independent of full-bed prices;
   fitting/labour and optional services continue to use the service price list.

Existing models, services and prices are not automatically opted in. Do not copy
fictional preview technical data into a live catalogue. Historical order JSON,
prices and allocations are retained; migration 10 does not rewrite old orders.
Do not run an older backend while accepting new hybrid bookings. Rolling down
removes configuration/prices; take backups and stop new writes before rollback.
Migration 9's multi-component reservations remain necessary for existing hybrids.

## Booking and pricing

**String setup → Hybrid** exposes independently searchable, material-filtered,
paginated mains and crosses selectors. Choices persist while searching/changing
pages and through the existing sign-in draft. Four knots are required. A single
pre-stretch choice applies to **both** components, must be offered by the stringer,
and cannot exceed either product's catalogue limit. Separate component pre-stretch
percentages and mixed customer/stock sourcing are not offered in this release.
The existing whole customer-supplied flow remains a full-bed configuration.

The server checks both active catalogue variants, business offerings, sport,
explicit component prices, sourced model lengths, service capability and technical
constraints. It adds distinct `mains_strings` and `crosses_strings` price lines.
Labour, knot, tension-difference, stencil, pre-stretch, express and optional fitted
grip charges are each applied once. One racket consumes one plan allowance unit
when accepted, regardless of string/grip allocation count. Cash payment semantics
are unchanged.

Quote fingerprints include both variants and their component prices, model and
service rules and optional grip. Changes require a fresh quote before a new
purchase. Order configuration snapshots both variant labels and exact cut lengths;
customer history and stringer order details show the booked components. Changes to
catalogue, component prices or service availability do not rewrite an existing
order. Replaying the same order key/quote returns the original order even after a
component is disabled or sells out.

## Physical stock

The first release supports **reels only** for hybrid components. Whole sets remain
whole units for full-bed orders; no half-set remainder is invented. Measured cut
pieces, splitting sets, and joining partial lengths across reels need a separate
stock model and are not implemented.

Each component needs a single reel with sufficient unreserved millimetres. If both
components use the same variant, a single reel must cover their **combined** cut
length. This conservative rule avoids counting the same available length twice;
it can reject a combination that could be fulfilled manually from two reels.
The component listing checks individual availability, so the final combination
may still fail a server quote when its combined demand exceeds a reel's balance.

Quotes check availability but do not reserve. Order creation reprices, then locks
and reserves both cut lengths and any grip in the existing business transaction.
The reservation components are `mains`, `crosses` and optional `grip`. Any missing
stock or late failure rolls back the order, all allocations, ledger movements and
notifications. Eligible cancellation releases all components. Starting work
consumes all components exactly once using the existing status workflow; this is
the recorded stock-use point, not a separately measured physical cutting time.
Both string allocations can reference the same reel, with separate audited rows.

Ambiguous order requests keep their original quote and idempotency key across
reload, including when an offer disappears while the customer retries. A pending
purchase must be resolved before editing the setup. Restoring a draft waits for
both component lookups before saving refreshed selections. A nonpending restored
selection that is no longer bookable requires another selection/full-bed setup.

## REST additions

All paths use `/api/v1`.

| Endpoint | Behaviour |
| --- | --- |
| `GET /stringers/{stringerID}/hybrid-inventory` | Public approved/open stringer, verified racket, active hybrid service and available reel-cut component offerings. Requires `racket_model_id` and `component=mains\|crosses`. Supports `q`, `material`, `brand`, `gauge_mm`, `variant_id`, `page`, `page_size`. Public data excludes physical/reserved balances. |
| `PUT /stringers/{stringerID}/inventory/{variantID}/hybrid-prices` | Current business member/admin replaces `{mains_price_cents, crosses_price_cents}`; each nullable or integer 0–1,000,000. Offering must already exist. Repeatable replacement; does not change stock/full-bed price. |
| Existing racket curation | Adds optional `mains_length_mm`, `crosses_length_mm`, `hybrid_technical_source`; all empty or all valid, independent of full-bed length. |
| Existing service replacement | Adds optional `hybrid` capability, default false; requires four knots. |
| Existing variants/offerings | Include optional nullable `mains_price_cents` and `crosses_price_cents`. |
| Existing quote request | Adds optional `hybrid: true` and `crosses_variant_id`. `variant_id` identifies mains. Both components must use `string_source: stock`. |
| Existing quote/order configuration | Adds optional mains/crosses display snapshots and integer component lengths. |

UUID, bounded pagination/search, strict JSON, session/origin, current membership,
admin curation and transaction checks use the existing application boundaries.
Generated OpenAPI and frontend TypeScript remain authoritative for field shapes.

## Preview and checks

Fresh preview seeds enable Baseline Workshop's tennis hybrid service and the
fictional **Tennis All Options** model (6.4 m mains, 5.6 m crosses), with explicit
component prices on its eligible reel offerings. Preview reruns preserve existing
edits and do not backfill configuration into an older marked database. Existing
preview databases can enable the feature through admin/service/inventory screens;
there is no destructive reset instruction or automatic live catalogue population.

Tests cover explicit/free/disabled prices, technical and sport rules, pre-stretch
intersection, curation validation, participant permissions, filtering/pagination,
quote invalidation, immutable retries, separate and identical-variant allocation,
last-cross contention, all-item rollback after notification/consumption failures,
cancellation, fulfilment and one-unit plan usage. Browser coverage includes both
selectors, independent search/pagination, reload/pending purchase, unavailable
restored selections, exact prices, admin curation, service rules and narrow phones.

Hybrid combination recommendations, separate per-component pre-stretch, measured
cut pieces and component-specific customer supply remain future work. The guided
finder continues recommending available full-bed offerings only and makes no
medical claims.
