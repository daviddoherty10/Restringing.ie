package marketplace

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestHybridMaterialRoundsOnce(t *testing.T) {
	b, s, r, v := pricingFixture()
	s.Hybrid = true
	b.Hybrid = true
	b.CrossesVariantID = "cross"
	c := v
	c.ID = "cross"
	for _, pair := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}, {1299, 1299}, {1299, 1000}, {1000, 1299}, {1400, 1000}, {1000000, 999999}} {
		v.MaterialPriceCents = pair[0]
		c.MaterialPriceCents = pair[1]
		q, e := priceComponents(b, s, r, &v, &c, nil)
		if e != nil {
			t.Fatal(e)
		}
		var mains, cross, total int
		for _, line := range q.Breakdown {
			switch line.Code {
			case "mains_strings":
				mains = line.AmountCents
			case "crosses_strings":
				cross = line.AmountCents
			}
			total += line.AmountCents
		}
		expected := (pair[0] + pair[1] + 1) / 2
		if mains+cross != expected || mains < 0 || cross < 0 || total != q.TotalCents || (pair[0] == 0 && mains != 0) || (pair[1] == 0 && cross != 0) {
			t.Fatalf("pair %v: mains=%d cross=%d total=%d quote=%d", pair, mains, cross, total, q.TotalCents)
		}
		if pair[0] == pair[1] && mains+cross != pair[0] {
			t.Fatal("same string twice changed full-bed price")
		}
		v.MaterialPriceCents, c.MaterialPriceCents = c.MaterialPriceCents, v.MaterialPriceCents
		swapped, e := priceComponents(b, s, r, &v, &c, nil)
		if e != nil || swapped.TotalCents != q.TotalCents {
			t.Fatal("swapping changed total", pair, e)
		}
	}
}
func addSimpleOffering(t *testing.T, f *fixture, gauge float64, price int) string {
	t.Helper()
	ctx := context.Background()
	id, e := f.store.SaveVariant(ctx, "", VariantInput{ProductID: f.variant.ProductID, GaugeMM: gauge, Colour: "simple", Active: true})
	if e != nil {
		t.Fatal(e)
	}
	response := f.request(t, "PUT", "/stringers/"+f.shop.ID+"/inventory/"+id, InventoryInput{Price: price, Available: true}, f.owner, nil)
	wantStatus(t, response, 200)
	v, e := loadVariant(ctx, f.store.Pool, f.shop.ID, id)
	if e != nil || v.TrackStock {
		t.Fatal("new offering did not default to manual availability", e)
	}
	return id
}
func TestIntegrationSimpleOfferingNeedsNoStockReceipt(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	id := addSimpleOffering(t, f, 1.30, 1499)
	var b = f.booking
	b.VariantID = id
	page, e := s.ListVariants(ctx, f.shop.ID, Filters{VariantID: id, Page: 1, PageSize: 10}, true)
	if e != nil || page.Total != 1 {
		t.Fatal("new manual offering missing publicly", page, e)
	}
	recommendations, e := s.Recommend(ctx, f.shop.ID, RecommendationRequest{RacketModelID: f.racket.ID, Experience: "club", SessionsPerWeek: 2, Preferences: map[string]int{"comfort": 5}})
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, rec := range recommendations.Results {
		if rec.Variant.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatal("manual offering missing from recommendations")
	}
	q, e := s.CreateQuote(ctx, f.customer, f.shop.ID, b)
	if e != nil || q.TotalCents != 3199 {
		t.Fatalf("full-bed quote %+v %v", q, e)
	}
	o, _, e := s.CreateOrder(ctx, f.customer, q.ID, "simple-full-bed-order-key")
	if e != nil {
		t.Fatal(e)
	}
	for _, status := range []string{"accepted", "received", "in_progress", "ready", "completed"} {
		if _, e = s.UpdateStatus(ctx, f.owner, o.ID, status); e != nil {
			t.Fatal(e)
		}
	}
	var count int
	if e = s.Pool.QueryRow(ctx, `SELECT count(*) FROM inventory_reservations WHERE order_id=$1`, o.ID).Scan(&count); e != nil || count != 0 {
		t.Fatal("manual full-bed allocated measured stock", e)
	}
	_, service, _, _ := pricingFixture()
	service.Hybrid = true
	if _, e = s.SaveService(ctx, f.owner, f.shop.ID, service); e != nil {
		t.Fatal(e)
	}
	b.Hybrid = true
	b.CrossesVariantID = id
	q, e = s.CreateQuote(ctx, f.customer, f.shop.ID, b)
	if e != nil || q.TotalCents != 3199 {
		t.Fatalf("same-string hybrid %+v %v", q, e)
	}
	o, _, e = s.CreateOrder(ctx, f.customer, q.ID, "simple-hybrid-order-key")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveInventory(ctx, f.owner, f.shop.ID, id, 1499, false); e != nil {
		t.Fatal(e)
	}
	replay, retried, e := s.CreateOrder(ctx, f.customer, q.ID, "simple-hybrid-order-key")
	if e != nil || !retried || replay.ID != o.ID {
		t.Fatal("withdrawal broke committed retry", e)
	}
	_, e = s.CreateQuote(ctx, f.customer, f.shop.ID, b)
	wantProblem(t, e, "invalid_configuration")
	if _, e = s.UpdateStatus(ctx, f.customer, o.ID, "cancelled"); e != nil {
		t.Fatal(e)
	}
	assertStringLedger(t, f, f.lot.ID, 1, 0)
}
func TestIntegrationTrackingModePreservesExistingAllocations(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	id := addSimpleOffering(t, f, 1.30, 1400)
	b := f.booking
	b.VariantID = id
	q, e := s.CreateQuote(ctx, f.customer, f.shop.ID, b)
	if e != nil {
		t.Fatal(e)
	}
	old, _, e := s.CreateOrder(ctx, f.customer, q.ID, "manual-before-tracking-key")
	if e != nil {
		t.Fatal(e)
	}
	// An omitted tracking property must not reset an existing tracked offering.
	wantStatus(t, f.request(t, "PUT", "/stringers/"+f.shop.ID+"/inventory/"+f.variant.ID, InventoryInput{Price: 1234, Available: true}, f.owner, nil), 200)
	v, e := loadVariant(ctx, s.Pool, f.shop.ID, f.variant.ID)
	if e != nil || !v.TrackStock {
		t.Fatal("legacy update silently disabled tracking", e)
	}
	if _, e = s.SaveInventory(ctx, f.owner, f.shop.ID, id, 1400, true, true); e != nil {
		t.Fatal(e)
	}
	_, e = s.CreateQuote(ctx, f.customer, f.shop.ID, b)
	wantProblem(t, e, "out_of_stock")
	lot, e := s.AddLot(ctx, f.owner, f.shop.ID, id, "set", 1, 12000)
	if e != nil {
		t.Fatal(e)
	}
	for _, status := range []string{"accepted", "received", "in_progress"} {
		if _, e = s.UpdateStatus(ctx, f.owner, old.ID, status); e != nil {
			t.Fatal(e)
		}
	}
	assertStringLedger(t, f, lot.ID, 1, 0) // Enabling tracking never allocates retroactively.
	q, e = s.CreateQuote(ctx, f.customer, f.shop.ID, b)
	if e != nil {
		t.Fatal(e)
	}
	tracked, _, e := s.CreateOrder(ctx, f.customer, q.ID, "tracked-after-enabling-key")
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.SaveInventory(ctx, f.owner, f.shop.ID, id, 1400, true, false)
	wantProblem(t, e, "stock_tracking_in_use")
	assertStringLedger(t, f, lot.ID, 1, 1)
	if _, e = s.UpdateStatus(ctx, f.customer, tracked.ID, "cancelled"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveInventory(ctx, f.owner, f.shop.ID, id, 1400, true, false); e != nil {
		t.Fatal(e)
	}
	q, e = s.CreateQuote(ctx, f.customer, f.shop.ID, b)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveInventory(ctx, f.owner, f.shop.ID, id, 1400, true, true); e != nil {
		t.Fatal(e)
	}
	_, _, e = s.CreateOrder(ctx, f.customer, q.ID, "mode-changed-quote-key")
	wantProblem(t, e, "quote_changed")
}
func TestIntegrationHybridMixedTrackingAndGripAtomicity(t *testing.T) {
	f, _, ml, _ := hybridFixture(t, 1, 1)
	s := f.store
	ctx := context.Background()
	id := addSimpleOffering(t, f, 1.30, 1001)
	b := f.booking
	b.CrossesVariantID = id
	p, g := gripFixture(t, f, 1)
	b.GripProductID = p.ID
	q, e := s.CreateQuote(ctx, f.customer, f.shop.ID, b)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Pool.Exec(ctx, `ALTER TABLE notifications ADD CONSTRAINT simple_injected_failure CHECK(false)`); e != nil {
		t.Fatal(e)
	}
	_, _, e = s.CreateOrder(ctx, f.customer, q.ID, "mixed-tracking-order-key")
	if e == nil {
		t.Fatal("expected late failure")
	}
	assertStringLedger(t, f, ml.ID, 1, 0)
	assertGripLedger(t, f, g.InventoryID, 1, 0)
	if _, e = s.Pool.Exec(ctx, `ALTER TABLE notifications DROP CONSTRAINT simple_injected_failure`); e != nil {
		t.Fatal(e)
	}
	o, _, e := s.CreateOrder(ctx, f.customer, q.ID, "mixed-tracking-order-key")
	if e != nil {
		t.Fatal(e)
	}
	var components []string
	rows, e := s.Pool.Query(ctx, `SELECT component FROM inventory_reservations WHERE order_id=$1 ORDER BY component`, o.ID)
	if e != nil {
		t.Fatal(e)
	}
	for rows.Next() {
		var c string
		if e = rows.Scan(&c); e != nil {
			t.Fatal(e)
		}
		components = append(components, c)
	}
	rows.Close()
	raw, _ := json.Marshal(components)
	if string(raw) != `["grip","mains"]` {
		t.Fatal("manual cross was allocated", components)
	}
	for _, status := range []string{"accepted", "received", "in_progress"} {
		if _, e = s.UpdateStatus(ctx, f.owner, o.ID, status); e != nil {
			t.Fatal(e)
		}
	}
	assertStringLedger(t, f, ml.ID, 0, 0)
	assertGripLedger(t, f, g.InventoryID, 0, 0)
}

func TestIntegrationTrackingMigrationPreservesReservationsAndNewDefault(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	q, e := s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.CreateOrder(ctx, f.customer, q.ID, "pre-tracking-migration-key"); e != nil {
		t.Fatal(e)
	}
	variant, e := s.SaveVariant(ctx, "", VariantInput{ProductID: f.variant.ProductID, GaugeMM: 1.35, Colour: "post-migration", Active: true})
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile("../../migrations/00012_optional_string_stock_tracking.sql")
	if e != nil {
		t.Fatal(e)
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	// Recreate the pre-12 schema around a real legacy offering/reservation.
	if _, e = tx.Exec(ctx, `ALTER TABLE stringer_inventory DROP COLUMN track_stock`); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, strings.Split(string(raw), "-- +goose Down")[0]); e != nil {
		t.Fatal(e)
	}
	var old, newMode bool
	if e = tx.QueryRow(ctx, `SELECT track_stock FROM stringer_inventory WHERE id=$1`, f.variant.InventoryID).Scan(&old); e != nil || !old {
		t.Fatal("migration disabled legacy tracking", e)
	}
	if e = tx.QueryRow(ctx, `INSERT INTO stringer_inventory(stringer_id,variant_id,material_price_cents,available) VALUES($1,$2,1400,true) RETURNING track_stock`, f.shop.ID, variant).Scan(&newMode); e != nil || newMode {
		t.Fatal("new schema default still requires stock receipt", e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	assertStringLedger(t, f, f.lot.ID, 1, 1)
}
