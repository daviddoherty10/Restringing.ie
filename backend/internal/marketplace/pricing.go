package marketplace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

func validSport(s string) bool { return s == "tennis" || s == "badminton" || s == "squash" }
func validMaterial(s string) bool {
	return slices.Contains([]string{"multifilament", "synthetic_gut", "polyester", "natural_gut", "nylon", "hybrid"}, s)
}

func ValidateService(s Service) error {
	f := map[string]string{}
	if s.Hybrid && !s.FourKnots {
		f["hybrid"] = "Hybrid stringing requires four-knot support."
	}
	if !validSport(s.Sport) {
		f["sport"] = "Choose tennis, badminton or squash."
	}
	if !s.TwoKnots && !s.FourKnots {
		f["knots"] = "Offer at least one knot pattern."
	}
	if s.MinTension <= 0 || s.MaxTension < s.MinTension || s.MaxTension > 10000 || s.Step < 1 || s.Step > 100 || s.MaxDifference < 0 || s.MaxDifference > 10000 {
		f["tension"] = "Supply valid bounds, a step of 1–100 centi-kg and a nonnegative maximum difference."
	}
	for _, n := range []int{s.LabourCents, s.OwnStringsCents, s.StencilCents, s.TwoKnotsCents, s.FourKnotsCents, s.DifferentTensionsCents, s.ExpressFeeCents} {
		if n < 0 || n > 1000000 {
			f["prices"] = "Each price must be 0–1000000 cents."
		}
	}
	if !validText(s.ExpressTurnaround, 160) || strings.ContainsAny(s.ExpressTurnaround, "\r\n\t") || (s.Express && strings.TrimSpace(s.ExpressTurnaround) == "") {
		f["express_turnaround"] = "Describe the offered turnaround in 1–160 characters on a single line when express is enabled."
	}
	for p, n := range s.Prestretch {
		if p < 1 || p > 30 || n < 0 || n > 1000000 {
			f["prestretch_options"] = "Percentages must be 1–30 and prices 0–1000000 cents."
		}
	}
	if len(f) > 0 {
		return invalid(f)
	}
	return nil
}

// Price is pure: only persisted catalogue/service/model data determines prices.
// Tensions are integer hundredths of a kg; no binary float money or tension maths.
func Price(b Booking, s Service, r Racket, v *Variant, grips ...*GripOffering) (Quote, error) {
	var grip *GripOffering
	if len(grips) > 0 {
		grip = grips[0]
	}
	return priceComponents(b, s, r, v, nil, grip)
}

func priceComponents(b Booking, s Service, r Racket, v, c *Variant, grip *GripOffering) (Quote, error) {
	f := map[string]string{}

	b.CustomerMains = strings.TrimSpace(b.CustomerMains)
	b.CustomerCrosses = strings.TrimSpace(b.CustomerCrosses)
	if b.Hybrid {
		if !s.Hybrid {
			f["hybrid"] = "This stringer does not offer hybrid setups."
		}
		if b.Knots != 4 {
			f["knots"] = "Hybrid setups require four knots."
		}
		if b.StringSource == "stock" {
			if v == nil || v.HalfRacketPriceCents == nil || v.Material == "hybrid" {
				f["variant_id"] = "Choose an offered single-string variant with a half-racket price."
			}
			if c == nil || !c.Available || c.InventoryID == "" || c.HalfRacketPriceCents == nil {
				f["crosses_variant_id"] = "Choose an available cross string with a half-racket price."
			}
		}
		if c != nil && (c.Sport != r.Sport || c.Material == "hybrid" || !strings.EqualFold(c.ID, b.CrossesVariantID)) {
			f["crosses_variant_id"] = "Choose a single cross string for this racket sport."
		}
		if v != nil && v.Material == "hybrid" {
			f["variant_id"] = "Choose an individual mains string, rather than a packaged hybrid."
		}
		if b.StringSource == "customer" {
			for field, item := range map[string]struct {
				id, text string
				variant  *Variant
			}{"customer_mains_description": {b.VariantID, b.CustomerMains, v}, "customer_crosses_description": {b.CrossesVariantID, b.CustomerCrosses, c}} {
				if (item.id == "" && (item.text == "" || !validText(item.text, 160))) || (item.id != "" && (item.variant == nil || item.text != "")) {
					f[field] = "Choose a catalogue string or describe the supplied string in 1–160 characters, not both."
				}
			}
		}
		if b.PrestretchPercent > 0 && (c == nil || b.PrestretchPercent > c.MaxPrestretchPercent) {
			f["prestretch_percent"] = "Pre-stretch needs two known catalogue strings and must satisfy both limits."
		}
	} else if b.CrossesVariantID != "" || b.CustomerMains != "" || b.CustomerCrosses != "" {
		f["hybrid"] = "Separate mains/crosses choices require a hybrid setup."
	}
	if b.StringSource != "customer" && (b.CustomerMains != "" || b.CustomerCrosses != "") {
		f["string_source"] = "Customer string descriptions can only be used with customer-supplied strings."
	}
	if b.GripProductID != "" {
		if grip == nil || !strings.EqualFold(grip.ID, b.GripProductID) || !grip.Active || !grip.Available || grip.InventoryID == "" {
			f["grip_product_id"] = "Choose an available grip offered by this stringer."
		} else {
			b.GripProductID = grip.ID
		}
	}
	if !s.Active || !r.Verified || r.Sport != s.Sport {
		f["racket_model_id"] = "A verified racket model and active service for this sport are required."
	}
	if b.StringSource != "stock" && b.StringSource != "customer" {
		f["string_source"] = "Choose stock or customer."
	}
	if b.StringSource == "customer" && !s.OwnStrings {
		f["string_source"] = "This stringer does not accept customer-supplied strings."
	}
	if b.StringSource == "stock" && (v == nil || !v.Available || v.InventoryID == "") {
		f["variant_id"] = "Choose an available string offered by this stringer."
	}
	if v != nil && v.Sport != r.Sport {
		f["variant_id"] = "This string is for a different racket sport."
	}
	for field, n := range map[string]int{"mains_tension_centi_kg": b.MainsTension, "crosses_tension_centi_kg": b.CrossesTension} {
		if s.Step < 1 || r.Step < 1 || n < s.MinTension || n > s.MaxTension || n < r.MinTension || n > r.MaxTension || n%s.Step != 0 || n%r.Step != 0 {
			f[field] = "Tension must satisfy both the verified racket range and this stringer's range and increments."
		}
	}
	diff := b.MainsTension - b.CrossesTension
	if diff < 0 {
		diff = -diff
	}
	if diff > 0 && (!s.DifferentTensions || diff > s.MaxDifference || diff > r.MaxDifference) {
		f["crosses_tension_centi_kg"] = "The requested tension difference is not supported."
	}
	if !slices.Contains(r.AllowedKnots, b.Knots) || (b.Knots != 2 && b.Knots != 4) || (b.Knots == 2 && !s.TwoKnots) || (b.Knots == 4 && !s.FourKnots) {
		f["knots"] = "This knot pattern is not supported by the racket and service."
	}
	if b.Knots != 4 && ((diff > 0 && r.DifferentRequiresFour) || (v != nil && v.Material == "hybrid")) {
		f["knots"] = "This configuration requires four knots."
	}
	if b.Express && !s.Express {
		f["express"] = "Express turnaround is not offered for this sport by this stringer."
	}
	if b.Stencil && !s.Stencil {
		f["stencil"] = "Stencilling is not offered."
	}
	if b.PrestretchPercent < 0 || b.PrestretchPercent > 30 {
		f["prestretch_percent"] = "Invalid percentage."
	}
	if b.PrestretchPercent > 0 {
		if _, ok := s.Prestretch[b.PrestretchPercent]; !ok {
			f["prestretch_percent"] = "This percentage is not offered."
		}
		if v == nil || b.PrestretchPercent > v.MaxPrestretchPercent {
			f["prestretch_percent"] = "This percentage exceeds the catalogue limit or the supplied string is unknown."
		}
	}
	if utf8.RuneCountInString(b.Instructions) > 500 || strings.ContainsRune(b.Instructions, 0) {
		f["instructions"] = "Use at most 500 characters without null characters."
	}
	if len(f) > 0 {
		return Quote{}, invalid(f)
	}
	b.Instructions = strings.TrimSpace(b.Instructions)
	q := Quote{Currency: "EUR", Configuration: Configuration{Booking: b, RacketSnapshot: r.Brand + " " + r.Model, StringSnapshot: "Customer-supplied strings"}, Breakdown: []PriceLine{}}
	add := func(code, label string, n int) {
		q.Breakdown = append(q.Breakdown, PriceLine{code, label, n})
		q.TotalCents += n
	}
	add("labour", "Restringing labour", s.LabourCents)
	if b.StringSource == "stock" {
		if b.Hybrid {
			add("mains_strings", "Mains string material", *v.HalfRacketPriceCents)
			add("crosses_strings", "Crosses string material", *c.HalfRacketPriceCents)
		} else {
			add("strings", "String material", v.MaterialPriceCents)
		}
	} else {
		add("own_strings", "Customer-supplied strings handling", s.OwnStringsCents)
	}
	if v != nil {
		q.Configuration.StringSnapshot = fmt.Sprintf("%s %s %.2f mm %s", v.Brand, v.Model, v.GaugeMM, v.Colour)
	}

	if b.Hybrid {
		mains, cross := b.CustomerMains, b.CustomerCrosses
		if v != nil {
			mains = variantLabel(*v)
		}
		if c != nil {
			cross = variantLabel(*c)
		}
		if b.StringSource == "customer" {
			mains = "Customer-supplied: " + mains
			cross = "Customer-supplied: " + cross
		}
		q.Configuration.MainsSnapshot = mains
		q.Configuration.CrossesSnapshot = cross
		q.Configuration.StringSnapshot = "Hybrid: mains " + mains + " / crosses " + cross
	}

	if b.Knots == 2 {
		add("knots", "Two-knot stringing", s.TwoKnotsCents)
	} else {
		add("knots", "Four-knot stringing", s.FourKnotsCents)
	}
	if diff > 0 {
		add("different_tensions", "Different mains/crosses tensions", s.DifferentTensionsCents)
	}
	if b.Stencil {
		add("stencil", "Stencilling", s.StencilCents)
	}
	if b.PrestretchPercent > 0 {
		add("prestretch", fmt.Sprintf("%d%% pre-stretch", b.PrestretchPercent), s.Prestretch[b.PrestretchPercent])
	}
	if b.Express {
		add("express", "Express turnaround", s.ExpressFeeCents)
		q.Configuration.ExpressTurnaroundSnapshot = strings.TrimSpace(s.ExpressTurnaround)
	}
	if b.GripProductID != "" {
		add("grip", "Grip including fitting", grip.FittedPriceCents)
		q.Configuration.GripSnapshot = gripLabel(*grip)
	}
	canonical, _ := json.Marshal(struct {
		Booking Booking
		Service Service
		Racket  Racket
		Variant *Variant
		Grip    *GripOffering `json:",omitempty"`
		Crosses *Variant      `json:",omitempty"`
	}{b, s, r, v, grip, c})
	hash := sha256.Sum256(canonical)
	q.Fingerprint = hex.EncodeToString(hash[:])
	return q, nil
}

func CanTransition(from, to string, manager bool) bool {
	if to == "cancelled" {
		return from == "placed" || (manager && (from == "accepted" || from == "received"))
	}
	if !manager {
		return false
	}
	next := map[string]string{"placed": "accepted", "accepted": "received", "received": "in_progress", "in_progress": "ready", "ready": "completed"}
	return next[from] != "" && next[from] == to
}
