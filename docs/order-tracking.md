# Order tracking, conversations and in-app notifications

Customers open `/orders` and follow an order at `/orders/{orderID}`. The tracking screen uses the existing fulfilment stages: placed → accepted → received → in_progress → ready → completed. The last stage is labelled **Collected** in the customer UI. Cancellation keeps the history and conversation available without suggesting unfinished stages were completed. Existing price snapshots, quota accounting, stock reservations and payment rules are unchanged.

## Collection arrangements

The screen shows real status events and their recorded timestamps, a current ready estimate, collection instructions, an issue notice, collection-update history, the original racket setup and price/payment information. No artificial percentage, countdown or delivery location is generated. A passed estimate is explicitly labelled as needing an update; it never automatically marks a racket ready.

A customer can request a **date for collecting the finished racket** after placing the booking. This is not a door-to-door pickup service or a guaranteed appointment. The date must be today through one year ahead in `Europe/Dublin`. A request remains separate from the stringer's estimate, including when an existing estimate is already present.

After accepting the order, current business members or platform admins can confirm/revise/clear the ready estimate, provide collection instructions, flag or resolve an issue, and explain the change in a required note. Confirming an estimate does not change fulfilment, price or payment. The estimate is an RFC3339 instant; a newly supplied estimate must be in the future and within one year. An unchanged historical estimate may be retained when editing instructions or resolving an issue. The form accepts the staff device's local time and displays its timezone; customer-facing times are rendered in Ireland time, including daylight saving.

Collection changes are append-only snapshots with an integer version. Requests include `expected_version`; a stale edit returns `409 tracking_changed`. Drafts are retained when polling sees a newer version; load and review the current details explicitly before saving again. Closed orders reject new collection changes; successful past requests can still be replayed.

## Conversations and notifications

One plain-text conversation belongs to each order. Its customer, current business owner/staff and existing platform-admin support role may read/write it. Other users receive 404. Sender roles are derived on the server; private login phone numbers and staff names are not exposed through this API. Messages allow 1–2000 characters and reject invalid/control text. Sending is limited to 20 messages per verified user per minute, serialized across businesses. Collection changes are limited to 20 per actor/order per minute. These are application limits, not SMS-provider limits.

Messages are returned newest first in bounded pages; the UI displays each page in reading order. An opaque message UUID identifies the last returned message to mark read. The database cursor moves only forward and cannot mark subsequently committed, unseen messages read. Cursors belong to individual users, so one staff member does not clear another member's unread messages. Message bodies are rendered as text, including links/HTML-like content; no attachments or rich HTML are accepted.

Persistent in-app notifications are created for:

- New booking: current active, phone-verified business members, excluding the actor.
- Status changes, including ready/cancelled/completed: other order participants.
- Message: other order participants.
- Collection request, revised estimate/instructions or issue flag/resolution: other order participants.

Recipients are the current active, verified customer and business members except the actor. Existing platform admins may support an order but are not automatically subscribed to every order. Multiple staff can receive the same event; each owns a separate read marker. Unique recipient/event keys prevent duplicate inbox entries. Notification creation is in the same database transaction as the originating change: partial booking/message/update commits are not permitted. A former staff member cannot read the order or its old notifications after losing membership; notification listing and read actions recheck current order access.

The shared account menu links to `/notifications` and shows the unread count. Inbox entries link directly to the authorized tracking screen; a stringer can edit collection details there and return to the portal to advance fulfilment. The existing dashboard also links each expanded incoming order to its tracking screen and conversation. Notifications remain unread until explicitly marked read. Reading a conversation updates its message cursor separately.

**No outbound job SMS, email, push notification, browser notification permission prompt or provider worker is enabled.** Twilio Verify remains the sign-in-only integration. These in-app events are a foundation for later outbound delivery, not a claim that an SMS was sent. Outbound delivery will need a separate transactional outbox, bounded retries, provider status reconciliation, preferences and configuration before activation. An already closed browser does not receive live alerts in this version.

## REST contract

All routes use the existing hashed session cookie/bearer boundary. Cookie-authenticated writes require the exact trusted Origin. JSON bodies retain the existing 16 KiB cap, reject unknown fields and cannot set prices, sender identity or notification recipients. The authoritative generated contract is `api/openapi.json`; TypeScript types are regenerated from it.

| Method | Endpoint | Contract |
| --- | --- | --- |
| GET | `/api/v1/orders/{orderID}/tracking` | Collection snapshot, current business name/area, effective customer/manager flags and timezone |
| GET | `/api/v1/orders/{orderID}/tracking-updates` | Paginated snapshot history, newest version first |
| POST | `/api/v1/orders/{orderID}/collection-request` | Customer: `{date, expected_version}` |
| POST | `/api/v1/orders/{orderID}/tracking-updates` | Manager: `{confirmed_ready_at, collection_instructions, issue_open, note, expected_version}` |
| GET | `/api/v1/orders/{orderID}/messages` | Paginated messages, newest first; per-user `unread_count` |
| POST | `/api/v1/orders/{orderID}/messages` | Participant: `{body}` |
| POST | `/api/v1/orders/{orderID}/messages/read` | Participant: `{last_message_id}` |
| GET | `/api/v1/notifications` | Own accessible notifications and total `unread_count` |
| POST | `/api/v1/notifications/{notificationID}/read` | Own accessible notification: `{}` |

The three collection/message creation routes require `Idempotency-Key` (same 16–100-character pattern as order creation). Reusing a key for another payload returns 409. Authorization is rechecked before replay; exact successful retries do not create new messages, versions or notifications. Message creation/replay returns 201. Collection creation/replay returns 200. Read actions are idempotent.

All list routes use the existing `page` and `page_size` bounds (1–10000 and 1–50). Tracking, messages and the inbox poll every 15 seconds while visible; the account-menu count polls every 30 seconds. Polls do not overlap within a feed and resume on returning to the page. Failed refreshes are visible, preserve readable cached data except on access errors, and offer retry. This is bounded polling, not WebSocket/SSE streaming. Large installations should revisit aggregate polling load and indexed cursor pagination.

The message and collection forms retain the payload and key for an ambiguous retry while the screen remains mounted and lock editing until it is resolved. Drafts/pending communications are not persisted across reloads. After leaving/reloading during an uncertain write, inspect the refreshed conversation/history before submitting again. Existing booking idempotency/reload recovery is unchanged.

## Database and implementation order

Migration `00007_order_tracking_and_conversations.sql` adds append-only `order_tracking_updates`, `order_messages` with server positions, per-user `order_message_reads`, and per-user `notifications`. There is no historical notification backfill or invented estimate for existing orders. Goose down drops only these new feature tables; reverting it intentionally loses their conversation/tracking/inbox data, so use down migrations only in disposable environments or with a deliberate backup plan.

Implementation follows schema/models → authorization and transactional writes → private REST/OpenAPI/generated types → shared React tracking/conversation/inbox → preview fixtures and regression tests. Tracking/message writes take the same business-then-order locks as existing fulfilment/stock mutations. Message rate limiting also takes a per-sender advisory lock. No external provider call occurs inside these transactions.

Apply migrations using the normal goose workflow before running the new API. Existing preview stacks can run `bash scripts/preview.sh seed` to migrate without resetting their edits. Fresh preview seeds include an issue/delay and two messages at `/orders/90000000-0000-4000-8000-000000040004`, a ready example ending `040005`, and customer/stringer inbox entries. Existing seeded databases keep their data; use the UI to create examples if they predate these fixture additions. Use the existing customer/stringer/staff preview sessions; no OTP or real notification is sent.

[Express turnaround](express-turnaround.md) is now an optional, server-quoted service with its original booked terms shown on the tracker. It does not fabricate a collection estimate. Fitted [grip add-ons](grip-addons.md) are now shown in booking details. True hybrid selection and media uploads remain separate roadmap work.
