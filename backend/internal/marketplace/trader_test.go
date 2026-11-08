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
