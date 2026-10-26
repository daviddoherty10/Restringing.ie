# Business plans and cash bookings

The marketplace charges the stringing business for a subscription. Customers pay
cash directly to the stringer. No percentage fee, automatic upgrade or customer
card charge is introduced. Paid-plan pricing is configured through a Stripe Price;
there is no invented production price in the application or preview seed.

## Rules

| Plan | Monthly allowance | Tools |
| --- | --- | --- |
| Free | Five accepted single-racket bookings per business | Catalogue, inventory, prices, services, orders and staff access |
| Paid | Unlimited accepted bookings while verified paid access is current | The same business tools |

Usage counts when a stringer first changes an order from `placed` to `accepted`.
Quotes and placed orders do not consume the allowance. Every accepted order counts,
including cash orders and orders accepted while on the paid plan. An accepted order
continues counting after cancellation. Cancellations before acceptance do not count.
This makes cancellation/recreation an ineffective way to reset the allowance.

Months are calendar months in `Europe/Dublin`, including the daylight-saving
boundary. They are independent of the subscription's billing anniversary. Usage is
shared across the business's owners/staff, not per user. A unique order reference
and the existing business transaction lock prevent double counting or two staff
members taking the fifth slot simultaneously. The database clock is read after the
lock is acquired. There is no monthly counter-reset job: usage is selected using
explicit month boundaries.

At capacity, new quotes, orders and acceptances are refused by the backend.
Existing accepted orders can continue through receipt, stringing and completion;
placed orders can be cancelled or accepted after an upgrade/reset. Customer order
history, profile, inventory, service and billing management remain accessible.
Public approved listings remain visible, with `booking_available: false`. This is
separate from the owner's `accepting_orders` setting. The public API does not
expose a business's subscription identifiers or usage counts.

When paid access ends, the free allowance applies immediately using the same
month's acceptance history. Downgrading does not erase usage or cancel orders.
Work arranged outside the platform cannot be measured by this system.

## Migration and API

Run the normal goose `up` workflow. Migration `00006_business_billing_and_cash.sql`
adds `business_subscriptions`, `booking_usage`, `billing_checkout_attempts`,
`billing_events`, `order_payment_events`, and `orders.payment_method`.
Existing first-acceptance timestamps are backfilled from status history, including
orders later cancelled. For existing accepted/fulfilled orders missing an acceptance
event, the order creation timestamp is the fallback. Existing bookings become cash
bookings; the previous release did not offer online payment collection. Every
business starts free until a verified subscription is recorded.

Endpoints under `/api/v1`:

| Endpoint | Access and behaviour |
| --- | --- |
| `GET /stringers/{id}/billing` | Members/admin: effective plan, month boundaries, accepted count, remaining free allowance, billing availability, price and subscription summary |
| `POST /stringers/{id}/billing/checkout` | Owner/admin only; `{}`; returns an existing open checkout or creates one for the configured recurring price |
| `POST /stringers/{id}/billing/portal` | Owner/admin only; `{}`; short-lived hosted billing-management URL |
| `POST /billing/stripe/webhook` | No session; raw-body Stripe signature and matching test/live mode required |
| `POST /orders/{id}/cash-receipt` | Associated members/admin only; `{}`; idempotently record a cash receipt |

The generated OpenAPI and TypeScript client describe the fields. Billing responses
exclude provider customer/subscription IDs. `manage_billing` is included in private
business permissions; staff can read usage but cannot start or manage subscriptions.
Cookie writes retain exact-Origin protection. No endpoint accepts a plan, price,
paid-until timestamp or paid status from the browser.

`POST /orders` accepts `payment_method: "cash"` (also the default for existing
clients). Other choices are rejected. A new order is always unpaid. Recording cash
requires an accepted, non-cancelled cash order. Confirmation changes payment status
only; it does not advance fulfilment or affect usage. The actor, timestamp and
receipt reason are audited. Repeated/concurrent confirmations create one receipt.
Customers cannot confirm their own payments. The UI explicitly asks the stringer to
confirm the full saved order amount was received.

Customers see cash due or cash received, labelled as confirmed by the stringer.
This is a manual payment record, not a payment-provider receipt or tax invoice.
Cancellation does not invent a cash refund; the dashboard reminds the stringer to
arrange any refund directly. Partial cash payments, cash receipt reversals, refunds,
and online customer payments are not included in this release. An unpaid cash
booking is not an abandoned online checkout and is not automatically cancelled.

## Configure subscription billing

Leave `BILLING_PROVIDER` empty to use the complete free/cash workflow without any
billing account or credentials. To enable owner-initiated hosted subscription checkout:

1. Create a Stripe sandbox/test account and a product for the Restringing.ie paid
   business plan. Create a **fixed monthly EUR Price**, quantity one, licensed usage,
   with **inclusive** tax behaviour and a positive amount. Choose the actual price
   before creating it; no live product or price has been created by this work.
2. Set backend-only variables (never `VITE_*` or committed credentials):

   ```dotenv
   BILLING_PROVIDER=stripe
   STRIPE_SECRET_KEY=sk_test_REPLACE
   STRIPE_MONTHLY_PRICE_ID=price_REPLACE
   STRIPE_WEBHOOK_SECRET=whsec_REPLACE
   ```

3. Configure the Stripe customer portal. Enable payment-method/invoice management
   and cancellation at period end. Disable arbitrary plan/quantity switching and
   automatic multi-currency pricing for this single-plan integration. Published
   terms/privacy links and the final commercial/tax configuration must be supplied
   in the actual billing account before launch.
4. Configure a **snapshot** webhook destination using API version
   `2025-03-31.basil` (also pinned on server API calls) at
   `https://api.restringing.ie/api/v1/billing/stripe/webhook`. Subscribe to:
   `checkout.session.completed`, `customer.subscription.created`,
   `customer.subscription.updated`, `customer.subscription.deleted`,
   `invoice.paid`, and `invoice.payment_failed`.
5. For local sandbox work use the Stripe CLI listener:

   ```bash
   stripe listen \
     --events checkout.session.completed,customer.subscription.created,customer.subscription.updated,customer.subscription.deleted,invoice.paid,invoice.payment_failed \
     --forward-to http://127.0.0.1:8082/api/v1/billing/stripe/webhook
   ```

   Use the listener's signing secret in the API process. Use matching sandbox API
   credentials and Price. The preview launcher rejects enabled live billing keys.
   Live destination secrets differ from CLI/test secrets.

The server fetches and validates the Price before offering checkout: active, fixed
positive EUR/month, licensed quantity, inclusive tax, matching test/live mode, no
quantity transformation or custom amount. Invalid prices/outages disable upgrades;
free tools and existing orders remain available. The portal requires an existing
provider customer. Checkout is restricted to approved businesses.

## Payment verification and retries

A browser return URL never changes access. The webhook verifies the raw-body HMAC
signature with a five-minute timestamp tolerance, matches the configured mode and
stored customer, then fetches the **current subscription** under the business lock.
It checks the server-created checkout metadata, allowed recorded Price, quantity
one, EUR/month, active state, no paused collection, and a paid latest invoice.
Paid access is bounded by the subscription item's `current_period_end`.
Trials, unpaid/incomplete/past-due/paused subscriptions do not grant unlimited use.
A scheduled period-end cancellation retains access through its confirmed paid term.
The paid-until check is performed on reads/writes, so stale webhooks cannot keep an
expired entitlement alive indefinitely. Replay events are deduplicated by event ID;
out-of-order events fetch current provider state instead of trusting event order.

Creating checkout persists its attempt and expiry **before** external calls.
Retries use that same provider idempotency key and reuse a stored open checkout;
customer creation is separately persisted before checkout. Provider calls have
bounded timeouts, fixed HTTPS destinations and no redirect following. Sensitive
provider responses are not logged or returned as API errors.

Known expired sessions can be replaced only after Stripe confirms they are expired.
If an unknown checkout result remains unresolved past its retry window, the API
fails closed with `billing_reconciliation_required` rather than risking another
subscription. An attempt with under 31 minutes left cannot create a new session;
resume a known URL or reconcile/expire it first. For a completed checkout awaiting
confirmation, replay the relevant provider event after fixing webhook delivery.
Operators must reconcile uncertain attempts against Stripe before resolving them;
there is deliberately no public force-reset or grant-paid endpoint.

Changing `STRIPE_MONTHLY_PRICE_ID` affects new checkout attempts. Existing verified
subscriptions retain their recorded Price identity. Review incomplete attempts and
portal settings when introducing a replacement price. This release has no renewal
poller: monitor failed webhook deliveries and replay them. Failed/missing renewal
confirmation conservatively returns the business to free when access expires.

## Preview and validation

```bash
bash scripts/preview.sh seed
AUTH_SMS_PROVIDER= BILLING_PROVIDER= bash scripts/preview.sh api
# Separate terminal:
bash scripts/preview.sh web
# Another terminal:
bash scripts/preview.sh session stringer
```

Open `http://127.0.0.1:5174/manage`, set the issued preview session, and inspect
**Plan & billing**. Fresh fixtures include a prior-month completed order to leave
room to demonstrate the fifth acceptance. Existing fixtures keep their edits and
historical usage; rerunning seed never resets an allowance. You can use another
fixture business or the documented isolated-volume reset for a fresh example.
No paid subscriptions or cash receipts are fabricated in the preview seed.

PostgreSQL tests cover quota races across staff, business isolation, Irish month/DST
boundaries, migrations/backfill, cancellation counting, paid expiry, webhook replay
and ordering, ownership, provider identity/price checks, checkout retries and cash
receipt audit/idempotency. Provider wire-contract tests use a mock transport.
Desktop/mobile browser tests cover usage, billing permissions/return/error states,
public availability and cash confirmation separate from fulfilment. Tests do not
send SMS, create real subscriptions or charge cards.

Provider references: [Checkout](https://docs.stripe.com/api/checkout/sessions/create),
[webhook verification and delivery](https://docs.stripe.com/webhooks),
[idempotency](https://docs.stripe.com/api/idempotent_requests),
[item-level subscription periods](https://docs.stripe.com/changelog/basil/2025-03-31/deprecate-subscription-current-period-start-and-end),
and [subscription lifecycle](https://docs.stripe.com/billing/subscriptions/overview).
