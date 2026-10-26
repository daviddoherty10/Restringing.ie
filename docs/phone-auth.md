# Phone sign-in

Customers, stringer owners/staff and administrators use the same phone-code flow.
New verified numbers create ordinary customers. Existing users retain their UUID,
account status, platform role and business memberships. No client can choose a role.

## Enable SMS

The implemented provider is [Twilio Verify](https://www.twilio.com/docs/verify/api).
The Go `VerificationProvider` interface allows another provider later. Credentials
are backend-only and must not appear in React/Vite configuration or source control.

1. Create a Twilio Verify Service with SMS enabled and a six-digit code. Keep the
   default ten-minute code validity; this app also expires challenges after ten
   minutes and allows at most five checks per challenge. Enable the service's fraud
   protection and only the destination countries you intend to serve. Trial account
   recipient restrictions still apply.
2. Create an API key authorised to access that service. Configure the backend's
   environment using your existing secret storage or untracked `.env`:

   ```dotenv
   AUTH_SMS_PROVIDER=twilio
   TWILIO_VERIFY_SERVICE_SID=VA_your_service_sid
   TWILIO_API_KEY_SID=SK_your_api_key_sid
   TWILIO_API_KEY_SECRET=your_api_key_secret
   AUTH_RATE_LIMIT_KEY=your_random_64_hex_characters
   AUTH_ALLOWED_CALLING_CODES=+353,+44
   ```

   The SID examples are placeholders; actual SIDs have the prefix plus 32 hex
   characters. Generate the rate-limit key once with `openssl rand -hex 32` and
   share it across API replicas. Partial/invalid enabled-provider configuration
   fails startup. An empty `AUTH_SMS_PROVIDER` deliberately disables SMS issuance;
   the sign-in page explains that SMS is unavailable. Existing sessions still work.
3. Apply migrations, then restart the API and frontend:

   ```bash
   # From the repository root, with your existing .env configured:
   set -a
   source .env
   set +a
   goose -dir backend/migrations postgres "$DATABASE_URL" up
   go -C backend run ./cmd/api
   # In another terminal:
   npm --prefix frontend ci
   npm --prefix frontend run dev
   ```

4. Visit `http://localhost:5173/signin`, request a code for a phone you control,
   verify it, open `/orders`, then sign out. A live SMS incurs provider charges.
   No live SMS is sent by the automated tests.

For the isolated preview database, first run `bash scripts/preview.sh seed` to
apply migration 00005, export the SMS environment variables in the API terminal,
then run `bash scripts/preview.sh api`. Use `http://127.0.0.1:5174/signin` when
running `bash scripts/preview.sh web`. The existing preview session CLI remains an
alternative for fictional accounts; those reserved fictional numbers cannot
receive real SMS. Signing in with your real number creates a separate preview user.

## Endpoints

All paths below are under `/api/v1`. JSON errors use the existing
`{error: {code, message}}` envelope. Auth responses always use `Cache-Control:
no-store`. POSTs require an `Origin` exactly equal to `FRONTEND_ORIGIN`, including
code request/verification and logout, to prevent login CSRF and cross-site sends.

| Method/path | Input | Result |
| --- | --- | --- |
| `GET /auth/config` | None | Availability, allowed calling codes and resend delay; no secrets |
| `POST /auth/phone/request` | `{phone_number: "+353871234567"}` | 201: opaque `challenge_id`, `expires_at`, `resend_after_seconds` |
| `POST /auth/phone/verify` | `{challenge_id, code}` | 200: `{id, admin}` and HttpOnly session cookie |
| `POST /auth/logout` | None | 200: `{signed_out: true}`, server revocation and expired cookie |
| `GET /me` | Session cookie | Current verified identity and platform admin flag |

Phone inputs require an explicit E.164 calling code. The backend strips spaces,
parentheses and hyphens; it does not guess a country. The frontend defaults local
entry to Ireland, offers the allowed calling codes, and accepts `+`/`00`
international entry. Syntax validation is separate from provider deliverability
validation. Users sign up and sign in on the same screen; there is no account
existence check or different "registered number" response.

## Session and verification boundaries

Twilio generates and checks the codes. The app stores no codes and never logs
request bodies, provider response bodies, credentials or session tokens. A random
32-byte challenge identifier is returned to the browser; only its SHA-256 hash is
stored with the phone, provider verification SID and attempt/expiry metadata.
Checks are bound to the issued provider SID, not a caller-supplied phone number.

An attempt is claimed and committed before the provider call. Simultaneous checks
cannot both issue sessions; process crashes cannot reset the attempt budget. After
approval, challenge consumption, live account checks, user creation and session
insertion run in one transaction. An uncertain provider failure fails closed and
requires another code; a provider-approved code cannot always be recovered after a
subsequent database/network failure. Request a new code in that case. A resend
invalidates earlier local challenges, even when the provider reuses its current
code. Suspended/closed accounts cannot sign in. Signup never restores their status.

Sessions are 32 random bytes encoded as unpadded base64url. Only SHA-256 of the
encoded token is stored. The cookie is host-only, Path `/`, HttpOnly, SameSite=Lax,
and Secure for HTTPS. It expires after seven days with no automatic renewal. A
successful sign-in revokes the previous session supplied by that browser and
issues a new token; other devices remain signed in. Logout revokes only the
current session and is safe to repeat, including after session expiry. A database
failure is reported rather than claiming server-side logout succeeded.

Production frontend/API must use HTTPS on the same site (e.g. `restringing.ie` and
`api.restringing.ie`), or route the API through the frontend origin. SameSite=Lax
cookies do not support unrelated cross-site deployments. `FRONTEND_ORIGIN` is the
exact frontend origin; public HTTP origins are refused. Loopback HTTP development
is supported. Use the same hostname consistently on both sides (`localhost` with
`localhost`, or `127.0.0.1` with `127.0.0.1`). React uses credentials/include and
never reads or persists the session token in browser storage.

Customers can also be stringer owners/staff. Backend authorization still checks
current memberships and roles on every private request. Administrator bootstrap,
phone-number changes, account recovery and staff invitations are operator/future
flows; this PR adds no public privilege assignment or unverified phone change.

## Abuse limits, proxy setup and retention

PostgreSQL stores atomic per-hour buckets, shared across all API instances:

- SMS requests: 200/hour globally, 20/hour per IP, 5/hour per phone.
- At least 60 seconds between sends to the same number, including failed sends.
- Verification checks: 60/hour per IP, 10/hour per phone and 5 per challenge.
- Default permitted calling codes: Ireland `+353` and United Kingdom `+44`.

Hourly buckets reset at the hour boundary; the per-phone cooldown applies across
that boundary. 429 responses include `Retry-After`, which the frontend reads via
CORS. Provider limits may be stricter. These deliberately conservative app limits
supplement provider fraud protection and spend controls; adjust constants only
with observed traffic and abuse requirements.

The limiter uses HMAC-SHA256 keys rather than storing raw IPs in the rate table.
It ignores forwarded headers by default. Behind a reverse proxy, configure only
that proxy's actual peer CIDRs in `AUTH_TRUSTED_PROXY_CIDRS`, for example
`10.20.0.0/24`. Do not trust the entire network or expose a route around your proxy.
The rightmost untrusted X-Forwarded-For hop becomes the client IP. Without proxy
configuration, all proxied users share their proxy's IP budget.

Each send performs bounded cleanup of challenge records expired over 24 hours ago
and expired rate buckets. Low-traffic installations may retain expired metadata
until the next request; a scheduled database maintenance job can enforce a stricter
retention deadline. Ordinary user/session/business records are not deleted by it.
Do not enable reverse-proxy/APM body or cookie/header capture for auth endpoints.

## Browser behaviour and tests

The navigation shows sign-in or account links plus sign-out. Unauthenticated order
and management pages link to sign-in with an allowlisted internal return route.
Booking sign-in stores only the setup and any pending idempotency identity in
sessionStorage, scoped to this tab/stringer for 30 minutes. It stores no tokens,
quote prices, recommendation answers or comfort concerns. The configuration and
selected stock are reloaded from the API after return; a fresh quote is required.
Pending order retries retain their original quote/key instead of creating another
order. Signing out clears temporary booking drafts.

Verification:

```bash
# Set TEST_DATABASE_URL to a disposable PostgreSQL database:
go -C backend test -race -count=1 ./...
npm --prefix frontend run lint
npm --prefix frontend run build
npm --prefix frontend test
# Browser tests use mocked API responses, never SMS credentials:
cd frontend
npx playwright install chromium
npm run test:e2e
```

PostgreSQL integration tests exercise signup through quote/order/history/logout,
existing identities and roles, account blocks, cookie flags, rotation/revocation,
expiry, replay, attempt/send budgets, concurrent sends/checks, country/Origin
boundaries, resend invalidation and provider failures. Provider unit tests verify
the actual Twilio request/response contract with a simulated HTTP transport.
Live SMS delivery still needs a manual check with your configured service and a
phone you control; automated success is not a claim of real provider delivery.
