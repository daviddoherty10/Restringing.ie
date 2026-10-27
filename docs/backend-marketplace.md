# Marketplace backend

## Repository review and implementation order

Reviewed `main` at `ce00f75eac6c2b7c182c343a5861d4abf17ce164` and the booking/finder work on `feat/advanced-string-options` at `13abdc31806ed392aeee13f9e729bfa2ed6862ca` before making changes. Main had PostgreSQL configuration, a chi health endpoint, and only `00001_create_users.sql`. The advanced React branch uses 28 fictional strings, local state, example prices, a universal 15–35 kg range, and no API requests. Those example IDs and prices are not production records.

The advanced UI subsequently reached main at `7041daf9d963408c0fabac3a78cb379b1b7770a8`; the completed backend branch is rebased onto that commit. Existing UI rendering is preserved. The only frontend additions are generated contract types and an API client. Rendering remains a demonstration until connected to these endpoints.

Implementation order:

1. Preserve migration 00001; add sessions, businesses, membership and approval auditing.
2. Add shared products/variants, verified racket constraints, per-sport services, and sets/reels.
3. Implement pure technical validation, integer pricing and expiring quotes.
4. Add transactional order creation, stock reservation, status history, cancellation and consumption.
5. Enforce authenticated customer, business owner/staff and administrator access.
6. Rank available strings using explained rules and transient preference answers.
7. Publish OpenAPI, generated TypeScript types, examples, integration tests and CI.

## Migrations

| Migration | Added tables |
| --- | --- |
| 00002 | `sessions`, `stringers`, `stringer_members`, `stringer_approval_events` |
| 00003 | `string_products`, `string_variants`, `racket_models`, `stringer_services`, `service_prestretch_options`, `stringer_inventory`, `inventory_lots`, `inventory_movements` |
| 00004 | `quotes`, `orders`, `order_status_events`, `inventory_reservations`; adds order reference to stock movements |
| 00005 | Phone OTP challenges and shared authentication rate limits |
| 00006 | Business subscriptions, accepted-booking usage, checkout attempts, webhook audit, cash payment method and receipt history |

All are reversible goose SQL migrations. Down migrations remove their tables and data; use rollback only on a disposable development database or under a deliberate recovery plan. They do not rewrite existing users or introduce passwords. Production data is never seeded with the React demo catalogue or unverified technical limits.

A product identifies brand/model/material/sport and editorial attributes. A variant identifies gauge and colour. Each stringer has one offering per variant, its own material charge and availability flag. Inventory lots reference that offering, so another business's quantities/prices cannot be chosen during booking. User UUIDs remain independent of mutable phone numbers.

## Run locally

From the repository root, with Go matching `backend/go.mod`, Docker Compose and goose installed:

```bash
cp .env.example .env
docker compose up -d --wait db
set -a
source .env
set +a
goose -dir backend/migrations postgres "$DATABASE_URL" up
goose -dir backend/migrations postgres "$DATABASE_URL" status
go -C backend run ./cmd/api
```

Do not overwrite an existing local `.env`. `FRONTEND_ORIGIN` must be an exact origin such as `http://localhost:5173`, with no trailing slash. Configure the HTTPS frontend origin and production PostgreSQL credentials/TLS before deployment.

The catalogue starts empty. Use the administrator endpoints to create real products, variants and verified racket models, then configure a business and its stock. The integration tests demonstrate provisioning without creating public fictional listings.

## API contract

`api/openapi.json` is the machine-readable REST contract. Success responses are the resource itself; paginated collections use `{data, page, page_size, total}`. Errors use `{error: {code, message, fields?}}`. Requests use `application/json`, accept at most 16 KiB and reject unknown fields. IDs are UUIDs, unlike the UI's numeric/sample IDs. Pagination defaults to 20, allows 1–50 items and uses deterministic ordering. Page range is 1–10000. Counts and pages are separate reads and can reflect concurrent catalogue edits; refresh when filters change.

| Method and path (under `/api/v1`) | Access / purpose |
| --- | --- |
| `GET /strings` | Public shared catalogue; `q`, `brand`, `material`, `sport`, `gauge_mm`, `page`, `page_size` |
| `GET /rackets` | Public verified models; `q`, `sport`, pagination |
| `GET /stringers` | Public approved businesses; `q`, `county`, `sport`, pagination |
| `GET /stringers/{id}` | Approved public profile |
| `GET /stringers/{id}/services` | Active sport services and prices |
| `GET /stringers/{id}/inventory` | Offered, enabled variants with unreserved stock; catalogue filters |
| `GET /stringers/{id}/booking-configuration?racket_model_id=...` | Effective racket/service tension bounds, increments and knot patterns |
| `POST /stringers/{id}/recommendations` | Public, transient preference matching |
| `GET /me`, `GET /me/stringers` | Current verified identity and own businesses/applications with effective permissions |
| `POST /stringers` | Authenticated application; creates owner membership with pending approval |
| `PUT /stringers/{id}` | Owner/admin replaces editable profile fields |
| `GET /stringers/{id}/manage/services` | Members/admin include inactive services |
| `PUT /stringers/{id}/services/{sport}` | Owner/admin replaces service policy and percentages |
| `GET /stringers/{id}/manage/inventory` | Members/admin include disabled offerings |
| `PUT /stringers/{id}/inventory/{variant_id}` | Members/admin sets material price and availability |
| `POST /stringers/{id}/inventory/{variant_id}/lots` | Members/admin receives physical sets/reel |
| `GET /stringers/{id}/inventory/lots` | Members/admin stock balances, paginated |
| `POST /stringers/{id}/inventory/lots/{lot_id}/adjustments` | Members/admin audited quantity correction |
| `POST /stringers/{id}/quotes` | Authenticated customer; validated 10-minute quote |
| `POST /orders` | Authenticated customer; quote ID and required idempotency header |
| `GET /orders` | Customer's history, optional status and pagination |
| `GET /orders/{id}` | Customer, associated stringer members or admin; includes status events |
| `GET /stringers/{id}/orders` | Members/admin incoming orders, optional status and pagination |
| `PATCH /orders/{id}/status` | Business workflow; customers can cancel their own placed order |
| `GET /admin/stringer-applications` | Admin pending queue, paginated |
| `PATCH /admin/stringers/{id}/approval` | Admin approve/reject/suspend; reason is audited |
| `POST /admin/string-products`, `PUT /admin/string-products/{id}` | Admin catalogue products and editorial ratings |
| `POST /admin/string-variants`, `PUT /admin/string-variants/{id}` | Admin gauges/colours and activation |
| `POST /admin/rackets`, `PUT /admin/rackets/{id}` | Admin verified technical constraints and source |

Profile input: `{name, description, area, county, accepting_orders}`. Names are required/max 120; descriptions max 2000; area/county max 120. Approval fields are not accepted here. Admin review accepts `{status: "approved"|"rejected"|"suspended", reason}` with max 1000 characters. Pending/rejected/suspended businesses cannot receive quotes or orders or appear in public search. Membership still permits managing existing orders after suspension.

Inventory input is `{material_price_cents, available}`. A public offering is not a guarantee of enough string for every racket: quotes and recommendations additionally check a single lot against the selected model's required length. `gauge_mm` is an exact filter (0.40–2.00); `q` also searches gauge/colour. Materials are `multifilament`, `synthetic_gut`, `polyester`, `natural_gut`, `nylon`, `hybrid`. Sports are `tennis`, `badminton`, `squash`.

Service input (complete replacement; percentage keys are JSON strings):

```json
{
  "sport": "tennis", "active": true, "labour_cents": 1500,
  "own_strings": true, "own_strings_cents": 0,
  "stencil": true, "stencil_cents": 300,
  "two_knots": true, "two_knots_cents": 0,
  "four_knots": true, "four_knots_cents": 0,
  "different_tensions": true, "different_tensions_cents": 0,
  "min_tension_centi_kg": 1500, "max_tension_centi_kg": 3500,
  "step_centi_kg": 50, "max_difference_centi_kg": 200,
  "prestretch_options": {"5": 400, "10": 400}
}
```

Those are illustrative service settings, not recommendations or universal racket limits. Each price is an integer 0–1000000 cents. At least one knot pattern is required. Bounds must be positive and ordered, maximum tension at most 10000 centi-kg, increments 1–100, maximum difference 0–10000. Pre-stretch keys must be 1–30 and each price 0–1000000 cents. No pre-stretch is always represented by 0 and adds no charge.

## Units and technical validation

- Money is **integer EUR cents**. Material price is the per-racket material supplement added to labour, not a reel purchase price. All configured customer-facing amounts must be tax-inclusive final charges for this EUR-only booking flow; tax accounting, delivery charges and invoicing are not implemented here. No hidden fees are added.
- Tensions use **integer centi-kilograms**: 24.5 kg is `2450`. The UI converts its display value; the server checks both model and service bounds and increments. There is no universal 15–35 kg rule.
- Sets use whole counts. Reels use integer millimetres: a 200 m reel is `200000`. Set lots additionally declare `set_length_mm` (1000–30000); reels use 0. Each lot quantity is 0–100000000. Receipt quantity is positive.
- Only verified racket models can be quoted. Curated records include brand/model/sport, ordered tension bounds, increment, maximum mains/crosses difference, allowed knots, whether different tensions require four knots, required string length and a technical source. Unknown models require manual curation instead of guessed safety limits.
- Both tensions are explicit. When the UI's independent-tension toggle is off, send equal values. Differences require stringer support and must satisfy both maximum-difference limits.
- Knots must be 2 or 4 and supported by both model and service. The model's four-knot rule applies when tensions differ. A catalogue hybrid setup also requires four knots.
- Stencilling, own strings, and each nonzero pre-stretch percentage require explicit service support. Pre-stretch must also be within the catalogue product limit. For unknown customer-supplied string material, nonzero pre-stretch is rejected; supply a known global variant if applicable.
- Additional instructions are max 500 Unicode characters, plain text, and never override technical fields. Render them as text in React; do not use HTML insertion.

Receipt examples:

```json
{"kind":"set","quantity":8,"set_length_mm":12000}
```

```json
{"kind":"reel","quantity":200000}
```

Adjustments use `{quantity_delta, reason}` (nonzero delta with magnitude at most 100000000; reason 1–200 characters). Balances cannot become negative or smaller than reserved stock. Each receipt, adjustment, reservation, release and consumption has an inventory movement.

## Quote → order

1. Read a live profile, verified racket model, booking configuration and inventory. The UI must replace demo IDs with these UUIDs.
2. Send configuration to `POST /stringers/{id}/quotes` using the verified session:

```json
{
  "racket_model_id": "00000000-0000-4000-8000-000000000001",
  "string_source": "stock",
  "variant_id": "00000000-0000-4000-8000-000000000002",
  "mains_tension_centi_kg": 2400,
  "crosses_tension_centi_kg": 2350,
  "knots": 4,
  "stencil": true,
  "prestretch_percent": 0,
  "instructions": "Please use a black stencil."
}
```

The UUIDs above are placeholders. For own strings use `string_source: "customer"`; `variant_id` can be omitted, subject to the pre-stretch restriction. The response includes immutable display snapshots, server-calculated `breakdown`, `total_cents`, `currency` and `expires_at`. It contains no fingerprint or client-editable total. Limit: approximately 30 quotes per customer per minute; configure ingress request limits too. A quote checks stock but does not hold it.

3. Display the returned breakdown. Change any option → request a fresh quote.
4. Send `POST /orders` with `{ "quote_id": "..." }` and an `Idempotency-Key` header (16–100 letters/digits/`._:-`, e.g. `crypto.randomUUID()`). Reuse that key and quote after a network timeout. A successful first request returns 201; a replay returns the same order with 200. A key reused with another quote returns 409. A different key cannot order the same quote twice.
5. Order creation locks the quote and business, revalidates approval/service/model/catalogue/price, compares the stored fingerprint, selects a sufficiently long set or single reel and reserves it, inserts the order and audit events, then commits. Any failure rolls everything back. A price or technical change requires a new quote (409); stock can also have sold out since quoting.
6. The order preserves the original configuration, product/model names and price lines, even after catalogue edits.

All business policy/inventory/order writes acquire the business row lock. Administrative catalogue/model writes are coordinated by shared locks while repricing. Idempotent attempts also take a transaction-scoped advisory lock. This deliberately serializes writes within one small stringer business; unrelated businesses remain independent. SQL values are parameterized. Constraint conflicts and retryable database deadlocks produce safe errors, not SQL details.

## Workflow and payment boundary

Normal transitions: `placed → accepted → received → in_progress → ready → completed`.

Customers can cancel only their own `placed` order. Business members/admin can cancel `placed`, `accepted` or `received` orders. Cancellation releases a reservation. Transitioning to `in_progress` consumes the allocated set or recorded model-specific reel length; it is not consumed again at completion. Cancellation after work starts and reopening terminal orders are deliberately rejected. Manual stock corrections need their own audited adjustment.

One order currently represents one racket and one stringer. The stock allocation is one full-bed string/packaged hybrid variant. Selecting two independent hybrid variants and multi-racket checkout are future extensions, not silently approximated as a single stock allocation.

**Customer bookings use cash paid directly to the stringer.** See [plans, usage and cash receipts](subscriptions-and-cash.md). Only associated business members/admin can confirm a cash receipt, with an audit record. Creating an order does not charge the customer. Every new order has `payment_status: "unpaid"`. No customer endpoint can mark it paid; the restricted cash-receipt endpoint records manual receipt separately from fulfilment. SMS issuance is now implemented through Twilio Verify; see [phone sign-in](phone-auth.md). Business subscription checkout and verified webhooks are implemented separately from customer payments. Online customer checkout remains a separate integration. Fulfilment currently follows the business's manual booking workflow; add a provider-confirmed payment gate before launching payment-required online fulfilment. Cash-booking stock remains reserved until cancellation or work starts; an unpaid-checkout hold/expiry worker must accompany the payment integration, with a defined expiry/refund policy. Do not treat an order response as a completed online purchase.

## Phone OTP and access control

There are no passwords, default administrator accounts, fake login endpoints or trusted `X-User-ID` headers. Customer identity always comes from a live session joined to an active, phone-verified user. Session revocation/expiry and current admin role are checked on every private request. A customer can simultaneously own or work for a business.

The [phone OTP integration](phone-auth.md) verifies the normalized phone with Twilio Verify before finding/creating the user and issuing a session:

1. Generate 32 cryptographically random bytes and encode with unpadded base64url.
2. Store SHA-256 of the **encoded token string** in `sessions.token_hash`, along with user ID and a bounded expiry.
3. Send the token via a `Secure`, `HttpOnly`, `SameSite=Lax` cookie named `session_token`, path `/`, under HTTPS. Use `credentials: 'include'` in React. A development-only cookie policy may be necessary for localhost HTTP.
4. Revoke the row for logout; do not log the raw token. Phone changes and recovery need verification and session revocation policy. Bootstrap platform administrators through a trusted operator process after phone ownership is verified.

Bearer authorization using the same opaque token is supported for API clients and integration tests. Tokens are not JWTs and are not accepted as arbitrary user IDs. Cookie-authenticated writes require an `Origin` exactly matching `FRONTEND_ORIGIN`; CORS permits credentials only for that origin. The TypeScript client does not put tokens into localStorage. Do not expose the API publicly with a fabricated development session issuer.

Public reads cannot inspect pending applications or other businesses' management data. Owners change profile/services; owner/staff manage stock and incoming orders; only platform admins curate the catalogue/technical constraints and approve listings. Order lookup returns 404 for unrelated users. The [stringer dashboard](stringer-dashboard.md) exposes current owner/staff permissions and operational management screens. Staff invitation/ownership-transfer UI and APIs are not included.

## Recommendation rules and data minimisation

Request example:

```json
{
  "racket_model_id": "00000000-0000-4000-8000-000000000001",
  "experience": "club",
  "sessions_per_week": 3,
  "preferences": {"comfort":5,"control":3,"power":2,"spin":2,"durability":4},
  "prioritise_arm_comfort": true
}
```

Experience is `new`, `club`, or `advanced`; frequency is 0–21 sessions/week. Preference weights are 0–5; omitted weights default to 1. Optional `current_variant_id` supplies current material context; optional current tension is accepted as context only and does not result in a tension recommendation. Current variant must match the racket sport. No diagnosis, injury history, free-text medical details, or health profile is accepted.

Candidates must belong to the selected, approved, accepting stringer, match the racket sport, be active/enabled, and have a sufficiently long unreserved set or reel. Each result reports its editorial ratings, exact score and individual reasons:

- Sum of preference weight × catalogue rating for comfort/control/power/spin/durability.
- New experience: +2 × comfort; advanced: +control.
- At least 3 sessions/week: +durability.
- Optional comfort emphasis: +4 × comfort, without predicting injury outcomes.
- Same material category as a known current setup: +1.

Return the top three, with UUID as a deterministic tie-breaker and algorithm version `weighted-catalogue-v1`. Catalogue ratings are editorial 1–5 attributes, not clinical evidence or guarantees. The response makes no medical claims and prescribes no tensions.

These inputs, including the optional comfort flag, are **not persisted** in users, quotes, orders, analytics or logs. HTTP request bodies are not logged. Keep reverse-proxy/APM body capture disabled for this route too. Responses use `Cache-Control: no-store`; send answers by POST rather than URL parameters. No recommendation history or health-profile table exists.

## Frontend integration and generated contracts

`frontend/src/lib/marketplace-api.ts` is the cookie-aware API client. Set `VITE_API_BASE_URL` to the backend base including `/api/v1`. It provides typed catalogue/profile/configuration/quote/order/recommendation calls and structured errors; no UI demo is silently converted into a real checkout.

From the repository root regenerate contracts after changing Go DTOs/routes:

```bash
go -C backend run ./cmd/openapi > api/openapi.json
npx --yes openapi-typescript@7.13.0 api/openapi.json -o frontend/src/lib/marketplace-contract.d.ts
```

The OpenAPI generator uses the Go response DTOs and explicit endpoint metadata. CI checks regeneration produces the committed contract, and a Go test compares every registered chi route/method with OpenAPI. Preserve the original idempotency key until you know whether order creation succeeded.

## Verification

Unit tests:

```bash
go -C backend test ./...
```

Integration tests require an explicitly selected disposable PostgreSQL database with permission to create schemas:

```bash
export TEST_DATABASE_URL='postgres://postgres:test@localhost:5432/marketplace_test?sslmode=disable'
go -C backend test -race -count=1 ./...
```

Tests create isolated `marketplace_test_*` schemas and remove only those schemas afterwards. They exercise migrations and rollback, session revocation/CSRF, role/tenant boundaries, catalogue pagination beyond 20 strings, model-specific technical constraints, quotes, immutable prices, last-stock races, concurrent idempotency, stock release/consumption, and recommendation inventory scope. CI also applies/rolls back/reapplies the real goose migrations against PostgreSQL 17. No live customer database is used.
