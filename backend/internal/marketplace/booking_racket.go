package marketplace

import "context"

// Model-free bookings use the stringer's sport service limits. These are not
// manufacturer limits; the stringer must inspect the racket before work.
func bookingRacket(ctx context.Context, db DB, business, modelID, sport string) (Racket, Service, error) {
	if modelID != "" {
		r, err := loadRacket(ctx, db, modelID)
		if err != nil {
			return r, Service{}, err
		}
		if sport != "" && sport != r.Sport {
			return r, Service{}, invalid(map[string]string{"sport": "The sport must match the selected racket."})
		}
		s, err := loadService(ctx, db, business, r.Sport)
		return r, s, err
	}
	if !validSport(sport) {
		return Racket{}, Service{}, invalid(map[string]string{"sport": "Choose tennis, badminton or squash."})
	}
	s, err := loadService(ctx, db, business, sport)
	if err != nil {
		return Racket{}, s, err
	}
	// Stock accounting allowances per full restring, not racket specifications.
	length := map[string]int{"tennis": 12000, "badminton": 10000, "squash": 11000}[sport]
	r := Racket{Sport: sport, MinTension: s.MinTension, MaxTension: s.MaxTension, Step: s.Step, MaxDifference: s.MaxDifference, AllowedKnots: []int{2, 4}, DifferentRequiresFour: true, StringLengthMM: length}
	return r, s, nil
}
