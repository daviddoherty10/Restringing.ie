package marketplace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeRacketPayments struct {
	staleChargeOnce *RacketCharge
	mu              sync.Mutex
	account         PaymentAccount
	session         RacketCheckout
	charge          RacketCharge
	keys            []string
	fail            bool
}

func newPaymentProvider() *fakeRacketPayments {
	p := &fakeRacketPayments{}
	p.account.ID = "acct_stringer"
	p.account.Configuration.Merchant.Capabilities.CardPayments.Status = "active"
	return p
}
func (p *fakeRacketPayments) CreateAccount(context.Context, string, string) (PaymentAccount, error) {
	return p.account, nil
}
func (p *fakeRacketPayments) Account(context.Context, string) (PaymentAccount, error) {
	return p.account, nil
}
func (p *fakeRacketPayments) Onboard(context.Context, string, string) (string, error) {
	return "https://accounts.stripe.com/setup/test", nil
}
func (p *fakeRacketPayments) CreateCheckout(_ context.Context, _ string, o Order, key, _ string, _ time.Time) (RacketCheckout, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.keys = append(p.keys, key)
	p.session = RacketCheckout{ID: "cs_racket", URL: "https://checkout.stripe.com/c/pay/racket", Currency: "eur", Total: o.TotalCents, Status: "open", PaymentStatus: "unpaid", Metadata: map[string]string{"order_id": o.ID}}
	if p.fail {
		p.fail = false
		return RacketCheckout{}, errors.New("ambiguous network failure")
	}
	return p.session, nil
}
func (p *fakeRacketPayments) Checkout(context.Context, string, string) (RacketCheckout, error) {
	return p.session, nil
}
func (p *fakeRacketPayments) CheckoutForIntent(context.Context, string, string) (RacketCheckout, error) {
	return p.session, nil
}
func (p *fakeRacketPayments) Expire(context.Context, string, string) (RacketCheckout, error) {
	p.session.Status = "expired"
	return p.session, nil
}
func (p *fakeRacketPayments) Charge(context.Context, string, string) (RacketCharge, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.staleChargeOnce != nil {
		c := *p.staleChargeOnce
		p.staleChargeOnce = nil
		return c, nil
	}
	return p.charge, nil
}
func (p *fakeRacketPayments) Dispute(context.Context, string, string) (RacketDispute, error) {
	return RacketDispute{Charge: "ch_racket", Status: "needs_response"}, nil
}
func paymentFixture(t *testing.T) (*fixture, *fakeRacketPayments) {
	f := newFixture(t)
	p := newPaymentProvider()
	f.store.Payments = &RacketPayments{Provider: p, FrontendOrigin: "https://app.test", WebhookSecret: "whsec_test"}
	if _, e := f.store.Pool.Exec(context.Background(), `INSERT INTO stringer_payment_settings(stringer_id,account_id,account_live,account_ready,online_enabled) VALUES($1,'acct_stringer',false,true,true)`, f.shop.ID); e != nil {
		t.Fatal(e)
	}
	return f, p
}
func onlineOrder(t *testing.T, f *fixture) Order {
	b := f.booking
	b.PaymentMethod = "online"
	q, e := f.store.CreateQuote(context.Background(), f.customer, f.shop.ID, b)
	if e != nil {
		t.Fatal(e)
	}
	o, _, e := f.store.CreateOrder(context.Background(), f.customer, q.ID, "racket-payment-order-key-001")
	if e != nil {
		t.Fatal(e)
	}
	return o
}
func racketEvent(f *fixture, eventID, kind, account, object string) *httptest.ResponseRecorder {
	raw := fmt.Sprintf(`{"id":"%s","type":"%s","account":"%s","livemode":false,"data":{"object":{"id":"%s"}}}`, eventID, kind, account, object)
	req := httptest.NewRequest("POST", "http://api.test/api/v1/payments/stripe/webhook", strings.NewReader(raw))
	req.Header.Set("Stripe-Signature", signature([]byte(raw), time.Now()))
	res := httptest.NewRecorder()
	Router(f.store, "http://localhost:5173").ServeHTTP(res, req)
	return res
}
func TestIntegrationPaymentSettingsPermissionsAndOnsiteReceipt(t *testing.T) {
	f, p := paymentFixture(t)
	_ = p
	ctx := context.Background()
	_, e := f.store.SavePaymentSettings(ctx, f.customer, f.shop.ID, PaymentSettingsInput{OnsiteCard: true})
	wantProblem(t, e, "forbidden")
	if _, e = f.store.SavePaymentSettings(ctx, f.owner, f.shop.ID, PaymentSettingsInput{OnsiteCard: true}); e != nil {
		t.Fatal(e)
	}
	legacy := f.booking
	if _, e = f.store.CreateQuote(ctx, f.customer, f.shop.ID, legacy); e == nil {
		t.Fatal("cash accepted by a card-only stringer")
	}
	b := f.booking
	b.PaymentMethod = "onsite"
	q, e := f.store.CreateQuote(ctx, f.customer, f.shop.ID, b)
	if e != nil {
		t.Fatal(e)
	}
	if !q.Configuration.OnsiteCard || q.Configuration.OnsiteCash {
		t.Fatal("wrong tender snapshot")
	}
	o, _, e := f.store.CreateOrder(ctx, f.customer, q.ID, "onsite-payment-order-key-001")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.store.ConfirmOnsite(ctx, f.owner, o.ID, "card"); e == nil {
		t.Fatal("receipt before acceptance")
	}
	if _, e = f.store.UpdateStatus(ctx, f.owner, o.ID, "accepted"); e != nil {
		t.Fatal(e)
	}
	if _, e = f.store.ConfirmOnsite(ctx, f.customer, o.ID, "card"); e == nil {
		t.Fatal("customer marked self paid")
	}
	if _, e = f.store.ConfirmOnsite(ctx, f.owner, o.ID, "cash"); e == nil {
		t.Fatal("unsupported tender accepted")
	}
	for i := 0; i < 2; i++ {
		if _, e = f.store.ConfirmOnsite(ctx, f.owner, o.ID, "card"); e != nil {
			t.Fatal(e)
		}
	}
	saved, e := f.store.GetOrder(ctx, f.customer, o.ID)
	if e != nil || saved.PaymentMethod != "onsite" || saved.PaymentStatus != "paid" || saved.Status != "accepted" || len(saved.PaymentEvents) != 1 {
		t.Fatal("receipt not idempotent or changed fulfilment", saved, e)
	}
	if _, e = f.store.SavePaymentSettings(ctx, f.owner, f.shop.ID, PaymentSettingsInput{}); e == nil {
		t.Fatal("all payment options disabled")
	}
}
func TestIntegrationOnlineCheckoutRetryWebhooksRefundsAndDisputes(t *testing.T) {
	f, p := paymentFixture(t)
	ctx := context.Background()
	o := onlineOrder(t, f)
	if _, e := f.store.StartOrderPayment(ctx, f.other, o.ID); e == nil {
		t.Fatal("foreign checkout")
	}
	if _, e := f.store.ConfirmCash(ctx, f.owner, o.ID); e == nil {
		t.Fatal("online payment marked manually")
	}
	p.fail = true
	if _, e := f.store.StartOrderPayment(ctx, f.customer, o.ID); e == nil {
		t.Fatal("ambiguous failure lost")
	}
	if _, e := f.store.StartOrderPayment(ctx, f.customer, o.ID); e != nil {
		t.Fatal(e)
	}
	if len(p.keys) != 2 || p.keys[0] != p.keys[1] {
		t.Fatal("checkout retry changed idempotency key", p.keys)
	}
	saved, e := f.store.GetOrder(ctx, f.customer, o.ID)
	if e != nil || saved.PaymentStatus != "unpaid" {
		t.Fatal("browser created paid status")
	}
	if _, e = f.store.UpdateStatus(ctx, f.owner, o.ID, "accepted"); e != nil {
		t.Fatal(e)
	}
	if _, e = f.store.UpdateStatus(ctx, f.owner, o.ID, "received"); e != nil {
		t.Fatal(e)
	}
	if _, e = f.store.UpdateStatus(ctx, f.owner, o.ID, "in_progress"); e == nil {
		t.Fatal("started unpaid online order")
	}
	p.session.Status = "complete"
	p.session.PaymentIntent = "pi_racket"
	wantStatus(t, racketEvent(f, "evt_unpaid", "checkout.session.completed", "acct_stringer", "cs_racket"), 200)
	saved, _ = f.store.GetOrder(ctx, f.customer, o.ID)
	if saved.PaymentStatus != "unpaid" {
		t.Fatal("async unpaid session marked paid")
	}
	p.session.PaymentStatus = "paid"
	p.session.Total++
	wantStatus(t, racketEvent(f, "evt_wrong", "checkout.session.async_payment_succeeded", "acct_stringer", "cs_racket"), 503)
	p.session.Total--
	for i := 0; i < 2; i++ {
		wantStatus(t, racketEvent(f, "evt_paid", "checkout.session.async_payment_succeeded", "acct_stringer", "cs_racket"), 200)
	}
	statuses := make(chan int, 16)
	for i := 0; i < 16; i++ {
		go func(n int) {
			statuses <- racketEvent(f, fmt.Sprintf("evt_parallel_%d", n), "checkout.session.completed", "acct_stringer", "cs_racket").Code
		}(i)
	}
	for i := 0; i < 16; i++ {
		if status := <-statuses; status != 200 {
			t.Fatalf("concurrent webhook failed: %d", status)
		}
	}
	saved, _ = f.store.GetOrder(ctx, f.customer, o.ID)
	if saved.PaymentStatus != "paid" || len(saved.PaymentEvents) != 1 {
		t.Fatal("duplicate payment event", saved)
	}
	if _, e = f.store.UpdateStatus(ctx, f.owner, o.ID, "in_progress"); e != nil {
		t.Fatal(e)
	}
	p.charge = RacketCharge{PaymentIntent: "pi_racket", Amount: o.TotalCents, Refunded: o.TotalCents, Currency: "eur", Metadata: map[string]string{"order_id": o.ID}}
	wantStatus(t, racketEvent(f, "evt_refund", "charge.refunded", "acct_stringer", "ch_racket"), 200)
	wantStatus(t, racketEvent(f, "evt_late_paid", "checkout.session.completed", "acct_stringer", "cs_racket"), 200)
	saved, _ = f.store.GetOrder(ctx, f.customer, o.ID)
	if saved.PaymentStatus != "refunded" {
		t.Fatal("late paid event undid refund")
	}
	wantStatus(t, racketEvent(f, "evt_dispute", "charge.dispute.created", "acct_stringer", "dp_racket"), 200)
	saved, _ = f.store.GetOrder(ctx, f.customer, o.ID)
	if !strings.Contains(saved.PaymentEvents[len(saved.PaymentEvents)-1].Reason, "dispute") {
		t.Fatal("dispute not recorded")
	}
}
func TestIntegrationCancelOnlineCheckoutExpiresBeforeReleasingStock(t *testing.T) {
	f, p := paymentFixture(t)
	ctx := context.Background()
	o := onlineOrder(t, f)
	if _, e := f.store.StartOrderPayment(ctx, f.customer, o.ID); e != nil {
		t.Fatal(e)
	}
	p.session.Status = "complete"
	if _, e := f.store.UpdateStatus(ctx, f.customer, o.ID, "cancelled"); e == nil {
		t.Fatal("cancelled processing payment")
	}
	p.session.Status = "open"
	if _, e := f.store.UpdateStatus(ctx, f.customer, o.ID, "cancelled"); e != nil {
		t.Fatal(e)
	}
	if p.session.Status != "expired" {
		t.Fatal("checkout not expired")
	}
}
func TestRacketProviderUsesAccountsV2AndScopedCheckout(t *testing.T) {
	calls := 0
	p := &StripeRacketPayments{Key: "rk_test_fixture", Client: &http.Client{Transport: billingTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Stripe-Version") != "2026-10-28.endive" {
			t.Fatal("unpinned API")
		}
		raw, _ := io.ReadAll(r.Body)
		response := `{"id":"acct_fixture","livemode":false}`
		if strings.Contains(r.URL.Path, "/v2/core/accounts") {
			var body map[string]any
			if json.Unmarshal(raw, &body) != nil || body["dashboard"] != "full" || body["type"] != nil {
				t.Fatal("invalid v2 account config", string(raw))
			}
		} else {
			if r.Header.Get("Stripe-Account") != "acct_fixture" {
				t.Fatal("unscoped checkout")
			}
			body := string(raw)
			if strings.Contains(body, "payment_method_types") || strings.Contains(body, "application_fee_amount") || !strings.Contains(body, "unit_amount%5D=2599") {
				t.Fatal("unsafe checkout", body)
			}
			response = `{"id":"cs_fixture"}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: http.Header{}}, nil
	})}}
	if _, e := p.CreateAccount(context.Background(), "business", "Name"); e != nil {
		t.Fatal(e)
	}
	if _, e := p.CreateCheckout(context.Background(), "acct_fixture", Order{ID: "order", TotalCents: 2599}, "attempt", "https://app.test", time.Now().Add(time.Hour)); e != nil {
		t.Fatal(e)
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}

func TestIntegrationPaymentWebhookRejectsUnsignedAndForeignAccountEvents(t *testing.T) {
	f, p := paymentFixture(t)
	ctx := context.Background()
	o := onlineOrder(t, f)
	if _, e := f.store.StartOrderPayment(ctx, f.customer, o.ID); e != nil {
		t.Fatal(e)
	}
	p.session.Status = "complete"
	p.session.PaymentStatus = "paid"
	p.session.PaymentIntent = "pi_racket"
	req := httptest.NewRequest("POST", "http://api.test/api/v1/payments/stripe/webhook", strings.NewReader(`{"id":"evt_forged"}`))
	res := httptest.NewRecorder()
	Router(f.store, "http://localhost:5173").ServeHTTP(res, req)
	wantStatus(t, res, 400)
	wantStatus(t, racketEvent(f, "evt_foreign", "checkout.session.completed", "acct_other", "cs_racket"), 200)
	saved, e := f.store.GetOrder(ctx, f.customer, o.ID)
	if e != nil || saved.PaymentStatus != "unpaid" {
		t.Fatal("foreign event changed payment", saved, e)
	}
	wantStatus(t, racketEvent(f, "evt_real", "checkout.session.completed", "acct_stringer", "cs_racket"), 200)
	for _, status := range []string{"accepted", "received"} {
		if _, e = f.store.UpdateStatus(ctx, f.owner, o.ID, status); e != nil {
			t.Fatal(e)
		}
	}
	p.charge = RacketCharge{PaymentIntent: "pi_racket", Amount: o.TotalCents, Currency: "eur", Metadata: map[string]string{"order_id": o.ID}}
	wantStatus(t, racketEvent(f, "evt_disputed", "charge.dispute.created", "acct_stringer", "dp_racket"), 200)
	saved, e = f.store.GetOrder(ctx, f.customer, o.ID)
	if e != nil || !saved.PaymentReviewRequired {
		t.Fatal("missing dispute guard", saved, e)
	}
	if _, e = f.store.UpdateStatus(ctx, f.owner, o.ID, "in_progress"); e == nil {
		t.Fatal("disputed payment started work")
	}
	// Owner refresh and thin account events use current capability state.
	p.account.Configuration.Merchant.Capabilities.CardPayments.Status = "inactive"
	raw := `{"id":"evt_account","type":"v2.core.account[configuration.merchant].updated","livemode":false,"related_object":{"id":"acct_stringer"}}`
	req = httptest.NewRequest("POST", "http://api.test/api/v1/payments/stripe/webhook", strings.NewReader(raw))
	req.Header.Set("Stripe-Signature", signature([]byte(raw), time.Now()))
	res = httptest.NewRecorder()
	Router(f.store, "http://localhost:5173").ServeHTTP(res, req)
	wantStatus(t, res, 200)
	options, e := f.store.paymentOptions(ctx, f.store.Pool, f.shop.ID)
	if e != nil || options.OnlineAvailable {
		t.Fatal("revoked capability still advertised", options, e)
	}
}

func TestIntegrationRefundBeforeCheckoutEventDoesNotRegress(t *testing.T) {
	f, p := paymentFixture(t)
	ctx := context.Background()
	o := onlineOrder(t, f)
	if _, e := f.store.StartOrderPayment(ctx, f.customer, o.ID); e != nil {
		t.Fatal(e)
	}
	p.session.Status = "complete"
	p.session.PaymentStatus = "paid"
	p.session.PaymentIntent = "pi_racket"
	p.charge = RacketCharge{PaymentIntent: "pi_racket", Amount: o.TotalCents, Refunded: o.TotalCents, Currency: "eur"}
	stale := p.charge
	stale.Refunded = 1
	p.staleChargeOnce = &stale
	wantStatus(t, racketEvent(f, "evt_refund_first", "charge.refunded", "acct_stringer", "ch_racket"), 200)
	wantStatus(t, racketEvent(f, "evt_paid_later", "checkout.session.completed", "acct_stringer", "cs_racket"), 200)
	saved, e := f.store.GetOrder(ctx, f.customer, o.ID)
	if e != nil || saved.PaymentStatus != "refunded" {
		t.Fatal("refund-first event lost", saved, e)
	}
}
