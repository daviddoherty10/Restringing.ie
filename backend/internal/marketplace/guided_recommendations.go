package marketplace

import (
	"fmt"
	"slices"
	"sort"
)

// SuggestedSetup is an editable proposal, not a persisted quote or reservation.
// Booking contains every technical choice. Only the quote/order paths authorise
// a purchase, rechecking current prices, service constraints and shared stock.
type SuggestedSetup struct {
	Booking             Booking     `json:"booking"`
	CrossesVariant      *Variant    `json:"crosses_variant,omitempty"`
	Summary             string      `json:"summary"`
	Explanations        []string    `json:"explanations"`
	EstimatedTotalCents int         `json:"estimated_total_cents"`
	Breakdown           []PriceLine `json:"breakdown"`
}

func softMaterial(material string) bool {
	return slices.Contains([]string{"multifilament", "synthetic_gut", "natural_gut", "nylon"}, material)
}

func startingBooking(req RecommendationRequest, r Racket, s Service) (Booking, bool) {
	b := Booking{RacketModelID: r.ID, StringSource: "stock"}
	if s.TwoKnots && slices.Contains(r.AllowedKnots, 2) {
		b.Knots = 2
	}
	if s.FourKnots && slices.Contains(r.AllowedKnots, 4) && (b.Knots == 0 || s.FourKnotsCents < s.TwoKnotsCents) {
		b.Knots = 4
	}
	if b.Knots == 0 || s.Step <= 0 || r.Step <= 0 {
		return b, false
	}
	// Enumerate the small validated centi-kg range instead of guessing a step.
	valid := []int{}
	for n := max(s.MinTension, r.MinTension); n <= min(s.MaxTension, r.MaxTension) && n <= 10000; n++ {
		if n%s.Step == 0 && n%r.Step == 0 {
			valid = append(valid, n)
		}
	}
	if len(valid) == 0 {
		return b, false
	}
	tension := valid[(len(valid)-1)/2]
	if r.Sport == "tennis" {
		// Prefer 22 kg for tennis, choosing the nearest legal increment (lower tie).
		tension = valid[0]
		distance := func(n int) int {
			if n < 2200 {
				return 2200 - n
			}
			return n - 2200
		}
		for _, n := range valid {
			if distance(n) < distance(tension) {
				tension = n
			}
		}
	}
	if slices.Contains(valid, req.CurrentTension) {
		tension = req.CurrentTension
	}
	b.MainsTension = tension
	b.CrossesTension = tension
	return b, true
}

func suggestedSetup(req RecommendationRequest, b Booking, r Racket, s Service, v Variant, c *Variant) *SuggestedSetup {
	b.VariantID = v.ID
	if c != nil {
		b.Hybrid = true
		b.CrossesVariantID = c.ID
		b.Knots = 4
	}
	q, err := priceComponents(b, s, r, &v, c, nil)
	if err != nil {
		return nil
	}
	summary := "One string throughout, with a balanced starting tension."
	reasons := []string{"One string throughout keeps this setup straightforward.", "The stringing method is chosen automatically from this racket and stringer's supported options."}
	if c != nil {
		summary = "Two strings: spin-focused mains with a softer-feel cross string."
		reasons[0] = "For regular, experienced play: combine polyester along the racket for spin and control with a softer-feel material across it. This still contains polyester."
	} else if req.Experience == "new" || req.PrioritiseArmComfort {
		reasons = append(reasons, "Your answers favour a softer-feel material over a full polyester setup; this is a playing preference, not a medical assessment.")
	}
	tensionReason := fmt.Sprintf("Equal tension in both directions: %.2f kg, chosen within the verified racket and stringer's shared range.", float64(b.MainsTension)/100)
	if b.MainsTension == req.CurrentTension {
		tensionReason = fmt.Sprintf("Keep your known tension of %.2f kg: it satisfies this racket and stringer's shared limits.", float64(b.MainsTension)/100)
	}
	reasons = append(reasons, tensionReason, "No pre-stretch: this optional extra treatment is left off for this starting setup.", "Standard turnaround, no stencil or new grip. You can add offered extras separately.")
	return &SuggestedSetup{Booking: b, CrossesVariant: c, Summary: summary, Explanations: reasons, EstimatedTotalCents: q.TotalCents, Breakdown: q.Breakdown}
}

// CompleteRecommendations retains transparent catalogue scores but presents
// complete, validated bookings, with distinct product alternatives.
func CompleteRecommendations(req RecommendationRequest, candidates, hybridCandidates []Variant, current *Variant, r Racket, s Service) Recommendations {
	out := Recommendations{Algorithm: "guided-setup-v3", Results: []Recommendation{}, Notice: "Starting setups from this stringer's available offerings. Catalogue ratings are editorial, not measured guarantees. Tension uses verified racket/service limits; the stringer should confirm suitability. Prices are estimates until a fresh server quote. Answers are not saved; no medical assessment is made."}
	base, ok := startingBooking(req, r, s)
	if !ok {
		out.Notice = "No compatible starting setup is available for this racket. Ask the stringer to review it."
		return out
	}
	eligible := []Variant{}
	for _, v := range candidates {
		if v.Sport != r.Sport || v.Material == "hybrid" {
			continue
		}
		if (req.Experience == "new" || req.PrioritiseArmComfort) && !softMaterial(v.Material) {
			continue
		}
		eligible = append(eligible, v)
	}
	// A hybrid is an intentional spin/feel compromise, not the beginner default.
	if req.Experience != "new" && !req.PrioritiseArmComfort && req.SessionsPerWeek >= 2 && req.Preferences["spin"] >= 4 && req.Preferences["comfort"] >= 3 && s.Hybrid && s.FourKnots && slices.Contains(r.AllowedKnots, 4) {
		mains, crosses := []Variant{}, []Variant{}
		for _, v := range hybridCandidates {
			if v.Sport != r.Sport || !v.Available || v.InventoryID == "" {
				continue
			}
			if v.Material == "polyester" {
				mains = append(mains, v)
			} else if softMaterial(v.Material) {
				crosses = append(crosses, v)
			}
		}
		mr := rankAll(req, mains, current)
		orderSetupCandidates(mr.Results, r)
		cr := req
		cr.Preferences = map[string]int{"comfort": 5, "power": 3, "control": 1, "spin": 0, "durability": 1}
		crossRank := rankAll(cr, crosses, nil)
		orderSetupCandidates(crossRank.Results, r)
		if len(mr.Results) > 0 && len(crossRank.Results) > 0 {
			m, c := mr.Results[0], crossRank.Results[0].Variant
			if setup := suggestedSetup(req, base, r, s, m.Variant, &c); setup != nil {
				m.Setup = setup
				out.Results = append(out.Results, m)
			}
		}
	}
	ranked := rankAll(req, eligible, current)
	orderSetupCandidates(ranked.Results, r)
	used := map[string]bool{}
	for _, rec := range ranked.Results {
		product := rec.Variant.ProductID
		if product == "" {
			product = rec.Variant.ID
		}
		if used[product] {
			continue
		}
		rec.Setup = suggestedSetup(req, base, r, s, rec.Variant, nil)
		if rec.Setup == nil {
			continue
		}
		used[product] = true
		out.Results = append(out.Results, rec)
		if len(out.Results) == 3 {
			break
		}
	}
	if len(out.Results) == 0 {
		out.Notice = "This stringer has no available string that matches this starting setup. Ask them for help or choose another stringer; no unsuitable substitute was selected."
	}
	return out
}

func orderSetupCandidates(results []Recommendation, r Racket) {
	// Prefer a representative gauge when the product score is tied. Price breaks
	// otherwise equal matches; it never substitutes for suitability.
	sort.SliceStable(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		distance := func(v Variant) float64 {
			target := 1.30
			if v.Material == "polyester" {
				target = 1.25
			}
			d := v.GaugeMM - target
			if d < 0 {
				return -d
			}
			return d
		}
		if r.Sport == "tennis" && distance(a.Variant) != distance(b.Variant) {
			return distance(a.Variant) < distance(b.Variant)
		}
		if a.Variant.MaterialPriceCents != b.Variant.MaterialPriceCents {
			return a.Variant.MaterialPriceCents < b.Variant.MaterialPriceCents
		}
		return a.Variant.ID < b.Variant.ID
	})
}
