# Restringing.ie

Go/chi and PostgreSQL marketplace API, with a React/TypeScript frontend.

- [Backend setup, data model, API examples and security boundaries](docs/backend-marketplace.md)
- [Business plans, monthly allowance and cash payments](docs/subscriptions-and-cash.md)
- [Use the stringer dashboard and preview roles](docs/stringer-dashboard.md)
- [Load fictional sample data and preview accounts locally](docs/preview-data.md)
- [OpenAPI REST contract](api/openapi.json)
- [Typed frontend API client](frontend/src/lib/marketplace-api.ts)

The frontend uses the marketplace API. Local fictional data is available through
the isolated preview workflow. Production requires curated catalogue/racket data
and configured SMS credentials; see [phone sign-in setup](docs/phone-auth.md).
Customer bookings use cash paid directly to the stringer. Business subscriptions
include a free five-booking monthly allowance and configurable paid billing. See
[subscription setup](docs/subscriptions-and-cash.md); online customer checkout remains a separate integration.
