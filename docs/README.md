# Documentation

Start with the [project overview](../README.md) for the product, architecture and current limits. To explore the app, follow the [local preview guide](preview-data.md). It includes sample data and role sessions without requiring SMS or billing credentials.

## Development and deployment

| Guide | What it covers |
| --- | --- |
| [Backend](backend-marketplace.md) | Local setup, data model, API contract, validation and order transactions |
| [Frontend](../frontend/README.md) | UI development, shared API client, generated types and browser tests |
| [Preview data](preview-data.md) | Isolated fixtures, role sessions, repeatable seeding and reset |
| [Docker and Coolify](../deploy/README.md) | Preview/landing routing, private services and persistent storage |
| [Prebuilt images](../deploy/PREBUILT.md) | GitHub-built AMD64/ARM64 releases and controlled Coolify updates |
| [Observability](observability.md) | Readiness, sanitized logs, PostHog errors and monitoring setup still required |

## Product and business rules

| Guide | What it covers |
| --- | --- |
| [Stringer dashboard](stringer-dashboard.md) | Orders, stock, services, profile management and role permissions |
| [Catalogue administration](admin-catalogue.md) | Products, variants and sourced racket verification |
| [Guided finder](guided-string-finder.md) | Explained starting setups, constraints and preview product sources |
| [Hybrid setups](hybrid-setups.md) | Mains/crosses selection, integer pricing and optional stock tracking |
| [Grips](grip-addons.md) | Fitted add-ons and atomic multi-item reservations |
| [Express turnaround](express-turnaround.md) | Optional fees and saved turnaround terms |
| [Order tracking](order-tracking.md) | Collection requests, conversations and in-app notifications |
| [Phone sign-in](phone-auth.md) | Twilio Verify, sessions, rate limits and proxy configuration |
| [Racket payments](racket-payments.md) | Owner payment choices, onsite receipts and direct Stripe Checkout |
| [Business plans and cash](subscriptions-and-cash.md) | Accepted-booking allowance, Stripe subscriptions and manual receipts |
| [SMS, staff and images](notifications-team-and-images.md) | Outbox delivery, team access and profile/banner uploads |
| [Product backlog](product-backlog.md) | Open ideas for payment choices, modular plans and onsite tournaments |
| [Delivery plan](marketplace-improvements.md) | Implemented features and proposed extensions |

## Reviews and launch requirements

[Compliance workflows](compliance-implementation.md) explains the implemented review screens and enforcement flags. The [release checklist](compliance-launch-checklist.md) tracks decisions still required before public bookings.

The following are dated evidence records. Their test counts, PR states and scan results describe the recorded review; they do not establish the state of a later deployment.

- [Compliance integration review](compliance-integration.md)
- [Security and operations review](security-and-operations.md)
- [Repository cleanup record](repository-maintenance.md)

Legal pages remain drafts. Review requirements, provider delivery checks, backup restoration and production configuration need separate operational evidence.
