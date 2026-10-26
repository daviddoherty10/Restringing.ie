# Restringing.ie

Go/chi and PostgreSQL marketplace API, with a React/TypeScript frontend.

- [Backend setup, data model, API examples and security boundaries](docs/backend-marketplace.md)
- [Load fictional sample data and preview accounts locally](docs/preview-data.md)
- [OpenAPI REST contract](api/openapi.json)
- [Typed frontend API client](frontend/src/lib/marketplace-api.ts)

The current frontend is an illustrative preview. Its fictional listings, string
catalogue and prices must be replaced with live API data before accepting orders.
The backend requires curated catalogue/racket data and a real phone OTP session
issuer. Booking submission does not collect payment; connect payment checkout and
verified provider webhooks before presenting it as an online purchase.
