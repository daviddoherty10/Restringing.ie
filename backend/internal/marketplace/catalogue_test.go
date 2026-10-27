package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
)

func catalogueResponse[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	wantStatus(t, w, 200)
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func TestIntegrationCatalogueAdministrationAccess(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.store.Pool.Exec(ctx, `INSERT INTO stringer_members(stringer_id,user_id,role) VALUES($1,$2,'staff')`, f.shop.ID, f.other.ID); err != nil {
		t.Fatal(err)
	}
	paths := []string{"/admin/string-products", "/admin/string-products/" + f.variant.ProductID, "/admin/string-variants", "/admin/string-variants/" + f.variant.ID, "/admin/rackets", "/admin/rackets/" + f.racket.ID}
	for _, path := range paths {
		wantStatus(t, f.request(t, "GET", path, nil, Actor{}, nil), 401)
		for _, a := range []Actor{f.customer, f.owner, f.other} {
			wantStatus(t, f.request(t, "GET", path, nil, a, nil), 403)
		}
		wantStatus(t, f.request(t, "GET", path, nil, f.admin, nil), 200)
	}
	for _, a := range []Actor{f.customer, f.owner, f.other} {
		_, err := f.store.CatalogueProducts(ctx, a, CatalogueFilters{})
		wantProblem(t, err, "forbidden")
		_, err = f.store.CatalogueVariants(ctx, a, CatalogueFilters{})
		wantProblem(t, err, "forbidden")
		_, err = f.store.CatalogueRackets(ctx, a, CatalogueFilters{})
		wantProblem(t, err, "forbidden")
		wantStatus(t, f.request(t, "POST", "/admin/string-products", Product{}, a, nil), 403)
		wantStatus(t, f.request(t, "PUT", "/admin/rackets/"+f.racket.ID, f.racket, a, nil), 403)
	}
	wantStatus(t, f.request(t, "GET", "/admin/string-products/not-a-uuid", nil, f.admin, nil), 400)
	wantStatus(t, f.request(t, "GET", "/admin/string-products/90000000-0000-4000-8000-000000099999", nil, f.admin, nil), 404)
}
func TestIntegrationPrivateCataloguePaginationFiltersAndPublicIsolation(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	lastProduct, lastVariant := "", ""
	for n := 0; n < 25; n++ {
		p, err := s.SaveProduct(ctx, "", Product{Brand: "Private catalogue", Model: fmt.Sprintf("String %02d", n), Sport: "tennis", Material: "multifilament", Comfort: 3, Control: 3, Power: 3, Spin: 3, Durability: 3, Active: n%2 == 0})
		if err != nil {
			t.Fatal(err)
		}
		id, err := s.SaveVariant(ctx, "", VariantInput{ProductID: p.ID, GaugeMM: 1.25, Colour: "White", Active: n%2 != 0})
		if err != nil {
			t.Fatal(err)
		}
		lastProduct, lastVariant = p.ID, id
	}
	page := catalogueResponse[Page[Product]](t, f.request(t, "GET", "/admin/string-products?q=Private%20catalogue&page_size=20", nil, f.admin, nil))
	second := catalogueResponse[Page[Product]](t, f.request(t, "GET", "/admin/string-products?q=Private%20catalogue&page_size=20&page=2", nil, f.admin, nil))
	if page.Total != 25 || len(page.Data) != 20 || len(second.Data) != 5 {
		t.Fatal("private pagination", page.Total, len(page.Data), len(second.Data))
	}
	seen := map[string]bool{}
	for _, p := range append(page.Data, second.Data...) {
		if seen[p.ID] {
			t.Fatal("unstable pagination")
		}
		seen[p.ID] = true
	}
	inactive := catalogueResponse[Page[Product]](t, f.request(t, "GET", "/admin/string-products?brand=private%20catalogue&active=false&sport=tennis&material=multifilament", nil, f.admin, nil))
	if inactive.Total != 12 {
		t.Fatal("inactive filter", inactive.Total)
	}
	variants := catalogueResponse[Page[CatalogueVariant]](t, f.request(t, "GET", "/admin/string-variants?q=Private%20catalogue&active=false&gauge_mm=1.25&page_size=20", nil, f.admin, nil))
	if variants.Total != 13 || variants.Data[0].Active || !variants.Data[0].ProductActive {
		t.Fatal("variant state filter", variants)
	}
	exact := catalogueResponse[Page[CatalogueVariant]](t, f.request(t, "GET", "/admin/string-variants?product_id="+lastProduct, nil, f.admin, nil))
	if exact.Total != 1 || exact.Data[0].ID != lastVariant {
		t.Fatal("parent filter", exact)
	}
	public := catalogueResponse[Page[Variant]](t, f.request(t, "GET", "/strings?q=Private%20catalogue", nil, Actor{}, nil))
	if public.Total != 0 {
		t.Fatal("inactive product/variant leaked publicly", public.Total)
	}
	for _, path := range []string{"/admin/string-products?active=all", "/admin/rackets?verified=maybe", "/admin/string-variants?product_id=bad", "/admin/string-products?page_size=51", "/admin/string-variants?gauge_mm=NaN"} {
		wantStatus(t, f.request(t, "GET", path, nil, f.admin, nil), 400)
	}
	for n := 0; n < 22; n++ {
		r := f.racket
		r.Brand = "Private rackets"
		r.Model = fmt.Sprintf("Model %02d", n)
		r.Verified = n%2 == 0
		if _, err := s.SaveRacket(ctx, "", r); err != nil {
			t.Fatal(err)
		}
	}
	rackets := catalogueResponse[Page[Racket]](t, f.request(t, "GET", "/admin/rackets?brand=private%20rackets&verified=false&page_size=20", nil, f.admin, nil))
	if rackets.Total != 11 {
		t.Fatal("unverified search", rackets.Total)
	}
	detail := catalogueResponse[Racket](t, f.request(t, "GET", "/admin/rackets/"+rackets.Data[0].ID, nil, f.admin, nil))
	if detail.Verified {
		t.Fatal("unverified detail not preserved")
	}
	publicRackets := catalogueResponse[Page[Racket]](t, f.request(t, "GET", "/rackets?q=Private%20rackets&page_size=50", nil, Actor{}, nil))
	if publicRackets.Total != 11 {
		t.Fatal("unverified model leaked", publicRackets.Total)
	}
}
func TestIntegrationCatalogueChangesKeepOrderHistoryAndExistingWork(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	q, err := s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if err != nil {
		t.Fatal(err)
	}
	order, _, err := s.CreateOrder(ctx, f.customer, q.ID, "admin-catalogue-history-001")
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.CatalogueProduct(ctx, f.admin, f.variant.ProductID)
	if err != nil {
		t.Fatal(err)
	}
	p.Active = false
	p.Model = "Retired model label"
	wantStatus(t, f.request(t, "PUT", "/admin/string-products/"+p.ID, p, f.admin, nil), 200)
	r := f.racket
	r.Verified = false
	r.Model = "Unverified edited model"
	wantStatus(t, f.request(t, "PUT", "/admin/rackets/"+r.ID, r, f.admin, nil), 200)
	private := catalogueResponse[Product](t, f.request(t, "GET", "/admin/string-products/"+p.ID, nil, f.admin, nil))
	if private.Active {
		t.Fatal("deactivation lost")
	}
	public := catalogueResponse[Page[Variant]](t, f.request(t, "GET", "/strings?variant_id="+f.variant.ID, nil, Actor{}, nil))
	if public.Total != 0 {
		t.Fatal("disabled product still public")
	}
	if _, err = s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking); err == nil {
		t.Fatal("disabled setup still quoteable")
	}
	saved, err := s.GetOrder(ctx, f.customer, order.ID)
	if err != nil || saved.Configuration.StringSnapshot != order.Configuration.StringSnapshot || saved.Configuration.RacketSnapshot != order.Configuration.RacketSnapshot || saved.TotalCents != order.TotalCents {
		t.Fatal("history rewritten", saved, err)
	}
	for _, status := range []string{"accepted", "received", "in_progress", "ready", "completed"} {
		if _, err = s.UpdateStatus(ctx, f.owner, order.ID, status); err != nil {
			t.Fatal("existing fulfilment blocked", status, err)
		}
	}
	invalid := r
	invalid.MinTension = invalid.MaxTension + 1
	wantStatus(t, f.request(t, "PUT", "/admin/rackets/"+r.ID, invalid, f.admin, nil), 422)
	invalid = r
	invalid.TechnicalSource = ""
	wantStatus(t, f.request(t, "PUT", "/admin/rackets/"+r.ID, invalid, f.admin, nil), 422)
	wantStatus(t, f.request(t, "POST", "/admin/string-variants", VariantInput{ProductID: p.ID, GaugeMM: 1.255}, f.admin, nil), 422)
}
