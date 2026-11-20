package marketplace

import (
	"context"
	"reflect"
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
	req := RecommendationRequest{Experience: "club", Preferences: map[string]int{"spin": 5, "comfort": 3}}
	out := CompleteRecommendations(req, []Variant{v, c}, []Variant{v, c}, nil, r, s)
	if len(out.Results) == 0 || !out.Results[0].Setup.Booking.Hybrid {
		t.Fatalf("expected explained hybrid %+v", out)
	}
	req.SessionsPerWeek = 7
	if morePlay := CompleteRecommendations(req, []Variant{v, c}, []Variant{v, c}, nil, r, s); !reflect.DeepEqual(out, morePlay) {
		t.Fatal("weekly play frequency changed the recommendations")
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

func TestTennisStartingTensionPrefers22AndRespectsLimits(t *testing.T) {
	_, s, r, _ := pricingFixture()
	req := RecommendationRequest{Experience: "new"}
	b, ok := startingBooking(req, r, s)
	if !ok || b.MainsTension != 2200 || b.CrossesTension != 2200 {
		t.Fatal("22 kg default missing", b)
	}
	r.MinTension = 2400
	b, ok = startingBooking(req, r, s)
	if !ok || b.MainsTension != 2400 {
		t.Fatal("lower model limit ignored", b)
	}
	r.MinTension = 1800
	r.MaxTension = 2100
	b, ok = startingBooking(req, r, s)
	if !ok || b.MainsTension != 2100 {
		t.Fatal("upper model limit ignored", b)
	}
	r.MaxTension = 3000
	r.Sport = "badminton"
	r.MinTension = 800
	r.MaxTension = 1400
	s.MinTension = 800
	s.MaxTension = 1400
	b, ok = startingBooking(req, r, s)
	if !ok || b.MainsTension != 1100 {
		t.Fatal("tennis default leaked into badminton", b)
	}
}

func TestIntegrationKnotChoicesIntersectServiceAndRacket(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	options, err := f.store.BookingOptions(ctx, f.shop.ID, f.racket.ID, "")
	if err != nil || len(options.AllowedKnots) != 2 || options.AllowedKnots[0] != 2 || options.AllowedKnots[1] != 4 {
		t.Fatal("compatible two-knot option missing", options, err)
	}
	for _, knots := range []int{2, 4} {
		b := f.booking
		b.Knots = knots
		if _, err = f.store.CreateQuote(ctx, f.customer, f.shop.ID, b); err != nil {
			t.Fatal("offered compatible method cannot be quoted", knots, err)
		}
	}
	if _, err = f.store.Pool.Exec(ctx, `UPDATE racket_models SET allowed_knots=ARRAY[4] WHERE id=$1`, f.racket.ID); err != nil {
		t.Fatal(err)
	}
	options, err = f.store.BookingOptions(ctx, f.shop.ID, f.racket.ID, "")
	if err != nil || !options.Service.TwoKnots || len(options.AllowedKnots) != 1 || options.AllowedKnots[0] != 4 {
		t.Fatal("racket restriction was not applied", options, err)
	}
	b := f.booking
	b.Knots = 2
	if _, err = f.store.CreateQuote(ctx, f.customer, f.shop.ID, b); err == nil {
		t.Fatal("racket restriction bypassed")
	}
}

func TestGuidedYoungerJuniorPrefersSyntheticGutThenMultifilament(t *testing.T) {
	_, service, racket, multi := pricingFixture()
	service.Hybrid = true
	racket.ID = ""
	syn := multi
	syn.ID = "synthetic-gut"
	syn.Material = "synthetic_gut"
	syn.Comfort = 1
	poly := multi
	poly.ID = "polyester"
	poly.Material = "polyester"
	req := RecommendationRequest{
		AgeGroup: "under_13", Experience: "advanced",
		Preferences: map[string]int{"spin": 5, "comfort": 5},
	}
	for _, tc := range []struct {
		name  string
		stock []Variant
		want  string
	}{
		{"synthetic gut first", []Variant{multi, syn, poly}, syn.ID},
		{"multifilament fallback", []Variant{multi, poly}, multi.ID},
		{"no suitable material", []Variant{poly}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := CompleteRecommendations(req, tc.stock, []Variant{poly, syn}, nil, racket, service)
			if tc.want == "" {
				if len(out.Results) != 0 {
					t.Fatal("recommended an unsuitable substitute", out.Results)
				}
				return
			}
			if len(out.Results) != 1 {
				t.Fatalf("expected one junior setup, got %d", len(out.Results))
			}
			booking := out.Results[0].Setup.Booking
			if booking.VariantID != tc.want || booking.Hybrid || !booking.TensionPending {
				t.Fatalf("unexpected junior setup: %+v", booking)
			}
		})
	}
}

func TestGuidedHybridUsesCurrentStringAndJuniorShape(t *testing.T) {
	_, service, racket, multi := pricingFixture()
	service.Hybrid = true
	racket.ID = ""
	multi.ProductID = "multi"
	syn := multi
	syn.ID, syn.ProductID, syn.Material = "syn", "syn", "synthetic_gut"
	poly := multi
	poly.ID, poly.ProductID, poly.Material, poly.Shape, poly.Comfort = "poly", "poly", "polyester", "shaped", 2
	round := poly
	round.ID, round.ProductID, round.Shape, round.Comfort = "round", "round", "round", 4
	round.Control = 5
	unknown := round
	unknown.ID, unknown.ProductID, unknown.Shape, unknown.Comfort = "unknown", "unknown", "unknown", 5
	unknown.Control = 1
	stock := []Variant{multi, syn, poly, round, unknown}

	for _, tc := range []struct {
		name           string
		req            RecommendationRequest
		current        *Variant
		mains, crosses string
	}{
		{"softer than current poly", RecommendationRequest{Experience: "club", Preferences: map[string]int{"comfort": 5}}, &poly, poly.ID, syn.ID},
		{"control with current multi", RecommendationRequest{Experience: "advanced", Preferences: map[string]int{"control": 5}}, &multi, multi.ID, round.ID},
		{"competitive teenage junior", RecommendationRequest{Experience: "advanced", Preferences: map[string]int{"control": 5}, AgeGroup: "13_to_17", BreaksSyntheticGut: true}, nil, round.ID, syn.ID},
		{"junior without frequent breakage", RecommendationRequest{Experience: "advanced", Preferences: map[string]int{"control": 5}, AgeGroup: "13_to_17"}, nil, syn.ID, ""},
		{"beginner easy depth", RecommendationRequest{Experience: "new", Preferences: map[string]int{"power": 5}}, nil, syn.ID, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := CompleteRecommendations(tc.req, stock, stock, tc.current, racket, service)
			if len(out.Results) == 0 {
				t.Fatal("no recommendation")
			}
			b := out.Results[0].Setup.Booking
			if b.VariantID != tc.mains || b.CrossesVariantID != tc.crosses || b.Hybrid != (tc.crosses != "") || !b.TensionPending {
				t.Fatalf("unexpected setup %+v", b)
			}
			if tc.current == &poly && tc.crosses != "" {
				v := out.Results[1].Variant
				if v.Material != "polyester" || v.Comfort <= poly.Comfort || out.Results[1].Setup.Booking.Hybrid {
					t.Fatalf("expected a softer full bed of poly, got %+v", v)
				}
			}
		})
	}
}

func TestAdvancedBalancedComparesStockedSoftAndControlSpinSetups(t *testing.T) {
	_, service, racket, multi := pricingFixture()
	multi.ProductID = "soft"
	poly := multi
	poly.ID, poly.ProductID, poly.Material = "poly", "poly", "polyester"
	poly.Comfort, poly.Control, poly.Spin = 1, 5, 5
	softerPoly := poly
	softerPoly.ID, softerPoly.ProductID = "soft-poly", "soft-poly"
	softerPoly.Comfort, softerPoly.Control, softerPoly.Spin = 5, 3, 3
	req := RecommendationRequest{Goal: "balanced", Experience: "advanced", AgeGroup: "adult"}
	stock := []Variant{multi, poly, softerPoly}
	result := CompleteRecommendations(req, stock, nil, nil, racket, service).Results
	if len(result) != 2 || result[0].Variant.ID != multi.ID || result[1].Variant.ID != poly.ID {
		t.Fatalf("expected soft and control-spin options, got %+v", result)
	}
	for _, rec := range result {
		if rec.Label == "" || rec.Setup == nil || rec.Setup.Booking.Hybrid || rec.Setup.Booking.VariantID != rec.Variant.ID {
			t.Fatalf("invalid comparison result %+v", rec)
		}
	}
	poly.Available, softerPoly.Available = false, false
	result = CompleteRecommendations(req, []Variant{multi, poly, softerPoly}, nil, nil, racket, service).Results
	if len(result) != 1 || result[0].Label != "" || result[0].Variant.ID != multi.ID {
		t.Fatalf("expected one fallback without a stocked polyester, got %+v", result)
	}
	req.PrioritiseArmComfort = true
	result = CompleteRecommendations(req, stock, nil, nil, racket, service).Results
	if len(result) != 1 || result[0].Variant.ID != multi.ID {
		t.Fatalf("expected softer-feel preference to keep the soft option, got %+v", result)
	}
}
