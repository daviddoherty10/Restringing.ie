package marketplace

import (
	"errors"
	"strings"
	"testing"
)

func pricingFixture() (Booking, Service, Racket, Variant) {
	b := Booking{RacketModelID: "racket", StringSource: "stock", VariantID: "variant", MainsTension: 2400, CrossesTension: 2400, Knots: 4}
	s := Service{Sport: "tennis", Active: true, LabourCents: 1500, OwnStrings: true, OwnStringsCents: 50, Stencil: true, StencilCents: 300, TwoKnots: true, FourKnots: true, FourKnotsCents: 200, DifferentTensions: true, DifferentTensionsCents: 100, MinTension: 1500, MaxTension: 3500, Step: 50, MaxDifference: 300, Prestretch: map[int]int{5: 400, 10: 500}}
	r := Racket{Brand: "Fixture", Model: "Test racket", Sport: "tennis", Verified: true, MinTension: 1800, MaxTension: 3000, Step: 50, MaxDifference: 200, AllowedKnots: []int{2, 4}, DifferentRequiresFour: true, StringLengthMM: 12000, TechnicalSource: "fictional test fixture"}
	v := Variant{ID: "variant", Brand: "Fixture", Model: "Soft", Sport: "tennis", GaugeMM: 1.25, Material: "multifilament", InventoryID: "inventory", Available: true, MaterialPriceCents: 1200, MaxPrestretchPercent: 10, Comfort: 5, Control: 3, Power: 4, Spin: 2, Durability: 2}
	return b, s, r, v
}
func TestPriceBreakdownAndOwnStrings(t *testing.T) {
	b, s, r, v := pricingFixture()
	b.CrossesTension = 2300
	b.Stencil = true
	b.PrestretchPercent = 5
	q, err := Price(b, s, r, &v)
	if err != nil {
		t.Fatal(err)
	}
	if q.TotalCents != 3700 || q.Currency != "EUR" || len(q.Breakdown) != 6 {
		t.Fatalf("unexpected quote: %+v", q)
	}
	if !strings.Contains(q.Configuration.StringSnapshot, "Fixture Soft") {
		t.Fatal("missing immutable string description")
	}
	b.StringSource = "customer"
	b.VariantID = ""
	b.Stencil = false
	b.PrestretchPercent = 0
	b.CrossesTension = b.MainsTension
	q, err = Price(b, s, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	if q.TotalCents != 1750 {
		t.Fatalf("own strings price %d", q.TotalCents)
	}
}
func TestTechnicalValidation(t *testing.T) {
	cases := []struct {
		name, field string
		change      func(*Booking, *Service, *Racket, *Variant)
	}{
		{"racket maximum", "mains_tension_centi_kg", func(b *Booking, s *Service, r *Racket, v *Variant) { b.MainsTension = 3100 }},
		{"increments", "mains_tension_centi_kg", func(b *Booking, s *Service, r *Racket, v *Variant) { b.MainsTension = 2425 }},
		{"unsupported sport", "variant_id", func(b *Booking, s *Service, r *Racket, v *Variant) { v.Sport = "badminton" }},
		{"unknown model", "racket_model_id", func(b *Booking, s *Service, r *Racket, v *Variant) { r.Verified = false }},
		{"service disabled", "racket_model_id", func(b *Booking, s *Service, r *Racket, v *Variant) { s.Active = false }},
		{"no own strings", "string_source", func(b *Booking, s *Service, r *Racket, v *Variant) { s.OwnStrings = false; b.StringSource = "customer" }},
		{"unavailable string", "variant_id", func(b *Booking, s *Service, r *Racket, v *Variant) { v.Available = false }},
		{"two knots not offered", "knots", func(b *Booking, s *Service, r *Racket, v *Variant) { b.Knots = 2; s.TwoKnots = false }},
		{"manufacturer four knot rule", "knots", func(b *Booking, s *Service, r *Racket, v *Variant) { r.AllowedKnots = []int{4}; b.Knots = 2 }},
		{"different tensions require four", "knots", func(b *Booking, s *Service, r *Racket, v *Variant) { b.CrossesTension = 2300; b.Knots = 2 }},
		{"tension difference too large", "crosses_tension_centi_kg", func(b *Booking, s *Service, r *Racket, v *Variant) { b.CrossesTension = 2100 }},
		{"different tensions not offered", "crosses_tension_centi_kg", func(b *Booking, s *Service, r *Racket, v *Variant) {
			b.CrossesTension = 2300
			s.DifferentTensions = false
		}},
		{"stencil unavailable", "stencil", func(b *Booking, s *Service, r *Racket, v *Variant) { b.Stencil = true; s.Stencil = false }},
		{"prestretch unavailable", "prestretch_percent", func(b *Booking, s *Service, r *Racket, v *Variant) { b.PrestretchPercent = 15 }},
		{"product prestretch restriction", "prestretch_percent", func(b *Booking, s *Service, r *Racket, v *Variant) {
			b.PrestretchPercent = 5
			v.MaxPrestretchPercent = 0
		}},
		{"instructions too long", "instructions", func(b *Booking, s *Service, r *Racket, v *Variant) { b.Instructions = strings.Repeat("é", 501) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, s, r, v := pricingFixture()
			tc.change(&b, &s, &r, &v)
			_, err := Price(b, s, r, &v)
			var p *Problem
			if !errors.As(err, &p) || p.Fields[tc.field] == "" {
				t.Fatalf("expected field %s, got %v", tc.field, err)
			}
		})
	}
	b, s, r, _ := pricingFixture()
	b.StringSource = "customer"
	b.VariantID = ""
	b.PrestretchPercent = 5
	if _, err := Price(b, s, r, nil); err == nil {
		t.Fatal("unknown supplied strings must not receive pre-stretch")
	}
}
func TestBadmintonHasItsOwnTensionRange(t *testing.T) {
	b, s, r, v := pricingFixture()
	s.Sport = "badminton"
	r.Sport = "badminton"
	v.Sport = "badminton"
	s.MinTension = 700
	s.MaxTension = 1400
	r.MinTension = 800
	r.MaxTension = 1300
	b.MainsTension = 1100
	b.CrossesTension = 1100
	if _, err := Price(b, s, r, &v); err != nil {
		t.Fatal(err)
	}
}
func TestFingerprintChangesWhenTermsChange(t *testing.T) {
	b, s, r, v := pricingFixture()
	first, _ := Price(b, s, r, &v)
	v.MaterialPriceCents++
	second, _ := Price(b, s, r, &v)
	if first.Fingerprint == second.Fingerprint {
		t.Fatal("price change did not invalidate quote")
	}
	v.MaterialPriceCents--
	r.MaxTension--
	third, _ := Price(b, s, r, &v)
	if first.Fingerprint == third.Fingerprint {
		t.Fatal("technical change did not invalidate quote")
	}
}
func TestStatusTransitions(t *testing.T) {
	for _, tc := range []struct {
		from, to      string
		manager, want bool
	}{
		{"placed", "accepted", true, true}, {"placed", "accepted", false, false}, {"placed", "completed", true, false},
		{"placed", "cancelled", false, true}, {"accepted", "cancelled", false, false}, {"received", "cancelled", true, true},
		{"in_progress", "cancelled", true, false}, {"completed", "ready", true, false}, {"ready", "completed", true, true},
	} {
		if got := CanTransition(tc.from, tc.to, tc.manager); got != tc.want {
			t.Errorf("%+v got %v", tc, got)
		}
	}
}
func TestRecommendationsAreTransparentAndStable(t *testing.T) {
	_, _, _, v := pricingFixture()
	v.ID = "b"
	v2 := v
	v2.ID = "a"
	hidden := v
	hidden.ID = "hidden"
	hidden.Available = false
	req := RecommendationRequest{Experience: "new", SessionsPerWeek: 4, Preferences: map[string]int{"comfort": 5, "control": 0}, PrioritiseArmComfort: true}
	if err := ValidateRecommendation(req); err != nil {
		t.Fatal(err)
	}
	out := Rank(req, []Variant{v, hidden, v2}, &v)
	if len(out.Results) != 2 || out.Results[0].Variant.ID != "a" || len(out.Results[0].Reasons) < 6 || out.Algorithm == "" {
		t.Fatalf("unexpected recommendation %+v", out)
	}
	req.Preferences["diagnosis"] = 5
	if ValidateRecommendation(req) == nil {
		t.Fatal("unknown health field accepted")
	}
}
