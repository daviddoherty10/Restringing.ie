package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func intp(n int) *int { return &n }
func hybridFixture(t *testing.T, mainMM, crossMM int) (*fixture, Variant, Lot, Lot) {
	t.Helper()
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	r := f.racket
	r.MainsLengthMM = 6400
	r.CrossesLengthMM = 5600
	r.HybridTechnicalSource = "Fictional separately verified cut lengths"
	var e error
	f.racket, e = s.SaveRacket(ctx, r.ID, r)
	if e != nil {
		t.Fatal(e)
	}
	_, service, _, v := pricingFixture()
	service.Hybrid = true
	if _, e = s.SaveService(ctx, f.owner, f.shop.ID, service); e != nil {
		t.Fatal(e)
	}
	p, e := s.SaveProduct(ctx, "", Product{Brand: "Fixture", Model: "Cross", Sport: "tennis", Material: "polyester", Comfort: 2, Control: 5, Power: 2, Spin: 5, Durability: 5, MaxPrestretchPercent: 5, Active: true})
	if e != nil {
		t.Fatal(e)
	}
	id, e := s.SaveVariant(ctx, "", VariantInput{ProductID: p.ID, GaugeMM: 1.3, Colour: "black", Active: true})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveInventory(ctx, f.owner, f.shop.ID, id, v.MaterialPriceCents, true); e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{f.variant.ID, id} {
		if _, e = s.SaveHybridPrices(ctx, f.owner, f.shop.ID, id, HybridPrices{intp(0), intp(725)}); e != nil {
			t.Fatal(e)
		}
	}
	c, e := loadVariant(ctx, s.Pool, f.shop.ID, id)
	if e != nil {
		t.Fatal(e)
	}
	mlot, e := s.AddLot(ctx, f.owner, f.shop.ID, f.variant.ID, "reel", mainMM, 0)
	if e != nil {
		t.Fatal(e)
	}
	clot, e := s.AddLot(ctx, f.owner, f.shop.ID, c.ID, "reel", crossMM, 0)
	if e != nil {
		t.Fatal(e)
	}
	f.booking.Hybrid = true
	f.booking.CrossesVariantID = c.ID
	f.booking.PrestretchPercent = 0
	return f, c, mlot, clot
}
func assertStringLedger(t *testing.T, f *fixture, lot string, quantity, reserved int) {
	t.Helper()
	var q, r, qd, rd, alloc int
	e := f.store.Pool.QueryRow(context.Background(), `SELECT quantity,reserved,(SELECT COALESCE(sum(quantity_delta),0) FROM inventory_movements WHERE lot_id=$1),(SELECT COALESCE(sum(reserved_delta),0) FROM inventory_movements WHERE lot_id=$1),(SELECT COALESCE(sum(quantity),0) FROM inventory_reservations WHERE lot_id=$1 AND state='reserved') FROM inventory_lots WHERE id=$1`, lot).Scan(&q, &r, &qd, &rd, &alloc)
	if e != nil || q != quantity || r != reserved || q != qd || r != rd || r != alloc {
		t.Fatalf("stock %s %d/%d ledger %d/%d alloc %d want %d/%d: %v", lot, q, r, qd, rd, alloc, quantity, reserved, e)
	}
}
func TestHybridPricingRules(t *testing.T) {
	b, s, r, v := pricingFixture()
	b.Hybrid = true
	b.CrossesVariantID = "cross"
	s.Hybrid = true
	r.MainsLengthMM = 6400
	r.CrossesLengthMM = 5600
	r.HybridTechnicalSource = "fixture"
	v.MainsPriceCents = intp(0)
	c := v
	c.ID = "cross"
	c.CrossesPriceCents = intp(725)
	q, e := priceComponents(b, s, r, &v, &c, nil)
	if e != nil || q.TotalCents != 2425 || q.Configuration.MainsLengthMM != 6400 || q.Breakdown[1].AmountCents != 0 || q.Breakdown[2].AmountCents != 725 {
		t.Fatalf("price %+v %v", q, e)
	}
	original := q.Fingerprint
	c.CrossesPriceCents = intp(726)
	changed, e := priceComponents(b, s, r, &v, &c, nil)
	if e != nil || changed.Fingerprint == original {
		t.Fatal("cross price missing from fingerprint", e)
	}
	tests := map[string]func(*Booking, *Service, *Racket, *Variant, *Variant){
		"two knots":       func(b *Booking, _ *Service, _ *Racket, _ *Variant, _ *Variant) { b.Knots = 2 },
		"unoffered":       func(_ *Booking, s *Service, _ *Racket, _ *Variant, _ *Variant) { s.Hybrid = false },
		"missing lengths": func(_ *Booking, _ *Service, r *Racket, _ *Variant, _ *Variant) { r.CrossesLengthMM = 0 },
		"unknown source":  func(_ *Booking, _ *Service, r *Racket, _ *Variant, _ *Variant) { r.HybridTechnicalSource = "" },
		"customer mixed":  func(b *Booking, _ *Service, _ *Racket, _ *Variant, _ *Variant) { b.StringSource = "customer" },
		"missing price":   func(_ *Booking, _ *Service, _ *Racket, v *Variant, _ *Variant) { v.MainsPriceCents = nil },
		"different sport": func(_ *Booking, _ *Service, _ *Racket, _ *Variant, c *Variant) { c.Sport = "squash" },
		"pre stretch": func(b *Booking, _ *Service, _ *Racket, _ *Variant, c *Variant) {
			b.PrestretchPercent = 10
			c.MaxPrestretchPercent = 5
		},
		"packaged hybrid": func(_ *Booking, _ *Service, _ *Racket, _ *Variant, c *Variant) { c.Material = "hybrid" },
		"no hybrid":       func(b *Booking, _ *Service, _ *Racket, _ *Variant, _ *Variant) { b.Hybrid = false },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			bb, ss, rr, m, cross := b, s, r, v, c
			change(&bb, &ss, &rr, &m, &cross)
			_, e := priceComponents(bb, ss, rr, &m, &cross, nil)
			wantProblem(t, e, "invalid_configuration")
		})
	}
}
func TestIntegrationHybridLifecycleAndSnapshots(t *testing.T) {
	f, c, ml, cl := hybridFixture(t, 12800, 11200)
	s := f.store
	ctx := context.Background()
	p, g := gripFixture(t, f, 2)
	f.booking.GripProductID = p.ID
	q, e := s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if e != nil {
		t.Fatal(e)
	}
	assertStringLedger(t, f, ml.ID, 12800, 0)
	assertStringLedger(t, f, cl.ID, 11200, 0)
	// Either component price changing invalidates the old quote.
	if _, e = s.SaveHybridPrices(ctx, f.owner, f.shop.ID, c.ID, HybridPrices{intp(0), intp(726)}); e != nil {
		t.Fatal(e)
	}
	_, _, e = s.CreateOrder(ctx, f.customer, q.ID, "hybrid-stale-price-key")
	wantProblem(t, e, "quote_changed")
	q, e = s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if e != nil {
		t.Fatal(e)
	}
	o, replay, e := s.CreateOrder(ctx, f.customer, q.ID, "hybrid-original-order-key")
	if e != nil || replay || o.TotalCents != 3125 {
		t.Fatalf("order %+v %v", o, e)
	}
	assertStringLedger(t, f, ml.ID, 12800, 6400)
	assertStringLedger(t, f, cl.ID, 11200, 5600)
	assertGripLedger(t, f, g.InventoryID, 2, 1)
	// Fulfilment and exact order retries use immutable snapshots after deactivation.
	if _, e = s.SaveHybridPrices(ctx, f.owner, f.shop.ID, c.ID, HybridPrices{}); e != nil {
		t.Fatal(e)
	}
	got, replay, e := s.CreateOrder(ctx, f.customer, q.ID, "hybrid-original-order-key")
	if e != nil || !replay || got.ID != o.ID || got.Configuration.CrossesSnapshot != o.Configuration.CrossesSnapshot {
		t.Fatal("immutable retry failed", e)
	}
	for _, status := range []string{"accepted", "received", "in_progress", "ready", "completed"} {
		if _, e = s.UpdateStatus(ctx, f.owner, o.ID, status); e != nil {
			t.Fatal(e)
		}
	}
	assertStringLedger(t, f, ml.ID, 6400, 0)
	assertStringLedger(t, f, cl.ID, 5600, 0)
	assertStringLedger(t, f, f.lot.ID, 1, 0)
	assertGripLedger(t, f, g.InventoryID, 1, 0)
	var usage int
	if e = s.Pool.QueryRow(ctx, `SELECT count(*) FROM booking_usage WHERE order_id=$1`, o.ID).Scan(&usage); e != nil || usage != 1 {
		t.Fatal("hybrid counted more than one racket", e)
	}
	// New setup, then cancellation, releases both components and grip once.
	if _, e = s.SaveHybridPrices(ctx, f.owner, f.shop.ID, c.ID, HybridPrices{intp(0), intp(725)}); e != nil {
		t.Fatal(e)
	}
	q, e = s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if e != nil {
		t.Fatal(e)
	}
	o, _, e = s.CreateOrder(ctx, f.customer, q.ID, "hybrid-cancel-order-key")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.UpdateStatus(ctx, f.customer, o.ID, "cancelled"); e != nil {
		t.Fatal(e)
	}
	if _, replay, e = s.CreateOrder(ctx, f.customer, q.ID, "hybrid-cancel-order-key"); e != nil || !replay {
		t.Fatal(e)
	}
	assertStringLedger(t, f, ml.ID, 6400, 0)
	assertStringLedger(t, f, cl.ID, 5600, 0)
	assertGripLedger(t, f, g.InventoryID, 1, 0)
}
func TestIntegrationHybridSameVariantAggregatesDemand(t *testing.T) {
	f, _, ml, _ := hybridFixture(t, 11999, 5600)
	s := f.store
	ctx := context.Background()
	f.booking.CrossesVariantID = f.variant.ID
	_, e := s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	wantProblem(t, e, "hybrid_out_of_stock")
	if _, e = s.AdjustLot(ctx, f.owner, f.shop.ID, ml.ID, 1, "Measured correction"); e != nil {
		t.Fatal(e)
	}
	q, e := s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if e != nil {
		t.Fatal(e)
	}
	o, _, e := s.CreateOrder(ctx, f.customer, q.ID, "same-variant-hybrid-order")
	if e != nil {
		t.Fatal(e)
	}
	assertStringLedger(t, f, ml.ID, 12000, 12000)
	var count, total int
	if e = s.Pool.QueryRow(ctx, `SELECT count(*),sum(quantity) FROM inventory_reservations WHERE order_id=$1`, o.ID).Scan(&count, &total); e != nil || count != 2 || total != 12000 {
		t.Fatal("same variant allocation", count, total, e)
	}
	if _, e = s.UpdateStatus(ctx, f.customer, o.ID, "cancelled"); e != nil {
		t.Fatal(e)
	}
	assertStringLedger(t, f, ml.ID, 12000, 0)
	// Whole sets never make a reel-only component available.
	if _, e = s.AdjustLot(ctx, f.owner, f.shop.ID, ml.ID, -12000, "Remove reel"); e != nil {
		t.Fatal(e)
	}
	_, e = s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	wantProblem(t, e, "hybrid_out_of_stock")
	assertStringLedger(t, f, f.lot.ID, 1, 0)
}
func TestIntegrationHybridLastCrossAndRollback(t *testing.T) {
	f, _, ml, cl := hybridFixture(t, 12800, 5600)
	s := f.store
	ctx := context.Background()
	p, g := gripFixture(t, f, 2)
	f.booking.GripProductID = p.ID
	q1, e := s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if e != nil {
		t.Fatal(e)
	}
	q2, e := s.CreateQuote(ctx, f.other, f.shop.ID, f.booking)
	if e != nil {
		t.Fatal(e)
	}
	// A failure after all three allocations rolls every row and movement back.
	if _, e = s.Pool.Exec(ctx, `ALTER TABLE notifications ADD CONSTRAINT injected_hybrid_failure CHECK (false)`); e != nil {
		t.Fatal(e)
	}
	_, _, e = s.CreateOrder(ctx, f.customer, q1.ID, "hybrid-rollback-order-key")
	if e == nil {
		t.Fatal("expected notification failure")
	}
	assertStringLedger(t, f, ml.ID, 12800, 0)
	assertStringLedger(t, f, cl.ID, 5600, 0)
	assertGripLedger(t, f, g.InventoryID, 2, 0)
	if _, e = s.Pool.Exec(ctx, `ALTER TABLE notifications DROP CONSTRAINT injected_hybrid_failure`); e != nil {
		t.Fatal(e)
	}
	type result struct {
		o Order
		a Actor
		e error
	}
	ch := make(chan result, 2)
	var wg sync.WaitGroup
	for i, pair := range []struct {
		a Actor
		q string
	}{{f.customer, q1.ID}, {f.other, q2.ID}} {
		wg.Add(1)
		go func(i int, a Actor, q string) {
			defer wg.Done()
			o, _, e := s.CreateOrder(ctx, a, q, fmt.Sprintf("hybrid-race-order-%02d", i))
			ch <- result{o, a, e}
		}(i, pair.a, pair.q)
	}
	wg.Wait()
	close(ch)
	successes := 0
	var winner result
	for r := range ch {
		if r.e == nil {
			successes++
			winner = r
		} else {
			wantProblem(t, r.e, "hybrid_out_of_stock")
		}
	}
	if successes != 1 {
		t.Fatal("last crosses oversold", successes)
	}
	assertStringLedger(t, f, ml.ID, 12800, 6400)
	assertStringLedger(t, f, cl.ID, 5600, 5600)
	assertGripLedger(t, f, g.InventoryID, 2, 1)
	for _, status := range []string{"accepted", "received"} {
		if _, e = s.UpdateStatus(ctx, f.owner, winner.o.ID, status); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = s.Pool.Exec(ctx, `ALTER TABLE inventory_movements ADD CONSTRAINT injected_hybrid_consumption CHECK (lot_id<>'`+ml.ID+`'::uuid OR quantity_delta>=0)`); e != nil {
		t.Fatal(e)
	}
	_, e = s.UpdateStatus(ctx, f.owner, winner.o.ID, "in_progress")
	if e == nil {
		t.Fatal("expected final mains ledger failure")
	}
	assertStringLedger(t, f, ml.ID, 12800, 6400)
	assertStringLedger(t, f, cl.ID, 5600, 5600)
	assertGripLedger(t, f, g.InventoryID, 2, 1)
	got, e := s.GetOrder(ctx, winner.a, winner.o.ID)
	if e != nil || got.Status != "received" {
		t.Fatal("partial work transition", e)
	}
	if _, e = s.Pool.Exec(ctx, `ALTER TABLE inventory_movements DROP CONSTRAINT injected_hybrid_consumption`); e != nil {
		t.Fatal(e)
	}
	if _, e = s.UpdateStatus(ctx, f.owner, winner.o.ID, "in_progress"); e != nil {
		t.Fatal(e)
	}
	assertStringLedger(t, f, ml.ID, 6400, 0)
	assertStringLedger(t, f, cl.ID, 0, 0)
	assertGripLedger(t, f, g.InventoryID, 1, 0)
}
func TestIntegrationHybridPublicFiltersHTTPAndCuration(t *testing.T) {
	f, c, _, _ := hybridFixture(t, 12800, 11200)
	s := f.store
	ctx := context.Background()
	page, e := s.HybridInventory(ctx, f.shop.ID, f.racket.ID, "mains", Filters{Page: 1, PageSize: 1})
	if e != nil || page.Total != 2 || len(page.Data) != 1 {
		t.Fatalf("pagination %+v %v", page, e)
	}
	page, e = s.HybridInventory(ctx, f.shop.ID, f.racket.ID, "crosses", Filters{Page: 1, PageSize: 5, VariantID: c.ID, Material: "polyester"})
	if e != nil || page.Total != 1 {
		t.Fatal("cross exact filters", e)
	}
	raw, _ := json.Marshal(page)
	if strings.Contains(string(raw), `"quantity"`) || strings.Contains(string(raw), `"reserved"`) {
		t.Fatal("public stock balances leaked")
	}
	path := "/stringers/" + f.shop.ID + "/inventory/" + c.ID + "/hybrid-prices"
	if r := f.request(t, "PUT", path, HybridPrices{intp(1), intp(2)}, f.customer, nil); r.Code != 403 {
		t.Fatal("foreign customer price write", r.Code, r.Body)
	}
	if r := f.request(t, "PUT", path, HybridPrices{intp(-1), intp(2)}, f.owner, nil); r.Code != 422 {
		t.Fatal("negative price", r.Code, r.Body)
	}
	if r := f.request(t, "PUT", path, HybridPrices{intp(0), nil}, f.owner, nil); r.Code != 200 {
		t.Fatal(r.Code, r.Body)
	}
	page, e = s.HybridInventory(ctx, f.shop.ID, f.racket.ID, "crosses", Filters{Page: 1, PageSize: 5, VariantID: c.ID})
	if e != nil || page.Total != 0 {
		t.Fatal("disabled component offered", e)
	}
	bad := f.booking
	bad.CrossesVariantID = "bad"
	if r := f.request(t, "POST", "/stringers/"+f.shop.ID+"/quotes", bad, f.customer, nil); r.Code != 400 {
		t.Fatal("invalid cross UUID", r.Code, r.Body)
	}
	// A valid global variant is not sufficient: the selected shop must offer it.
	foreign, e := s.SaveVariant(ctx, "", VariantInput{ProductID: c.ProductID, GaugeMM: 1.35, Colour: "foreign-only", Active: true})
	if e != nil {
		t.Fatal(e)
	}
	bad = f.booking
	bad.CrossesVariantID = foreign
	if response := f.request(t, "POST", "/stringers/"+f.shop.ID+"/quotes", bad, f.customer, nil); response.Code != 404 {
		t.Fatal("unoffered global cross accepted", response.Code, response.Body)
	}
	if _, e = s.SaveHybridPrices(ctx, f.other, f.shop.ID, c.ID, HybridPrices{intp(0), intp(0)}); e == nil {
		t.Fatal("nonmember modified prices")
	}
	if _, e = s.Pool.Exec(ctx, `INSERT INTO stringer_members(stringer_id,user_id,role) VALUES($1,$2,'staff')`, f.shop.ID, f.other.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveHybridPrices(ctx, f.other, f.shop.ID, c.ID, HybridPrices{intp(0), intp(0)}); e != nil {
		t.Fatal("staff cannot manage component prices", e)
	}
	_, service, _, _ := pricingFixture()
	service.Hybrid = true
	if _, e = s.SaveService(ctx, f.other, f.shop.ID, service); e == nil {
		t.Fatal("staff changed owner service capability")
	}
	r := f.racket
	r.CrossesLengthMM = 0
	if _, e = s.SaveRacket(ctx, r.ID, r); e == nil {
		t.Fatal("partial technical data accepted")
	}
	r = f.racket
	r.HybridTechnicalSource = ""
	if _, e = s.SaveRacket(ctx, r.ID, r); e == nil {
		t.Fatal("missing source accepted")
	}
	if r := f.request(t, "GET", "/stringers/"+f.shop.ID+"/hybrid-inventory?racket_model_id="+f.racket.ID+"&component=bad", nil, Actor{}, nil); r.Code != 400 {
		t.Fatal("invalid component", r.Code, r.Body)
	}
}
