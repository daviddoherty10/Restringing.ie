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
	if r.ID == "" {
		b.Sport = r.Sport
		b.TensionPending = true
	}
	if req.junior() {
		b.TensionPending = true
	}
	if s.TwoKnots && slices.Contains(r.AllowedKnots, 2) {
		b.Knots = 2
	}
	if s.FourKnots && slices.Contains(r.AllowedKnots, 4) && (b.Knots == 0 || s.FourKnotsCents < s.TwoKnotsCents) {
		b.Knots = 4
	}
	low, high := max(s.MinTension, r.MinTension), min(s.MaxTension, r.MaxTension)
	if b.Knots == 0 || s.Step <= 0 || r.Step <= 0 || low > high {
		return b, false
	}
	if req.TensionBand != "" {
		b.TensionBand, b.TensionPending = req.TensionBand, true
		return b, true
	}
	if r.Sport == "tennis" && (req.TensionHelp || req.CurrentTensionLB > 0 || req.CurrentTension > 0 || juniorSpin(req, r) || req.AgeGroup == "adult" || req.AgeGroup == "15_to_17") {
		b.TensionPending = true
		if req.TensionHelp {
			return b, true
		}
		pounds := req.CurrentTensionLB
		switch {
		case req.CurrentTension > 0:
			n := req.CurrentTension
			if n >= low && n <= high && n%s.Step == 0 && n%r.Step == 0 {
				b.MainsTension, b.CrossesTension, b.TensionPending = n, n, false
			}
		case pounds > 0 || !juniorSpin(req, r):
			if pounds == 0 {
				pounds = 4800
			}
			if converted := float64(pounds) * kgPerPound; converted >= float64(low) && converted <= float64(high) {
				b.MainsTensionLB, b.CrossesTensionLB, b.TensionPending = pounds, pounds, false
			}
		}
		return b, true
	}
	// Enumerate the small validated centi-kg range instead of guessing a step.
	valid := []int{}
	for n := low; n <= high && n <= 10000; n++ {
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
	if !b.TensionPending {
		b.MainsTension = tension
		b.CrossesTension = tension
	}
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
	if c != nil && c.Material == "synthetic_gut" && wantsSofterPoly(req, &v) {
		summary = "Your current polyester in the mains with synthetic gut crosses."
		reasons[0] = "Keep the polyester you know in the mains and try synthetic gut in the crosses for a softer feel."
	}
	if c != nil && v.Material == "multifilament" {
		summary = "Multifilament mains with polyester crosses for more control."
		reasons[0] = "Keep the feel of multifilament in the mains and try polyester in the crosses for more control."
	}
	if c != nil && req.olderJunior() {
		summary = "Round polyester mains with synthetic gut crosses."
		reasons[0] = "For a competitive junior who regularly breaks synthetic gut, this pairs a round polyester with a high catalogue comfort rating with synthetic gut crosses. Ask your stringer to confirm this setup."
	}
	if c != nil && adultBalanced(req, r) {
		summary = "Polyester mains with synthetic gut crosses."
		reasons[0] = "Polyester mains give you control and spin, with synthetic gut crosses for a softer feel than a full bed of polyester."
	}
	tensionReason := fmt.Sprintf("Equal tension in both directions: %.2f kg, chosen within the verified racket and stringer's shared range.", float64(b.MainsTension)/100)
	if !b.TensionPending && b.MainsTension > 0 && b.MainsTension == req.CurrentTension {
		tensionReason = fmt.Sprintf("Keep your known tension of %.2f kg: it satisfies this racket and stringer's shared limits.", float64(b.MainsTension)/100)
	}
	if b.MainsTensionLB > 0 {
		if req.CurrentTensionLB > 0 {
			tensionReason = fmt.Sprintf("Keep your known tension of %g lb in both directions.", float64(b.MainsTensionLB)/100)
		} else {
			tensionReason = "Start at 48 lb in both directions, within the stringer's tension limits."
		}
	}
	if b.TensionPending {
		tensionReason = "Your stringer will help choose tension and confirm the stringing pattern after inspecting your racket."
		if !req.TensionHelp && (req.CurrentTension > 0 || req.CurrentTensionLB > 0 || !juniorSpin(req, r) && (req.AgeGroup == "adult" || req.AgeGroup == "15_to_17")) {
			tensionReason = "The requested or starting tension is outside the supported settings. Agree a tension with your stringer."
		}
		if b.TensionBand != "" {
			tensionReason = fmt.Sprintf("Ask your stringer to aim for the %s part of this racket’s appropriate tension range.", b.TensionBand)
		}
		if c == nil {
			summary = "Strings matched to your play; tension to agree with your stringer."
		}
		reasons[1] = "Stringing method is a service preference, to be confirmed by the stringer."
	}
	reasons = append(reasons, tensionReason, "No pre-stretch: this optional extra treatment is left off for this starting setup.", "Standard turnaround, no stencil or new grip. You can add offered extras separately.")
	return &SuggestedSetup{Booking: b, CrossesVariant: c, Summary: summary, Explanations: reasons, EstimatedTotalCents: q.TotalCents, Breakdown: q.Breakdown}
}

// CompleteRecommendations retains transparent catalogue scores but presents
// complete, validated bookings, with distinct product alternatives.
func CompleteRecommendations(req RecommendationRequest, candidates, hybridCandidates []Variant, current *Variant, r Racket, s Service) Recommendations {
	return completeRecommendations(req, candidates, hybridCandidates, current, r, s, nil)
}

func completeRecommendations(req RecommendationRequest, candidates, hybridCandidates []Variant, current *Variant, r Racket, s Service, pairAvailable func(Variant, Variant) bool) Recommendations {
	out := Recommendations{Algorithm: "guided-setup-v3", Results: []Recommendation{}, Notice: "Starting setups from this stringer's available offerings. Catalogue ratings are editorial, not measured guarantees. Tension uses verified racket/service limits; the stringer should confirm suitability. Prices are estimates until a fresh server quote. Answers are not saved; no medical assessment is made."}
	if r.ID == "" {
		out.Notice = "Strings matched to your sport and preferences. Tension and stringing pattern need confirmation by your stringer after inspecting your racket. Catalogue ratings are editorial; prices are estimates until a fresh quote. Finder answers are not saved."
	}
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
		if req.junior() && v.Material != "synthetic_gut" && v.Material != "multifilament" {
			continue
		}
		if (req.Experience == "new" || req.PrioritiseArmComfort) && !softMaterial(v.Material) {
			continue
		}
		eligible = append(eligible, v)
	}
	if juniorSpin(req, r) {
		poly := []Variant{}
		for _, v := range candidates {
			if v.Sport != r.Sport || v.Material != "polyester" {
				continue
			}
			if current != nil && current.Material == "polyester" && current.Shape == "round" {
				if v.Comfort <= current.Comfort {
					continue
				}
			} else if v.Comfort < 4 {
				continue
			}
			poly = append(poly, v)
		}
		spin := req
		spin.Experience = "club"
		spin.Preferences = map[string]int{"spin": 5, "comfort": 0, "control": 0, "power": 0, "durability": 0}
		ranked := rankAll(spin, poly, nil)
		orderSetupCandidates(ranked.Results, r)
		for _, rec := range ranked.Results {
			if setup := suggestedSetup(req, base, r, s, rec.Variant, nil); setup != nil {
				setup.Summary = "A spin-friendly full bed of polyester chosen for a softer feel."
				setup.Explanations[0] = "You already play with a full bed of round polyester. This keeps one string throughout and uses the catalogue comfort and spin ratings to choose the replacement."
				rec.Setup = setup
				out.Results = append(out.Results, rec)
				return out
			}
		}
		out.Notice += " There is no stocked polyester that fits the requested softer spin setup. Compare the softer full-bed options or ask the stringer."
	}
	softerPoly := r.Sport == "tennis" && wantsSofterPoly(req, current)
	reverse := r.Sport == "tennis" && wantsMultiControl(req, current)
	if guidedHybridEligible(req, current, r, s) {
		mains, crosses := []Variant{}, []Variant{}
		for _, v := range hybridCandidates {
			if v.Sport != r.Sport || !v.Available || v.InventoryID == "" {
				continue
			}
			switch {
			case adultBalanced(req, r):
				if v.Material == "polyester" {
					mains = append(mains, v)
				} else if v.Material == "synthetic_gut" {
					crosses = append(crosses, v)
				}
			case req.olderJunior():
				if v.Material == "polyester" && v.Shape == "round" && v.Comfort >= 4 {
					mains = append(mains, v)
				} else if v.Material == "synthetic_gut" {
					crosses = append(crosses, v)
				}
			case softerPoly:
				if sameStringProduct(v, current) {
					mains = append(mains, v)
				} else if v.Material == "synthetic_gut" {
					crosses = append(crosses, v)
				}
			case reverse:
				if sameStringProduct(v, current) {
					mains = append(mains, v)
				} else if v.Material == "polyester" {
					crosses = append(crosses, v)
				}
			case v.Material == "polyester":
				mains = append(mains, v)
			case softMaterial(v.Material):
				crosses = append(crosses, v)
			}
		}
		mr := rankAll(req, mains, current)
		orderSetupCandidates(mr.Results, r)
		cr := req
		cr.Preferences = map[string]int{"comfort": 5, "power": 3, "control": 1, "spin": 0, "durability": 1}
		if reverse {
			cr = req
		}
		crossRank := rankAll(cr, crosses, nil)
		orderSetupCandidates(crossRank.Results, r)
	findPair:
		for _, m := range mr.Results {
			for _, cross := range crossRank.Results {
				c := cross.Variant
				if setup := suggestedSetup(req, base, r, s, m.Variant, &c); setup != nil && (pairAvailable == nil || pairAvailable(m.Variant, c)) {
					m.Setup = setup
					out.Results = append(out.Results, m)
					break findPair
				}
			}
		}
	}
	if adultBalanced(req, r) && len(out.Results) == 0 {
		out.Notice += " Polyester mains with synthetic gut crosses aren't available as a hybrid here. Compare the available full beds."
		if comparison := balancedComparison(req, eligible, base, r, s); len(comparison) == 2 {
			out.Results = comparison
			return out
		}
	}
	if softerPoly {
		eligible = slices.DeleteFunc(eligible, func(v Variant) bool {
			return v.Material == "polyester" && v.Comfort <= current.Comfort
		})
	}
	ranked := rankAll(req, eligible, current)
	orderSetupCandidates(ranked.Results, r)
	priority := func(v Variant) bool {
		switch {
		case req.junior() || req.Experience == "new" && r.Sport == "tennis":
			return v.Material == "synthetic_gut"
		case softerPoly:
			return v.Material == "polyester"
		case current != nil && req.Preferences["durability"] == 5:
			return current.Material == "multifilament" && sameStringProduct(v, current) && v.GaugeMM > current.GaugeMM ||
				current.Material == "polyester" && current.Shape == "shaped" && v.Material == "polyester" && v.Shape == "round"
		default:
			return false
		}
	}
	sort.SliceStable(ranked.Results, func(i, j int) bool {
		return priority(ranked.Results[i].Variant) && !priority(ranked.Results[j].Variant)
	})
	if current != nil && current.Material == "multifilament" && req.Preferences["durability"] == 5 && !req.junior() {
		out.Notice += " If you are breaking multifilament crosses, ask your stringer whether a thicker gauge or replacing shaped polyester mains with round polyester would suit your setup."
	}
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
		if req.youngerJunior() {
			out.Notice = "For a younger junior player, we suggest a full bed of synthetic gut first, with multifilament as the fallback. Your stringer will confirm tension."
			break
		}
		if len(out.Results) == 3 {
			break
		}
	}
	if req.youngerJunior() && len(out.Results) == 0 {
		out.Notice = "No suitable synthetic gut or multifilament is available for this junior setup."
	} else if len(out.Results) == 0 {
		out.Notice = "This stringer has no available string that matches this starting setup. Ask them for help or choose another stringer; no unsuitable substitute was selected."
	}
	return out
}

// Compare two stocked setup types before ranking individual strings within each.
func balancedComparison(req RecommendationRequest, candidates []Variant, base Booking, r Racket, s Service) []Recommendation {
	soft, poly := []Variant{}, []Variant{}
	for _, v := range candidates {
		if softMaterial(v.Material) {
			soft = append(soft, v)
		} else if v.Material == "polyester" {
			poly = append(poly, v)
		}
	}
	if len(soft) == 0 || len(poly) == 0 {
		return nil
	}
	controlSpin := req
	controlSpin.Preferences = map[string]int{"comfort": 1, "control": 5, "power": 1, "spin": 5, "durability": 2}
	options := []struct {
		request    RecommendationRequest
		candidates []Variant
		label      string
		summary    string
	}{
		{req, soft, "Softer feel", "A full bed chosen for a softer feel and easier depth."},
		{controlSpin, poly, "Control and spin", "A full bed of polyester chosen for control and spin."},
	}
	var comparison []Recommendation
	for _, option := range options {
		ranked := rankAll(option.request, option.candidates, nil)
		orderSetupCandidates(ranked.Results, r)
		for _, rec := range ranked.Results {
			setup := suggestedSetup(req, base, r, s, rec.Variant, nil)
			if setup == nil {
				continue
			}
			setup.Summary = option.summary
			rec.Setup, rec.Label = setup, option.label
			comparison = append(comparison, rec)
			break
		}
	}
	return comparison
}

func sameStringProduct(v Variant, current *Variant) bool {
	return current != nil && (v.ID == current.ID || current.ProductID != "" && v.ProductID == current.ProductID)
}

func wantsSofterPoly(req RecommendationRequest, current *Variant) bool {
	return current != nil && current.Material == "polyester" && req.Preferences["comfort"] == 5 &&
		req.Experience != "new" && !req.PrioritiseArmComfort && !req.junior()
}

func wantsMultiControl(req RecommendationRequest, current *Variant) bool {
	return current != nil && current.Material == "multifilament" && req.Preferences["control"] == 5
}

func adultBalanced(req RecommendationRequest, r Racket) bool {
	return r.Sport == "tennis" && req.Goal == "balanced" && (req.Experience == "club" || req.Experience == "advanced") && !req.junior() && !req.PrioritiseArmComfort
}

func juniorSpin(req RecommendationRequest, r Racket) bool {
	return r.Sport == "tennis" && req.olderJunior() && req.Goal == "spin" && req.Experience == "advanced" && req.RoundPolyFullBed && !req.PrioritiseArmComfort
}

func guidedHybridEligible(req RecommendationRequest, current *Variant, r Racket, s Service) bool {
	if req.youngerJunior() || req.Experience == "new" || req.PrioritiseArmComfort ||
		!s.Hybrid || !s.FourKnots || !slices.Contains(r.AllowedKnots, 4) {
		return false
	}
	if req.olderJunior() {
		return req.Experience == "advanced" && req.BreaksSyntheticGut && (req.Goal == "balanced" || req.Preferences["control"] == 5)
	}
	return adultBalanced(req, r) || r.Sport == "tennis" && (wantsSofterPoly(req, current) || wantsMultiControl(req, current)) ||
		req.Preferences["spin"] >= 4 && req.Preferences["comfort"] >= 3
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
