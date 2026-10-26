package marketplace

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type RecurringPrice struct {
	Interval      string `json:"interval"`
	IntervalCount int    `json:"interval_count"`
	UsageType     string `json:"usage_type"`
}
type BillingPrice struct {
	ID            string          `json:"id"`
	Active        bool            `json:"active"`
	Live          bool            `json:"livemode"`
	Currency      string          `json:"currency"`
	UnitAmount    *int            `json:"unit_amount"`
	BillingScheme string          `json:"billing_scheme"`
	TaxBehavior   string          `json:"tax_behavior"`
	Recurring     *RecurringPrice `json:"recurring"`
	Transform     json.RawMessage `json:"transform_quantity"`
	CustomAmount  json.RawMessage `json:"custom_unit_amount"`
}
type BillingCheckout struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Status string `json:"status"`
}
type BillingSubscription struct {
	ID                string            `json:"id"`
	Customer          string            `json:"customer"`
	Status            string            `json:"status"`
	Live              bool              `json:"livemode"`
	CancelAtPeriodEnd bool              `json:"cancel_at_period_end"`
	Metadata          map[string]string `json:"metadata"`
	PauseCollection   json.RawMessage   `json:"pause_collection"`
	Items             struct {
		Data []struct {
			Quantity  int          `json:"quantity"`
			PeriodEnd int64        `json:"current_period_end"`
			Price     BillingPrice `json:"price"`
		} `json:"data"`
	} `json:"items"`
	LatestInvoice *struct {
		Paid bool `json:"paid"`
	} `json:"latest_invoice"`
}
type BillingProvider interface {
	Price(context.Context, string) (BillingPrice, error)
	CreateCustomer(context.Context, string) (string, error)
	CreateCheckout(context.Context, string, string, string, string, string, time.Time) (BillingCheckout, error)
	Checkout(context.Context, string) (BillingCheckout, error)
	Portal(context.Context, string, string) (string, error)
	Subscription(context.Context, string) (BillingSubscription, error)
}

func jsonEmpty(v json.RawMessage) bool { return len(v) == 0 || string(v) == "null" }
func validPrice(p BillingPrice, id string, live bool) bool {
	return p.ID == id && p.Active && p.Live == live && p.Currency == "eur" && p.UnitAmount != nil && *p.UnitAmount > 0 && *p.UnitAmount <= 1000000 && p.BillingScheme == "per_unit" && p.TaxBehavior == "inclusive" && p.Recurring != nil && p.Recurring.Interval == "month" && p.Recurring.IntervalCount == 1 && p.Recurring.UsageType == "licensed" && jsonEmpty(p.Transform) && jsonEmpty(p.CustomAmount)
}

// Stripe calls use one pinned API version, fixed HTTPS origin and bounded I/O.
// Credentials and raw provider errors are never returned to callers or logged.
type StripeBilling struct {
	SecretKey string
	Client    *http.Client
}

var providerID = regexp.MustCompile(`^[A-Za-z]+_[A-Za-z0-9_]+$`)

func (p *StripeBilling) call(ctx context.Context, method, path, key string, form url.Values, out any) error {
	var body io.Reader
	if method == http.MethodPost {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://api.stripe.com/v1/"+path, body)
	if err != nil {
		return err
	}
	req.SetBasicAuth(p.SecretKey, "")
	req.Header.Set("Stripe-Version", "2025-03-31.basil")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 4 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	res, err := client.Do(req)
	if err != nil {
		return errors.New("billing provider unavailable")
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 || res.StatusCode < 200 || res.StatusCode >= 300 {
		return errors.New("billing provider unavailable")
	}
	if json.Unmarshal(raw, out) != nil {
		return errors.New("invalid billing response")
	}
	return nil
}
func (p *StripeBilling) Price(ctx context.Context, id string) (BillingPrice, error) {
	var v BillingPrice
	if !providerID.MatchString(id) {
		return v, errors.New("invalid price")
	}
	err := p.call(ctx, "GET", "prices/"+id, "", nil, &v)
	return v, err
}
func (p *StripeBilling) CreateCustomer(ctx context.Context, id string) (string, error) {
	var v struct {
		ID string `json:"id"`
	}
	err := p.call(ctx, "POST", "customers", "restringing-customer:"+id, url.Values{"metadata[stringer_id]": {id}}, &v)
	if err == nil && (!strings.HasPrefix(v.ID, "cus_") || !providerID.MatchString(v.ID)) {
		err = errors.New("invalid customer response")
	}
	return v.ID, err
}
func (p *StripeBilling) CreateCheckout(ctx context.Context, customer, business, attempt, price, returnURL string, expires time.Time) (BillingCheckout, error) {
	var v BillingCheckout
	form := url.Values{"mode": {"subscription"}, "customer": {customer}, "client_reference_id": {business}, "line_items[0][price]": {price}, "line_items[0][quantity]": {"1"}, "success_url": {returnURL}, "cancel_url": {returnURL}, "expires_at": {strconv.FormatInt(expires.Unix(), 10)}, "subscription_data[metadata][stringer_id]": {business}, "subscription_data[metadata][billing_attempt_id]": {attempt}, "metadata[stringer_id]": {business}}
	err := p.call(ctx, "POST", "checkout/sessions", "restringing-subscription:"+attempt, form, &v)
	if err == nil && (!providerID.MatchString(v.ID) || !stripeURL(v.URL, "checkout.stripe.com")) {
		err = errors.New("invalid checkout response")
	}
	return v, err
}
func (p *StripeBilling) Checkout(ctx context.Context, id string) (BillingCheckout, error) {
	var v BillingCheckout
	if !providerID.MatchString(id) {
		return v, errors.New("invalid checkout")
	}
	err := p.call(ctx, "GET", "checkout/sessions/"+id, "", nil, &v)
	return v, err
}
func (p *StripeBilling) Portal(ctx context.Context, customer, returnURL string) (string, error) {
	var v struct {
		URL string `json:"url"`
	}
	err := p.call(ctx, "POST", "billing_portal/sessions", "", url.Values{"customer": {customer}, "return_url": {returnURL}}, &v)
	if err == nil && !stripeURL(v.URL, "billing.stripe.com") {
		err = errors.New("invalid portal response")
	}
	return v.URL, err
}
func (p *StripeBilling) Subscription(ctx context.Context, id string) (BillingSubscription, error) {
	var v BillingSubscription
	if !providerID.MatchString(id) {
		return v, errors.New("invalid subscription")
	}
	err := p.call(ctx, "GET", "subscriptions/"+id+"?expand%5B%5D=latest_invoice", "", nil, &v)
	if err == nil && v.ID != id {
		err = errors.New("subscription mismatch")
	}
	return v, err
}
func stripeURL(raw, host string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && u.Host == host && u.User == nil
}
func verifyStripeSignature(body []byte, header, secret string, now time.Time) bool {
	var timestamp string
	signatures := []string{}
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		if kv[0] == "t" {
			if timestamp != "" {
				return false
			}
			timestamp = kv[1]
		}
		if kv[0] == "v1" {
			signatures = append(signatures, kv[1])
		}
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || secret == "" || seconds < now.Unix()-300 || seconds > now.Unix()+300 {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	expected := mac.Sum(nil)
	for _, value := range signatures {
		actual, e := hex.DecodeString(value)
		if e == nil && hmac.Equal(actual, expected) {
			return true
		}
	}
	return false
}
