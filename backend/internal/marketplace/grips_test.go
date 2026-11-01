package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func gripFixture(t *testing.T, f *fixture, units int) (GripProduct, GripStock) {
	t.Helper()
	ctx := context.Background()
	p, e := f.store.SaveGripProduct(ctx, f.admin, "", "grip-fixture-create-key", GripInput{Brand: "Fixture", Model: "Soft Grip", Kind: "overgrip", Colour: "White", Active: true})
	if e != nil {
		t.Fatal(e)
	}
	stock, e := f.store.SaveGripOffering(ctx, f.owner, f.shop.ID, p.ID, GripOfferingInput{FittedPriceCents: 699, Available: true})
	if e != nil {
		t.Fatal(e)
	}
	if units > 0 {
		if _, e = f.store.AdjustGripStock(ctx, f.owner, f.shop.ID, p.ID, "grip-fixture-stock-key", GripStockInput{QuantityDelta: units, Reason: "Initial fixture stock"}); e != nil {
			t.Fatal(e)
		}
	}
	return p, stock
}
func gripFiltersFixture() GripFilters {
	return GripFilters{CatalogueFilters: CatalogueFilters{Filters: Filters{Page: 1, PageSize: 20}}}
}
func assertGripLedger(t *testing.T, f *fixture, inventory string, quantity, reserved int) {
	t.Helper()
	var q, r, qd, rd, alloc int
	e := f.store.Pool.QueryRow(context.Background(), `SELECT quantity,reserved,(SELECT COALESCE(sum(quantity_delta),0) FROM grip_inventory_movements WHERE inventory_id=$1),(SELECT COALESCE(sum(reserved_delta),0) FROM grip_inventory_movements WHERE inventory_id=$1),(SELECT COALESCE(sum(quantity),0) FROM inventory_reservations WHERE grip_inventory_id=$1 AND state='reserved') FROM grip_inventory WHERE id=$1`, inventory).Scan(&q, &r, &qd, &rd, &alloc)
	if e != nil || q != quantity || r != reserved || q != qd || r != rd || r != alloc {
		t.Fatalf("grip ledger %d/%d sums %d/%d allocation %d want %d/%d: %v", q, r, qd, rd, alloc, quantity, reserved, e)
	}
}
func TestIntegrationGripCataloguePricingAndPermissions(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	p, stock := gripFixture(t, f, 1)
	replay, e := s.SaveGripProduct(ctx, f.admin, "", "grip-fixture-create-key", p.GripInput)
	if e != nil || replay.ID != p.ID {
		t.Fatalf("catalogue replay: %+v %v", replay, e)
	}
	changed := p.GripInput
	changed.Model = "Another grip"
	_, e = s.SaveGripProduct(ctx, f.admin, "", "grip-fixture-create-key", changed)
	wantProblem(t, e, "idempotency_conflict")
	_, e = s.SaveGripProduct(ctx, f.owner, "", "owner-grip-create-key", changed)
	wantProblem(t, e, "forbidden")
	if _, e = s.Pool.Exec(ctx, `INSERT INTO stringer_members(stringer_id,user_id,role) VALUES($1,$2,'staff')`, f.shop.ID, f.other.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveGripOffering(ctx, f.other, f.shop.ID, p.ID, GripOfferingInput{FittedPriceCents: 599, Available: true}); e != nil {
		t.Fatal(e)
	}
	_, e = s.SaveGripOffering(ctx, f.customer, f.shop.ID, p.ID, GripOfferingInput{FittedPriceCents: 1, Available: true})
	wantProblem(t, e, "forbidden")
	_, e = s.ManageGrips(ctx, f.customer, f.shop.ID, gripFiltersFixture())
	wantProblem(t, e, "forbidden")
	offered, e := s.OfferedGrips(ctx, f.shop.ID, gripFiltersFixture())
	if e != nil || offered.Total != 1 || offered.Data[0].FittedPriceCents != 599 {
		t.Fatalf("offered: %+v %v", offered, e)
	}
	raw, _ := json.Marshal(offered)
	if strings.Contains(string(raw), `"quantity"`) || strings.Contains(string(raw), `"reserved"`) {
		t.Fatal("public grip offers exposed private balances")
	}
	b := f.booking
	b.GripProductID = p.ID
	quote, e := s.CreateQuote(ctx, f.customer, f.shop.ID, b)
	if e != nil || quote.TotalCents != 3499 || !strings.Contains(quote.Configuration.GripSnapshot, "Soft Grip") {
		t.Fatalf("grip price: %+v %v", quote, e)
	}
	// Price metadata edits invalidate an unsubmitted quote without reserving stock.
	if _, e = s.SaveGripOffering(ctx, f.owner, f.shop.ID, p.ID, GripOfferingInput{FittedPriceCents: 699, Available: true}); e != nil {
		t.Fatal(e)
	}
	_, _, e = s.CreateOrder(ctx, f.customer, quote.ID, "grip-stale-order-key")
	wantProblem(t, e, "quote_changed")
	quote, e = s.CreateQuote(ctx, f.customer, f.shop.ID, b)
	if e != nil {
		t.Fatal(e)
	}
	order, _, e := s.CreateOrder(ctx, f.customer, quote.ID, "grip-original-order-key")
	if e != nil {
		t.Fatal(e)
	}
	p.Model = "Changed catalogue name"
	p.Active = false
	if _, e = s.SaveGripProduct(ctx, f.admin, p.ID, "", p.GripInput); e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveGripOffering(ctx, f.owner, f.shop.ID, p.ID, GripOfferingInput{FittedPriceCents: 1, Available: false}); e != nil {
		t.Fatal(e)
	}
	existing, e := s.GetOrder(ctx, f.customer, order.ID)
	if e != nil || existing.TotalCents != 3599 || !strings.Contains(existing.Configuration.GripSnapshot, "Soft Grip") {
		t.Fatalf("historical terms changed: %+v %v", existing, e)
	}
	replayOrder, reused, e := s.CreateOrder(ctx, f.customer, quote.ID, "grip-original-order-key")
	if e != nil || !reused || replayOrder.ID != order.ID {
		t.Fatalf("replay after disable: %+v %v", replayOrder, e)
	}
	offered, e = s.OfferedGrips(ctx, f.shop.ID, gripFiltersFixture())
	if e != nil || offered.Total != 0 {
		t.Fatal("disabled grip still offered", e)
	}
	// Catalogue disable does not prevent fulfilment of an existing order.
	for _, status := range []string{"accepted", "received", "in_progress", "ready", "completed"} {
		if _, e = s.UpdateStatus(ctx, f.owner, order.ID, status); e != nil {
			t.Fatal(e)
		}
	}
	assertGripLedger(t, f, stock.InventoryID, 0, 0)
	usage, e := s.Usage(ctx, f.owner, f.shop.ID)
	if e != nil || usage.AcceptedBookings != 1 {
		t.Fatalf("grip counted as another restring: %+v %v", usage, e)
	}
}
func TestIntegrationGripReservationsCancellationAndConsumption(t *testing.T) {
	for _, own := range []bool{false, true} {
		t.Run(fmt.Sprintf("customer_strings_%v", own), func(t *testing.T) {
			f := newFixture(t)
			s := f.store
			ctx := context.Background()
			p, stock := gripFixture(t, f, 2)
			b := f.booking
			b.GripProductID = p.ID
			if own {
				b.StringSource = "customer"
				b.VariantID = ""
			}
			q, e := s.CreateQuote(ctx, f.customer, f.shop.ID, b)
			if e != nil {
				t.Fatal(e)
			}
			o, _, e := s.CreateOrder(ctx, f.customer, q.ID, "grip-cancel-order-key")
			if e != nil {
				t.Fatal(e)
			}
			assertGripLedger(t, f, stock.InventoryID, 2, 1)
			_, e = s.AdjustGripStock(ctx, f.owner, f.shop.ID, p.ID, "remove-reserved-grip-key", GripStockInput{QuantityDelta: -2, Reason: "Cannot remove reserved units"})
			wantProblem(t, e, "stock_conflict")
			if _, e = s.UpdateStatus(ctx, f.customer, o.ID, "cancelled"); e != nil {
				t.Fatal(e)
			}
			assertGripLedger(t, f, stock.InventoryID, 2, 0)
			q, e = s.CreateQuote(ctx, f.customer, f.shop.ID, b)
			if e != nil {
				t.Fatal(e)
			}
			o, _, e = s.CreateOrder(ctx, f.customer, q.ID, "grip-consume-order-key")
			if e != nil {
				t.Fatal(e)
			}
			for _, status := range []string{"accepted", "received", "in_progress"} {
				if _, e = s.UpdateStatus(ctx, f.owner, o.ID, status); e != nil {
					t.Fatal(e)
				}
			}
			assertGripLedger(t, f, stock.InventoryID, 1, 0)
			_, e = s.UpdateStatus(ctx, f.owner, o.ID, "in_progress")
			wantProblem(t, e, "invalid_transition")
			assertGripLedger(t, f, stock.InventoryID, 1, 0)
			var components int
			if e = s.Pool.QueryRow(ctx, `SELECT count(*) FROM inventory_reservations WHERE order_id=$1`, o.ID).Scan(&components); e != nil {
				t.Fatal(e)
			}
			want := 2
			if own {
				want = 1
			}
			if components != want {
				t.Fatalf("components=%d want %d", components, want)
			}
		})
	}
}
func TestIntegrationConcurrentLastGripAndAtomicFailures(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	p, stock := gripFixture(t, f, 1)
	if _, e := s.AddLot(ctx, f.owner, f.shop.ID, f.variant.ID, "set", 1, 12000); e != nil {
		t.Fatal(e)
	}
	b := f.booking
	b.GripProductID = p.ID
	q1, e := s.CreateQuote(ctx, f.customer, f.shop.ID, b)
	if e != nil {
		t.Fatal(e)
	}
	q2, e := s.CreateQuote(ctx, f.other, f.shop.ID, b)
	if e != nil {
		t.Fatal(e)
	}
	type result struct {
		o      Order
		a      Actor
		q, key string
		e      error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for i, pair := range []struct {
		a Actor
		q string
	}{{f.customer, q1.ID}, {f.other, q2.ID}} {
		wg.Add(1)
		go func(i int, a Actor, q string) {
			defer wg.Done()
			key := fmt.Sprintf("last-grip-order-%03d", i)
			o, _, e := s.CreateOrder(ctx, a, q, key)
			results <- result{o, a, q, key, e}
		}(i, pair.a, pair.q)
	}
	wg.Wait()
	close(results)
	success := 0
	var winner, loser result
	for r := range results {
		if r.e == nil {
			success++
			winner = r
		} else {
			wantProblem(t, r.e, "grip_out_of_stock")
			loser = r
		}
	}
	if success != 1 {
		t.Fatalf("last grip sold %d times", success)
	}
	var orders, stringReservations, gripReservations int
	if e = s.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM orders),(SELECT COALESCE(sum(reserved),0) FROM inventory_lots),(SELECT count(*) FROM inventory_reservations WHERE component='grip')`).Scan(&orders, &stringReservations, &gripReservations); e != nil || orders != 1 || stringReservations != 1 || gripReservations != 1 {
		t.Fatalf("partial stock/order: %d/%d/%d %v", orders, stringReservations, gripReservations, e)
	}
	assertGripLedger(t, f, stock.InventoryID, 1, 1)
	if _, e = s.UpdateStatus(ctx, winner.a, winner.o.ID, "cancelled"); e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.CreateOrder(ctx, loser.a, loser.q, loser.key); e != nil {
		t.Fatal("released grip not reusable", e)
	}
	assertGripLedger(t, f, stock.InventoryID, 1, 1)
}
func TestIntegrationGripNotificationFailureRollsBackAllItems(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	p, stock := gripFixture(t, f, 1)
	b := f.booking
	b.GripProductID = p.ID
	q, e := s.CreateQuote(ctx, f.customer, f.shop.ID, b)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Pool.Exec(ctx, `ALTER TABLE notifications ADD CONSTRAINT grip_fail_notification CHECK(kind<>'new_order')`); e != nil {
		t.Fatal(e)
	}
	_, _, e = s.CreateOrder(ctx, f.customer, q.ID, "grip-notification-failure")
	if e == nil {
		t.Fatal("expected notification failure")
	}
	assertGripLedger(t, f, stock.InventoryID, 1, 0)
	var count, qty int
	if e = s.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM orders),(SELECT reserved FROM inventory_lots WHERE id=$1)`, f.lot.ID).Scan(&count, &qty); e != nil || count != 0 || qty != 0 {
		t.Fatal("failed order stranded string reservation", e)
	}
	// The same quote/key succeeds once the failed transaction's cause is resolved.
	if _, e = s.Pool.Exec(ctx, `ALTER TABLE notifications DROP CONSTRAINT grip_fail_notification`); e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.CreateOrder(ctx, f.customer, q.ID, "grip-notification-failure"); e != nil {
		t.Fatal(e)
	}
}
func TestIntegrationGripStockRetryAndCatalogueFilters(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	p, stock := gripFixture(t, f, 1)
	var wg sync.WaitGroup
	ids := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m, e := s.AdjustGripStock(ctx, f.owner, f.shop.ID, p.ID, "grip-receipt-retry-key", GripStockInput{QuantityDelta: 4, Reason: "Delivery"})
			if e != nil {
				t.Error(e)
			}
			ids <- m.ID
		}()
	}
	wg.Wait()
	close(ids)
	id := ""
	for v := range ids {
		if id != "" && id != v {
			t.Fatal("duplicate receipt")
		}
		id = v
	}
	assertGripLedger(t, f, stock.InventoryID, 5, 0)
	_, e := s.AdjustGripStock(ctx, f.owner, f.shop.ID, p.ID, "grip-receipt-retry-key", GripStockInput{QuantityDelta: 5, Reason: "Delivery"})
	wantProblem(t, e, "idempotency_conflict")
	for i := 0; i < 23; i++ {
		_, e = s.SaveGripProduct(ctx, f.admin, "", fmt.Sprintf("grip-page-key-%03d", i), GripInput{Brand: "Catalogue", Model: fmt.Sprintf("Grip %02d", i), Kind: "replacement_grip", Colour: "Black", Active: i != 22})
		if e != nil {
			t.Fatal(e)
		}
	}
	filters := gripFiltersFixture()
	filters.Kind = "replacement_grip"
	filters.PageSize = 5
	page, e := s.ListGripProducts(ctx, Actor{}, filters, false)
	if e != nil || page.Total != 22 || len(page.Data) != 5 {
		t.Fatalf("public catalogue pagination %+v %v", page, e)
	}
	filters.Active = "false"
	private, e := s.ListGripProducts(ctx, f.admin, filters, true)
	if e != nil || private.Total != 1 {
		t.Fatalf("private inactive %+v %v", private, e)
	}
	_, e = s.ListGripProducts(ctx, f.customer, filters, true)
	wantProblem(t, e, "forbidden")
	history, e := s.GripMovements(ctx, f.owner, f.shop.ID, p.ID, Filters{Page: 1, PageSize: 1})
	if e != nil || history.Total != 2 || len(history.Data) != 1 {
		t.Fatalf("stock history %+v %v", history, e)
	}
}
func TestIntegrationGripHTTPAndForeignOffers(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	p, _ := gripFixture(t, f, 1)
	wantStatus(t, f.request(t, "GET", "/grips?kind=bad", nil, Actor{}, nil), 400)
	wantStatus(t, f.request(t, "GET", "/grips?page_size=500", nil, Actor{}, nil), 400)
	wantStatus(t, f.request(t, "GET", "/admin/grips", nil, f.customer, nil), 403)
	wantStatus(t, f.request(t, "POST", "/admin/grips", p.GripInput, f.admin, nil), 400)
	wantStatus(t, f.request(t, "GET", "/stringers/"+f.shop.ID+"/manage/grips", nil, Actor{}, nil), 401)
	wantStatus(t, f.request(t, "POST", "/stringers/"+f.shop.ID+"/grips/"+p.ID+"/stock-events", GripStockInput{QuantityDelta: 1, Reason: "more"}, f.owner, nil), 400)
	// Customer prices/snapshots are not accepted in the booking body.
	wantStatus(t, f.request(t, "POST", "/stringers/"+f.shop.ID+"/quotes", map[string]any{"racket_model_id": f.racket.ID, "grip_product_id": p.ID, "fitted_price_cents": 0}, f.customer, nil), 400)
	wantStatus(t, f.request(t, "POST", "/stringers/"+f.shop.ID+"/quotes", map[string]any{"racket_model_id": f.racket.ID, "grip_product_id": "invalid"}, f.customer, nil), 400)
	other, e := s.Apply(ctx, f.other, ProfileInput{Name: "Other grip stock", AcceptingOrders: true})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Review(ctx, f.admin, other.ID, "approved", "test"); e != nil {
		t.Fatal(e)
	}
	_, service, _, _ := pricingFixture()
	if _, e = s.SaveService(ctx, f.other, other.ID, service); e != nil {
		t.Fatal(e)
	}
	b := f.booking
	b.StringSource = "customer"
	b.VariantID = ""
	b.GripProductID = p.ID
	_, e = s.CreateQuote(ctx, f.customer, other.ID, b)
	wantProblem(t, e, "not_found")
	if _, e = s.SaveGripOffering(ctx, f.other, other.ID, p.ID, GripOfferingInput{FittedPriceCents: 1, Available: true}); e != nil {
		t.Fatal(e)
	}
	_, e = s.CreateQuote(ctx, f.customer, other.ID, b)
	wantProblem(t, e, "grip_out_of_stock")
}

func TestIntegrationGripWorkStartFailureDoesNotPartiallyConsume(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	p, stock := gripFixture(t, f, 1)
	b := f.booking
	b.GripProductID = p.ID
	q, e := s.CreateQuote(ctx, f.customer, f.shop.ID, b)
	if e != nil {
		t.Fatal(e)
	}
	o, _, e := s.CreateOrder(ctx, f.customer, q.ID, "grip-work-start-order")
	if e != nil {
		t.Fatal(e)
	}
	for _, status := range []string{"accepted", "received"} {
		if _, e = s.UpdateStatus(ctx, f.owner, o.ID, status); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = s.Pool.Exec(ctx, `ALTER TABLE grip_inventory_movements ADD CONSTRAINT injected_consume_failure CHECK(quantity_delta>=0)`); e != nil {
		t.Fatal(e)
	}
	_, e = s.UpdateStatus(ctx, f.owner, o.ID, "in_progress")
	if e == nil {
		t.Fatal("expected consumption ledger failure")
	}
	assertGripLedger(t, f, stock.InventoryID, 1, 1)
	var quantity, reserved int
	if e = s.Pool.QueryRow(ctx, `SELECT quantity,reserved FROM inventory_lots WHERE id=$1`, f.lot.ID).Scan(&quantity, &reserved); e != nil || quantity != 1 || reserved != 1 {
		t.Fatalf("string was partially consumed %d/%d %v", quantity, reserved, e)
	}
	got, e := s.GetOrder(ctx, f.customer, o.ID)
	if e != nil || got.Status != "received" {
		t.Fatal("work status partially committed", e)
	}
	if _, e = s.Pool.Exec(ctx, `ALTER TABLE grip_inventory_movements DROP CONSTRAINT injected_consume_failure`); e != nil {
		t.Fatal(e)
	}
	if _, e = s.UpdateStatus(ctx, f.owner, o.ID, "in_progress"); e != nil {
		t.Fatal(e)
	}
	assertGripLedger(t, f, stock.InventoryID, 0, 0)
}
func TestGripPriceAndValidation(t *testing.T) {
	b, s, r, v := pricingFixture()
	b.GripProductID = "grip"
	_, e := Price(b, s, r, &v)
	wantProblem(t, e, "invalid_configuration")
	grip := GripOffering{GripProduct: GripProduct{ID: "grip", GripInput: GripInput{Brand: "Fixture", Model: "Grip", Kind: "overgrip", Colour: "White", Active: true}}, InventoryID: "inventory", Available: true, FittedPriceCents: 0}
	q, e := Price(b, s, r, &v, &grip)
	if e != nil || q.TotalCents != 2900 || q.Breakdown[len(q.Breakdown)-1].Code != "grip" || q.Configuration.GripSnapshot == "" {
		t.Fatalf("zero fitted price dropped: %+v %v", q, e)
	}
	_, e = normalizeGrip(GripInput{Brand: "A", Model: "B", Kind: "string"})
	wantProblem(t, e, "invalid_configuration")
	_, e = normalizeGrip(GripInput{Brand: "A", Model: "B", Kind: "overgrip", Colour: strings.Repeat("é", 81)})
	wantProblem(t, e, "invalid_configuration")
}
