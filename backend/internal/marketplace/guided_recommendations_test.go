package marketplace

import (
	"context"
	"reflect"
	"strings"
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

func TestGuidedBalancedHybridAndJuniorSpinProfiles(t *testing.T) {
	_, service, racket, multi := pricingFixture()
	service.Hybrid = true
	multi.ProductID = multi.ID
	syn := multi
	syn.ID, syn.ProductID, syn.Material = "syn", "syn", "synthetic_gut"
	current := multi
	current.ID, current.ProductID, current.Material = "current", "current", "polyester"
	current.Shape, current.Comfort, current.Spin = "round", 2, 5
	soft := current
	soft.ID, soft.ProductID, soft.Comfort, soft.Spin = "soft", "soft", 4, 4
	stiff := current
	stiff.ID, stiff.ProductID, stiff.Comfort = "stiff", "stiff", 1
	stock := []Variant{multi, syn, current, soft, stiff}

	for _, level := range []string{"club", "advanced"} {
		req := RecommendationRequest{AgeGroup: "adult", Experience: level, Goal: "balanced"}
		result := CompleteRecommendations(req, stock, stock, nil, racket, service).Results
		b := result[0].Setup.Booking
		if !b.Hybrid || b.CrossesVariantID != syn.ID || b.Knots != 4 || b.TensionPending ||
			b.MainsTensionLB != 4800 || b.CrossesTensionLB != 4800 || b.MainsTension != 0 {
			t.Fatalf("%s Balanced: %+v", level, b)
		}
	}
	for _, shape := range []string{"round", "shaped"} {
		soft.Shape = shape
		req := RecommendationRequest{AgeGroup: "15_to_17", Experience: "advanced", Goal: "spin", RoundPolyFullBed: true}
		for _, pounds := range []int{0, 4725} {
			req.CurrentTensionLB = pounds
			result := CompleteRecommendations(req, []Variant{multi, current, soft, stiff}, nil, &current, racket, service).Results
			b := result[0].Setup.Booking
			want := pounds
			if want == 0 {
				want = 4800
			}
			if b.Hybrid || b.VariantID != soft.ID || b.MainsTensionLB != want || b.TensionPending {
				t.Fatalf("%s junior Spin: %+v", shape, b)
			}
		}
		req.PrioritiseArmComfort = true
		for _, rec := range CompleteRecommendations(req, stock, stock, &current, racket, service).Results {
			if rec.Setup.Booking.Hybrid || rec.Variant.Material == "polyester" {
				t.Fatal("explicit softer feel suggested polyester", rec)
			}
		}
	}
	for _, age := range []string{"13_to_14", "15_to_17", "13_to_17"} {
		b, ok := startingBooking(RecommendationRequest{AgeGroup: age}, racket, service)
		if !ok || b.TensionPending != (age != "15_to_17") {
			t.Fatalf("%s: unexpected tension %+v", age, b)
		}
	}
}

func TestIntegrationBalancedHybridQuoteUsesExactPounds(t *testing.T) {
	f, poly, _, portion := hybridFixture(t, 2, 2)
	ctx := context.Background()
	if _, err := f.store.Pool.Exec(ctx, "UPDATE string_products SET material='synthetic_gut' WHERE id=$1", f.variant.ProductID); err != nil {
		t.Fatal(err)
	}
	req := RecommendationRequest{Sport: "tennis", AgeGroup: "adult", Experience: "club", Goal: "balanced"}
	out, err := f.store.Recommend(ctx, f.shop.ID, req)
	if err != nil || len(out.Results) == 0 {
		t.Fatalf("Balanced setup: %+v, %v", out, err)
	}
	setup := out.Results[0].Setup
	quote, err := f.store.CreateQuote(ctx, f.customer, f.shop.ID, setup.Booking)
	b := quote.Configuration.Booking
	if err != nil || !b.Hybrid || b.VariantID != poly.ID || b.CrossesVariantID != f.variant.ID ||
		b.MainsTensionLB != 4800 || b.CrossesTensionLB != 4800 || quote.TotalCents != setup.EstimatedTotalCents {
		t.Fatalf("unexpected Balanced quote %+v, %v", quote, err)
	}
	if _, err := f.store.Pool.Exec(ctx, "UPDATE inventory_lots SET quantity=0 WHERE id=$1", portion.ID); err != nil {
		t.Fatal(err)
	}
	out, err = f.store.Recommend(ctx, f.shop.ID, req)
	if err != nil || len(out.Results) == 0 || out.Results[0].Setup.Booking.Hybrid {
		t.Fatalf("expected offered fallback: %+v, %v", out, err)
	}
}

func TestCalibratedTennisSetups(t *testing.T) {
	_, service, racket, template := pricingFixture()
	service.Hybrid = true
	variant := func(id, material, shape string, gauge float64) Variant {
		v := template
		v.ID, v.ProductID, v.Model = id, id, id
		v.Material, v.Shape, v.GaugeMM = material, shape, gauge
		return v
	}
	round := variant("unbranded round", "polyester", "round", 1.25)
	shaped := variant("unbranded shaped", "polyester", "shaped", 1.25)
	shaped.Spin, shaped.Comfort = 5, 3
	syn := variant("Syn Gut", "synthetic_gut", "unknown", 1.30)
	thick := variant("Syn Gut 15", "synthetic_gut", "unknown", 1.35)
	multi := variant("multi", "multifilament", "unknown", 1.35)
	nylon := variant("plain nylon", "synthetic_gut", "unknown", 1.40)
	stock := []Variant{multi, nylon, shaped, round, syn, thick}
	for _, tc := range []struct {
		name, level, goal, feedback, age string
		priorRound                       bool
		stock                            []Variant
		main, cross                      string
	}{
		{"broken crosses", "club", "durability", "breaks_crosses", "adult", false, stock, round.ID, nylon.ID},
		{"junior wants spin", "advanced", "spin", "", "15_to_17", true, stock, shaped.ID, ""},
		{"balanced", "advanced", "balanced", "", "adult", false, stock, round.ID, syn.ID},
		{"no syn gut", "club", "balanced", "", "adult", false, []Variant{multi, shaped, round}, round.ID, multi.ID},
		{"too firm", "club", "spin", "firm_poly", "adult", false, stock, round.ID, syn.ID},
		{"shots go long", "club", "control", "hybrid_long", "adult", false, stock, shaped.ID, syn.ID},
		{"declines poly", "club", "spin", "no_poly", "adult", false, stock, multi.ID, ""},
		{"beginner declines poly", "new", "spin", "no_poly", "adult", false, stock, syn.ID, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := RecommendationRequest{Goal: tc.goal, Experience: tc.level, SetupFeedback: tc.feedback, AgeGroup: tc.age, RoundPolyFullBed: tc.priorRound}
			rs := CompleteRecommendations(req, tc.stock, tc.stock, nil, racket, service).Results
			if len(rs) == 0 {
				t.Fatal("missing setup")
			}
			b := rs[0].Setup.Booking
			if b.VariantID != tc.main || b.CrossesVariantID != tc.cross || b.Hybrid != (tc.cross != "") || b.MainsTensionLB != 4800 {
				t.Fatalf("unexpected setup: %+v", b)
			}
			for _, rec := range rs {
				if tc.feedback == "no_poly" && (rec.Variant.Material == "polyester" || rec.Setup.Booking.Hybrid) {
					t.Fatal("polyester despite opt-out")
				}
			}
		})
	}
}

func TestGuidedDurabilitySelectsThickerGaugeAndCarriesTensionGuide(t *testing.T) {
	_, service, racket, thin := pricingFixture()
	thin.ProductID, thin.GaugeMM = "multi", 1.30
	thick := thin
	thick.ID, thick.GaugeMM = "thick", 1.35
	unavailable := thick
	unavailable.ID, unavailable.GaugeMM, unavailable.Available = "unavailable", 1.40, false
	req := RecommendationRequest{Goal: "power", Experience: "club", AgeGroup: "adult", PreferThicker: true, TensionBand: "higher"}
	result := CompleteRecommendations(req, []Variant{thin, thick, unavailable}, nil, nil, racket, service).Results
	if len(result) != 1 || result[0].Variant.ID != thick.ID {
		t.Fatalf("expected thickest available gauge, got %+v", result)
	}
	b := result[0].Setup.Booking
	if b.TensionBand != "higher" || !b.TensionPending || b.MainsTension != 0 || b.MainsTensionLB != 0 {
		t.Fatalf("expected a qualitative guide only, got %+v", b)
	}
	if q, err := Price(b, service, racket, &thick); err != nil || q.Configuration.TensionBand != "higher" {
		t.Fatalf("guide lost in quote: %+v, %v", q, err)
	}
	b.TensionPending, b.MainsTension, b.CrossesTension = false, 2200, 2200
	if _, err := Price(b, service, racket, &thick); err == nil {
		t.Fatal("quote accepted both an exact tension and a rough guide")
	}
}

func TestGroupedGoalMatrix(t *testing.T) {
	_, svc, r, tpl := pricingFixture()
	svc.Hybrid = true
	stock := []Variant{}
	for _, id := range []string{"round", "soft", "power", "shaped", "synthetic_gut", "multifilament", "hybrid"} {
		v := tpl
		v.ID = id
		v.ProductID = id
		v.Material = "polyester"
		v.Shape = "round"
		v.Power = 2
		v.Comfort = 3
		switch id {
		case "round":
			v.Control = 5
		case "soft":
			v.Comfort = 5
		case "power":
			v.Power = 5
		case "shaped":
			v.Shape = "shaped"
			v.Spin = 5
		default:
			v.Material = id
		}
		stock = append(stock, v)
	}
	for _, level := range []string{"class_1_4", "class_5_7", "beginner"} {
		for _, goal := range []string{"balanced", "comfort", "power", "spin"} {
			t.Run(level+"/"+goal, func(t *testing.T) {
				req := RecommendationRequest{AgeGroup: "adult", Experience: level, Goal: goal, TensionHelp: true}
				for _, soft := range []bool{false, true} {
					req.PrioritiseArmComfort = soft
					results := CompleteRecommendations(req, stock, stock, nil, r, svc).Results
					if len(results) != 1 {
						t.Fatalf("got %d results", len(results))
					}
					rec := results[0]
					b := rec.Setup.Booking
					hybrid := !soft && (goal == "balanced" || goal == "spin" && level == "beginner" || (goal == "comfort" || goal == "power") && level == "class_1_4")
					if hybrid != b.Hybrid || !b.TensionPending {
						t.Fatal(b)
					}
					if hybrid {
						if b.CrossesVariantID != "synthetic_gut" || rec.Variant.Material != "polyester" || goal == "comfort" && rec.Variant.ID != "soft" || goal == "power" && rec.Variant.ID != "power" {
							t.Fatal(b)
						}
						continue
					}
					want := "multifilament"
					if level == "beginner" {
						want = "synthetic_gut"
					} else if goal == "spin" && !soft {
						want = "shaped"
					}
					if b.VariantID != want {
						t.Fatalf("got %s want %s", b.VariantID, want)
					}
				}
			})
		}
	}
}

func TestSpinUsesAvailablePolyesterWhenShapeIsMissing(t *testing.T) {
	_, service, racket, v := pricingFixture()
	v.Material = "polyester"
	v.Spin = 5
	v.Shape = "unknown"
	shaped := v
	shaped.ID = "shaped"
	shaped.Shape = "shaped"
	shaped.Spin = 3
	req := RecommendationRequest{Sport: "tennis", AgeGroup: "adult", Experience: "class_5_7", Goal: "spin"}
	for _, tc := range []struct {
		stock             []Variant
		want, description string
	}{
		{[]Variant{v, shaped}, shaped.ID, "shaped polyester"},
		{[]Variant{v}, v.ID, "unrecorded shape"},
	} {
		results := CompleteRecommendations(req, tc.stock, nil, nil, racket, service).Results
		if len(results) != 1 || results[0].Variant.ID != tc.want || !strings.Contains(results[0].Setup.Summary, tc.description) {
			t.Fatal(results)
		}
	}
}
