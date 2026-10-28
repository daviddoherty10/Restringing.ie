# Grip add-ons and multi-item stock reservations

Customers can optionally add one fitted overgrip or replacement grip to a racket booking. **No new grip** is the default. The stringer's fitted price includes both the individual grip and fitting; the server adds one `grip` price line. It is an add-on to the same racket order, not another restringing for the monthly allowance.

## Catalogue and stock

Platform administrators use **Shared catalogue → Grips** to create/edit a grip brand, model, type, colour and description. Each type/colour is its own shared product. New products start inactive in the UI; activation is deliberate. Case-insensitive brand/model/type/colour identity is unique, avoiding duplicate shared products. Catalogue creation uses a required retry key, so an uncertain request can be retried without inserting a second product. Deactivation preserves existing offerings, reservations, movements and historical orders.

Business owners and current staff with inventory access use **Stringer portal → Grips** to select catalogue products, set fitted EUR prices and availability, and receive/correct physical stock. Selecting an already-stocked catalogue product loads its current offering before editing; it never silently resets the existing price or stock. Business membership is checked by the backend on every private action.

Grip stock uses whole individual units, separately from string sets/reels. Each offering has physical quantity, reserved quantity and a calculated free balance. Saving price/availability does not change quantities. Receiving/correcting stock records a signed nonzero `quantity_delta` and a required reason. A correction cannot make stock negative, remove reserved units or exceed 1000000 physical units. Fitted prices allow 0–1000000 integer cents; a zero-price grip is still a selectable offered option.

Manual stock changes require `Idempotency-Key`, unique per actor across grip stock writes. The server serializes retries, checks the exact offering/delta/normalized reason and returns the original movement on replay. Reusing a key for another payload returns 409. The ledger includes manual changes and order reservations/releases/consumption and can be inspected in the grip editor. Private physical/reserved balances are never returned by the public offering endpoint.

## Booking, quote and fulfilment

The booking picker searches/filters/paginates only active catalogue products the selected stringer has enabled with at least one unreserved unit. Selection survives filters, pagination, sign-in and reload. It restores against the selected stringer's current availability; a sold-out saved grip blocks a new quote until the customer chooses another grip or no grip. A failed optional grip-list request does not prevent booking without a grip. Each form change clears a reviewed quote, and the original pending order attempt still blocks setup changes until resolved.

`Booking.grip_product_id` is optional. The API validates it as a UUID and looks up this business's offering. Clients cannot supply a price, inventory ID or snapshot. The pure pricing function checks the selected offered product and adds its stored fitted price. Its metadata/availability/price participate in the quote fingerprint; free stock counts do not. Quotes check stock but do not reserve it. Price/catalogue/availability changes before checkout invalidate or reject the quote. Actual stock is checked again under the existing business lock at order creation.

Orders save an immutable `grip_snapshot` with brand/model/type/colour, alongside the chosen product ID, fitted price line and original total. Stringer incoming-order cards flag a grip add-on, and customer/stringer order details show the booked grip. Later catalogue/price changes never rewrite historical order snapshots or totals. Existing cash receipts/refund limitations, express terms, technical string constraints, notifications and monthly-plan rules remain in force. Customer-supplied strings can be combined with a stocked grip when the underlying service supports those strings.

Migration 00009 extends `inventory_reservations` from one row per order to `(order_id, component)`. Old reservations retain their quantities, states, lot IDs and movement history under `full_bed`. A grip allocation uses component `grip` and a grip inventory reference instead of a string lot. Database constraints prevent mixed/empty resource references and enforce one grip unit. Reserved rows are now transitioned as a set, with no single-row-per-order assumption.

The entire booking, all string/grip reservations and movements, status event and in-app notification commit in one transaction. A failure anywhere rolls everything back. Cancellation before work releases all still-reserved items. First transition to `in_progress` consumes/allocates the string and grip to the job; later fulfilment updates cannot consume them again. Grip fitting is part of the delivered job; the ledger records the unit as issued when work starts, rather than claiming a separate real-time fitting timestamp. Starting work fails atomically if any movement/balance change fails.

The component model and transition helper allow future `mains`/`crosses` allocations. **True hybrid selection, sourced component lengths and component pricing are not implemented here.** No full-bed length is split or half-set balance invented. Those rules must be added explicitly in the next feature.

## REST contracts

All API paths below start with `/api/v1`. The usual real session, current role/membership, exact Origin for cookie writes, strict JSON and bounded pagination rules apply. The generated OpenAPI/TypeScript contract remains authoritative.

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/grips` | Active shared products; `q`, `brand`, `kind`, `grip_product_id`, pagination |
| GET | `/stringers/{stringerID}/grips` | Bookable offered products and fitted prices; no private balances |
| GET | `/stringers/{stringerID}/manage/grips` | Members/admin: include existing disabled offerings and stock balances |
| PUT | `/stringers/{stringerID}/grips/{gripProductID}` | Members/admin: replace `{fitted_price_cents, available}`; stock untouched |
| POST | `/stringers/{stringerID}/grips/{gripProductID}/stock-events` | Members/admin: `{quantity_delta, reason}`; required retry key; 200 on create/replay |
| GET | `/stringers/{stringerID}/grips/{gripProductID}/stock-events` | Members/admin: paginated ledger, newest first |
| GET | `/admin/grips` | Admin-only list including inactive products; optional `active` filter |
| POST | `/admin/grips` | Admin-only creation with required retry key; 201 on create/replay |
| PUT | `/admin/grips/{gripProductID}` | Admin-only replacement of catalogue metadata/activation |

Product fields are `{brand, model, kind, colour, description, active}`, with `kind` restricted to `overgrip` or `replacement_grip`. Product/movement IDs and prices come from the server. Search uses the existing page 1–10000/page-size 1–50 bounds. The booking picker uses five items per page; management/catalogue uses ten.

The new write forms lock their payload during an ambiguous failure and offer a retry using the same key while mounted. Pending grip catalogue/stock retry identities are not persisted across leaving/reloading the screen. After an uncertain write and reload, inspect the catalogue or stock ledger before submitting a new action. Existing order attempts retain their prior session-draft retry protection across reloads.

## Migration, rollout and preview

Apply `00009_grip_addons_and_item_reservations.sql` before starting the updated API. Deploy the matching backend during a controlled restart: the old backend's single-reservation assumption must never process new orders containing multiple items. Existing orders without a grip require no JSON backfill or price change. Existing standard quotes remain compatible when no grip is selected. Rollback removes grip allocations and feature tables while retaining historical order JSON; use it only deliberately with backups/disposable data. A later true-hybrid migration must unwind its multi-component rows before rolling back past this reservation migration.

`bash scripts/preview.sh seed` applies migrations to the isolated preview database and preserves all existing edits. Fresh previews include three available fictional fitted grip options, one empty-stock option and a consumed grip on the in-progress order ending `040004`. Existing seeded databases are not overwritten: use the admin preview session to add catalogue products and the stringer/staff session to enable and receive them. No real payment, SMS or outbound provider operation is performed.

Validation includes concurrent last-unit booking, atomic booking/status/notification rollback, release and exactly-once consumption with stock and customer-supplied strings, correction/retry limits, current business permissions, public balance privacy, catalogue pagination/activation, strict client price rejection, immutable order snapshots, monthly usage and preview ledger conservation. Browser tests cover catalogue safe retries, staff offering edits, stock timeout recovery, reserved-unit protection, optional/zero-priced selections, pagination, sign-in/reload preservation, sold-out choices, business-switch races and narrow layouts.
