package marketplace

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
	_ "time/tzdata"

	"github.com/jackc/pgx/v5"
)

const FreeBookingLimit = 5

var ireland = func() *time.Location {
	l, e := time.LoadLocation("Europe/Dublin")
	if e != nil {
		panic(e)
	}
	return l
}()

func monthBounds(now time.Time) (time.Time, time.Time) {
	local := now.In(ireland)
	start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, ireland)
	return start, start.AddDate(0, 1, 0)
}
func databaseTime(ctx context.Context, db DB) (time.Time, error) {
	var now time.Time
	err := db.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now)
	return now, err
}
func businessUsage(ctx context.Context, db DB, id string, now time.Time) (BusinessUsage, error) {
	start, end := monthBounds(now)
	u := BusinessUsage{Plan: "free", FreeLimit: FreeBookingLimit, PeriodStart: start, ResetsAt: end, Timezone: "Europe/Dublin", Currency: "EUR", SubscriptionStatus: "none"}
	err := db.QueryRow(ctx, `SELECT count(*) FROM booking_usage WHERE stringer_id=$1 AND accepted_at >= $2 AND accepted_at < $3`, id, start, end).Scan(&u.AcceptedBookings)
	if err != nil {
		return u, err
	}
	err = db.QueryRow(ctx, `SELECT status,paid_until,cancel_at_period_end FROM business_subscriptions WHERE stringer_id=$1`, id).Scan(&u.SubscriptionStatus, &u.PaidUntil, &u.CancelAtPeriodEnd)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return u, err
	}
	if u.SubscriptionStatus == "active" && u.PaidUntil != nil && u.PaidUntil.After(now) {
		u.Plan = "paid"
	}
	u.Remaining = max(0, FreeBookingLimit-u.AcceptedBookings)
	u.CanAccept = u.Plan == "paid" || u.Remaining > 0
	return u, nil
}

type BillingService struct {
	Provider                               BillingProvider
	PriceID, WebhookSecret, FrontendOrigin string
	Live                                   bool
}

func (s *Store) Usage(ctx context.Context, a Actor, id string) (BusinessUsage, error) {
	if err := authorize(ctx, s.Pool, a, id, false); err != nil {
		return BusinessUsage{}, err
	}
	now, err := databaseTime(ctx, s.Pool)
	if err != nil {
		return BusinessUsage{}, err
	}
	u, err := businessUsage(ctx, s.Pool, id, now)
	if err != nil {
		return u, err
	}
	var customer *string
	if err = s.Pool.QueryRow(ctx, `SELECT customer_id FROM business_subscriptions WHERE stringer_id=$1`, id).Scan(&customer); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return u, err
	}
	u.PortalAvailable = s.Billing != nil && customer != nil
	if s.Billing != nil {
		if price, e := s.Billing.Provider.Price(ctx, s.Billing.PriceID); e == nil && validPrice(price, s.Billing.PriceID, s.Billing.Live) {
			u.BillingAvailable = true
			u.MonthlyPriceCents = price.UnitAmount
		}
	}
	return u, nil
}
func billingUnavailable() error {
	return problem(503, "billing_unavailable", "Subscription billing is temporarily unavailable. Existing orders and free-plan tools remain available.")
}
func (s *Store) checkoutAttempt(ctx context.Context, a Actor, id string) (string, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	shop, err := lockStringer(ctx, tx, id)
	if err != nil {
		return "", err
	}
	if err = authorize(ctx, tx, a, id, true); err != nil {
		return "", err
	}
	if shop.ApprovalStatus != "approved" {
		return "", problem(409, "approval_required", "Wait for business approval before subscribing.")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO business_subscriptions(stringer_id) VALUES($1) ON CONFLICT DO NOTHING`, id); err != nil {
		return "", err
	}
	var status string
	if err = tx.QueryRow(ctx, `SELECT status FROM business_subscriptions WHERE stringer_id=$1`, id).Scan(&status); err != nil {
		return "", err
	}
	if status != "none" && status != "canceled" && status != "incomplete_expired" {
		return "", problem(409, "subscription_exists", "Manage the existing subscription through billing settings.")
	}
	var attempt, price string
	var session *string
	var expiry time.Time
	err = tx.QueryRow(ctx, `SELECT id::text,price_id,session_id,expires_at FROM billing_checkout_attempts WHERE stringer_id=$1 AND NOT resolved`, id).Scan(&attempt, &price, &session, &expiry)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if err == nil {
		if !expiry.After(time.Now()) {
			if session == nil {
				return "", problem(409, "billing_reconciliation_required", "An earlier checkout has an uncertain result. Contact support before starting another subscription.")
			}
			previous, e := s.Billing.Provider.Checkout(ctx, *session)
			if e != nil {
				return "", billingUnavailable()
			}
			if previous.Status != "expired" {
				return "", problem(409, "checkout_pending", "The earlier checkout is awaiting billing confirmation. Refresh billing or manage the existing subscription.")
			}
			if _, err = tx.Exec(ctx, `UPDATE billing_checkout_attempts SET resolved=true WHERE id=$1`, attempt); err != nil {
				return "", err
			}
			attempt = ""
		} else if price != s.Billing.PriceID {
			return "", problem(409, "checkout_pending", "Finish or let the existing checkout expire before starting a different plan.")
		}
	}
	if attempt == "" {
		err = tx.QueryRow(ctx, `INSERT INTO billing_checkout_attempts(stringer_id,price_id,expires_at) VALUES($1,$2,clock_timestamp()+interval '1 hour') RETURNING id::text`, id, s.Billing.PriceID).Scan(&attempt)
		if err != nil {
			return "", err
		}
	}
	return attempt, tx.Commit(ctx)
}
func (s *Store) ensureBillingCustomer(ctx context.Context, a Actor, id string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = lockStringer(ctx, tx, id); err != nil {
		return err
	}
	if err = authorize(ctx, tx, a, id, true); err != nil {
		return err
	}
	var customer *string
	if err = tx.QueryRow(ctx, `SELECT customer_id FROM business_subscriptions WHERE stringer_id=$1`, id).Scan(&customer); err != nil {
		return err
	}
	if customer == nil {
		value, e := s.Billing.Provider.CreateCustomer(ctx, id)
		if e != nil {
			return billingUnavailable()
		}
		if _, err = tx.Exec(ctx, `UPDATE business_subscriptions SET customer_id=$2,updated_at=clock_timestamp() WHERE stringer_id=$1`, id, value); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (s *Store) StartSubscription(ctx context.Context, a Actor, id string) (RedirectURL, error) {
	if err := authorize(ctx, s.Pool, a, id, true); err != nil {
		return RedirectURL{}, err
	}
	if s.Billing == nil {
		return RedirectURL{}, billingUnavailable()
	}
	price, err := s.Billing.Provider.Price(ctx, s.Billing.PriceID)
	if err != nil || !validPrice(price, s.Billing.PriceID, s.Billing.Live) {
		return RedirectURL{}, billingUnavailable()
	}
	attempt, err := s.checkoutAttempt(ctx, a, id)
	if err != nil {
		return RedirectURL{}, err
	}
	if err = s.ensureBillingCustomer(ctx, a, id); err != nil {
		return RedirectURL{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return RedirectURL{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = lockStringer(ctx, tx, id); err != nil {
		return RedirectURL{}, err
	}
	if err = authorize(ctx, tx, a, id, true); err != nil {
		return RedirectURL{}, err
	}
	var customer, status, priceID string
	var url *string
	var expiry time.Time
	var resolved bool
	if err = tx.QueryRow(ctx, `SELECT customer_id,status FROM business_subscriptions WHERE stringer_id=$1`, id).Scan(&customer, &status); err != nil {
		return RedirectURL{}, err
	}
	if status != "none" && status != "canceled" && status != "incomplete_expired" {
		return RedirectURL{}, problem(409, "subscription_exists", "Manage the existing subscription through billing settings.")
	}
	if err = tx.QueryRow(ctx, `SELECT checkout_url,expires_at,resolved,price_id FROM billing_checkout_attempts WHERE id=$1 AND stringer_id=$2`, attempt, id).Scan(&url, &expiry, &resolved, &priceID); err != nil {
		return RedirectURL{}, err
	}
	if resolved || url == nil && time.Until(expiry) < 31*time.Minute {
		return RedirectURL{}, problem(409, "checkout_pending", "Refresh billing before retrying checkout.")
	}
	if url == nil {
		session, e := s.Billing.Provider.CreateCheckout(ctx, customer, id, attempt, priceID, s.Billing.FrontendOrigin+"/manage?business="+id+"&billing=return", expiry)
		if e != nil {
			return RedirectURL{}, billingUnavailable()
		}
		url = &session.URL
		if _, err = tx.Exec(ctx, `UPDATE billing_checkout_attempts SET session_id=$2,checkout_url=$3 WHERE id=$1`, attempt, session.ID, session.URL); err != nil {
			return RedirectURL{}, err
		}
	}
	return RedirectURL{URL: *url}, tx.Commit(ctx)
}
func (s *Store) BillingPortal(ctx context.Context, a Actor, id string) (RedirectURL, error) {
	if err := authorize(ctx, s.Pool, a, id, true); err != nil {
		return RedirectURL{}, err
	}
	if s.Billing == nil {
		return RedirectURL{}, billingUnavailable()
	}
	var customer *string
	err := s.Pool.QueryRow(ctx, `SELECT customer_id FROM business_subscriptions WHERE stringer_id=$1`, id).Scan(&customer)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && customer == nil {
		return RedirectURL{}, problem(409, "no_subscription", "Start a subscription first.")
	}
	if err != nil {
		return RedirectURL{}, err
	}
	url, err := s.Billing.Provider.Portal(ctx, *customer, s.Billing.FrontendOrigin+"/manage?business="+id+"&billing=return")
	if err != nil {
		return RedirectURL{}, billingUnavailable()
	}
	return RedirectURL{URL: url}, nil
}

func (s *Store) ConfirmCash(ctx context.Context, a Actor, id string) (Order, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback(ctx)
	var business string
	if err = tx.QueryRow(ctx, `SELECT stringer_id::text FROM orders WHERE id=$1`, id).Scan(&business); err != nil {
		return Order{}, missing(err)
	}
	if _, err = lockStringer(ctx, tx, business); err != nil {
		return Order{}, err
	}
	if err = authorize(ctx, tx, a, business, false); err != nil {
		return Order{}, err
	}
	o, err := jsonOrder(tx.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return o, err
	}
	if o.PaymentMethod != "cash" {
		return Order{}, problem(409, "payment_method_mismatch", "Only cash payments can be confirmed here.")
	}
	if o.PaymentStatus == "paid" {
		return o, tx.Commit(ctx)
	}
	if o.PaymentStatus != "unpaid" || o.Status == "placed" || o.Status == "cancelled" {
		return Order{}, problem(409, "cash_receipt_not_allowed", "Cash can be recorded only for an accepted, non-cancelled, unpaid order.")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO order_payment_events(order_id,actor_id,from_status,to_status,reason) VALUES($1,$2,'unpaid','paid','Cash receipt confirmed by the stringer')`, id, a.ID); err != nil {
		return Order{}, err
	}
	o, err = jsonOrder(tx.QueryRow(ctx, `UPDATE orders SET payment_status='paid',updated_at=clock_timestamp() WHERE id=$1 RETURNING `+orderCols, id))
	if err != nil {
		return o, err
	}
	return o, tx.Commit(ctx)
}

type stripeEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Live bool   `json:"livemode"`
	Data struct {
		Object struct {
			ID           string `json:"id"`
			Customer     string `json:"customer"`
			Subscription string `json:"subscription"`
			Parent       struct {
				SubscriptionDetails struct {
					Subscription string `json:"subscription"`
				} `json:"subscription_details"`
			} `json:"parent"`
		} `json:"object"`
	} `json:"data"`
}

func (s *Store) BillingWebhook(w http.ResponseWriter, r *http.Request) {
	if s.Billing == nil {
		sendError(w, r, billingUnavailable())
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 256<<10))
	if err != nil || !verifyStripeSignature(body, r.Header.Get("Stripe-Signature"), s.Billing.WebhookSecret, time.Now()) {
		sendError(w, r, problem(400, "invalid_signature", "Invalid webhook signature."))
		return
	}
	var event stripeEvent
	if json.Unmarshal(body, &event) != nil || event.ID == "" || event.Live != s.Billing.Live {
		sendError(w, r, problem(400, "invalid_billing_event", "Invalid billing event."))
		return
	}
	if err = s.processBillingEvent(r.Context(), event); err != nil {
		sendError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"received": true})
}
func (s *Store) processBillingEvent(ctx context.Context, e stripeEvent) error {
	object := e.Data.Object
	subID := object.Subscription
	switch e.Type {
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		subID = object.ID
	case "checkout.session.completed", "invoice.paid", "invoice.payment_failed":
		if subID == "" {
			subID = object.Parent.SubscriptionDetails.Subscription
		}
	default:
		return nil
	}
	if subID == "" || object.Customer == "" {
		return nil
	}
	var id string
	if err := s.Pool.QueryRow(ctx, `SELECT stringer_id::text FROM business_subscriptions WHERE customer_id=$1`, object.Customer).Scan(&id); errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = lockStringer(ctx, tx, id); err != nil {
		return err
	}
	var seen bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM billing_events WHERE event_id=$1)`, e.ID).Scan(&seen); err != nil {
		return err
	}
	if seen {
		return nil
	}
	// Fetch current state under the business lock. Delivery order never decides
	// entitlement, and a browser redirect can never grant the paid plan.
	sub, err := s.Billing.Provider.Subscription(ctx, subID)
	if err != nil {
		return billingUnavailable()
	}
	if sub.Customer != object.Customer || sub.Metadata["stringer_id"] != id || sub.Live != s.Billing.Live {
		return problem(400, "billing_identity_mismatch", "Subscription identity mismatch.")
	}
	var expectedPrice string
	err = tx.QueryRow(ctx, `SELECT price_id FROM billing_checkout_attempts WHERE id::text=$1 AND stringer_id=$2`, sub.Metadata["billing_attempt_id"], id).Scan(&expectedPrice)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return problem(400, "billing_identity_mismatch", "Subscription checkout mismatch.")
	}
	var current *string
	var status string
	if err = tx.QueryRow(ctx, `SELECT subscription_id,status FROM business_subscriptions WHERE stringer_id=$1`, id).Scan(&current, &status); err != nil {
		return err
	}
	if current != nil && *current != sub.ID && (status != "canceled" && status != "incomplete_expired" || sub.Status != "active") {
		return nil
	}
	var until *time.Time
	if len(sub.Items.Data) == 1 {
		item := sub.Items.Data[0]
		if item.Quantity == 1 && item.Price.ID == expectedPrice && item.Price.Currency == "eur" && item.Price.Recurring != nil && item.Price.Recurring.Interval == "month" && item.Price.Recurring.IntervalCount == 1 && sub.LatestInvoice != nil && sub.LatestInvoice.Paid && sub.Status == "active" && jsonEmpty(sub.PauseCollection) {
			v := time.Unix(item.PeriodEnd, 0)
			until = &v
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE business_subscriptions SET subscription_id=$2,status=$3,paid_until=$4,cancel_at_period_end=$5,updated_at=clock_timestamp() WHERE stringer_id=$1`, id, sub.ID, sub.Status, until, sub.CancelAtPeriodEnd); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE billing_checkout_attempts SET resolved=true WHERE id::text=$1 AND stringer_id=$2`, sub.Metadata["billing_attempt_id"], id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO billing_events(event_id,stringer_id,event_type) VALUES($1,$2,$3)`, e.ID, id, e.Type); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Re-read after acquiring the business lock: a query that waited for another
// acceptance must not rely on the snapshot used to acquire that lock.
func requireBookingCapacity(ctx context.Context, db DB, id string) error {
	now, err := databaseTime(ctx, db)
	if err != nil {
		return err
	}
	u, err := businessUsage(ctx, db, id, now)
	if err != nil {
		return err
	}
	if !u.CanAccept {
		return problem(409, "booking_limit_reached", "This stringer cannot accept more bookings this month.")
	}
	return nil
}
