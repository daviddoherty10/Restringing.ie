package marketplace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type RacketPayments struct {
	Provider                      RacketPaymentProvider
	FrontendOrigin, WebhookSecret string
	Live                          bool
}
type PaymentSettingsInput struct {
	OnsiteCash    bool `json:"onsite_cash"`
	OnsiteCard    bool `json:"onsite_card"`
	OnlineEnabled bool `json:"online_enabled"`
}
type PaymentOptions struct {
	PaymentSettingsInput
	OnlineAvailable  bool `json:"online_available"`
	OnlineConfigured bool `json:"online_configured"`
	OnlineReady      bool `json:"online_ready"`
}

func paymentUnavailable() error {
	return problem(503, "payment_unavailable", "Online payments are unavailable. Please retry or contact your stringer.")
}
func (s *Store) paymentOptions(ctx context.Context, db DB, id string) (PaymentOptions, error) {
	var out PaymentOptions
	var live *bool
	e := db.QueryRow(ctx, `SELECT COALESCE(p.onsite_cash,true),COALESCE(p.onsite_card,false),COALESCE(p.online_enabled,false),COALESCE(p.account_ready,false),p.account_live FROM stringers s LEFT JOIN stringer_payment_settings p ON p.stringer_id=s.id WHERE s.id=$1`, id).Scan(&out.OnsiteCash, &out.OnsiteCard, &out.OnlineEnabled, &out.OnlineReady, &live)
	out.OnlineConfigured = s.Payments != nil
	out.OnlineReady = out.OnlineReady && s.Payments != nil && live != nil && *live == s.Payments.Live
	out.OnlineAvailable = out.OnlineEnabled && out.OnlineReady
	return out, missing(e)
}
func (s *Store) PaymentSettings(ctx context.Context, a Actor, id string) (PaymentOptions, error) {
	if e := authorize(ctx, s.Pool, a, id, false); e != nil {
		return PaymentOptions{}, e
	}
	if s.Payments != nil {
		var account *string
		var live *bool
		e := s.Pool.QueryRow(ctx, `SELECT account_id,account_live FROM stringer_payment_settings WHERE stringer_id=$1`, id).Scan(&account, &live)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return PaymentOptions{}, e
		}
		if account != nil && live != nil && *live == s.Payments.Live {
			state, e := s.Payments.Provider.Account(ctx, *account)
			if e != nil {
				return PaymentOptions{}, paymentUnavailable()
			}
			if state.ID != *account || state.Live != s.Payments.Live {
				return PaymentOptions{}, paymentUnavailable()
			}
			if _, e = s.Pool.Exec(ctx, `UPDATE stringer_payment_settings SET account_ready=$2 WHERE stringer_id=$1 AND account_id=$3`, id, state.Ready(), *account); e != nil {
				return PaymentOptions{}, e
			}
		}
	}
	return s.paymentOptions(ctx, s.Pool, id)
}
func (s *Store) SavePaymentSettings(ctx context.Context, a Actor, id string, in PaymentSettingsInput) (PaymentOptions, error) {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return PaymentOptions{}, e
	}
	defer tx.Rollback(ctx)
	if _, e = lockStringer(ctx, tx, id); e != nil {
		return PaymentOptions{}, e
	}
	if e = authorize(ctx, tx, a, id, true); e != nil {
		return PaymentOptions{}, e
	}
	current, e := s.paymentOptions(ctx, tx, id)
	if e != nil {
		return current, e
	}
	if !in.OnsiteCash && !in.OnsiteCard && !in.OnlineEnabled {
		return current, invalid(map[string]string{"payments": "Offer at least one payment option."})
	}
	if in.OnlineEnabled && !current.OnlineReady {
		return current, invalid(map[string]string{"online_enabled": "Complete Stripe setup and refresh its status before enabling online payments."})
	}
	_, e = tx.Exec(ctx, `INSERT INTO stringer_payment_settings(stringer_id,onsite_cash,onsite_card,online_enabled) VALUES($1,$2,$3,$4) ON CONFLICT(stringer_id) DO UPDATE SET onsite_cash=$2,onsite_card=$3,online_enabled=$4`, id, in.OnsiteCash, in.OnsiteCard, in.OnlineEnabled)
	if e != nil {
		return current, e
	}
	out, e := s.paymentOptions(ctx, tx, id)
	if e != nil {
		return out, e
	}
	return out, tx.Commit(ctx)
}
func safeStripeURL(raw string, hosts ...string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil {
		return false
	}
	for _, host := range hosts {
		if u.Host == host {
			return true
		}
	}
	return false
}
func (s *Store) StartPaymentOnboarding(ctx context.Context, a Actor, id string) (RedirectURL, error) {
	if s.Payments == nil {
		return RedirectURL{}, paymentUnavailable()
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return RedirectURL{}, e
	}
	defer tx.Rollback(ctx)
	shop, e := lockStringer(ctx, tx, id)
	if e != nil {
		return RedirectURL{}, e
	}
	if e = authorize(ctx, tx, a, id, true); e != nil {
		return RedirectURL{}, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO stringer_payment_settings(stringer_id) VALUES($1) ON CONFLICT DO NOTHING`, id); e != nil {
		return RedirectURL{}, e
	}
	var account *string
	var live *bool
	if e = tx.QueryRow(ctx, `SELECT account_id,account_live FROM stringer_payment_settings WHERE stringer_id=$1`, id).Scan(&account, &live); e != nil {
		return RedirectURL{}, e
	}
	if account != nil && (live == nil || *live != s.Payments.Live) {
		return RedirectURL{}, problem(409, "payment_environment_mismatch", "This business is linked in a different Stripe environment. Contact support before changing environments.")
	}
	if account == nil {
		state, e := s.Payments.Provider.CreateAccount(ctx, id, shop.Name)
		if e != nil || !strings.HasPrefix(state.ID, "acct_") || state.Live != s.Payments.Live {
			return RedirectURL{}, paymentUnavailable()
		}
		account = &state.ID
		if _, e = tx.Exec(ctx, `UPDATE stringer_payment_settings SET account_id=$2,account_live=$3,account_ready=$4 WHERE stringer_id=$1`, id, state.ID, state.Live, state.Ready()); e != nil {
			return RedirectURL{}, e
		}
	}
	// Persist account identity before requesting a short-lived onboarding link.
	if e = tx.Commit(ctx); e != nil {
		return RedirectURL{}, e
	}
	link, e := s.Payments.Provider.Onboard(ctx, *account, s.Payments.FrontendOrigin+"/manage?business="+id+"&payments=return")
	if e != nil || !safeStripeURL(link, "accounts.stripe.com", "connect.stripe.com") {
		return RedirectURL{}, paymentUnavailable()
	}
	return RedirectURL{URL: link}, nil
}
func (s *Store) validatePaymentChoice(ctx context.Context, db DB, id string, b Booking, q *Quote) error {
	options, e := s.paymentOptions(ctx, db, id)
	if e != nil {
		return e
	}
	method := orderPaymentMethod(b)
	if method == "cash" && !options.OnsiteCash || method == "onsite" && !options.OnsiteCash && !options.OnsiteCard || method == "online" && !options.OnlineAvailable || !oneOf(method, "cash", "onsite", "online") {
		return problem(422, "payment_method_unavailable", "Choose a payment option currently offered by this stringer.")
	}
	if b.PaymentMethod == "" {
		return nil
	} // Preserve old cash quote fingerprints.
	q.Configuration.OnsiteCash = options.OnsiteCash
	q.Configuration.OnsiteCard = options.OnsiteCard
	fingerprint, _ := json.Marshal([]any{q.Fingerprint, options.OnsiteCash, options.OnsiteCard, options.OnlineAvailable})
	hash := sha256.Sum256(fingerprint)
	q.Fingerprint = hex.EncodeToString(hash[:])
	return nil
}
func (s *Store) ConfirmOnsite(ctx context.Context, a Actor, id, method string) (Order, error) {
	if !oneOf(method, "cash", "card") {
		return Order{}, invalid(map[string]string{"method": "Choose cash or card received onsite."})
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return Order{}, e
	}
	defer tx.Rollback(ctx)
	var business string
	if e = tx.QueryRow(ctx, `SELECT stringer_id::text FROM orders WHERE id=$1`, id).Scan(&business); e != nil {
		return Order{}, missing(e)
	}
	if _, e = lockStringer(ctx, tx, business); e != nil {
		return Order{}, e
	}
	if e = authorize(ctx, tx, a, business, false); e != nil {
		return Order{}, e
	}
	o, e := jsonOrder(tx.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id=$1 FOR UPDATE`, id))
	if e != nil {
		return o, e
	}
	if o.PaymentMethod != "onsite" {
		return o, problem(409, "payment_method_mismatch", "This booking did not select onsite payment.")
	}
	if method == "cash" && !o.Configuration.OnsiteCash || method == "card" && !o.Configuration.OnsiteCard {
		return o, invalid(map[string]string{"method": "Choose an onsite method offered on this booking."})
	}
	if o.PaymentStatus == "paid" {
		return o, tx.Commit(ctx)
	}
	if o.PaymentStatus != "unpaid" || o.Status == "placed" || o.Status == "cancelled" {
		return o, problem(409, "onsite_receipt_not_allowed", "Payment can be recorded only for an accepted, non-cancelled, unpaid onsite order.")
	}
	reason := "Onsite " + method + " receipt confirmed by the stringer"
	if _, e = tx.Exec(ctx, `INSERT INTO order_payment_events(order_id,actor_id,from_status,to_status,reason) VALUES($1,$2,'unpaid','paid',$3)`, id, a.ID, reason); e != nil {
		return o, e
	}
	o, e = jsonOrder(tx.QueryRow(ctx, `UPDATE orders SET payment_status='paid',updated_at=clock_timestamp() WHERE id=$1 RETURNING `+orderCols, id))
	if e != nil {
		return o, e
	}
	return o, tx.Commit(ctx)
}
func validRacketCheckout(c RacketCheckout, o Order, live bool) bool {
	return c.ID != "" && c.Metadata["order_id"] == o.ID && c.Currency == "eur" && c.Total == o.TotalCents && c.Live == live
}
func checkoutPaid(c RacketCheckout) bool {
	return c.PaymentStatus == "paid" || c.PaymentStatus == "no_payment_required" && c.Total == 0
}
func (s *Store) StartOrderPayment(ctx context.Context, a Actor, id string) (RedirectURL, error) {
	if s.Payments == nil {
		return RedirectURL{}, paymentUnavailable()
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return RedirectURL{}, e
	}
	defer tx.Rollback(ctx)
	var business string
	if e = tx.QueryRow(ctx, `SELECT stringer_id::text FROM orders WHERE id=$1 AND customer_id=$2`, id, a.ID).Scan(&business); e != nil {
		return RedirectURL{}, missing(e)
	}
	if _, e = lockStringer(ctx, tx, business); e != nil {
		return RedirectURL{}, e
	}
	o, e := jsonOrder(tx.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id=$1 FOR UPDATE`, id))
	if e != nil {
		return RedirectURL{}, e
	}
	if o.PaymentMethod != "online" || o.PaymentStatus != "unpaid" || o.Status == "cancelled" {
		return RedirectURL{}, problem(409, "checkout_not_allowed", "This order cannot start an online payment.")
	}
	var account string
	var live bool
	if e = tx.QueryRow(ctx, `SELECT account_id,account_live FROM stringer_payment_settings WHERE stringer_id=$1`, business).Scan(&account, &live); e != nil {
		return RedirectURL{}, paymentUnavailable()
	}
	state, e := s.Payments.Provider.Account(ctx, account)
	if e != nil || state.ID != account || !state.Ready() || state.Live != s.Payments.Live || live != s.Payments.Live {
		return RedirectURL{}, paymentUnavailable()
	}
	var attempt string
	var session *string
	var expires time.Time
	e = tx.QueryRow(ctx, `SELECT id::text,session_id,expires_at FROM order_checkout_attempts WHERE order_id=$1 AND NOT resolved`, id).Scan(&attempt, &session, &expires)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return RedirectURL{}, e
	}
	if session != nil {
		c, err := s.Payments.Provider.Checkout(ctx, account, *session)
		if err != nil || c.ID != *session || !validRacketCheckout(c, o, live) {
			return RedirectURL{}, paymentUnavailable()
		}
		if checkoutPaid(c) {
			return RedirectURL{URL: s.Payments.FrontendOrigin + "/orders/" + id}, nil
		}
		if c.Status == "open" && safeStripeURL(c.URL, "checkout.stripe.com") {
			return RedirectURL{URL: c.URL}, nil
		}
		if c.Status != "expired" {
			return RedirectURL{}, problem(409, "payment_processing", "Your payment is processing. Check this order again before trying to pay.")
		}
		if _, e = tx.Exec(ctx, `UPDATE order_checkout_attempts SET resolved=true WHERE id=$1`, attempt); e != nil {
			return RedirectURL{}, e
		}
		attempt = ""
	}
	if attempt == "" {
		e = tx.QueryRow(ctx, `INSERT INTO order_checkout_attempts(order_id,account_id,expires_at) VALUES($1,$2,now()+interval '35 minutes') RETURNING id::text,expires_at`, id, account).Scan(&attempt, &expires)
		if e != nil {
			return RedirectURL{}, e
		}
	}
	// shortcut: an unresolved expired checkout needs support reconciliation; automate provider lookup before unattended recovery.
	if !expires.After(time.Now()) {
		return RedirectURL{}, problem(409, "payment_setup_pending", "A previous checkout needs reconciliation. Contact support before starting another payment.")
	}
	if e = tx.Commit(ctx); e != nil {
		return RedirectURL{}, e
	}
	c, e := s.Payments.Provider.CreateCheckout(ctx, account, o, "racket-checkout:"+attempt, s.Payments.FrontendOrigin, expires)
	if e != nil || !validRacketCheckout(c, o, live) || !safeStripeURL(c.URL, "checkout.stripe.com") {
		return RedirectURL{}, paymentUnavailable()
	}
	if _, e = s.Pool.Exec(ctx, `UPDATE order_checkout_attempts SET session_id=$2,payment_intent=NULLIF($3,'') WHERE id=$1 AND NOT resolved AND (session_id IS NULL OR session_id=$2)`, attempt, c.ID, c.PaymentIntent); e != nil {
		return RedirectURL{}, e
	}
	return RedirectURL{URL: c.URL}, nil
}

// Cancel an open hosted checkout before releasing stock. A completed asynchronous
// payment must settle first; a browser redirect never decides payment status.
func (s *Store) cancelOnlineCheckout(ctx context.Context, db DB, o Order) error {
	var account string
	var session *string
	e := db.QueryRow(ctx, `SELECT account_id,session_id FROM order_checkout_attempts WHERE order_id=$1 AND NOT resolved`, o.ID).Scan(&account, &session)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	if s.Payments == nil || session == nil {
		return problem(409, "payment_setup_pending", "Finish loading your payment page before cancelling this booking.")
	}
	c, e := s.Payments.Provider.Checkout(ctx, account, *session)
	if e != nil || c.ID != *session || !validRacketCheckout(c, o, s.Payments.Live) {
		return paymentUnavailable()
	}
	if c.Status == "open" {
		c, e = s.Payments.Provider.Expire(ctx, account, *session)
		if e != nil || c.ID != *session || c.Status != "expired" {
			return problem(409, "payment_processing", "Payment may be processing. Refresh the order before cancelling.")
		}
	}
	if c.Status != "expired" && o.PaymentStatus == "unpaid" {
		return problem(409, "payment_processing", "Payment is processing. Wait for confirmation before cancelling.")
	}
	return nil
}
func (s *Store) RacketPaymentWebhook(w http.ResponseWriter, r *http.Request) {
	if s.Payments == nil {
		sendError(w, r, paymentUnavailable())
		return
	}
	body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 256<<10))
	if e != nil || !verifyStripeSignature(body, r.Header.Get("Stripe-Signature"), s.Payments.WebhookSecret, time.Now()) {
		sendError(w, r, problem(400, "invalid_signature", "Invalid payment webhook signature."))
		return
	}
	var event struct {
		RelatedObject struct {
			ID string `json:"id"`
		} `json:"related_object"`
		ID      string `json:"id"`
		Type    string `json:"type"`
		Account string `json:"account"`
		Live    bool   `json:"livemode"`
		Data    struct {
			Object struct {
				ID string `json:"id"`
			} `json:"object"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &event) != nil || event.ID == "" || event.Live != s.Payments.Live {
		sendError(w, r, problem(400, "invalid_payment_event", "Invalid connected payment event."))
		return
	}
	if strings.HasPrefix(event.Type, "v2.core.account.") || strings.HasPrefix(event.Type, "v2.core.account[") {
		event.Account = event.RelatedObject.ID
		event.Type = "account.updated"
	}
	if event.Account == "" {
		sendError(w, r, problem(400, "invalid_payment_event", "A connected account is required."))
		return
	}
	if event.Type == "account.updated" {
		state, err := s.Payments.Provider.Account(r.Context(), event.Account)
		if err != nil || state.ID != event.Account || state.Live != s.Payments.Live {
			sendError(w, r, paymentUnavailable())
			return
		}
		_, err = s.Pool.Exec(r.Context(), `UPDATE stringer_payment_settings SET account_ready=$2 WHERE account_id=$1 AND account_live=$3`, event.Account, state.Ready(), event.Live)
		if err != nil {
			sendError(w, r, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"received": true})
		return
	}
	if !oneOf(event.Type, "checkout.session.completed", "checkout.session.async_payment_succeeded", "checkout.session.async_payment_failed", "charge.refunded", "charge.dispute.created", "charge.dispute.closed") {
		writeJSON(w, 200, map[string]bool{"received": true})
		return
	}
	if e = s.reconcileRacketEvent(r.Context(), event.ID, event.Type, event.Account, event.Data.Object.ID); e != nil {
		sendError(w, r, e)
		return
	}
	writeJSON(w, 200, map[string]bool{"received": true})
}
func (s *Store) reconcileRacketEvent(ctx context.Context, eventID, kind, account, object string) error {
	if strings.HasPrefix(kind, "charge.") {
		return s.recordChargeEvent(ctx, eventID, kind, account, object)
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	// Resolve only sessions created by this application in the signed connected account.
	var id, business string
	e = tx.QueryRow(ctx, `SELECT o.id::text,o.stringer_id::text FROM order_checkout_attempts a JOIN orders o ON o.id=a.order_id WHERE a.account_id=$1 AND a.session_id=$2`, account, object).Scan(&id, &business)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	if _, e = lockStringer(ctx, tx, business); e != nil {
		return e
	}
	var seen bool
	if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM racket_payment_events WHERE event_id=$1)`, eventID).Scan(&seen); e != nil || seen {
		return e
	}
	o, e := jsonOrder(tx.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id=$1 FOR UPDATE`, id))
	if e != nil {
		return e
	}
	c, e := s.Payments.Provider.Checkout(ctx, account, object)
	if e != nil || c.ID != object || !validRacketCheckout(c, o, s.Payments.Live) {
		return paymentUnavailable()
	}
	if checkoutPaid(c) && o.PaymentStatus == "unpaid" {
		if _, e = tx.Exec(ctx, `UPDATE orders SET payment_status='paid',updated_at=clock_timestamp() WHERE id=$1`, id); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO order_payment_events(order_id,from_status,to_status,reason) VALUES($1,'unpaid','paid','Online payment confirmed by Stripe')`, id); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(ctx, `UPDATE order_checkout_attempts SET payment_intent=NULLIF($3,'') WHERE order_id=$1 AND session_id=$2`, id, c.ID, c.PaymentIntent); e != nil {
		return e
	}
	if kind == "checkout.session.async_payment_failed" && !checkoutPaid(c) {
		if _, e = tx.Exec(ctx, `UPDATE order_checkout_attempts SET resolved=true WHERE session_id=$1 AND account_id=$2`, object, account); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(ctx, `INSERT INTO racket_payment_events(event_id,order_id,event_type,account_id) VALUES($1,$2,$3,$4)`, eventID, id, kind, account); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (s *Store) recordChargeEvent(ctx context.Context, eventID, kind, account, object string) error {
	// Re-fetch the charge in the signed account before recording refunds or disputes.
	disputeID := object
	reason := "Refund status confirmed by Stripe"
	var review *bool
	if strings.HasPrefix(kind, "charge.dispute.") {
		d, e := s.Payments.Provider.Dispute(ctx, account, object)
		if e != nil || d.Charge == "" {
			return paymentUnavailable()
		}
		object = d.Charge
		value := !oneOf(d.Status, "won", "warning_closed")
		review = &value
		reason = "Payment dispute recorded by Stripe; review it in your Stripe Dashboard"
	}
	c, e := s.Payments.Provider.Charge(ctx, account, object)
	if e != nil {
		return paymentUnavailable()
	}
	checkout, e := s.Payments.Provider.CheckoutForIntent(ctx, account, c.PaymentIntent)
	if e != nil {
		return paymentUnavailable()
	}
	if checkout.ID == "" {
		return nil
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var id, business string
	e = tx.QueryRow(ctx, `SELECT o.id::text,o.stringer_id::text FROM order_checkout_attempts a JOIN orders o ON o.id=a.order_id WHERE a.account_id=$1 AND a.session_id=$2`, account, checkout.ID).Scan(&id, &business)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	if _, e = lockStringer(ctx, tx, business); e != nil {
		return e
	}
	var seen bool
	if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM racket_payment_events WHERE event_id=$1)`, eventID).Scan(&seen); e != nil || seen {
		return e
	}
	o, e := jsonOrder(tx.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id=$1 FOR UPDATE`, id))
	if e != nil {
		return e
	}
	// Re-read financial state after the business lock; a queued older event must not undo a newer refund.
	c, e = s.Payments.Provider.Charge(ctx, account, object)
	if e != nil {
		return paymentUnavailable()
	}
	if strings.HasPrefix(kind, "charge.dispute.") {
		d, err := s.Payments.Provider.Dispute(ctx, account, disputeID)
		if err != nil || d.Charge != object {
			return paymentUnavailable()
		}
		value := !oneOf(d.Status, "won", "warning_closed")
		review = &value
	}
	if c.Live != s.Payments.Live || c.Currency != "eur" || c.Amount != o.TotalCents || c.Refunded < 0 || c.Refunded > c.Amount {
		return paymentUnavailable()
	}
	if checkout.PaymentIntent != c.PaymentIntent || !validRacketCheckout(checkout, o, s.Payments.Live) {
		return paymentUnavailable()
	}
	if review != nil {
		if _, e = tx.Exec(ctx, `UPDATE orders SET payment_review_required=$2 WHERE id=$1`, id, *review); e != nil {
			return e
		}
	}
	next := "paid"
	if c.Refunded == c.Amount {
		next = "refunded"
	} else if c.Refunded > 0 {
		next = "partially_refunded"
	}
	if next != o.PaymentStatus || strings.HasPrefix(kind, "charge.dispute.") {
		if _, e = tx.Exec(ctx, `UPDATE orders SET payment_status=$2,updated_at=clock_timestamp() WHERE id=$1`, id, next); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO order_payment_events(order_id,from_status,to_status,reason) VALUES($1,$2,$3,$4)`, id, o.PaymentStatus, next, reason); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(ctx, `INSERT INTO racket_payment_events(event_id,order_id,event_type,account_id) VALUES($1,$2,$3,$4)`, eventID, id, kind, account); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func registerRacketPaymentPublic(r chi.Router, s *Store) {
	r.Get("/stringers/{stringerID}/payment-options", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		id := chi.URLParam(r, "stringerID")
		if _, e := s.GetStringer(r.Context(), id); e != nil {
			return nil, e
		}
		return s.paymentOptions(r.Context(), s.Pool, id)
	}))
	r.Post("/payments/stripe/webhook", s.RacketPaymentWebhook)
}
func registerRacketPaymentPrivate(r chi.Router, s *Store) {
	r.Get("/stringers/{stringerID}/manage/payment-options", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.PaymentSettings(r.Context(), actor(r), chi.URLParam(r, "stringerID"))
	}))
	r.Put("/stringers/{stringerID}/manage/payment-options", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in PaymentSettingsInput
		if e := decode(w, r, &in); e != nil {
			return nil, e
		}
		return s.SavePaymentSettings(r.Context(), actor(r), chi.URLParam(r, "stringerID"), in)
	}))
	r.Post("/stringers/{stringerID}/payments/onboarding", wrap(201, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in struct{}
		if e := decode(w, r, &in); e != nil {
			return nil, e
		}
		return s.StartPaymentOnboarding(r.Context(), actor(r), chi.URLParam(r, "stringerID"))
	}))
	r.Post("/orders/{orderID}/payment-checkout", wrap(201, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in struct{}
		if e := decode(w, r, &in); e != nil {
			return nil, e
		}
		return s.StartOrderPayment(r.Context(), actor(r), chi.URLParam(r, "orderID"))
	}))
	r.Post("/orders/{orderID}/onsite-receipt", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in struct {
			Method string `json:"method"`
		}
		if e := decode(w, r, &in); e != nil {
			return nil, e
		}
		return s.ConfirmOnsite(r.Context(), actor(r), chi.URLParam(r, "orderID"), in.Method)
	}))
}
