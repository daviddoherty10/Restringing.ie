# Restringing.ie

Restringing.ie is a marketplace for racket restringing in Ireland. Players can find a local stringer, choose a setup and follow their racket through to collection. Stringing businesses manage their services, stock and incoming orders in the same app.

The application has a Go/chi and PostgreSQL marketplace API, with a React/TypeScript frontend. It is under development. Customer bookings use cash paid directly to the stringer; The Stripe integration handles business subscriptions when configured. The public main-domain site is a coming-soon page, and the application has a separate preview deployment.

## Try it locally

The [local preview guide](docs/preview-data.md) is the quickest way to explore the app. It runs an isolated database with sample businesses, orders and short-lived customer, owner, staff and administrator sessions. You can try booking and management without Twilio or Stripe credentials.

You will need Docker Compose, Node 24/npm, Go matching the backend module and goose. Preview businesses and racket models are fictional. Product identities have source references, but prices, ratings and racket constraints are test data and must not be used as real stringing advice.

For a short walkthrough:

1. Browse stringers and open a profile.
2. Use the guided finder or configure a full-bed or hybrid setup, then request a server-priced quote.
3. Use a customer preview session to place an order and open its tracking page.
4. Switch to the stringer session to accept the order, update progress and inspect stock.
5. Use the administrator session to review catalogue records and business applications.

The preview guide explains startup, role switching and how to reset only the preview database.

## Engineering overview

The backend validates racket and service constraints, calculates prices in integer cents and issues expiring quotes. Order creation rechecks the quote and reserves tracked materials in one PostgreSQL transaction. Idempotency keys let clients retry an uncertain request without creating another order.

Authentication uses Twilio Verify and opaque sessions whose tokens are hashed in the database. Private requests check current roles and business membership. Customers, owners, staff and platform administrators have different permissions, including within the same business.

The React frontend uses a [typed frontend API client](frontend/src/lib/marketplace-api.ts). The [OpenAPI REST contract](api/openapi.json) and generated TypeScript declarations keep the client and server interfaces in step. Tests cover PostgreSQL transactions and concurrency, provider HTTP contracts, and desktop/mobile browser flows. Browser tests use mocked API responses; they do not establish live SMS or payment delivery.

| Area | Where to look |
| --- | --- |
| API, business rules and storage | [Backend setup, data model, API examples and security boundaries](docs/backend-marketplace.md) |
| Booking and inventory decisions | [Hybrid pricing and optional stock tracking](docs/hybrid-setups.md) |
| Business workflow and permissions | [Use the stringer dashboard and preview roles](docs/stringer-dashboard.md) |
| Subscription entitlement and usage | [Business plans, monthly allowance and cash payments](docs/subscriptions-and-cash.md) |
| Order communication | [Tracking, collection requests and conversations](docs/order-tracking.md) |
| Optional booking services | [Express turnaround](docs/express-turnaround.md) and [fitted grip add-ons](docs/grip-addons.md) |
| Shared product data | [Admin catalogue, variants and verified racket models](docs/admin-catalogue.md) |
| Local sample data and role sessions | [Load fictional sample data and preview accounts locally](docs/preview-data.md) |

## Current limits

The free plan counts five accepted bookings per business per Irish calendar month. Paid access depends on a verified current subscription. Order fulfilment and cash receipt records are separate states.

SMS order alerts, staff invitations/removal and profile/banner uploads are implemented; see [setup and limits](docs/notifications-team-and-images.md). SMS sending stays disabled until configured, and users must opt in. Email/push alerts, catalogue image uploads, ownership transfer and online customer payments remain future work. Legal pages are drafts, and booking/trader enforcement flags default to disabled pending review. Production also needs curated catalogue/racket data, configured SMS credentials and operational checks. See [phone sign-in setup](docs/phone-auth.md) and [compliance integration, validation and outstanding launch decisions](docs/compliance-integration.md).

## Documentation

The [documentation index](docs/README.md) groups setup guides, feature references and dated review records. Start with the [frontend guide](frontend/README.md) for UI development or the [deployment guide](deploy/README.md) for Docker and Coolify. The [requested marketplace improvements and delivery plan](docs/marketplace-improvements.md) separates implemented features from proposed work.
