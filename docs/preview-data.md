# Local preview data

Use this to preview the live API-backed frontend with believable **fictional** data. It runs a separate PostgreSQL database, API port and frontend port. It does not seed the normal development/production database, change `.env`, call an SMS provider, or charge a payment method.

Requirements: Docker Desktop/Compose, the Go version in `backend/go.mod`, goose, Node/npm. The scripts are run from the repository root. If necessary on macOS, install goose with `brew install goose`.

## Start a preview

Prepare the database once (safe to repeat):

```bash
bash scripts/preview.sh seed
```

This starts the dedicated PostgreSQL container, applies the existing goose migrations to it, and loads the dataset in one transaction. If it is already seeded, it prints the current counts and preserves your edits, orders and stock changes.

Start the preview API in one terminal:

```bash
bash scripts/preview.sh api
```

In a second terminal, start the live frontend:

```bash
cd frontend
npm ci
cd ..
bash scripts/preview.sh web
```

Open **http://127.0.0.1:5174/search**. Use `127.0.0.1` consistently rather than `localhost`, so frontend Origin checks and preview cookies match. The default Vite port is deliberately different from your regular development session.

The frontend must contain the API integration from [PR #15](https://github.com/daviddoherty10/Restringing.ie/pull/15). If that PR is not merged, the old demo UI will still show its hardcoded examples rather than the seeded database. The backend seed is independent and does not merge or change that frontend PR.

To try an unmerged live frontend alongside this backend branch, keep `seed`/`api` running from this checkout and use a separate worktree for the frontend:

```bash
git fetch origin
git worktree add ../Restringing-live-preview origin/feat/live-marketplace-frontend
cd ../Restringing-live-preview/frontend
npm ci
VITE_API_BASE_URL=http://127.0.0.1:8082/api/v1 npm run dev -- --host 127.0.0.1 --port 5174 --strictPort
```

The worktree is only needed until the live frontend is merged; afterwards use the normal `web` command above.

## What is included

| Data | Preview scenarios |
| --- | --- |
| 40 shared product/variant records | 28 tennis strings, 6 badminton and 6 squash across three fictional brands; material, gauge and colour variations, including a packaged hybrid |
| 7 stringer businesses | Six approved listings in Dublin, Galway, Cork and Limerick, including a paused business; one pending application in Westmeath |
| 164 per-stringer offerings | Shared catalogue IDs with different prices; enabled, disabled and out-of-stock examples; 26 bookable catalogue entries at the first stringer for pagination |
| Service policies | Full-option and restricted stringers; two/four knots, separate tensions, own strings, stencils and varied pre-stretch percentages |
| Sets and reels | Set lengths, millimetre-based reel quantities, reservations, stock movements and consumed/released allocations |
| Four fictional racket models | Tennis with all options, tennis four-knot-only, badminton and squash |
| Eight historical orders | Seven customer orders spanning placed, accepted, received, in progress, ready, completed and cancelled; one other customer's order for isolation checks |
| Customer, owner, staff and administrator identities | Optional short-lived sessions to preview protected screens; no automatic sign-in |

All orders have `payment_status: unpaid`. No actual payment or SMS delivery occurred. Some manually progressed sample orders are completed but unpaid, matching the current backend's separate fulfilment/payment states.

Every business/product/model is labelled as preview data. The racket `verified` flag exists solely to exercise booking validation in this isolated dataset. Its tension limits, lengths, product ratings and pre-stretch settings are **fictional test inputs**, not manufacturer guidance or advice for restringing a real racket. Populate real, sourced technical data separately for production.

The phone fields use Ofcom's [fictional mobile range](https://www.ofcom.org.uk/phones-and-broadband/phone-numbers/numbers-for-drama), not real customer contact details. The preview tool has no SMS integration.

Useful URLs:

- Search: http://127.0.0.1:5174/search
- First profile: http://127.0.0.1:5174/stringers/90000000-0000-4000-8000-000000000101
- Booking form: http://127.0.0.1:5174/stringers/90000000-0000-4000-8000-000000000101/book
- Customer history: http://127.0.0.1:5174/orders
- Business portal: http://127.0.0.1:5174/manage
- API health: http://127.0.0.1:8082/api/v1/health
- API catalogue: http://127.0.0.1:8082/api/v1/strings?page_size=20

Whether a particular management screen is visible depends on the current frontend implementation. The sample data does not add missing dashboard UI.

## Preview customer/stringer/admin sessions

Public search, profiles, inventory, configuration and recommendations need no login. For quotations, order history, bookings and management:

```bash
bash scripts/preview.sh session customer
```

The command prints a browser-console instruction. Open Developer Tools **on http://127.0.0.1:5174**, inspect the short instruction generated by your local tool, and execute it in that page's console. It sets a one-hour preview cookie and reloads the page. The instruction refuses to run on another origin.

Other roles:

```bash
bash scripts/preview.sh session stringer
bash scripts/preview.sh session staff
bash scripts/preview.sh session admin
```

Run the printed instruction after each role change. The stringer owns the first business; staff belongs to that business without owner permissions; the admin can review the pending application. The customer sees only their seven fixture orders. Sessions still pass through the normal database-backed authentication and access-control middleware.

This is a deliberate local development credential, with a JavaScript-set cookie for convenience. It is not an OTP login implementation or the production HttpOnly cookie-issuance flow. The CLI is not imported by the API executable and adds no HTTP login/backdoor route. Tokens are random, hashed in the database, expire in one hour, and can only be minted for the fixed fictional identities after validating the preview database marker. The script never creates a session in your normal database.

To leave a preview session, clear `session_token` in the browser's storage tools, or on the preview origin run:

```js
document.cookie = 'session_token=; Path=/; Max-Age=0; SameSite=Lax'
location.reload()
```

For curl/automated clients, the lower-level command returns JSON instead of a browser instruction:

```bash
go -C backend run ./cmd/preview session -role customer
```

Use its token as `Authorization: Bearer …`. Treat it as a credential even though it belongs only to fictional local accounts; do not commit it.

## Isolation and repeatability

| Component | Default |
| --- | --- |
| Docker Compose project | `restringing-preview` |
| Database | `restringing_preview` |
| PostgreSQL host port | `127.0.0.1:5544` |
| Docker volume | Separate `preview_postgres_data` volume under the preview project |
| API | `127.0.0.1:8082` |
| Frontend / allowed Origin | `http://127.0.0.1:5174` |

The tool deliberately ignores the application's `DATABASE_URL`. URL validation permits only loopback hosts and the exact database name `restringing_preview`, rejects connection-override parameters, and verifies the connected database's name again before writing. First-time seeding requires empty application tables; a dataset marker and version identify subsequent runs. A transaction/advisory lock prevents partial or duplicate seeds. User-created preview changes are never reset on a normal rerun.

If the PostgreSQL preview port is occupied, choose another preview DB port consistently for all commands:

```bash
PREVIEW_DB_PORT=5545 bash scripts/preview.sh seed
PREVIEW_DB_PORT=5545 bash scripts/preview.sh api
PREVIEW_DB_PORT=5545 bash scripts/preview.sh session customer
```

The frontend still uses API port 8082. Port collisions fail rather than silently choosing another frontend port and breaking CORS. Do not start this fixture API as a public production service.

## Stop or reset

Stop the API and frontend terminals with Ctrl-C. Stop PostgreSQL while keeping your preview changes:

```bash
docker compose -f compose.preview.yaml stop
```

To start over with the original fixtures, stop the API/frontend first, then remove **only the separate preview project's volume**:

```bash
docker compose -f compose.preview.yaml down -v
bash scripts/preview.sh seed
```

This removes preview accounts, orders and sessions. It does not target the volume in `compose.yaml`. Do not use application goose down/up as a fixture reset: the metadata marker is intentionally outside production migrations.

## Tests

`go test ./...` runs URL/CLI guards and skips database tests without an explicit test URL. With `TEST_DATABASE_URL` pointing to a disposable local PostgreSQL server whose test role can create databases, the tests create `restringing_preview` if needed, use isolated `preview_seed_test_*` schemas within it, and clean up only those schemas. Existing public preview data is not seeded, changed or deleted by the tests.

Tests cover first seeding and reruns, retained edits, shared pricing, API pagination, recommendations, quotations/new bookings, all role sessions, stock ledger consistency, parallel seeds, refusal to mix existing data, and rollback after an injected failure. CI also starts the actual separate Compose stack, runs the seed script twice, and removes only that disposable preview stack afterwards. Production API routes/contracts remain unchanged.

The [stringer dashboard guide](stringer-dashboard.md) covers order, inventory,
service and profile management with the stringer/staff/admin preview sessions.
