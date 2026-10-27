package marketplace

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestExpressPricingIsExplicitAndSnapshotTermsAreImmutable(t *testing.T) {
	b, s, r, v := pricingFixture()
	s.Express = true
	s.ExpressFeeCents = 799
	s.ExpressTurnaround = "Within 24 hours of receipt, after acceptance."
	standard, e := Price(b, s, r, &v)
	if e != nil {
		t.Fatal(e)
	}
	if standard.TotalCents != 2900 || standard.Configuration.ExpressTurnaroundSnapshot != "" {
		t.Fatalf("standard charged express: %+v", standard)
	}
	b.Express = true
	express, e := Price(b, s, r, &v)
	if e != nil {
		t.Fatal(e)
	}
	if express.TotalCents != 3699 || express.Configuration.ExpressTurnaroundSnapshot != s.ExpressTurnaround {
		t.Fatalf("express: %+v", express)
	}
	last := express.Breakdown[len(express.Breakdown)-1]
	if last.Code != "express" || last.AmountCents != 799 {
		t.Fatal("express fee not itemized")
	}
	original := express.Configuration.ExpressTurnaroundSnapshot
	s.ExpressTurnaround = "Within 48 hours of receipt."
	changed, _ := Price(b, s, r, &v)
	if changed.Fingerprint == express.Fingerprint || express.Configuration.ExpressTurnaroundSnapshot != original {
		t.Fatal("changed terms did not invalidate quote or rewrote snapshot")
	}
	s.ExpressFeeCents = 0
	free, e := Price(b, s, r, &v)
	if e != nil || free.TotalCents != 2900 || !free.Configuration.Express || free.Breakdown[len(free.Breakdown)-1].Code != "express" {
		t.Fatalf("free express lost: %+v %v", free, e)
	}
	b.StringSource = "customer"
	b.VariantID = ""
	own, e := Price(b, s, r, nil)
	if e != nil || !own.Configuration.Express {
		t.Fatalf("own strings: %+v %v", own, e)
	}
	s.Express = false
	_, e = Price(b, s, r, nil)
	var p *Problem
	if !errors.As(e, &p) || p.Fields["express"] == "" {
		t.Fatalf("unsupported express accepted: %v", e)
	}
}
func TestExpressServiceValidation(t *testing.T) {
	_, base, _, _ := pricingFixture()
	for _, tc := range []struct {
		name        string
		fee         int
		description string
		enabled     bool
		valid       bool
	}{
		{"defaults", 0, "", false, true}, {"normal", 999, "Within 24 hours of receipt.", true, true}, {"free", 0, "Same day after acceptance.", true, true},
		{"negative", -1, "Within a day.", true, false}, {"too expensive", 1000001, "Within a day.", true, false}, {"blank", 100, "  ", true, false}, {"long", 100, strings.Repeat("é", 161), true, false}, {"control", 100, "bad\x00text", true, false}, {"multiline", 100, "first\nsecond", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := base
			s.Express = tc.enabled
			s.ExpressFeeCents = tc.fee
			s.ExpressTurnaround = tc.description
			if got := ValidateService(s); (got == nil) != tc.valid {
				t.Fatalf("validation: %v", got)
			}
		})
	}
}
func TestIntegrationExpressQuoteChangesAndHistoricalOrders(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	services, e := s.Services(ctx, f.shop.ID, true)
	if e != nil {
		t.Fatal(e)
	}
	service := services[0]
	if service.Express || service.ExpressFeeCents != 0 || service.ExpressTurnaround != "" {
		t.Fatal("existing/default services opted into express")
	}
	booking := f.booking
	booking.Express = true
	_, e = s.CreateQuote(ctx, f.customer, f.shop.ID, booking)
	wantProblem(t, e, "invalid_configuration")
	service.Express = true
	service.ExpressFeeCents = 799
	service.ExpressTurnaround = " Within 24 hours of receipt. "
	saved, e := s.SaveService(ctx, f.owner, f.shop.ID, service)
	if e != nil {
		t.Fatal(e)
	}
	if saved.ExpressTurnaround != "Within 24 hours of receipt." {
		t.Fatal("description not normalized")
	}
	public, e := s.Services(ctx, f.shop.ID, false)
	if e != nil || !public[0].Express || public[0].ExpressFeeCents != 799 {
		t.Fatalf("public offer: %+v %v", public, e)
	}
	standard, e := s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if e != nil || standard.TotalCents != 2900 {
		t.Fatalf("standard: %+v %v", standard, e)
	}
	quote, e := s.CreateQuote(ctx, f.customer, f.shop.ID, booking)
	if e != nil {
		t.Fatal(e)
	}
	service.ExpressFeeCents = 899
	if _, e = s.SaveService(ctx, f.owner, f.shop.ID, service); e != nil {
		t.Fatal(e)
	}
	_, _, e = s.CreateOrder(ctx, f.customer, quote.ID, "express-stale-fee-key")
	wantProblem(t, e, "quote_changed")
	quote, e = s.CreateQuote(ctx, f.customer, f.shop.ID, booking)
	if e != nil {
		t.Fatal(e)
	}
	service.ExpressTurnaround = "Within 48 hours of receipt."
	if _, e = s.SaveService(ctx, f.owner, f.shop.ID, service); e != nil {
		t.Fatal(e)
	}
	_, _, e = s.CreateOrder(ctx, f.customer, quote.ID, "express-stale-terms-key")
	wantProblem(t, e, "quote_changed")
	quote, e = s.CreateQuote(ctx, f.customer, f.shop.ID, booking)
	if e != nil {
		t.Fatal(e)
	}
	service.Express = false
	if _, e = s.SaveService(ctx, f.owner, f.shop.ID, service); e != nil {
		t.Fatal(e)
	}
	_, _, e = s.CreateOrder(ctx, f.customer, quote.ID, "express-disabled-key")
	wantProblem(t, e, "invalid_configuration")
	service.Express = true
	if _, e = s.SaveService(ctx, f.owner, f.shop.ID, service); e != nil {
		t.Fatal(e)
	}
	quote, e = s.CreateQuote(ctx, f.customer, f.shop.ID, booking)
	if e != nil {
		t.Fatal(e)
	}
	order, _, e := s.CreateOrder(ctx, f.customer, quote.ID, "express-order-retry-key")
	if e != nil {
		t.Fatal(e)
	}
	service.Express = false
	service.ExpressFeeCents = 1
	service.ExpressTurnaround = "Changed after booking."
	if _, e = s.SaveService(ctx, f.owner, f.shop.ID, service); e != nil {
		t.Fatal(e)
	}
	replay, reused, e := s.CreateOrder(ctx, f.customer, quote.ID, "express-order-retry-key")
	if e != nil || !reused || replay.ID != order.ID {
		t.Fatalf("retry after disable: %+v %v", replay, e)
	}
	got, e := s.GetOrder(ctx, f.customer, order.ID)
	if e != nil || got.TotalCents != 3799 || !got.Configuration.Express || got.Configuration.ExpressTurnaroundSnapshot != "Within 48 hours of receipt." {
		t.Fatalf("immutable order: %+v %v", got, e)
	}
	if _, e = s.UpdateStatus(ctx, f.owner, order.ID, "accepted"); e != nil {
		t.Fatal(e)
	}
	tracking, e := s.Tracking(ctx, f.customer, order.ID)
	if e != nil || tracking.ConfirmedReadyAt != nil {
		t.Fatalf("express invented an appointment: %+v %v", tracking, e)
	}
	usage, e := s.Usage(ctx, f.owner, f.shop.ID)
	if e != nil || usage.AcceptedBookings != 1 {
		t.Fatalf("express counted twice: %+v %v", usage, e)
	}
}
func TestIntegrationExpressPermissionsAndStrictQuoteInput(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	services, e := s.Services(ctx, f.shop.ID, true)
	if e != nil {
		t.Fatal(e)
	}
	service := services[0]
	service.Express = true
	service.ExpressFeeCents = 500
	service.ExpressTurnaround = "Within one day of receiving the racket."
	if _, e = s.Pool.Exec(ctx, `INSERT INTO stringer_members(stringer_id,user_id,role) VALUES($1,$2,'staff')`, f.shop.ID, f.other.ID); e != nil {
		t.Fatal(e)
	}
	_, e = s.SaveService(ctx, f.other, f.shop.ID, service)
	wantProblem(t, e, "forbidden")
	_, e = s.SaveService(ctx, f.customer, f.shop.ID, service)
	wantProblem(t, e, "forbidden")
	if _, e = s.SaveService(ctx, f.admin, f.shop.ID, service); e != nil {
		t.Fatal(e)
	}
	wantStatus(t, f.request(t, "POST", "/stringers/"+f.shop.ID+"/quotes", map[string]any{"racket_model_id": f.racket.ID, "string_source": "stock", "variant_id": f.variant.ID, "mains_tension_centi_kg": 2400, "crosses_tension_centi_kg": 2400, "knots": 4, "express": true, "express_fee_cents": 0}, f.customer, nil), 400)
	own := f.booking
	own.StringSource = "customer"
	own.VariantID = ""
	own.Express = true
	q, e := s.CreateQuote(ctx, f.customer, f.shop.ID, own)
	if e != nil || q.TotalCents != 2250 {
		t.Fatalf("own express quote: %+v %v", q, e)
	}
	other, e := s.Apply(ctx, f.other, ProfileInput{Name: "Other service", AcceptingOrders: true})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Review(ctx, f.admin, other.ID, "approved", "test"); e != nil {
		t.Fatal(e)
	}
	service.Express = false
	if _, e = s.SaveService(ctx, f.other, other.ID, service); e != nil {
		t.Fatal(e)
	}
	_, e = s.CreateQuote(ctx, f.customer, other.ID, own)
	wantProblem(t, e, "invalid_configuration")
}
