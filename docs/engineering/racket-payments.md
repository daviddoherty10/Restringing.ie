# Racket payments

Business owners choose payment options in **Stringer dashboard → Settings → Payments**:

- Cash onsite.
- Card onsite, using the business's own terminal.
- Online payment through the business's Stripe account, once setup is ready.

At least one option must remain enabled. Staff can view the setup and record
onsite receipts, but only owners and administrators can change payment settings
or open Stripe onboarding. Existing businesses default to onsite cash.

## Booking and payment records

Customers see only the available options. Their selection is part of the server
quote, saved draft and order. Changing it requires a new quote. Onsite bookings
save which tenders were offered, so later settings changes do not rewrite history.
A stringer records cash or card received only after accepting the booking. This
records payment separately from fulfilment and does not operate a card terminal.
Older cash bookings and their receipt endpoint retain their existing meaning.

For online bookings, customers continue from their order to Stripe-hosted Checkout.
The server uses the saved EUR order total and connected account. It takes no
transaction fee. Business subscriptions remain a separate Stripe integration.

Payment becomes paid only after a signed webhook and a fresh Stripe lookup verify
the account, session, order, amount, currency and environment. The browser return
page cannot mark it paid. Checkout retries reuse a persisted attempt and provider
idempotency key. Expired sessions can be replaced; an ambiguous attempt that
outlives its expiry needs support reconciliation before another payment is opened.

An unpaid online booking cannot start stringing. Cancellation expires an open
checkout before releasing stock; a completed asynchronous payment must settle
before cancellation. Cancellation does not itself refund a payment.

## Refunds and disputes

The stringer uses their full Stripe Dashboard for refunds and formal payment
disputes. The app records verified refund and dispute updates. An unresolved
dispute prevents starting work. Restringing.ie can help investigate service issues
using its existing case workflow, booking history and conversations; it does not
automatically decide card disputes or move funds between businesses.

Manual cash financial facts remain separate from provider reconciliation. Onsite
card and online payments are not classified as cash receipts.

## Configure a sandbox first

The integration creates Accounts v2 merchant accounts with full Stripe Dashboard
access, Stripe-managed fee collection and negative-balance liability, and direct
charges to the stringer. No platform application fee is requested. Hosted Stripe
onboarding collects the business's verification and payout information.

Enable Connect in the platform's Stripe sandbox and set backend-only variables:

```dotenv
RACKET_PAYMENTS_PROVIDER=stripe
RACKET_STRIPE_KEY=
RACKET_STRIPE_WEBHOOK_SECRET=
```

Use a restricted sandbox key with the required connected-account, account-link,
Checkout, charge and dispute permissions. Keep the key in the deployment secret
store. These variables are independent of subscription billing credentials.
The local preview script refuses live racket-payment keys.

Configure a connected-account webhook destination at
`POST /api/v1/payments/stripe/webhook` for:

- `checkout.session.completed`
- `checkout.session.async_payment_succeeded`
- `checkout.session.async_payment_failed`
- `charge.refunded`
- `charge.dispute.created` and `charge.dispute.closed`
- Account updates, including Accounts v2 merchant capability updates.

The handler supports v1 `account.updated` and v2 account events with a
`related_object` account ID. It fetches current capability state rather than
relying on event delivery order. Checkout always checks current readiness again.
Register and verify the applicable Stripe event destination/signing secret for the
chosen event format; use sandbox delivery tests before enabling production.

Run migration `00021_racket_payments.sql` before the API starts. A rollback refuses
to relabel onsite orders, discard checkout history or lose a linked Stripe account. Roll forward if
payment history exists. Then an owner opens **Set up or update Stripe**, completes
the hosted flow, refreshes status and enables online payments. Returning from
onboarding alone never enables payments.

Before live use, verify actual sandbox onboarding, Checkout, delayed payments,
refunds, disputes and webhook delivery, and review the business's published
payment/refund wording through the existing legal-release workflow. Automated
provider-contract tests do not establish that a live Stripe account is configured.

Official references: [Accounts v2 merchant setup](https://docs.stripe.com/connect/saas/tasks/create),
[hosted onboarding](https://docs.stripe.com/connect/saas/tasks/onboard),
[direct charges](https://docs.stripe.com/connect/direct-charges),
[Checkout fulfilment](https://docs.stripe.com/checkout/fulfillment).
