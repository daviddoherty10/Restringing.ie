package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestIntegrationManagementPermissionsMatchWrites(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	id := f.shop.ID
	if _, err := f.store.Pool.Exec(ctx, `INSERT INTO stringer_members(stringer_id,user_id,role) VALUES($1,$2,'staff')`, id, f.customer.ID); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		actor Actor
		role  string
		edit  bool
	}{{f.owner, "owner", true}, {f.customer, "staff", false}, {f.admin, "admin", true}} {
		w := f.request(t, "GET", "/stringers/"+id+"/manage/profile", nil, tc.actor, nil)
		wantStatus(t, w, 200)
		var v ManagementProfile
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		if v.MembershipRole != tc.role || v.Permissions.EditProfile != tc.edit || v.Permissions.EditServices != tc.edit || !v.Permissions.ManageInventory || !v.Permissions.ManageOrders {
			t.Fatalf("incorrect permissions: %+v", v)
		}
	}
	list, err := f.store.MyStringers(ctx, f.customer)
	if err != nil || len(list) != 1 || list[0].MembershipRole != "staff" || list[0].Permissions.EditProfile {
		t.Fatalf("membership list %+v %v", list, err)
	}
	wantStatus(t, f.request(t, "GET", "/stringers/"+id+"/manage/profile", nil, f.other, nil), 404)
	wantStatus(t, f.request(t, "GET", "/stringers/"+id+"/manage/summary", nil, f.other, nil), 403)
	wantStatus(t, f.request(t, "PUT", "/stringers/"+id, ProfileInput{Name: "Forged staff edit"}, f.customer, nil), 403)
	_, service, _, _ := pricingFixture()
	wantStatus(t, f.request(t, "PUT", "/stringers/"+id+"/services/"+service.Sport, service, f.customer, nil), 403)
	wantStatus(t, f.request(t, "PUT", "/stringers/"+id+"/inventory/"+f.variant.ID, map[string]any{"material_price_cents": 1099, "available": true}, f.customer, nil), 200)
	// The flags describe authorization; sending flags back cannot confer a role.
	wantStatus(t, f.request(t, "PUT", "/stringers/"+id, map[string]any{"name": "Forged owner", "permissions": map[string]bool{"edit_profile": true}}, f.customer, nil), 400)
	public := f.request(t, "GET", "/stringers/"+id, nil, Actor{}, nil)
	wantStatus(t, public, 200)
	if strings.Contains(public.Body.String(), "membership_role") || strings.Contains(public.Body.String(), "permissions") {
		t.Fatal("public profile leaked private membership")
	}
	if _, err = f.store.Pool.Exec(ctx, `DELETE FROM stringer_members WHERE stringer_id=$1 AND user_id=$2`, id, f.customer.ID); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, f.request(t, "GET", "/stringers/"+id+"/manage/profile", nil, f.customer, nil), 404)
	wantStatus(t, f.request(t, "PUT", "/stringers/"+id+"/inventory/"+f.variant.ID, map[string]any{"material_price_cents": 1, "available": true}, f.customer, nil), 403)
	// Pending businesses can configure their private workspace before approval.
	pending, err := f.store.Apply(ctx, f.owner, ProfileInput{Name: "Pending dashboard fixture"})
	if err != nil {
		t.Fatal(err)
	}
	private, err := f.store.ManagementProfile(ctx, f.owner, pending.ID)
	if err != nil || private.ApprovalStatus != "pending" || !private.Permissions.EditServices {
		t.Fatal(private, err)
	}
	wantStatus(t, f.request(t, "GET", "/stringers/"+pending.ID, nil, Actor{}, nil), 404)
}
func TestIntegrationDashboardSummaryAndCustomerContactAreScoped(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	if _, err := s.AddLot(ctx, f.owner, f.shop.ID, f.variant.ID, "set", 10, 12000); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE users SET first_name='Preview',last_name='Customer' WHERE id=$1`, f.customer.ID); err != nil {
		t.Fatal(err)
	}
	orders := []Order{}
	for i := 0; i < 3; i++ {
		q, err := s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
		if err != nil {
			t.Fatal(err)
		}
		o, _, err := s.CreateOrder(ctx, f.customer, q.ID, fmt.Sprintf("dashboard-order-%03d", i))
		if err != nil {
			t.Fatal(err)
		}
		orders = append(orders, o)
	}
	for _, status := range []string{"accepted", "received", "in_progress", "ready", "completed"} {
		if _, err := s.UpdateStatus(ctx, f.owner, orders[0].ID, status); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.UpdateStatus(ctx, f.owner, orders[2].ID, "cancelled"); err != nil {
		t.Fatal(err)
	}
	other, err := s.Apply(ctx, f.other, ProfileInput{Name: "Other business", AcceptingOrders: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Review(ctx, f.admin, other.ID, "approved", "test"); err != nil {
		t.Fatal(err)
	}
	_, service, _, _ := pricingFixture()
	if _, err = s.SaveService(ctx, f.other, other.ID, service); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveInventory(ctx, f.other, other.ID, f.variant.ID, 500, true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddLot(ctx, f.other, other.ID, f.variant.ID, "set", 5, 12000); err != nil {
		t.Fatal(err)
	}
	q, err := s.CreateQuote(ctx, f.customer, other.ID, f.booking)
	if err != nil {
		t.Fatal(err)
	}
	otherOrder, _, err := s.CreateOrder(ctx, f.customer, q.ID, "other-dashboard-order-001")
	if err != nil {
		t.Fatal(err)
	}
	w := f.request(t, "GET", "/stringers/"+f.shop.ID+"/manage/summary", nil, f.owner, nil)
	wantStatus(t, w, 200)
	var summary OrderSummary
	json.Unmarshal(w.Body.Bytes(), &summary)
	if summary.TotalOrders != 3 || summary.ActiveOrders != 1 || summary.CompletedOrders != 1 || summary.CancelledOrders != 1 || summary.UnpaidOrders != 2 || summary.BookedValueCents != int64(orders[0].TotalCents+orders[1].TotalCents) || summary.Currency != "EUR" {
		t.Fatalf("summary included cancellation or another business: %+v", summary)
	}
	private := f.request(t, "GET", "/stringers/"+f.shop.ID+"/orders/"+orders[0].ID, nil, f.owner, nil)
	wantStatus(t, private, 200)
	var incoming IncomingOrder
	json.Unmarshal(private.Body.Bytes(), &incoming)
	if incoming.Customer.FirstName != "Preview" || incoming.Customer.LastName != "Customer" || incoming.Customer.Phone == "" || len(incoming.Events) != 6 {
		t.Fatal("missing fulfilment details", incoming)
	}
	wantStatus(t, f.request(t, "GET", "/stringers/"+f.shop.ID+"/orders/"+orders[0].ID, nil, f.customer, nil), 403)
	wantStatus(t, f.request(t, "GET", "/stringers/"+f.shop.ID+"/orders/"+otherOrder.ID, nil, f.owner, nil), 404)
	wantStatus(t, f.request(t, "GET", "/stringers/"+other.ID+"/orders/"+orders[0].ID, nil, f.admin, nil), 404)
	own := f.request(t, "GET", "/orders/"+orders[0].ID, nil, f.customer, nil)
	wantStatus(t, own, 200)
	if strings.Contains(own.Body.String(), "phone_number") || strings.Contains(own.Body.String(), "first_name") {
		t.Fatal("customer-contact fields leaked outside business-scoped detail route")
	}
	empty, err := s.Apply(ctx, f.owner, ProfileInput{Name: "Empty business"})
	if err != nil {
		t.Fatal(err)
	}
	zero, err := s.OrderSummary(ctx, f.owner, empty.ID)
	if err != nil || zero.TotalOrders != 0 || zero.BookedValueCents != 0 {
		t.Fatal("empty summary", zero, err)
	}
}
func TestIntegrationStockLotFiltersAndInactiveLabels(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	s := f.store
	for i := 0; i < 22; i++ {
		if _, err := s.AddLot(ctx, f.owner, f.shop.ID, f.variant.ID, "reel", 200000, 0); err != nil {
			t.Fatal(err)
		}
	}
	base := "/stringers/" + f.shop.ID + "/inventory/lots"
	w := f.request(t, "GET", base+"?page=2&page_size=10&variant_id="+f.variant.ID, nil, f.owner, nil)
	wantStatus(t, w, 200)
	var page Page[StockLot]
	json.Unmarshal(w.Body.Bytes(), &page)
	if page.Total != 23 || len(page.Data) != 10 || page.Data[0].VariantID != f.variant.ID || page.Data[0].Brand == "" || page.Data[0].GaugeMM != 1.25 {
		t.Fatal("unlabelled or unpaginated stock", page)
	}
	filtered := f.request(t, "GET", base+"?q=does-not-exist", nil, f.owner, nil)
	wantStatus(t, filtered, 200)
	json.Unmarshal(filtered.Body.Bytes(), &page)
	if page.Total != 0 || len(page.Data) != 0 {
		t.Fatal("search not applied")
	}
	wantStatus(t, f.request(t, "GET", base+"?variant_id=invalid", nil, f.owner, nil), 400)
	wantStatus(t, f.request(t, "GET", base, nil, f.other, nil), 403)
	exact := f.request(t, "GET", "/stringers/"+f.shop.ID+"/manage/inventory?variant_id="+f.variant.ID, nil, f.owner, nil)
	wantStatus(t, exact, 200)
	var variants Page[Variant]
	json.Unmarshal(exact.Body.Bytes(), &variants)
	if variants.Total != 1 || len(variants.Data) != 1 || variants.Data[0].ID != f.variant.ID {
		t.Fatal("exact offering lookup", variants)
	}
	// Global deactivation must not hide the physical lot from its business ledger.
	if _, err := s.Pool.Exec(ctx, `UPDATE string_products SET active=false WHERE id=$1`, f.variant.ProductID); err != nil {
		t.Fatal(err)
	}
	lots, err := s.Lots(ctx, f.owner, f.shop.ID, Filters{Page: 1, PageSize: 10, Sport: f.variant.Sport})
	if err != nil || lots.Total != 23 || lots.Data[0].Model == "" {
		t.Fatal("inactive stock became unidentifiable", lots, err)
	}
	if _, err = s.AdjustLot(ctx, f.owner, f.shop.ID, lots.Data[0].ID, 1, "physical recount"); err != nil {
		t.Fatal(err)
	}
}
