# Restringing.ie

Restringing.ie is a marketplace for racket restringing in Ireland. Players find a local stringer, choose a setup and track their booking through to collection. Stringers manage services, inventory and orders from a dashboard.

The public site is a coming-soon page. The marketplace runs separately as a preview. Stringers can offer onsite cash or card, and can enable online checkout through their own Stripe account once configured. Business subscriptions are handled separately.

## Try the marketplace

Follow the [local demo guide](docs/demo/README.md) to start the app with sample businesses, bookings and accounts. You can try the player, stringer and administrator flows without SMS or payment credentials.

## How it works

A React and TypeScript frontend calls a Go API backed by PostgreSQL. The API validates setups and prices in integer cents, then saves the booking and reserves stock in one transaction. The frontend uses types generated from the [OpenAPI contract](api/openapi.json).

## Documentation

Browse the [documentation index](docs/README.md), or go straight to the section you need.

| Section | Start here for |
| --- | --- |
| [Local demo](docs/demo/README.md) | Setup, sample data and a walkthrough of each role |
| [Business and product](docs/business/README.md) | Proposal, financial model and open product decisions |
| [Hosting and operations](docs/hosting/README.md) | Docker, Coolify, monitoring and the launch checklist |
| [Engineering](docs/engineering/README.md) | Development setup, architecture, APIs and feature behaviour |
| [Archive](docs/archive/README.md) | Previous reviews and design decisions |
