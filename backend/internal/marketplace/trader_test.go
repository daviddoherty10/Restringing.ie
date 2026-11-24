package marketplace

import (
	"context"
	"testing"
)

func TestIntegrationTraderReviewGateAndIdentityChanges(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.store.EnforceTrader = true
	q, e := f.store.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if e == nil {
		t.Fatal("unverified quote allowed", q)
	}
	d := TraderDocument{LegalName: "TEST private identity", BusinessType: "sole_trader", Address: "TEST private address", Email: "private@example.invalid", Phone: "TEST private phone", Public: testLegalDocument().Trader, Declaration: true}
	if _, e = f.store.SubmitTrader(ctx, f.other, f.shop.ID, TraderSubmission{Document: d}); e == nil {
		t.Fatal("unrelated customer submitted")
	}
	r, e := f.store.SubmitTrader(ctx, f.owner, f.shop.ID, TraderSubmission{Document: d})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.store.Trader(ctx, f.customer, f.shop.ID); e == nil {
		t.Fatal("private record leaked")
	}
	review := TraderReview{Version: r.Version, Status: "verified", Reason: "TEST checked evidence"}
	if _, e = f.store.ReviewTrader(ctx, f.admin, f.shop.ID, review); e == nil {
		t.Fatal("unchecked verification")
	}
	review.IdentityChecked = true
	review.ContactChecked = true
	r, e = f.store.ReviewTrader(ctx, f.admin, f.shop.ID, review)
	if e != nil {
		t.Fatal(e)
	}
	identity, e := f.store.PublicTrader(ctx, f.shop.ID)
	if e != nil || identity.Name != d.Public.Name {
		t.Fatal("public identity", e)
	}
	q, e = f.store.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.store.SubmitTrader(ctx, f.owner, f.shop.ID, TraderSubmission{Version: r.Version, Document: d})
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = f.store.CreateOrder(ctx, f.customer, q.ID, "trader-invalidated-key"); e == nil {
		t.Fatal("changed identity order allowed")
	}
	if _, e = f.store.ReviewTrader(ctx, f.admin, f.shop.ID, review); e == nil {
		t.Fatal("stale verification accepted")
	}
}

func TestIntegrationListingSearchPublicAddress(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	d := TraderDocument{LegalName: "Private business", BusinessType: "sole_trader", Address: "Private lane", Email: "private@example.invalid", Phone: "private phone", Declaration: true, Public: testLegalDocument().Trader}
	d.Public.Address = "8 Example Road, Dublin, D02 XY12"
	r, err := f.store.SubmitTrader(ctx, f.owner, f.shop.ID, TraderSubmission{Document: d})
	if err != nil {
		t.Fatal(err)
	}
	search := func(q string, expected int) []Stringer {
		t.Helper()
		p, err := f.store.ListStringers(ctx, Filters{Query: q, Page: 1, PageSize: 12})
		if err != nil || p.Total != expected || len(p.Data) != expected {
			t.Fatalf("search %q: %+v, %v", q, p, err)
		}
		return p.Data
	}
	search("Example Road", 0)
	if search(f.shop.Name, 1)[0].PublicIdentity != nil {
		t.Fatal("unverified contact was published")
	}
	_, err = f.store.ReviewTrader(ctx, f.admin, f.shop.ID, TraderReview{Version: r.Version, Status: "verified", Reason: "Fixture review", IdentityChecked: true, ContactChecked: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"example road", "d02xy12", "D02 XY12", "Dublin", "Carrickmines"} {
		contact := search(q, 1)[0].PublicIdentity
		if contact == nil || *contact != d.Public {
			t.Fatal("incorrect public contact", contact)
		}
	}
	search(d.Address, 0)
}

func TestIntegrationNearbyStringers(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	for _, shop := range [][3]string{
		{"Workshop A", "Dún Laoghaire", "Dublin"}, {"Workshop B", "Blackrock", "Dublin"},
		{"Workshop C", "Blackrock", "Cork"}, {"Workshop D", "Galway City", "Galway"},
	} {
		if _, err := s.Pool.Exec(ctx, `INSERT INTO stringers(name,area,county,approval_status) VALUES ($1,$2,$3,'approved')`, shop[0], shop[1], shop[2]); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{"Carrickmines", "Carrickmines, Co. Dublin", "D18", "d18 xy12", "D18XY12", "Dun Laoghaire"} {
		result, err := s.ListStringers(ctx, Filters{Query: query, Page: 1, PageSize: 10})
		if err != nil || result.Total != 2 || len(result.Data) != 2 {
			t.Fatalf("%q: %+v, %v", query, result, err)
		}
		for _, v := range result.Data {
			if v.County != "Dublin" {
				t.Fatalf("%q included %s", query, v.County)
			}
		}
	}
	page, err := s.ListStringers(ctx, Filters{Query: "Dun Laoghaire", Page: 1, PageSize: 1})
	if err != nil || page.Total != 2 || len(page.Data) != 1 || page.Data[0].Area != "Dún Laoghaire" {
		t.Fatalf("direct match should rank first: %+v, %v", page, err)
	}
	result, err := s.ListStringers(ctx, Filters{Query: "Blackrock, Cork", Page: 1, PageSize: 10})
	if err != nil || result.Total != 1 || result.Data[0].County != "Cork" {
		t.Fatalf("ambiguous town: %+v, %v", result, err)
	}
}
