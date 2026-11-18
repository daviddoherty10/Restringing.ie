package marketplace

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestModelFreePriceRequiresExplicitSportAndTensionChoice(t *testing.T) {
	b, s, r, v := pricingFixture()
	b.RacketModelID = ""
	b.Sport = "tennis"
	b.RacketDetails = " My older racket "
	b.TensionPending = true
	b.MainsTension = 0
	b.CrossesTension = 0
	r.ID = ""
	r.Verified = false
	q, err := Price(b, s, r, &v)
	if err != nil || q.Configuration.RacketSnapshot != "My older racket (tennis)" || !q.Configuration.TensionPending {
		t.Fatal("model-free quote failed", q, err)
	}
	for _, bad := range []Booking{
		func() Booking { v := b; v.Sport = ""; return v }(),
		func() Booking { v := b; v.Sport = "badminton"; return v }(),
		func() Booking { v := b; v.MainsTension = 2200; return v }(),
		func() Booking { v := b; v.TensionPending = false; return v }(),
		func() Booking { v := b; v.RacketDetails = strings.Repeat("x", 161); return v }(),
		func() Booking { v := b; v.RacketDetails = "bad\x00details"; return v }(),
	} {
		if _, err := Price(bad, s, r, &v); err == nil {
			t.Fatalf("invalid booking accepted: %+v", bad)
		}
	}
	b.TensionPending = false
	b.MainsTension = 2400
	b.CrossesTension = 2400
	if _, err = Price(b, s, r, &v); err != nil {
		t.Fatal("valid preferred tension rejected", err)
	}
	b.MainsTension = s.MaxTension + 50
	if _, err = Price(b, s, r, &v); err == nil {
		t.Fatal("service tension limit ignored")
	}
}

func TestIntegrationSportBookingWithoutCatalogueSurvivesQuoteOrderAndRetry(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.store.Pool.Exec(ctx, `UPDATE racket_models SET verified=false`); err != nil {
		t.Fatal(err)
	}
	opts, err := f.store.BookingOptions(ctx, f.shop.ID, "", "tennis")
	if err != nil || opts.Racket != nil || opts.Step <= 0 {
		t.Fatal("service options failed", opts, err)
	}
	wantStatus(t, f.request(t, "GET", "/stringers/"+f.shop.ID+"/booking-configuration?sport=tennis", nil, Actor{}, nil), 200)
	wantStatus(t, f.request(t, "GET", "/stringers/"+f.shop.ID+"/booking-configuration?sport=unknown", nil, Actor{}, nil), 422)
	out, err := f.store.Recommend(ctx, f.shop.ID, RecommendationRequest{Sport: "tennis", Experience: "new"})
	if err != nil || len(out.Results) == 0 {
		t.Fatal("model-free recommendation failed", out, err)
	}
	b := out.Results[0].Setup.Booking
	if b.RacketModelID != "" || b.Sport != "tennis" || !b.TensionPending || b.MainsTension != 0 || b.CrossesTension != 0 {
		t.Fatal("unexpected suggestion", b)
	}
	b.RacketDetails = "Older frame, model unknown"
	response := f.request(t, "POST", "/stringers/"+f.shop.ID+"/quotes", b, f.customer, nil)
	wantStatus(t, response, 201)
	var q Quote
	if err = json.Unmarshal(response.Body.Bytes(), &q); err != nil {
		t.Fatal(err)
	}
	o, _, err := f.store.CreateOrder(ctx, f.customer, q.ID, "model-free-order-retry-001")
	if err != nil || !o.Configuration.TensionPending || o.Configuration.RacketDetails != b.RacketDetails {
		t.Fatal("order lost preference", o, err)
	}
	retry, replayed, err := f.store.CreateOrder(ctx, f.customer, q.ID, "model-free-order-retry-001")
	if err != nil || !replayed || retry.ID != o.ID {
		t.Fatal("retry duplicated order", retry, err)
	}
	var reserved int
	if err = f.store.Pool.QueryRow(ctx, `SELECT reserved FROM inventory_lots WHERE id=$1`, f.lot.ID).Scan(&reserved); err != nil || reserved != 1 {
		t.Fatal("stock not reserved exactly once", reserved, err)
	}
}
