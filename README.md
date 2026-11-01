# Restringing.ie

Go/chi and PostgreSQL marketplace API, with a React/TypeScript frontend.

- [Requested marketplace improvements and delivery plan](docs/marketplace-improvements.md)
- [Admin catalogue, variants and verified racket models](docs/admin-catalogue.md)
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

Order tracking, collection arrangements, conversations and in-app alerts: [implementation and API guide](docs/order-tracking.md).

Optional express turnaround: [configuration, pricing and preview guide](docs/express-turnaround.md).

Fitted grip add-ons and multi-item stock: [implementation and API guide](docs/grip-addons.md).
