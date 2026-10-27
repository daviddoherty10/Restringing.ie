package marketplace

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type billingTransport func(*http.Request) (*http.Response, error)

func (f billingTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestStripeWireContract(t *testing.T) {
	ctx := context.Background()
	seen := map[string]int{}
	attempt := "90000000-0000-4000-8000-000000000001"
	expires := time.Now().Add(time.Hour).Truncate(time.Second)
	p := &StripeBilling{SecretKey: "sk_test_notreal", Client: &http.Client{Transport: billingTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme != "https" || r.URL.Host != "api.stripe.com" || r.Header.Get("Stripe-Version") != "2025-03-31.basil" {
			t.Fatal("unbounded provider origin or version")
		}
		username, password, ok := r.BasicAuth()
		if !ok || username != "sk_test_notreal" || password != "" {
			t.Fatal("provider auth")
		}
		seen[r.URL.Path]++
		body := `{}`
		switch r.URL.Path {
		case "/v1/prices/price_test":
			body = `{"id":"price_test","active":true,"currency":"eur","unit_amount":1900,"billing_scheme":"per_unit","tax_behavior":"inclusive","recurring":{"interval":"month","interval_count":1,"usage_type":"licensed"}}`
		case "/v1/customers":
			if r.Header.Get("Idempotency-Key") != "restringing-customer:business" {
				t.Fatal("customer retry key")
			}
			body = `{"id":"cus_test"}`
		case "/v1/checkout/sessions":
			if r.Header.Get("Idempotency-Key") != "restringing-subscription:"+attempt {
				t.Fatal("checkout retry key")
			}
			raw, _ := io.ReadAll(r.Body)
			form, e := url.ParseQuery(string(raw))
			if e != nil {
				t.Fatal(e)
			}
			for k, v := range map[string]string{"mode": "subscription", "customer": "cus_test", "line_items[0][price]": "price_test", "line_items[0][quantity]": "1", "subscription_data[metadata][stringer_id]": "business", "subscription_data[metadata][billing_attempt_id]": attempt, "expires_at": fmt.Sprint(expires.Unix()), "success_url": "https://app.example/manage", "cancel_url": "https://app.example/manage"} {
				if form.Get(k) != v {
					t.Fatalf("incorrect %s", k)
				}
			}
			body = `{"id":"cs_test_123","url":"https://checkout.stripe.com/c/pay/test"}`
		case "/v1/billing_portal/sessions":
			body = `{"url":"https://billing.stripe.com/p/session/test"}`
		case "/v1/subscriptions/sub_test":
			if r.URL.Query().Get("expand[]") != "latest_invoice" {
				t.Fatal("latest invoice not expanded")
			}
			body = `{"id":"sub_test","customer":"cus_test","status":"active"}`
		default:
			t.Fatalf("unexpected provider path %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}}
	price, err := p.Price(ctx, "price_test")
	if err != nil || !validPrice(price, "price_test", false) {
		t.Fatal(price, err)
	}
	if _, err = p.CreateCustomer(ctx, "business"); err != nil {
		t.Fatal(err)
	}
	if _, err = p.CreateCheckout(ctx, "cus_test", "business", attempt, "price_test", "https://app.example/manage", expires); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Portal(ctx, "cus_test", "https://app.example/manage"); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Subscription(ctx, "sub_test"); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 5 {
		t.Fatal("missing provider requests")
	}
	for _, mutate := range []func(*BillingPrice){func(v *BillingPrice) { v.Currency = "usd" }, func(v *BillingPrice) { v.Live = true }, func(v *BillingPrice) { v.TaxBehavior = "exclusive" }, func(v *BillingPrice) { v.Active = false }, func(v *BillingPrice) {
		v.Recurring = &RecurringPrice{Interval: "year", IntervalCount: 1, UsageType: "licensed"}
	}} {
		v := price
		mutate(&v)
		if validPrice(v, "price_test", false) {
			t.Fatal("unsupported price accepted")
		}
	}
}
func TestStripeFailureAndRedirectBounds(t *testing.T) {
	for _, c := range []struct {
		status int
		body   string
	}{{302, `{}`}, {401, `{"error":{"message":"private provider details"}}`}, {200, `not json`}, {200, `{"id":"cs_test_123","url":"https://evil.example/pay"}`}} {
		p := &StripeBilling{Client: &http.Client{Transport: billingTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: c.status, Body: io.NopCloser(strings.NewReader(c.body)), Header: http.Header{}}, nil
		})}}
		_, err := p.CreateCheckout(context.Background(), "cus_test", "business", "attempt", "price_test", "https://app.example", time.Now().Add(time.Hour))
		if err == nil || strings.Contains(err.Error(), "private provider details") {
			t.Fatal("invalid or private provider response escaped", err)
		}
	}
}
