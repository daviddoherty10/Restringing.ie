# Restringing.ie

Go/chi and PostgreSQL marketplace API, with a React/TypeScript frontend.

- [Backend setup, data model, API examples and security boundaries](docs/backend-marketplace.md)
- [Use the stringer dashboard and preview roles](docs/stringer-dashboard.md)
- [Load fictional sample data and preview accounts locally](docs/preview-data.md)
- [OpenAPI REST contract](api/openapi.json)
- [Typed frontend API client](frontend/src/lib/marketplace-api.ts)

The frontend uses the marketplace API. Local fictional data is available through
the isolated preview workflow. Production requires curated catalogue/racket data
and configured SMS credentials; see [phone sign-in setup](docs/phone-auth.md).
Booking submission does not collect payment; payment checkout and verified
provider webhooks are the next integration before online purchases.
