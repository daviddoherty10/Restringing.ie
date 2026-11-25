package marketplace

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type PaymentAccount struct {
	ID            string `json:"id"`
	Live          bool   `json:"livemode"`
	Configuration struct {
		Merchant struct {
			Capabilities struct {
				CardPayments struct {
					Status string `json:"status"`
				} `json:"card_payments"`
			} `json:"capabilities"`
		} `json:"merchant"`
	} `json:"configuration"`
}

func (a PaymentAccount) Ready() bool {
	return a.Configuration.Merchant.Capabilities.CardPayments.Status == "active"
}

type RacketCheckout struct {
	ID            string            `json:"id"`
	URL           string            `json:"url"`
	Status        string            `json:"status"`
	PaymentStatus string            `json:"payment_status"`
	Currency      string            `json:"currency"`
	Total         int               `json:"amount_total"`
	Live          bool              `json:"livemode"`
	PaymentIntent string            `json:"payment_intent"`
	Metadata      map[string]string `json:"metadata"`
}
type RacketDispute struct {
	Charge string `json:"charge"`
	Status string `json:"status"`
}
type RacketCharge struct {
	Metadata      map[string]string `json:"metadata"`
	PaymentIntent string            `json:"payment_intent"`
	Amount        int               `json:"amount"`
	Refunded      int               `json:"amount_refunded"`
	Currency      string            `json:"currency"`
	Live          bool              `json:"livemode"`
	Disputed      bool              `json:"disputed"`
}
type RacketPaymentProvider interface {
	CreateAccount(context.Context, string, string) (PaymentAccount, error)
	Account(context.Context, string) (PaymentAccount, error)
	Onboard(context.Context, string, string) (string, error)
	CreateCheckout(context.Context, string, Order, string, string, time.Time) (RacketCheckout, error)
	Checkout(context.Context, string, string) (RacketCheckout, error)
	CheckoutForIntent(context.Context, string, string) (RacketCheckout, error)
	Expire(context.Context, string, string) (RacketCheckout, error)
	Charge(context.Context, string, string) (RacketCharge, error)
	Dispute(context.Context, string, string) (RacketDispute, error)
}

// A provider instance keeps credentials private and all requests on Stripe's fixed origin.
type StripeRacketPayments struct {
	Key    string
	Client *http.Client
}

func (p *StripeRacketPayments) call(ctx context.Context, method, path, account, key string, body io.Reader, jsonBody bool, out any) error {
	req, e := http.NewRequestWithContext(ctx, method, "https://api.stripe.com/"+path, body)
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+p.Key)
	req.Header.Set("Stripe-Version", "2026-10-28.endive")
	if account != "" {
		req.Header.Set("Stripe-Account", account)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	if method == "POST" {
		if jsonBody {
			req.Header.Set("Content-Type", "application/json")
		} else {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	res, e := client.Do(req)
	if e != nil {
		return errors.New("payment provider unavailable")
	}
	defer res.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if e != nil || len(raw) > 1<<20 || res.StatusCode < 200 || res.StatusCode >= 300 {
		return errors.New("payment provider unavailable")
	}
	if json.Unmarshal(raw, out) != nil {
		return errors.New("invalid payment provider response")
	}
	return nil
}
func (p *StripeRacketPayments) CreateAccount(ctx context.Context, id, name string) (PaymentAccount, error) {
	body, _ := json.Marshal(map[string]any{"dashboard": "full", "identity": map[string]string{"country": "ie"}, "defaults": map[string]any{"currency": "eur", "responsibilities": map[string]string{"fees_collector": "stripe", "losses_collector": "stripe"}}, "configuration": map[string]any{"merchant": map[string]any{"capabilities": map[string]any{"card_payments": map[string]bool{"requested": true}}}}, "metadata": map[string]string{"stringer_id": id}, "include": []string{"configuration.merchant"}})
	var a PaymentAccount
	e := p.call(ctx, "POST", "v2/core/accounts", "", "racket-account:"+id, strings.NewReader(string(body)), true, &a)
	return a, e
}
func (p *StripeRacketPayments) Account(ctx context.Context, id string) (PaymentAccount, error) {
	var a PaymentAccount
	e := p.call(ctx, "GET", "v2/core/accounts/"+url.PathEscape(id)+"?include[0]=configuration.merchant", "", "", nil, false, &a)
	return a, e
}
func (p *StripeRacketPayments) Onboard(ctx context.Context, id, back string) (string, error) {
	body, _ := json.Marshal(map[string]any{"account": id, "use_case": map[string]any{"type": "account_onboarding", "account_onboarding": map[string]any{"return_url": back, "refresh_url": back, "collection_options": map[string]string{"fields": "eventually_due"}}}})
	var out struct {
		URL string `json:"url"`
	}
	e := p.call(ctx, "POST", "v2/core/account_links", "", "", strings.NewReader(string(body)), true, &out)
	return out.URL, e
}
func (p *StripeRacketPayments) CreateCheckout(ctx context.Context, account string, o Order, key, origin string, expires time.Time) (RacketCheckout, error) {
	hash := sha256.Sum256([]byte(key))
	suffix := make([]byte, 8)
	for i := range suffix {
		suffix[i] = 'a' + hash[i]%26
	}
	f := url.Values{"mode": {"payment"}, "success_url": {origin + "/orders/" + o.ID + "?payment=return"}, "cancel_url": {origin + "/orders/" + o.ID}, "expires_at": {strconv.FormatInt(expires.Unix(), 10)}, "client_reference_id": {o.ID}, "metadata[order_id]": {o.ID}, "payment_intent_data[metadata][order_id]": {o.ID}, "line_items[0][quantity]": {"1"}, "line_items[0][price_data][currency]": {"eur"}, "line_items[0][price_data][unit_amount]": {strconv.Itoa(o.TotalCents)}, "line_items[0][price_data][product_data][name]": {"Racket restringing"}, "integration_identifier": {"restringing-racket-" + string(suffix)}}
	var out RacketCheckout
	e := p.call(ctx, "POST", "v1/checkout/sessions", account, key, strings.NewReader(f.Encode()), false, &out)
	return out, e
}
func (p *StripeRacketPayments) Checkout(ctx context.Context, account, id string) (RacketCheckout, error) {
	var out RacketCheckout
	e := p.call(ctx, "GET", "v1/checkout/sessions/"+url.PathEscape(id), account, "", nil, false, &out)
	return out, e
}
func (p *StripeRacketPayments) Expire(ctx context.Context, account, id string) (RacketCheckout, error) {
	var out RacketCheckout
	e := p.call(ctx, "POST", "v1/checkout/sessions/"+url.PathEscape(id)+"/expire", account, "expire:"+id, strings.NewReader(""), false, &out)
	return out, e
}
func (p *StripeRacketPayments) Charge(ctx context.Context, account, id string) (RacketCharge, error) {
	var out RacketCharge
	e := p.call(ctx, "GET", "v1/charges/"+url.PathEscape(id), account, "", nil, false, &out)
	return out, e
}

func (p *StripeRacketPayments) Dispute(ctx context.Context, account, id string) (RacketDispute, error) {
	var out RacketDispute
	e := p.call(ctx, "GET", "v1/disputes/"+url.PathEscape(id), account, "", nil, false, &out)
	return out, e
}

func (p *StripeRacketPayments) CheckoutForIntent(ctx context.Context, account, intent string) (RacketCheckout, error) {
	var out struct {
		Data []RacketCheckout `json:"data"`
	}
	e := p.call(ctx, "GET", "v1/checkout/sessions?payment_intent="+url.QueryEscape(intent)+"&limit=1", account, "", nil, false, &out)
	if e != nil {
		return RacketCheckout{}, e
	}
	if len(out.Data) == 0 {
		return RacketCheckout{}, nil
	}
	return out.Data[0], nil
}
