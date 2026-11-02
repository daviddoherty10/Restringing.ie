package marketplace

import (
	"context"
	"testing"
)

func TestGuidedBeginnerSetupUsesCompatibleCompleteDefaults(t *testing.T) {
	_, s, r, v := pricingFixture()
	s.TwoKnots = true
	s.TwoKnotsCents = 0
	s.FourKnotsCents = 100
	s.Step = 75
	r.Step = 50
	r.AllowedKnots = []int{2, 4}
	v.Material = "multifilament"
	v.GaugeMM = 1.30
	poly := v
	poly.ID = "poly"
	poly.Material = "polyester"
	poly.Durability = 5
	poly.Comfort = 5
	req := RecommendationRequest{Experience: "new", SessionsPerWeek: 7, Preferences: map[string]int{"durability": 5}}
	out := CompleteRecommendations(req, []Variant{poly, v}, nil, nil, r, s)
	if len(out.Results) != 1 || out.Results[0].Setup == nil {
		t.Fatalf("no complete setup: %+v", out)
	}
	setup := out.Results[0].Setup
	b := setup.Booking
	if b.Hybrid || b.VariantID != v.ID || b.Knots != 2 || b.PrestretchPercent != 0 || b.Stencil || b.Express || b.GripProductID != "" || b.MainsTension != b.CrossesTension {
		t.Fatalf("unexpected defaults %+v", b)
	}
	if b.MainsTension%75 != 0 || b.MainsTension%50 != 0 {
		t.Fatal("incompatible tension step")
	}
	quote, err := Price(b, s, r, &v)
	if err != nil || quote.TotalCents != setup.EstimatedTotalCents {
		t.Fatal("suggestion cannot be quoted", err)
	}
	out = CompleteRecommendations(req, []Variant{poly}, nil, nil, r, s)
	if len(out.Results) != 0 {
		t.Fatal("beginner received a polyester fallback")
	}
	req.Experience = "advanced"
	req.PrioritiseArmComfort = true
	if len(CompleteRecommendations(req, []Variant{poly}, nil, nil, r, s).Results) != 0 {
		t.Fatal("soft-feel preference received polyester")
	}
}

func TestGuidedHybridIsConditionalAndStillValidated(t *testing.T) {
	_, s, r, v := pricingFixture()
	s.Hybrid = true
	r.AllowedKnots = []int{2, 4}
	s.TwoKnots = true
	v.Material = "polyester"
	v.ProductID = "poly-product"
	c := v
	c.ID = "cross"
	c.ProductID = "soft-product"
	c.Material = "multifilament"
	req := RecommendationRequest{Experience: "club", SessionsPerWeek: 2, Preferences: map[string]int{"spin": 5, "comfort": 3}}
	out := CompleteRecommendations(req, []Variant{v, c}, []Variant{v, c}, nil, r, s)
	if len(out.Results) == 0 || !out.Results[0].Setup.Booking.Hybrid {
		t.Fatalf("expected explained hybrid %+v", out)
	}
	setup := out.Results[0].Setup
	if setup.Booking.Knots != 4 || setup.Booking.CrossesVariantID != c.ID {
		t.Fatal("incomplete hybrid")
	}
	q, err := priceComponents(setup.Booking, s, r, &v, &c, nil)
	if err != nil || q.TotalCents != setup.EstimatedTotalCents {
		t.Fatal("invalid hybrid estimate", err)
	}
	s.Hybrid = false
	for _, rec := range CompleteRecommendations(req, []Variant{v, c}, []Variant{v, c}, nil, r, s).Results {
		if rec.Setup.Booking.Hybrid {
			t.Fatal("disabled hybrid recommended")
		}
	}
	s.Hybrid = true
	r.AllowedKnots = []int{2}
	for _, rec := range CompleteRecommendations(req, []Variant{v, c}, []Variant{v, c}, nil, r, s).Results {
		if rec.Setup.Booking.Hybrid {
			t.Fatal("two-knot-only racket received hybrid")
		}
	}
}

func TestGuidedKeepsValidKnownTensionAndDistinctProducts(t *testing.T) {
	_, s, r, v := pricingFixture()
	v.GaugeMM = 1.30
	v.ProductID = "shared-product"
	v.Material = "multifilament"
	thin := v
	thin.ID = "thin"
	thin.GaugeMM = 1.20
	other := v
	other.ID = "other"
	other.ProductID = "other-product"
	req := RecommendationRequest{Experience: "club", CurrentTension: 2400}
	out := CompleteRecommendations(req, []Variant{thin, v, other}, nil, nil, r, s)
	if len(out.Results) != 2 {
		t.Fatal("duplicate products crowd out alternatives", out)
	}
	for _, rec := range out.Results {
		if rec.Setup.Booking.MainsTension != 2400 || rec.Variant.GaugeMM != 1.30 {
			t.Fatal("known valid tension or representative gauge lost")
		}
	}
	s.MinTension = r.MaxTension + 1
	if len(CompleteRecommendations(req, []Variant{v}, nil, nil, r, s).Results) != 0 {
		t.Fatal("incompatible racket returned bookable setup")
	}
}

func TestIntegrationGuidedSetupRequotesCurrentPriceAndStock(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	req := RecommendationRequest{RacketModelID: f.racket.ID, Experience: "club"}
	out, err := f.store.Recommend(ctx, f.shop.ID, req)
	if err != nil || len(out.Results) == 0 || out.Results[0].Setup == nil {
		t.Fatal("no complete scoped setup", err)
	}
	rec := out.Results[0]
	before := rec.Setup.EstimatedTotalCents
	if _, err = f.store.SaveInventory(ctx, f.owner, f.shop.ID, rec.Variant.ID, rec.Variant.MaterialPriceCents+500, true); err != nil {
		t.Fatal(err)
	}
	q, err := f.store.CreateQuote(ctx, f.customer, f.shop.ID, rec.Setup.Booking)
	if err != nil || q.TotalCents != before+500 {
		t.Fatal("estimate was trusted instead of current server price", err)
	}
	if _, err = f.store.SaveInventory(ctx, f.owner, f.shop.ID, rec.Variant.ID, 999, false); err != nil {
		t.Fatal(err)
	}
	out, err = f.store.Recommend(ctx, f.shop.ID, req)
	if err != nil || len(out.Results) != 0 {
		t.Fatal("withdrawn string remained recommended", err)
	}
	if _, err = f.store.CreateQuote(ctx, f.customer, f.shop.ID, rec.Setup.Booking); err == nil {
		t.Fatal("withdrawn setup quoted")
	}
}

func TestIntegrationGuidedHybridUsesPortionAvailabilityAndRechecksIt(t *testing.T) {
	f, c, _, clot := hybridFixture(t, 2, 2)
	ctx := context.Background()
	req := RecommendationRequest{RacketModelID: f.racket.ID, Experience: "club", SessionsPerWeek: 2, Preferences: map[string]int{"spin": 5, "comfort": 3}}
	out, err := f.store.Recommend(ctx, f.shop.ID, req)
	if err != nil || len(out.Results) == 0 || !out.Results[0].Setup.Booking.Hybrid {
		t.Fatal("available hybrid not suggested", err, out)
	}
	b := out.Results[0].Setup.Booking
	if b.VariantID != c.ID || b.CrossesVariantID != f.variant.ID {
		t.Fatal("hybrid components were reversed or unoffered", b)
	}
	if _, err = f.store.CreateQuote(ctx, f.customer, f.shop.ID, b); err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.Pool.Exec(ctx, `UPDATE inventory_lots SET quantity=0 WHERE id=$1`, clot.ID); err != nil {
		t.Fatal(err)
	}
	out, err = f.store.Recommend(ctx, f.shop.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range out.Results {
		if rec.Setup.Booking.Hybrid {
			t.Fatal("empty tracked portion recommended")
		}
	}
	if _, err = f.store.CreateQuote(ctx, f.customer, f.shop.ID, b); err == nil {
		t.Fatal("stale recommended hybrid quoted")
	}
}
