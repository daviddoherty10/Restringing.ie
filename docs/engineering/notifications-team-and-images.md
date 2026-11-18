# SMS alerts, staff access and business images

Migration 20 adds an SMS outbox, staff invitations and business images. Deploy the matching API and frontend after applying migrations. All three features work with the existing session and business permission checks.

## SMS order alerts

Users opt in under Notifications. Alerts default to off. A booking, status change, message or collection update creates its in-app notification and SMS outbox entry in the same database transaction. Only recipients who were opted in at the time get an outbox entry; enabling SMS does not send historical alerts.

The API worker sends a generic order-update message with a link that requires sign-in. It does not include names, conversation text, prices or instructions. It checks the user's current account, opt-in and order access again before dispatch. Removed staff cannot receive pending messages for their former business. Notifications older than 24 hours are cancelled before sending.

Sending is disabled until `JOB_SMS_PROVIDER=twilio` and all these backend secrets are configured:

```dotenv
JOB_SMS_PROVIDER=twilio
JOB_SMS_ACCOUNT_SID=AC_REPLACE
JOB_SMS_MESSAGING_SERVICE_SID=MG_REPLACE
JOB_SMS_API_KEY_SID=SK_REPLACE
JOB_SMS_API_KEY_SECRET=REPLACE
JOB_SMS_AUTH_TOKEN=REPLACE
```

Use a messaging-capable sender in a Twilio Messaging Service. Verify sender registration, destination permissions and trial restrictions for Ireland/UK. These settings are separate from Twilio Verify sign-in credentials. The auth token validates callbacks; API calls use the API key. Keep all secrets out of Vite variables, source control and logs. `FRONTEND_ORIGIN` must be the exact public HTTPS app origin.

Configure the Messaging Service's incoming webhook to:

```text
https://YOUR_APP_ORIGIN/api/v1/notifications/sms/inbound
```

Enable Advanced Opt-Out on that service and verify that STOP callbacks include `OptOutType=STOP`. Signed STOP callbacks disable the local preference; provider error 21610 also disables it. START does not automatically opt the user back in locally. Users can turn alerts off in the app at any time. Ordinary incoming messages are acknowledged without a reply; the order conversation remains the supported reply channel.

Every outbound message includes a signed delivery callback URL under `/api/v1/notifications/sms/status?delivery=...`. Signature validation uses the configured public origin, not forwarded Host headers. Callbacks are bounded, account-checked and applied without regressing a delivered state when an older callback arrives.

The worker runs once per second per API instance. Database locking prevents two workers claiming the same entry. Explicit HTTP 429 responses get up to five attempts with backoff and jitter. Timeouts, 5xx, malformed successful responses and crashed sends become `unknown`; they are held for reconciliation instead of automatically sending another potentially billable message. A successful API response means accepted, not delivered. Carrier callbacks establish later states.

Supported destinations are `+353` and `+44`. Starting limits are 50 deliveries per recipient and 1,000 globally over a rolling 24 hours. Excess work is deferred and expires after 24 hours. Configure provider spend limits too. These app limits are deliberately small and do not replace provider controls. Multiple updates may produce multiple texts; there is no digest or quiet-hours scheduler in this version.

For operational checks, inspect aggregate states without exposing phones or message content:

```sql
SELECT state, count(*), min(updated_at)
FROM notification_deliveries
GROUP BY state;
```

Investigate `unknown`, `failed` and old `accepted`/`sent` entries in Twilio before deciding on any resend. Reconcile against the message SID or callback delivery UUID; do not bulk-reset unknown entries to pending. A missing callback leaves delivery unconfirmed. Provider callbacks must reach the app even if the preview uses an access gateway; exempt only these signed webhook routes, not private application routes.

Before enabling real sends, test with a phone you control: opt in, trigger an order event, verify the generic link and delivered status, opt out, and confirm subsequent updates stay in-app. No real messages were sent during implementation. Email and push delivery are not implemented.

## Staff invitations and removal

Owners and platform administrators have a Team tab in the Stringer dashboard. Invite a colleague by international phone number. Invitations last seven days and confer no access until accepted. The colleague signs in with that verified number, opens the dashboard and accepts the pending invitation. The invitation UUID alone grants no authority. Invitations are shown in-app; no invitation SMS is sent.

Only staff membership can be granted. Staff cannot invite colleagues, remove team members or change owners. Owners cannot be removed through these controls. Revoking an invitation prevents acceptance; removing staff immediately removes current membership access. Replaying an already accepted invitation does not restore removed access. A business supports up to 50 members and pending invitations, with 20 invitation writes per hour. Team changes have an audit record.

Ownership transfer and self-service account recovery remain separate work. Membership removal affects access to existing orders and their notifications, so review outstanding work before removing a colleague.

## Profile and banner images

Owners and administrators upload or remove images from the Profile tab. The public business page shows them only while the business is publicly eligible; otherwise it keeps the existing initials layout.

Uploads accept PNG or JPEG up to 1 MB, four megapixels and 4096 pixels per side. The server decodes the actual file, resizes it to at most 1024 pixels per side, and re-encodes it as PNG to remove metadata. Processed files must also fit within 1 MB. SVG, arbitrary remote URLs and undecodable files are rejected.

There are two image slots per business. Replacement overwrites the current slot and removal deletes it. Images are stored in PostgreSQL, so they use the existing database backup and restore process. At most 2 MB of image data is retained per business. Review storage growth before expanding uploads to catalogue products or introducing original-resolution galleries; object storage is the next step if those become necessary. Images must be owned or licensed for publication. There is no content moderation service.

## API and verification

The OpenAPI contract and generated TypeScript describe the new team, preference and image endpoints. Image uploads are the exception to the normal 16 KiB JSON cap: they allow a bounded base64 JSON request of up to 1.5 MB. All authenticated writes retain exact-Origin protection. Public image responses are sanitized PNGs with no-store and nosniff headers.

Tests cover concurrent invitation acceptance, wrong-phone access, owner protection, revoked/expired invitations, membership removal, image ownership/visibility and malformed uploads; outbox concurrency, opt-out, uncertain delivery and signed callback ordering; configuration defaults; and desktop/mobile controls. Provider wire tests use a mock HTTP transport. Real delivery, sender registration and deployed backup restoration still need operator verification.

Review new processor data and retention alongside the existing privacy workflow. Account export includes SMS preferences. Closure disables SMS, deletes matching invitations and removes notifications with their outbox rows. Team audit records retain UUID references, not invited phone numbers. Set retention for invitation/audit/provider records before launch, and separately handle Twilio's retained records through the reviewed processor process.
