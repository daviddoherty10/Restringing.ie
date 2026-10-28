# Express turnaround

A stringer can offer an optional express service for each racket sport. Owners and platform administrators configure it in **Stringer portal → Services**; staff retain read-only service settings and can fulfil incoming orders as before.

The offer has three fields on the existing service API:

| Field | Meaning |
| --- | --- |
| `express` | Whether express is currently offered for this business/sport |
| `express_fee_cents` | Extra service fee in integer EUR cents, 0–1000000 |
| `express_turnaround` | Single-line description, at most 160 characters; required when enabled |

A useful description states both the turnaround and when it starts, e.g. “Within 24 hours of receiving your racket, after booking acceptance.” A zero fee is permitted and does not disable the offer. Disabling express preserves the configured fee/description for future re-enabling. The owner can still edit these retained values while disabled.

## Booking and pricing

The public service/profile and booking-configuration responses advertise the option. Standard turnaround is the default. The customer explicitly chooses express before requesting a quote. `Booking.express` is an optional boolean; omitted/false retains standard booking behaviour. Client-supplied prices or turnaround snapshots are rejected as undocumented request fields.

For express, the backend verifies that this business's service for the verified racket sport offers it, then adds an `express` price line using its stored `express_fee_cents`. Existing racket, stock, knot, tension, pre-stretch, own-string, approval and monthly plan checks continue to apply. Express works with customer-supplied strings when both options are offered. It remains one restringing for the monthly allowance and uses the existing cash/payment rules.

The order configuration saves the selected boolean and an immutable `express_turnaround_snapshot`. Its existing breakdown/total hold the agreed fee. Incoming orders, expanded details, customer booking details and the tracker display the booked express terms. Subsequent service changes do not rewrite existing order prices or descriptions. Notifications remain the existing in-app order/status messages; no SMS or other provider integration is activated.

All service terms participate in the quote fingerprint, including the availability, fee and description. Changing any service terms before checkout requires a new quote. Disabling a selected express offer rejects checkout. After a successful order, retrying the same order idempotency key returns the original order even if express has since been disabled or repriced. Existing stock reservation and transaction boundaries are unchanged.

The React booking draft retains the selected option across sign-in/reload and requires a fresh quote. Legacy drafts without the field restore as standard. If a saved express choice is no longer available, the UI keeps it visible as unavailable and requires selecting standard/reviewing the setup before requesting another quote. Changing the turnaround choice clears a previously reviewed quote; pending uncertain order submissions keep their original retry identity and block setup changes.

## Collection estimates

Express is a purchased turnaround description, not an automatically allocated appointment. It does not fabricate opening hours, reserved capacity or a ready timestamp. The stringer confirms a ready estimate after acceptance through the existing collection-tracking workflow. A customer's early collection-date request never implicitly selects or charges for express. Timing revisions/issues continue to use tracking updates and order conversations; they do not silently revise the original booked express description or fee. Refunds/changes to an accepted price remain a separate future workflow.

## Migration and preview

Migration `00008_express_turnaround.sql` adds service fields with express disabled, a zero fee and an empty description by default, plus database price/length/nonempty-when-enabled constraints. Existing orders and their JSON snapshots are not backfilled or repriced. Previously issued quotes may be invalidated by the extended service fingerprint and need refreshing. Apply migrations before starting the updated API; deploy the matching frontend/API together. The service PUT remains a full replacement, so service editors must include the new fields.

Fresh preview seeds offer a fictional €7 express option on Baseline's tennis service and include one express in-progress order ending `040004`. Existing seeded databases are preserved; enable express manually through the preview stringer session if the database predates these examples. Rerunning `bash scripts/preview.sh seed` migrates the isolated preview database without resetting it.

No new endpoints are required. Existing service GET/PUT, booking-configuration, quote, order creation/read/history and dashboard endpoints carry the extended generated contract. `api/openapi.json` and `frontend/src/lib/marketplace-contract.d.ts` are regenerated from the Go models.

Validation covers explicit standard/express pricing, zero-fee express, supplied strings, forbidden service edits, strict quote inputs, business scope, changed terms/fees/availability before checkout, idempotent order replay after disabling express, immutable snapshots, plan counting, and the absence of an invented collection estimate. Browser checks cover owner/staff controls, quote invalidation, sign-in/reload preservation, unavailable choices, the public profile at 320px, and the tracker’s original express terms.
