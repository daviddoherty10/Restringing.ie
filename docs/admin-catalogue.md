# Admin catalogue and racket management

Open `/admin/catalogue` as a platform administrator. The **Admin catalogue** account
link and **Manage shared catalogue** dashboard link are visible to administrators.
Customer, business owner and staff accounts do not gain global editing rights.
Both the private reads and existing writes enforce the verified session's current
platform role on the server. The UI does not fall back to public data if a private
request fails. Sign-in can return safely to the exact `/admin/catalogue` route.

This release uses the existing product, variant and racket schema. No migration,
production seed or real manufacturer data is added.

## Local preview

From the application branch containing these screens:

```bash
npm --prefix frontend ci
bash scripts/preview.sh seed
AUTH_SMS_PROVIDER= BILLING_PROVIDER= bash scripts/preview.sh api
```

In a second terminal:

```bash
bash scripts/preview.sh web
```

In another terminal:

```bash
bash scripts/preview.sh session admin
```

Open <http://127.0.0.1:5174/admin/catalogue> and apply the origin-guarded JavaScript
printed by the preview session command in the browser console. Sessions expire
after an hour. No Twilio or Stripe credentials are needed. Use customer/stringer/
staff preview sessions to check the access-denied screen.

Preview strings, racket constraints and technical sources are fictional. The
verified flag in the isolated fixture is for testing only. Do not copy those
values into production or use them to restring a real racket. Edits remain local
and persist on seed reruns; use the documented separate-volume reset only if you
want a fresh fixture. Production administrator bootstrap remains the trusted
operator process described in [phone authentication](phone-auth.md).

## String products

The **String products** tab supports brand/model search, sport/material/state
filters and pages of ten results. All states are included initially. You can:

- Create or edit the shared brand, model, sport, material and description.
- Enter comfort/control/power/spin/durability editorial ratings from 1 to 5.
- Set the catalogue's maximum pre-stretch percentage (0–30).
- Activate/deactivate the product.
- Open **Variants** for one exact product, including an inactive product.

New products start **inactive**, with blank ratings requiring deliberate entry.
Ratings describe catalogue preferences, not clinical evidence or medical outcomes.
Zero pre-stretch disables it for that product; use sourced product information
before allowing nonzero percentages. Each stringer's stock, availability and
material charge remain managed through its own dashboard.

Publishing a usable string requires both an active product and an active variant.
A product without variants does not appear as a bookable string. Deactivation does
not delete the record, stock lots, reservations, order snapshots or prices saved
on previous orders.

## Gauges and colours

The **String variants** tab can search all variants or show an exact product's
variants, with state/sport filters and pagination. Each row shows both the variant
and parent product's activation state. Inactive parents do not hide their children
from the administrator.

To create a variant, choose a product using the product search and open its
**Variants** action. There is no assumption that the product is in the first page
of fifty results. The editor loads that product afresh before creating the variant.
Enter a gauge of 0.40–2.00 mm with at most two decimal places and an optional colour
label. New variants start inactive. The product relationship is fixed in this UI;
existing API replacement semantics are otherwise preserved.

Changing an existing variant's gauge/colour changes how all businesses identify
that stock item. For a genuinely different physical product, create a new variant
and deactivate the old one instead. Existing order display snapshots remain saved.
There are no hard-delete controls.

## Racket models and verification

The **Racket models** tab includes verified and unverified records. Search by
brand/model, filter sport/verification, and paginate results. Creating/editing a
record requires the existing complete technical contract, even when unverified:

| Field | Entry unit / requirement |
| --- | --- |
| Brand, model, sport | Exact supported model identity |
| Minimum / maximum tension | kg, at most two decimals; positive and ordered |
| Tension increment | kg, 0.01–1.00 |
| Maximum mains/crosses difference | kg, 0–100 |
| Required string length | metres, 1–30 with at most three decimals |
| Allowed knots | At least one of two/four knots |
| Different tensions require four knots | Explicit sourced yes/no choice |
| Technical source | Document title, URL/reference, model/version and section; max 1000 characters |
| Verified | Explicit administrator confirmation of the model-specific constraints |

All new technical fields and knot rules start blank/unselected. No universal racket
limits or guessed string lengths are supplied. Tensions become integer centi-kg
and length becomes integer millimetres before submission; the backend validates
all fields again. Technical sources are rendered as plain text.

New models start **unverified**. Complete but not-yet-reviewed records can be saved
without verification. Only verified models are returned by the public model search
and accepted by booking validation. Turning verification off prevents new bookings
using that model; it does not prevent fulfilling an order already accepted/placed
with a saved setup. Catalogue validation failures at booking submission now clear
the rejected pending setup so the customer can select current verified data.

## API additions

All paths below are relative to `/api/v1`, require the existing verified session,
and are restricted to platform administrators. No private data has been added to
the existing public catalogue or racket routes.

| Method / path | Response / filters |
| --- | --- |
| `GET /admin/string-products` | `ProductPage`; `q`, `brand`, `sport`, `material`, `active`, pagination |
| `GET /admin/string-products/{productID}` | One full `Product`, including inactive |
| `GET /admin/string-variants` | `CatalogueVariantPage`; `q`, `brand`, `sport`, `material`, `gauge_mm`, `product_id`, `active`, pagination |
| `GET /admin/string-variants/{variantID}` | One `CatalogueVariant`, including own/parent activation and product labels |
| `GET /admin/rackets` | `RacketPage`; `q`, `brand`, `sport`, `verified`, pagination |
| `GET /admin/rackets/{racketID}` | One full `Racket`, including unverified |

Omit `active`/`verified` for all states; otherwise use `true` or `false`. Invalid
booleans, UUIDs, sport/material/gauge and pagination values are rejected. Pagination
uses the same stable ordering, 1–50 page size and 1–10000 page bounds as the public
API. Count/page reads can reflect concurrent changes.

Editors use the existing `POST` and replacement `PUT` endpoints for products,
variants and rackets. Current server validation, duplicate identity constraints,
Origin checks and quote/order transaction protection are preserved. Search results
are not used as a substitute for a fresh private detail read before editing.
OpenAPI and the generated TypeScript client declarations describe the new reads.

After a successful save the list reloads. A record may disappear from the current
filter after activation/verification changes; the confirmation message explains
this. Edits use the existing last-write replacement semantics; this release does
not add optimistic edit versions or a catalogue audit-history table. Avoid having
multiple administrators edit the same record simultaneously.

Creates are not automatically retried. After an ambiguous network/server failure,
the form disables another create and directs the administrator back to search for
the record before trying again. Duplicate identities are also constrained in the
database. This does not add general POST idempotency. Tabs/back are disabled during
a save; late reads from another editor are ignored. Server role checks still apply
if administrator permission changes while a form is open.

## Verification

PostgreSQL race/integration tests cover private reads and existing writes for admin,
owner, staff, customer and anonymous actors; searches beyond twenty records;
inactive parent/variant combinations; unverified models; exact parent filters;
invalid input; and order-history/fulfilment preservation after deactivation.

Desktop/mobile browser tests cover product editing/creation, intentional activation,
variant gauge precision and parent identity, technical unit conversion and explicit
verification, access denial, late private responses, uncertain creation, and recovery
from catalogue validation failures during booking. Existing authentication,
marketplace, billing and stringer-dashboard tests remain in place.
