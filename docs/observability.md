# Operating the preview

Start with external outage alerts, Coolify's server/container metrics and API logs.
PostHog adds browser error tracking; it does not replace an external uptime check.
There is no need to run a separate monitoring stack on the application VPS yet.

## External uptime alerts

Use a hosted monitor outside the Hetzner VPS, for example
[Better Stack](https://betterstack.com/docs/uptime/monitoring-start/).
Create these HTTP monitors after deploying the readiness endpoint:

| Monitor | Expected result |
| --- | --- |
| `https://restringing.ie/` | HTTP 200 and `Restringing.ie` in the body |
| `https://preview.restringing.ie/` | HTTP 200 |
| `https://preview.restringing.ie/api/v1/ready` | HTTP 200 and `"status":"ok"` |

Start with a one-minute interval, five-second timeout and an alert after two
consecutive failures. Enable recovery and TLS-expiry alerts where available.
Select your alert email in the provider's dashboard, confirm it and send a test
alert. If the preview is behind an access gate, use dedicated monitor credentials
in the provider's secret settings, never in the URL.

`GET /api/v1/health` is a process-only liveness check. The new
`GET /api/v1/ready` also pings PostgreSQL with a two-second deadline and returns
503 `not_ready` when unavailable. It never calls Twilio or Stripe, sends SMS,
creates payments or exposes database errors. Keep liveness separate from
readiness: restarting the API repeatedly will not fix a database outage.
Readiness checks connectivity, not migration completeness or the booking flow.

## Coolify metrics and notifications

Enable [Sentinel metrics](https://coolify.io/docs/core/observability/monitoring/sentinel)
on the existing server. Inspect CPU, memory, disk usage and container restarts.
Investigate sustained CPU/memory above 80%, disk above 80%, and unexpected
restarts. These are starting thresholds, not measured capacity limits.
Both Compose deployments now rotate API logs at 10 MB, retaining three files.
Check rotation for the other containers too so they cannot fill the disk.

In the team's [Notifications](https://coolify.io/docs/core/notifications/overview),
configure an email channel and enable deployment failure, container/server
failure and backup failure events supported by the installed Coolify version.
Send a test notification. Coolify alone cannot notify you reliably when the
server hosting Coolify itself is unreachable; keep the external monitor.

## API request logs

The API writes JSON to stdout. In Coolify's API logs, `http_request` entries include
method, matched route template, response status, duration in milliseconds and
request ID. Responses include the same `X-Request-ID`; use it to correlate a
response with its log entry. Server errors (5xx) are logged at ERROR level.
Unknown paths use `unmatched` or a static router wildcard rather than the URL.

Request logs omit URL/query values, path identifiers, cookies, authorization
headers, request/response bodies, phone numbers and user identities. Request IDs
are included; do not put personal data in an `X-Request-ID` header. Restrict log
access, choose a short retention period and avoid copying raw operational logs
into public issues. A log drain is optional later; local logs are lost with the
server and are not a durable audit store.

If `/ready` fails, check PostgreSQL container health and disk space, then API logs
and the latest deployment/migration result. If only the frontend fails, check the
web container and proxy routing. If the API is healthy but login or billing fails,
check `/api/v1/auth/config`, Twilio delivery failures and Stripe webhook delivery
history. An unavailable provider is not proof that the whole app needs a restart.

## PostHog: use the existing project

The connected project is now named **Restringing.ie**, ID **353943**, in EU Cloud.
The frontend captures unhandled JavaScript errors, promise rejections and React
render errors using [PostHog Error Tracking](https://posthog.com/docs/error-tracking/installation/react).
Product autocapture, replay, pageviews, console capture, surveys, feature-flag
fetching, persistence and user identification are disabled. Only `$exception`
events pass the payload filter. Each page load gets a random identifier; the
filter drops error messages and browser/user properties, retaining the release
and same-origin `/assets/*.js` stack locations with query strings removed.
Location enrichment is disabled in the payload. The vendor still receives network
requests; this is data minimization, not a claim of zero personal-data processing.
Redacted messages/types reduce debugging detail; use stack locations and releases.

The project token and host are configured in the ignored `frontend/.env.local`
for local verification. A missing configuration is a production no-op and emits
a development console error without preventing the app from rendering.

For **prebuilt images**, add these repository Actions **variables** before publishing:

| Variable | Value |
| --- | --- |
| `VITE_POSTHOG_PROJECT_TOKEN` | Public project token from PostHog project settings |
| `VITE_POSTHOG_HOST` | `https://eu.i.posthog.com` |
| `POSTHOG_PROJECT_ID` | `353943` |
| `POSTHOG_HOST` | `https://eu.posthog.com` |

The workflow supplies `VITE_RELEASE` from the commit SHA. Setting Vite variables
only in Coolify's runtime environment will not change an already-built image.
For source builds, set the public Vite values in `deploy/.env`; Compose forwards
them to the web build. Never put a personal API key in a `VITE_` variable.

[Private source-map uploads](https://posthog.com/docs/error-tracking/upload-source-maps/vite)
are wired through the Vite plugin and GitHub's `POSTHOG_PERSONAL_API_KEY` secret,
using a BuildKit secret mount only for the web image. Give that key error-tracking
write and organization read access. It is not an image build argument or browser
variable. This secret is **pending at the user's request**: builds work without
it, upload is skipped, and error stacks remain minified. When enabled, the plugin
uploads maps and deletes them before Nginx receives the build output. Do not serve
maps publicly. Native ARM64 and AMD64 builds may both upload their matching maps.

Review retention and processor/privacy arrangements, configure an error alert
and usage limit, and verify a synthetic error after deploying. Keep analytics and
replay disabled. A same-origin ingestion proxy can help if ad blockers drop error
events, but is deferred until needed.

## Verification and remaining setup

Run `go -C backend test -race ./internal/marketplace` with `TEST_DATABASE_URL`
pointing only at a disposable PostgreSQL database. Tests cover database readiness,
unavailable connections, unchanged liveness and omission of private request data
from route-template logs.

After deploying, verify the three monitor targets over HTTPS, correlate a log
entry with its response request ID, and send test notifications. Rehearse database
outages only in an isolated test environment. Backup completion alerts are not a
substitute for a restore test.

PostHog's project is configured and a sanitized local browser event was accepted
by EU ingestion. Browser verification also checked captured stack locations,
omission of synthetic private values, no analytics events and no persistence.
The public preview is not instrumented until its new image is built and deployed.
External monitors, notification delivery and Sentinel remain operator setup
steps; repository changes alone do not activate them. The preview uses a pinned
image tag: after merging and image publishing, explicitly select the new commit's
AMD64 tag and redeploy through Coolify.
