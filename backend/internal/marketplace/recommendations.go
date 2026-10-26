package marketplace

import (
	"fmt"
	"sort"
)

type RecommendationRequest struct {
	RacketModelID        string         `json:"racket_model_id"`
	Experience           string         `json:"experience"`
	SessionsPerWeek      int            `json:"sessions_per_week"`
	Preferences          map[string]int `json:"preferences"`
	CurrentVariantID     string         `json:"current_variant_id,omitempty"`
	CurrentTension       int            `json:"current_tension_centi_kg,omitempty"`
	PrioritiseArmComfort bool           `json:"prioritise_arm_comfort"`
}
type Recommendation struct {
	Variant Variant  `json:"variant"`
	Score   int      `json:"score"`
	Reasons []string `json:"reasons"`
}
type Recommendations struct {
	Algorithm string           `json:"algorithm"`
	Results   []Recommendation `json:"results"`
	Notice    string           `json:"notice"`
}

func ValidateRecommendation(r RecommendationRequest) error {
	f := map[string]string{}
	if r.Experience != "new" && r.Experience != "club" && r.Experience != "advanced" {
		f["experience"] = "Choose new, club or advanced."
	}
	if r.SessionsPerWeek < 0 || r.SessionsPerWeek > 21 {
		f["sessions_per_week"] = "Use 0–21."
	}
	if r.CurrentTension < 0 || r.CurrentTension > 10000 {
		f["current_tension_centi_kg"] = "Invalid tension."
	}
	for k, v := range r.Preferences {
		if (k != "comfort" && k != "control" && k != "power" && k != "spin" && k != "durability") || v < 0 || v > 5 {
			f["preferences"] = "Use comfort, control, power, spin and durability weights from 0–5."
		}
	}
	if len(f) > 0 {
		return invalid(f)
	}
	return nil
}

// Rank only accepts candidates already scoped to a stringer's active inventory.
// The optional comfort flag and all recommendation answers are request-only.
func Rank(r RecommendationRequest, candidates []Variant, current *Variant) Recommendations {
	out := Recommendations{Algorithm: "weighted-catalogue-v1", Results: []Recommendation{}, Notice: "Preference matching using editorial string attributes; not a medical assessment or a guarantee of comfort or performance. No tension prescription is made. Answers are not saved."}
	weights := map[string]int{"comfort": 1, "control": 1, "power": 1, "spin": 1, "durability": 1}
	for k, v := range r.Preferences {
		weights[k] = v
	}
	for _, v := range candidates {
		if !v.Available || v.InventoryID == "" {
			continue
		}
		m := Recommendation{Variant: v, Reasons: []string{}}
		ratings := map[string]int{"comfort": v.Comfort, "control": v.Control, "power": v.Power, "spin": v.Spin, "durability": v.Durability}
		for _, k := range []string{"comfort", "control", "power", "spin", "durability"} {
			m.Score += weights[k] * ratings[k]
			if weights[k] > 0 {
				m.Reasons = append(m.Reasons, fmt.Sprintf("%s: weight %d × catalogue rating %d", k, weights[k], ratings[k]))
			}
		}
		if r.Experience == "new" {
			m.Score += 2 * v.Comfort
			m.Reasons = append(m.Reasons, "New-player preference: +2 × comfort rating")
		}
		if r.Experience == "advanced" {
			m.Score += v.Control
			m.Reasons = append(m.Reasons, "Advanced-player preference: +control rating")
		}
		if r.SessionsPerWeek >= 3 {
			m.Score += v.Durability
			m.Reasons = append(m.Reasons, "Frequent play: +durability rating")
		}
		if r.PrioritiseArmComfort {
			m.Score += 4 * v.Comfort
			m.Reasons = append(m.Reasons, "Requested comfort emphasis: +4 × comfort rating; this does not predict injury outcomes")
		}
		if current != nil && v.Material == current.Material {
			m.Score++
			m.Reasons = append(m.Reasons, "Same material category as current setup: +1")
		}
		out.Results = append(out.Results, m)
	}
	sort.Slice(out.Results, func(i, j int) bool {
		if out.Results[i].Score == out.Results[j].Score {
			return out.Results[i].Variant.ID < out.Results[j].Variant.ID
		}
		return out.Results[i].Score > out.Results[j].Score
	})
	if len(out.Results) > 3 {
		out.Results = out.Results[:3]
	}
	return out
}
