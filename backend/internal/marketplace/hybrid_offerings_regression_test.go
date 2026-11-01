package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestIntegrationNormalOfferingsAreAvailableForHybridsWithoutPortionSetup(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	_, service, _, _ := pricingFixture()
	service.Hybrid = true
	if _, e := s.SaveService(ctx, f.owner, f.shop.ID, service); e != nil {
		t.Fatal(e)
	}
	var variants []string
	var lots []Lot
	for i := 0; i < 24; i++ {
		id, e := s.SaveVariant(ctx, "", VariantInput{ProductID: f.variant.ProductID, GaugeMM: 1.30, Colour: fmt.Sprintf("Existing stock %02d", i+1), Active: true})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = s.SaveInventory(ctx, f.owner, f.shop.ID, id, 1001+i, true, true); e != nil {
			t.Fatal(e)
		}
		lot, e := s.AddLot(ctx, f.owner, f.shop.ID, id, "set", 1, 12000)
		if e != nil {
			t.Fatal(e)
		}
		variants = append(variants, id)
		lots = append(lots, lot)
	}
	filter := Filters{Query: "Existing stock", Page: 1, PageSize: 5, IndividualOnly: true}
	full, e := s.ListVariants(ctx, f.shop.ID, filter, true)
	if e != nil || full.Total != 24 {
		t.Fatal("full-bed fixture", full, e)
	}
	for _, component := range []string{"mains", "crosses"} {
		for page := 1; page <= 5; page++ {
			filter.Page = page
			result, e := s.HybridInventory(ctx, f.shop.ID, f.racket.ID, component, filter)
			expected := 5
			if page == 5 {
				expected = 4
			}
			if e != nil || result.Total != 24 || len(result.Data) != expected {
				t.Fatalf("%s page %d only showed portion stock: %+v %v", component, page, result, e)
			}
			for _, v := range result.Data {
				if !v.TrackStock || v.TrackHybridStock {
					t.Fatal("legacy full-bed mode changed", v)
				}
			}
		}
	}
	response := f.request(t, "GET", "/stringers/"+f.shop.ID+"/hybrid-inventory?racket_model_id="+f.racket.ID+"&component=crosses&q=Existing%20stock&page_size=5&page=2", nil, Actor{}, nil)
	wantStatus(t, response, 200)
	var body Page[Variant]
	if e = json.Unmarshal(response.Body.Bytes(), &body); e != nil || body.Total != 24 || len(body.Data) != 5 {
		t.Fatal("HTTP regression", e, response.Body)
	}
	// A previously configured, tracked hybrid component still reserves its stock.
	old, e := s.SaveVariant(ctx, "", VariantInput{ProductID: f.variant.ProductID, GaugeMM: 1.35, Colour: "Old explicit hybrid", Active: true})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveInventory(ctx, f.owner, f.shop.ID, old, 900, true, false, true); e != nil {
		t.Fatal(e)
	}
	oldLot, e := s.AddLot(ctx, f.owner, f.shop.ID, old, "half_racket", 1, 0)
	if e != nil {
		t.Fatal(e)
	}
	b := f.booking
	b.Hybrid = true
	b.VariantID = variants[0]
	b.CrossesVariantID = old
	quote, e := s.CreateQuote(ctx, f.customer, f.shop.ID, b)
	if e != nil || quote.TotalCents != 2651 {
		t.Fatalf("legacy full-bed string could not supply mains %+v %v", quote, e)
	}
	order, _, e := s.CreateOrder(ctx, f.customer, quote.ID, "normal-and-old-hybrid-key")
	if e != nil {
		t.Fatal(e)
	}
	assertStringLedger(t, f, lots[0].ID, 1, 0)
	assertStringLedger(t, f, oldLot.ID, 1, 1)
	disabled := false
	_, e = s.SaveOffering(ctx, f.owner, f.shop.ID, old, InventoryInput{Price: 900, Available: true, TrackHybridStock: &disabled})
	wantProblem(t, e, "stock_tracking_in_use")
	if _, e = s.UpdateStatus(ctx, f.customer, order.ID, "cancelled"); e != nil {
		t.Fatal(e)
	}
	assertStringLedger(t, f, oldLot.ID, 1, 0)
	// A normal offering can supply either role and both together at its full price.
	b.CrossesVariantID = b.VariantID
	quote, e = s.CreateQuote(ctx, f.customer, f.shop.ID, b)
	if e != nil || quote.TotalCents != 2701 {
		t.Fatal("same normal string hybrid", quote, e)
	}
	order, _, e = s.CreateOrder(ctx, f.customer, quote.ID, "normal-in-both-halves-key")
	if e != nil {
		t.Fatal(e)
	}
	for _, status := range []string{"accepted", "received", "in_progress"} {
		if _, e = s.UpdateStatus(ctx, f.owner, order.ID, status); e != nil {
			t.Fatal(e)
		}
	}
	assertStringLedger(t, f, lots[0].ID, 1, 0)
	// Its separately tracked full-bed balance still reserves normally.
	b.Hybrid = false
	b.CrossesVariantID = ""
	quote, e = s.CreateQuote(ctx, f.customer, f.shop.ID, b)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.CreateOrder(ctx, f.customer, quote.ID, "full-bed-still-tracks-key"); e != nil {
		t.Fatal(e)
	}
	assertStringLedger(t, f, lots[0].ID, 1, 1)
}
func TestIntegrationHybridTrackingUpgradeKeepsExistingPortionsAndReservations(t *testing.T) {
	f, _, main, _ := hybridFixture(t, 2, 2)
	s := f.store
	ctx := context.Background()
	normal, e := s.SaveVariant(ctx, "", VariantInput{ProductID: f.variant.ProductID, GaugeMM: 1.30, Colour: "Normal legacy offering", Active: true})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveInventory(ctx, f.owner, f.shop.ID, normal, 1400, true, true); e != nil {
		t.Fatal(e)
	}
	normalLot, e := s.AddLot(ctx, f.owner, f.shop.ID, normal, "set", 1, 12000)
	if e != nil {
		t.Fatal(e)
	}
	quote, e := s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if e != nil {
		t.Fatal(e)
	}
	order, _, e := s.CreateOrder(ctx, f.customer, quote.ID, "pre-hybrid-mode-migration")
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile("../../migrations/00013_explicit_hybrid_stock_tracking.sql")
	if e != nil {
		t.Fatal(e)
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, strings.Split(string(raw), "-- +goose Down")[1]); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, strings.Split(string(raw), "-- +goose Down")[0]); e != nil {
		t.Fatal(e)
	}
	var full, hybrid bool
	if e = tx.QueryRow(ctx, `SELECT track_stock,track_hybrid_stock FROM stringer_inventory WHERE stringer_id=$1 AND variant_id=$2`, f.shop.ID, normal).Scan(&full, &hybrid); e != nil || !full || hybrid {
		t.Fatal("normal legacy offering still requires portions", e, full, hybrid)
	}
	if e = tx.QueryRow(ctx, `SELECT track_hybrid_stock FROM stringer_inventory WHERE stringer_id=$1 AND variant_id=$2`, f.shop.ID, f.variant.ID).Scan(&hybrid); e != nil || !hybrid {
		t.Fatal("old hybrid allocations lost tracking", e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	assertStringLedger(t, f, main.ID, 2, 1)
	assertStringLedger(t, f, normalLot.ID, 1, 0)
	if _, e = s.UpdateStatus(ctx, f.customer, order.ID, "cancelled"); e != nil {
		t.Fatal(e)
	}
	assertStringLedger(t, f, main.ID, 2, 0)
}
