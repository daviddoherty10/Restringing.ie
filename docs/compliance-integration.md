# Compliance integration review — 16 November 2026

This integration targets `main`; it is not a deployment or approval to launch.
Legal pages remain drafts. `BOOKING_LEGAL_ENFORCEMENT` and
`TRADER_VERIFICATION_ENFORCEMENT` remain false by default.

## Refreshed repository state

`main` was fetched and verified at `a926457d5b205320a94691ac4c04458e8bc1ae1b`.
GitHub's current PR pages and Git refs confirmed the outstanding stack:

| PR | Current head | Target / state observed |
| --- | --- | --- |
| #30 | `2782b06` compliance foundations | `main`, draft |
| #31 | `b290e88` compliant checkout | foundations, merged |
| #32 | `d785a6f` account management | checkout, merged |
| #33 | `1f8f3d9` trader compliance | account management, merged |
| #34 | `78a822d` marketplace transparency | trader compliance, merged |
| #35 | `88c1716` tax reporting readiness | account management, open |
| #36 | `b4d1323` compliance hardening | tax reporting readiness, open |

The hardening head contains the functional changes from all seven stages.
The foundations and checkout heads have additional merge commits, but their
merge-base-to-head file diffs against the hardening stack are empty. No functional
changes were omitted merely because those two heads are not ancestors.
GitHub's checks page for `b4d1323` showed the backend test job succeeded; that is
historical source-branch evidence, separate from the integration checks below.
The checks page for `a926457` showed the native image-publishing matrix. GitHub API
access was blocked by the environment policy during this review, so no aggregate
CI status is inferred from the old PR descriptions.

## Integrated changes and deployment conflict resolution

The merge retains reviewed booking versions and immutable confirmations; privacy
requests, reviewed exports and account closure; private trader verification and a
separate public identity; complaints/notices and independent decision reviews;
append-only actual cash facts; and protected compliance histories (migrations
14–19). Applied migrations 1–13 are unchanged. Generated OpenAPI and TypeScript
contracts were regenerated with the repository generator and openapi-typescript
7.13.0; their output matches the hardening branch.

The backend Dockerfile conflict was resolved with Go 1.27.3 and the existing
PostgreSQL-only Goose tags. The Go module minimum now matches that container patch
version, so CI uses the same reviewed toolchain rather than 1.27.1. Dependency
hardening from the compliance branch is retained.

Current `main` deployment fixes remain: repository-root build contexts, separate
preview and landing services, private database/API, migration-first startup,
existing project/volume names, prebuilt AMD64 defaults and native AMD64/ARM64
publishing for API/migrate/web/landing. The publishing workflow is unchanged;
no release images were published by this task.

Both build-based and prebuilt Compose variants now forward the two enforcement
flags with disabled defaults. `scripts/check-deployment.py`, run in backend CI,
renders both variants and checks flag defaults and independent opt-ins, API setting
parity, resolved contexts, private services, migration ordering, volume identity,
AMD64 defaults and explicit ARM64/commit-tag selection. Rendering test opt-ins does
not start services or enable an application workflow. CI also retains lean Goose
installation. Nginx syntax checks cover both web and landing configurations.

The existing account-lock safeguard is retained. A new database concurrency test
pauses a booking before order insertion, observes closure waiting on its account
lock, then verifies closure refuses the newly committed order dependency. A
configuration regression checks that only explicit `true` activates either flag
and that ambiguous values fail startup.

## Validation of the integrated result

- Go 1.27.3: formatting, vet, build and the complete PostgreSQL race suite passed
  (165 existing tests, no skips). The new concurrent-booking/closure regression
  passed separately under the race detector; the configuration regression passed.
- All 19 Goose migrations passed upgrade, rollback to zero and reapply on a
  disposable local database. Preview seeding ran twice without duplicate fixtures.
- OpenAPI and generated TypeScript consistency passed.
- Frontend lint, production build and all 10 unit tests passed. `npm audit` reported
  zero vulnerabilities across all dependencies.
- Both deployment configuration checks and both Nginx syntax checks passed.
- Desktop/mobile browser suite: 178 of 180 passed on the initial run. The desktop
  seven-page axe/keyboard test exceeded its 30-second budget after the audits;
  rerunning it alone with a 120-second budget passed (17.6 seconds for the run).
  The mobile collection-update assertion failed once and passed unchanged on its
  focused rerun (10.4 seconds). This intermittent failure was not reproduced or
  hidden by a production-code change. A pinned-browser CI run remains required;
  the initial suite is not represented as a clean 180-test pass.
- All four local AMD64 images built. An isolated Compose stack migrated to 19 and
  passed API health/catalogue, unauthenticated private-endpoint rejection, web and
  landing HTML/CSP, landing-script and container-health checks. No release images
  were published and no VPS was contacted.
- A fresh govulncheck run could not fetch `vuln.go.dev` because of the environment
  policy. Prior branch scan results are not represented as a fresh successful scan.

The pinned Playwright browser download was also policy-blocked. The local browser
run uses installed Chromium 156 with a temporary configuration; the repository's
Playwright 1.58.2 configuration and browser pin remain unchanged. This is a local
compatibility check, not evidence of a fresh pinned-browser CI run.

Container checks needed cloud-local adaptations outside the repository: a
BuildKit secret mount supplied the trusted proxy CA during Go/npm installation;
read permissions on imported migration/public static files were normalized; and
loopback-only health probes used `wget -Y off` to avoid the injected egress proxy.
TLS, package integrity and Go module verification stayed enabled. Application
Dockerfiles and deployed health checks do not contain these cloud-specific mounts
or proxy overrides. Source and prebuilt Compose paths were both rendered; the
isolated runtime used the build-based path with the freshly built local images.

## Business and launch decisions still required

- Confirm the operator and contracting trader, final identities/contacts, approved
  wording, withdrawal/early-performance treatment, contract formation and durable
  confirmation delivery. Publishing a document is not activation approval.
- Approve retention schedules, lawful retained records, processor arrangements,
  privacy/complaints queue coverage and an independent reviewer. Review DSA/P2B,
  accessibility and DAC7/MRDP applicability with qualified advisers; no tax IDs or
  automatic tax filings were added.
- Resolve the allowance discrepancy: the backend continues to count **five
  accepted bookings per business per Irish calendar month**, not completed jobs.
  This integration deliberately does not change counting or historical usage.
- Confirm the intended **EUR 15/month** Stripe test Price, no automatic VAT
  collection, and applicable invoicing/tax treatment. No provider configuration or
  live billing change was made. Test checkout, signatures/webhooks, cancellation
  and renewal in the sandbox before a separate live activation decision.
- Arrange a controlled real-device Twilio signup/login/logout test. Repository
  tests do not establish delivery, working credentials or real-device cookies.
- Separately verify VPS migration state, HTTPS/proxy/cookies, image architecture,
  a complete staged booking/fulfilment journey, built-image vulnerability scans,
  manual accessibility and an isolated backup-restore rehearsal. No VPS database
  was accessed and no deployment was performed here.

Actual cash facts remain separate from booked value, completion and legacy paid
markers. Nothing infers historical receipts. Media uploads, outbound job alerts,
staff invitations and ownership transfer remain separate product work.
