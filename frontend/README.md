# Restringing.ie frontend

React 19, TypeScript and Vite, backed by the Go marketplace API. Use Node 24,
matching backend CI. Install dependencies with the committed lockfile:

```bash
cd frontend
npm ci
```

## Develop with local data

The recommended full-stack workflow uses the isolated fictional preview dataset.
From the repository root, with Go matching `backend/go.mod`, Docker Compose and
Goose available:

```bash
bash scripts/preview.sh seed
```

Then run these in separate terminals, also from the repository root:

```bash
bash scripts/preview.sh api
bash scripts/preview.sh web
```

The web launcher sets `VITE_API_BASE_URL` and uses port 5174; the API uses 8082.
Use `127.0.0.1` consistently for origins and session cookies. See
[preview data and role sessions](../docs/preview-data.md) for sign-in to fictional
customer, stringer, staff and administrator accounts. Preview sessions do not send
SMS. Keep SMS, billing and legal enforcement disabled unless separately approved.

For a separately running API, use `npm run dev` from this directory and set
`VITE_API_BASE_URL` to its `/api/v1` URL if it differs from the default. Match the
API's `FRONTEND_ORIGIN` to the frontend's exact origin. Vite variables are public
build-time configuration; never put credentials in them.

## Checks

From this directory:

```bash
npm run lint
npm run build
npm test
npx playwright install chromium
npm run test:e2e
```

Playwright starts its own Vite server on port 4173 and uses mocked API responses
for desktop/mobile tests. These checks do not prove real SMS or Stripe delivery.
`npm run build` checks TypeScript and creates the production bundle in `dist/`.

## Working on the UI

The UI covers player bookings and the stringer's daily workflow. Players can search profiles, book full-bed or hybrid setups with optional grips/express service, and track orders or send messages. Business and administrator screens handle management, the shared catalogue and compliance review.

Reuse the shared cookie-aware [API client](src/lib/marketplace-api.ts). Regenerate TypeScript declarations from OpenAPI when the contract changes. Keep `marketplace-contract.d.ts` generated. See [backend setup](../docs/backend-marketplace.md).

Legal information remains draft, and enforcement defaults to disabled. See
[compliance review](../docs/compliance-implementation.md) and the
[launch checklist](../docs/compliance-launch-checklist.md) before activation.
Production uses relative `/api/v1` through Nginx; see the
[deployment guide](../deploy/README.md). Media uploads, outbound job alerts and
staff invitations remain separate work, as described in the
[product delivery plan](../docs/marketplace-improvements.md).
