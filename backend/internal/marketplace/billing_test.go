package marketplace

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestIrishMonthBounds(t *testing.T) {
	cases := []struct{ now, start, end string }{
		{"2026-03-31T23:30:00Z", "2026-03-31T23:00:00Z", "2026-04-30T23:00:00Z"},
		{"2026-10-25T01:30:00Z", "2026-09-30T23:00:00Z", "2026-11-01T00:00:00Z"},
		{"2026-12-31T23:59:59Z", "2026-12-01T00:00:00Z", "2027-01-01T00:00:00Z"},
	}
	for _, c := range cases {
		now, _ := time.Parse(time.RFC3339, c.now)
		s, e := monthBounds(now)
		if s.UTC().Format(time.RFC3339) != c.start || e.UTC().Format(time.RFC3339) != c.end {
			t.Fatalf("%s: %s %s", c.now, s, e)
		}
	}
}
func billingOrder(t *testing.T, f *fixture, n int) Order {
	t.Helper()
	ctx := context.Background()
	b := f.booking
	b.StringSource = "customer"
	b.VariantID = ""
	b.PrestretchPercent = 0
	q, err := f.store.CreateQuote(ctx, f.customer, f.shop.ID, b)
	if err != nil {
		t.Fatal(err)
	}
	o, _, err := f.store.CreateOrder(ctx, f.customer, q.ID, fmt.Sprintf("billing-test-order-%03d", n))
	if err != nil {
		t.Fatal(err)
	}
	return o
}
func TestIntegrationMonthlyQuotaRaceAndExistingWork(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	orders := []Order{}
	for n := 0; n < 6; n++ {
		orders = append(orders, billingOrder(t, f, n))
	}
	for n := 0; n < 4; n++ {
		if _, err := s.UpdateStatus(ctx, f.owner, orders[n].ID, "accepted"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Pool.Exec(ctx, `INSERT INTO stringer_members(stringer_id,user_id,role) VALUES($1,$2,'staff')`, f.shop.ID, f.other.ID); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for n, a := range []Actor{f.owner, f.other} {
		wg.Add(1)
		go func(n int, a Actor) {
			defer wg.Done()
			_, e := s.UpdateStatus(ctx, a, orders[n+4].ID, "accepted")
			results <- e
		}(n, a)
	}
	wg.Wait()
	close(results)
	success, blocked := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else {
			wantProblem(t, e, "booking_limit_reached")
			blocked++
		}
	}
	if success != 1 || blocked != 1 {
		t.Fatalf("race: %d accepted, %d blocked", success, blocked)
	}
	u, err := s.Usage(ctx, f.owner, f.shop.ID)
	if err != nil || u.AcceptedBookings != 5 || u.CanAccept || u.Plan != "free" {
		t.Fatal(u, err)
	}
	shop, err := s.GetStringer(ctx, f.shop.ID)
	if err != nil || shop.BookingAvailable {
		t.Fatal(shop, err)
	}
	_, err = s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	wantProblem(t, err, "booking_limit_reached")
	existing, replay, err := s.CreateOrder(ctx, f.customer, orders[0].QuoteID, "billing-test-order-000")
	if err != nil || !replay || existing.ID != orders[0].ID {
		t.Fatal("idempotent retry blocked", err)
	}
	if _, err = s.UpdateStatus(ctx, f.owner, orders[0].ID, "received"); err != nil {
		t.Fatal("existing fulfilment blocked", err)
	}
	if _, err = s.UpdateStatus(ctx, f.owner, orders[1].ID, "cancelled"); err != nil {
		t.Fatal(err)
	}
	u, _ = s.Usage(ctx, f.owner, f.shop.ID)
	if u.AcceptedBookings != 5 {
		t.Fatal("cancellation reset usage")
	}
	next := u.ResetsAt.Add(time.Second)
	u, err = businessUsage(ctx, s.Pool, f.shop.ID, next)
	if err != nil || u.AcceptedBookings != 0 || !u.CanAccept {
		t.Fatal("month reset", u, err)
	}
	other, err := s.Apply(ctx, f.owner, ProfileInput{Name: "Separate business"})
	if err != nil {
		t.Fatal(err)
	}
	u, err = s.Usage(ctx, f.owner, other.ID)
	if err != nil || u.AcceptedBookings != 0 {
		t.Fatal("business usage mixed", u, err)
	}
}
func TestIntegrationCashReceiptAccessAuditAndRetry(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	o := billingOrder(t, f, 1)
	wantStatus(t, f.request(t, "POST", "/orders/"+o.ID+"/cash-receipt", map[string]any{}, f.customer, nil), 403)
	_, err := f.store.ConfirmCash(ctx, f.owner, o.ID)
	wantProblem(t, err, "cash_receipt_not_allowed")
	if _, err = f.store.UpdateStatus(ctx, f.owner, o.ID, "accepted"); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, f.request(t, "POST", "/orders/"+o.ID+"/cash-receipt", map[string]any{"payment_status": "paid"}, f.owner, nil), 400)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := f.store.ConfirmCash(ctx, f.owner, o.ID); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	saved, err := f.store.GetOrder(ctx, f.customer, o.ID)
	if err != nil || saved.PaymentStatus != "paid" || saved.PaymentMethod != "cash" || saved.Status != "accepted" || len(saved.PaymentEvents) != 1 {
		t.Fatal(saved, err)
	}
	var actor string
	if err = f.store.Pool.QueryRow(ctx, `SELECT actor_id::text FROM order_payment_events WHERE order_id=$1`, o.ID).Scan(&actor); err != nil || actor != f.owner.ID {
		t.Fatal("missing receipt actor", err)
	}
	if _, err = f.store.UpdateStatus(ctx, f.owner, o.ID, "cancelled"); err != nil {
		t.Fatal(err)
	}
	saved, err = f.store.GetOrder(ctx, f.customer, o.ID)
	if err != nil || saved.PaymentStatus != "paid" {
		t.Fatal("cancellation invented refund", err)
	}
	other := billingOrder(t, f, 2)
	if _, err = f.store.Pool.Exec(ctx, `UPDATE orders SET payment_method='online' WHERE id=$1`, other.ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.store.ConfirmCash(ctx, f.owner, other.ID)
	wantProblem(t, err, "payment_method_mismatch")
}

type fakeBilling struct {
	checkoutCalls int
	failCheckout  bool
	sub           BillingSubscription
	sessionStatus string
	price         BillingPrice
}

func newFakeBilling() *fakeBilling {
	amount := 1900
	return &fakeBilling{price: BillingPrice{ID: "price_test", Active: true, Currency: "eur", UnitAmount: &amount, BillingScheme: "per_unit", TaxBehavior: "inclusive", Recurring: &RecurringPrice{Interval: "month", IntervalCount: 1, UsageType: "licensed"}}}
}
func (f *fakeBilling) Price(context.Context, string) (BillingPrice, error)    { return f.price, nil }
func (f *fakeBilling) CreateCustomer(context.Context, string) (string, error) { return "cus_test", nil }
func (f *fakeBilling) CreateCheckout(_ context.Context, customer, business, attempt, price, ret string, end time.Time) (BillingCheckout, error) {
	f.checkoutCalls++
	f.sub.ID = "sub_test"
	f.sub.Customer = customer
	f.sub.Status = "active"
	f.sub.Metadata = map[string]string{"stringer_id": business, "billing_attempt_id": attempt}
	raw := fmt.Sprintf(`{"items":{"data":[{"quantity":1,"current_period_end":%d,"price":{"id":"%s","currency":"eur","recurring":{"interval":"month","interval_count":1}}}]},"latest_invoice":{"paid":true},"pause_collection":null}`, time.Now().Add(30*24*time.Hour).Unix(), price)
	if err := json.Unmarshal([]byte(raw), &f.sub); err != nil {
		panic(err)
	}
	if f.failCheckout {
		return BillingCheckout{}, errors.New("ambiguous network failure")
	}
	return BillingCheckout{ID: "cs_test", URL: "https://checkout.stripe.com/c/pay/test", Status: "open"}, nil
}
func (f *fakeBilling) Checkout(context.Context, string) (BillingCheckout, error) {
	return BillingCheckout{ID: "cs_test", Status: f.sessionStatus}, nil
}
func (f *fakeBilling) Portal(context.Context, string, string) (string, error) {
	return "https://billing.stripe.com/p/session/test", nil
}
func (f *fakeBilling) Subscription(context.Context, string) (BillingSubscription, error) {
	return f.sub, nil
}
func signature(body []byte, now time.Time) string {
	stamp := fmt.Sprint(now.Unix())
	mac := hmac.New(sha256.New, []byte("whsec_test"))
	mac.Write([]byte(stamp + "."))
	mac.Write(body)
	return "t=" + stamp + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}
func billingEvent(f *fixture, p *fakeBilling, eventID, eventType string) *httptest.ResponseRecorder {
	raw := fmt.Sprintf(`{"id":"%s","type":"%s","livemode":false,"data":{"object":{"id":"sub_test","customer":"cus_test"}}}`, eventID, eventType)
	req := httptest.NewRequest("POST", "http://api.test/api/v1/billing/stripe/webhook", strings.NewReader(raw))
	req.Header.Set("Stripe-Signature", signature([]byte(raw), time.Now()))
	res := httptest.NewRecorder()
	Router(f.store, "http://localhost:5173").ServeHTTP(res, req)
	return res
}
func TestIntegrationSubscriptionVerificationExpiryAndOwnership(t *testing.T) {
	f := newFixture(t)
	p := newFakeBilling()
	f.store.Billing = &BillingService{Provider: p, PriceID: "price_test", WebhookSecret: "whsec_test", FrontendOrigin: "http://localhost:5173"}
	ctx := context.Background()
	if _, err := f.store.StartSubscription(ctx, f.customer, f.shop.ID); err == nil {
		t.Fatal("customer subscribed business")
	}
	if _, err := f.store.Pool.Exec(ctx, `INSERT INTO stringer_members(stringer_id,user_id,role) VALUES($1,$2,'staff')`, f.shop.ID, f.other.ID); err != nil {
		t.Fatal(err)
	}
	_, err := f.store.StartSubscription(ctx, f.other, f.shop.ID)
	wantProblem(t, err, "forbidden")
	url, err := f.store.StartSubscription(ctx, f.owner, f.shop.ID)
	if err != nil || url.URL == "" {
		t.Fatal(url, err)
	}
	if _, err = f.store.StartSubscription(ctx, f.owner, f.shop.ID); err != nil || p.checkoutCalls != 1 {
		t.Fatal("checkout retry duplicated", err, p.checkoutCalls)
	}
	u, _ := f.store.Usage(ctx, f.owner, f.shop.ID)
	if u.Plan != "free" {
		t.Fatal("checkout redirect activated paid access")
	}
	bad := httptest.NewRequest("POST", "http://api.test/api/v1/billing/stripe/webhook", strings.NewReader(`{}`))
	res := httptest.NewRecorder()
	Router(f.store, "http://localhost:5173").ServeHTTP(res, bad)
	wantStatus(t, res, 400)
	wantStatus(t, billingEvent(f, p, "evt_first", "customer.subscription.updated"), 200)
	wantStatus(t, billingEvent(f, p, "evt_first", "customer.subscription.updated"), 200)
	u, err = f.store.Usage(ctx, f.owner, f.shop.ID)
	if err != nil || u.Plan != "paid" || !u.CanAccept {
		t.Fatal(u, err)
	}
	expired := u.PaidUntil.Add(time.Second)
	u, err = businessUsage(ctx, f.store.Pool, f.shop.ID, expired)
	if err != nil || u.Plan != "free" {
		t.Fatal("stale entitlement", u, err)
	}
	p.sub.Status = "canceled"
	wantStatus(t, billingEvent(f, p, "evt_cancel", "customer.subscription.deleted"), 200)
	// An older delivery fetches current canceled state, so it cannot restore paid.
	wantStatus(t, billingEvent(f, p, "evt_delayed", "customer.subscription.created"), 200)
	u, _ = f.store.Usage(ctx, f.owner, f.shop.ID)
	if u.Plan != "free" {
		t.Fatal("out-of-order delivery restored paid")
	}
	p.sub.Status = "active"
	p.sub.Items.Data[0].Price.ID = "price_other"
	wantStatus(t, billingEvent(f, p, "evt_wrong_price", "customer.subscription.updated"), 200)
	u, _ = f.store.Usage(ctx, f.owner, f.shop.ID)
	if u.Plan != "free" {
		t.Fatal("wrong price granted entitlement")
	}
}
func TestIntegrationAmbiguousSubscriptionCheckout(t *testing.T) {
	f := newFixture(t)
	p := newFakeBilling()
	p.failCheckout = true
	f.store.Billing = &BillingService{Provider: p, PriceID: "price_test", FrontendOrigin: "http://localhost:5173"}
	ctx := context.Background()
	_, err := f.store.StartSubscription(ctx, f.owner, f.shop.ID)
	wantProblem(t, err, "billing_unavailable")
	var before, after string
	if err = f.store.Pool.QueryRow(ctx, `SELECT id::text FROM billing_checkout_attempts WHERE stringer_id=$1`, f.shop.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	p.failCheckout = false
	if _, err = f.store.StartSubscription(ctx, f.owner, f.shop.ID); err != nil {
		t.Fatal(err)
	}
	if err = f.store.Pool.QueryRow(ctx, `SELECT id::text FROM billing_checkout_attempts WHERE stringer_id=$1`, f.shop.ID).Scan(&after); err != nil || after != before {
		t.Fatal("retry changed checkout identity", err)
	}
}
func TestStripeSignature(t *testing.T) {
	now := time.Now()
	body := []byte(`{"id":"evt_test"}`)
	good := signature(body, now)
	if !verifyStripeSignature(body, good, "whsec_test", now) {
		t.Fatal("valid signature rejected")
	}
	for _, v := range []struct {
		body   []byte
		header string
	}{{[]byte(`{}`), good}, {body, signature(body, now.Add(-6*time.Minute))}, {body, signature(body, now.Add(6*time.Minute))}, {body, "t=bad,v1=bad"}} {
		if verifyStripeSignature(v.body, v.header, "whsec_test", now) {
			t.Fatal("invalid signature accepted")
		}
	}
}

func TestIntegrationBillingMigrationBackfillsAcceptedHistory(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	accepted := billingOrder(t, f, 1)
	placed := billingOrder(t, f, 2)
	if _, err := f.store.UpdateStatus(ctx, f.owner, accepted.ID, "accepted"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.UpdateStatus(ctx, f.owner, accepted.ID, "cancelled"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../migrations/00006_business_billing_and_cash.sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if _, err = f.store.Pool.Exec(ctx, parts[1]); err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.Pool.Exec(ctx, parts[0]); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = f.store.Pool.QueryRow(ctx, `SELECT count(*) FROM booking_usage WHERE order_id=$1`, accepted.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("cancelled acceptance missing from backfill", count, err)
	}
	if err = f.store.Pool.QueryRow(ctx, `SELECT count(*) FROM booking_usage WHERE order_id=$1`, placed.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("unaccepted booking counted", count, err)
	}
}

func TestIntegrationPaidAllowanceAndRenewalFailure(t *testing.T) {
	f := newFixture(t)
	p := newFakeBilling()
	f.store.Billing = &BillingService{Provider: p, PriceID: "price_test", WebhookSecret: "whsec_test", FrontendOrigin: "http://localhost:5173"}
	ctx := context.Background()
	orders := []Order{}
	for n := 0; n < 6; n++ {
		orders = append(orders, billingOrder(t, f, n))
	}
	if _, err := f.store.StartSubscription(ctx, f.owner, f.shop.ID); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, billingEvent(f, p, "evt_paid", "customer.subscription.updated"), 200)
	for _, o := range orders {
		if _, err := f.store.UpdateStatus(ctx, f.owner, o.ID, "accepted"); err != nil {
			t.Fatal("paid allowance blocked", err)
		}
	}
	u, _ := f.store.Usage(ctx, f.owner, f.shop.ID)
	if u.Plan != "paid" || u.AcceptedBookings != 6 || !u.CanAccept {
		t.Fatal(u)
	}
	p.sub.CancelAtPeriodEnd = true
	wantStatus(t, billingEvent(f, p, "evt_scheduled", "customer.subscription.updated"), 200)
	u, _ = f.store.Usage(ctx, f.owner, f.shop.ID)
	if u.Plan != "paid" || !u.CancelAtPeriodEnd {
		t.Fatal("scheduled cancellation revoked paid time", u)
	}
	// An unconfirmed renewal must not extend entitlement.
	p.sub.LatestInvoice.Paid = false
	p.sub.Status = "past_due"
	wantStatus(t, billingEvent(f, p, "evt_renewal_failed", "customer.subscription.updated"), 200)
	u, _ = f.store.Usage(ctx, f.owner, f.shop.ID)
	if u.Plan != "free" || u.CanAccept || u.AcceptedBookings != 6 {
		t.Fatal("renewal failure reset usage or kept paid", u)
	}
	if _, err := f.store.UpdateStatus(ctx, f.owner, orders[0].ID, "received"); err != nil {
		t.Fatal("downgrade blocked existing fulfilment", err)
	}
	if _, err := f.store.UpdateStatus(ctx, f.owner, orders[0].ID, "in_progress"); err != nil {
		t.Fatal(err)
	}
}
func TestIntegrationSubscriptionIdentityAndMode(t *testing.T) {
	f := newFixture(t)
	p := newFakeBilling()
	f.store.Billing = &BillingService{Provider: p, PriceID: "price_test", WebhookSecret: "whsec_test", FrontendOrigin: "http://localhost:5173"}
	ctx := context.Background()
	if _, err := f.store.StartSubscription(ctx, f.owner, f.shop.ID); err != nil {
		t.Fatal(err)
	}
	original := p.sub.Metadata["stringer_id"]
	p.sub.Metadata["stringer_id"] = "another-business"
	wantStatus(t, billingEvent(f, p, "evt_wrong_business", "customer.subscription.updated"), 400)
	p.sub.Metadata["stringer_id"] = original
	p.sub.Live = true
	wantStatus(t, billingEvent(f, p, "evt_wrong_mode", "customer.subscription.updated"), 400)
	p.sub.Live = false
	attempt := p.sub.Metadata["billing_attempt_id"]
	p.sub.Metadata["billing_attempt_id"] = "unknown"
	wantStatus(t, billingEvent(f, p, "evt_wrong_attempt", "customer.subscription.updated"), 400)
	p.sub.Metadata["billing_attempt_id"] = attempt
	p.sub.PauseCollection = json.RawMessage(`{"behavior":"void"}`)
	wantStatus(t, billingEvent(f, p, "evt_paused_collection", "customer.subscription.updated"), 200)
	u, _ := f.store.Usage(ctx, f.owner, f.shop.ID)
	if u.Plan != "free" {
		t.Fatal("paused collection granted paid")
	}
	p.price.TaxBehavior = "exclusive"
	u, err := f.store.Usage(ctx, f.owner, f.shop.ID)
	if err != nil || u.BillingAvailable {
		t.Fatal("invalid price advertised", u, err)
	}
	_, err = f.store.StartSubscription(ctx, f.owner, f.shop.ID)
	wantProblem(t, err, "billing_unavailable")
}
