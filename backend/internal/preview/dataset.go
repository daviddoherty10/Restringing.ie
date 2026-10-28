package preview

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	m "github.com/daviddoherty10/Restringing.ie/backend/internal/marketplace"
	"github.com/jackc/pgx/v5"
)

const fictional = "Fictional preview data only. Not a real business, product specification, manufacturer instruction or medical recommendation."

type business struct {
	id, owner          int
	name, area, county string
	sports             []string
	pending, paused    bool
}

var businesses = []business{
	{101, 4, "Preview — Baseline Workshop", "Dún Laoghaire", "Dublin", []string{"tennis"}, false, false},
	{102, 5, "Preview — Racket Room", "Blackrock", "Dublin", []string{"tennis", "badminton"}, false, false},
	{103, 6, "Preview — West Court", "Galway City", "Galway", []string{"tennis", "squash"}, false, false},
	{104, 7, "Preview — String Studio", "Cork City", "Cork", []string{"tennis", "badminton", "squash"}, false, false},
	{105, 8, "Preview — Rally Workshop", "Limerick City", "Limerick", []string{"tennis"}, false, false},
	{106, 9, "Preview — Paused Stringer", "Swords", "Dublin", []string{"tennis"}, false, true},
	{107, 10, "Preview — Pending Application", "Athlone", "Westmeath", nil, true, false},
}

func racketFixtures() []m.Racket {
	return []m.Racket{
		{ID: ID(3001), Brand: "Preview Rackets", Model: "Tennis All Options", Sport: "tennis", MinTension: 1800, MaxTension: 3000, Step: 50, MaxDifference: 200, AllowedKnots: []int{2, 4}, DifferentRequiresFour: true, StringLengthMM: 12000, TechnicalSource: fictional, Verified: true},
		{ID: ID(3002), Brand: "Preview Rackets", Model: "Tennis Four Knots Only", Sport: "tennis", MinTension: 2000, MaxTension: 2800, Step: 50, MaxDifference: 100, AllowedKnots: []int{4}, DifferentRequiresFour: true, StringLengthMM: 12000, TechnicalSource: fictional, Verified: true},
		{ID: ID(3003), Brand: "Preview Rackets", Model: "Badminton Preview", Sport: "badminton", MinTension: 800, MaxTension: 1400, Step: 50, MaxDifference: 100, AllowedKnots: []int{4}, DifferentRequiresFour: true, StringLengthMM: 10000, TechnicalSource: fictional, Verified: true},
		{ID: ID(3004), Brand: "Preview Rackets", Model: "Squash Preview", Sport: "squash", MinTension: 1000, MaxTension: 1800, Step: 50, MaxDifference: 100, AllowedKnots: []int{2, 4}, DifferentRequiresFour: true, StringLengthMM: 11000, TechnicalSource: fictional, Verified: true},
	}
}

func productFixture(i int) m.Variant {
	materials := []string{"multifilament", "polyester", "synthetic_gut", "natural_gut"}
	names := []string{"Comfort", "Control", "Everyday", "Touch"}
	materialIndex := i % 4
	sport := "tennis"
	gauge := []float64{1.20, 1.25, 1.30, 1.35}[i%4]
	if i >= 28 && i < 34 {
		sport = "badminton"
		gauge = []float64{0.65, 0.68, 0.70}[i%3]
	}
	if i >= 34 {
		sport = "squash"
		gauge = []float64{1.10, 1.15, 1.20}[i%3]
	}
	v := m.Variant{ID: ID(2001 + i), ProductID: ID(1001 + i), Brand: "Preview Strings", Model: fmt.Sprintf("%s %02d", names[materialIndex], i/4+1), Description: fictional, Sport: sport, Material: materials[materialIndex], GaugeMM: gauge, Colour: []string{"Silver", "Natural", "Blue", "Black"}[i%4], Comfort: 4, Control: 3, Power: 4, Spin: 3, Durability: 3, MaxPrestretchPercent: 10}
	v.Brand = []string{"Preview Strings", "Preview Court Lab", "Preview Rally Works"}[i%3]
	if v.Material == "polyester" {
		v.Comfort = 2
		v.Control = 5
		v.Spin = 5
		v.Durability = 5
		v.MaxPrestretchPercent = 0
	}
	if v.Material == "natural_gut" || v.Material == "multifilament" {
		v.Comfort = 5
	}
	if v.Material == "multifilament" {
		v.MaxPrestretchPercent = 15
	}
	if i == 25 {
		v.Model = "Packaged Hybrid Preview"
		v.Material = "hybrid"
		v.MaxPrestretchPercent = 0
	}
	return v
}

func serviceFixture(b business, sport string, sportIndex int) m.Service {
	s := m.Service{ID: ID(10000 + (b.id-100)*10 + sportIndex), Sport: sport, Active: true, LabourCents: 1500 + (b.id-101)*150, OwnStrings: true, Stencil: true, StencilCents: 300, TwoKnots: true, FourKnots: true, FourKnotsCents: 100, DifferentTensions: true, DifferentTensionsCents: 100, MinTension: 1500, MaxTension: 3500, Step: 50, MaxDifference: 200, Prestretch: map[int]int{5: 300, 10: 400}}
	if b.id == 101 && sport == "tennis" {
		s.Express = true
		s.ExpressFeeCents = 700
		s.ExpressTurnaround = "Within 24 hours of receiving your racket, after booking acceptance."
	}
	if b.id == 101 || b.id == 104 {
		s.Prestretch[15] = 600
	}
	if sport == "badminton" {
		s.MinTension = 700
		s.MaxTension = 1600
		s.MaxDifference = 100
		s.TwoKnots = false
	}
	if sport == "squash" {
		s.MinTension = 800
		s.MaxTension = 2000
		s.MaxDifference = 100
	}
	if b.id == 102 {
		s.OwnStrings = false
		s.Stencil = false
		s.TwoKnots = false
		s.DifferentTensions = false
		s.Prestretch = map[int]int{}
	}
	if b.id == 105 {
		s.Stencil = false
		s.Prestretch = map[int]int{5: 250}
	}
	return s
}

type offering struct {
	variant     m.Variant
	lotID, kind string
	length      int
}

func seedDataset(ctx context.Context, tx pgx.Tx) error {
	// Reserved fictional mobile range, never sent to an SMS provider:
	// https://www.ofcom.org.uk/phones-and-broadband/phone-numbers/numbers-for-drama
	for i := 1; i <= 11; i++ {
		name := fmt.Sprintf("Preview User %02d", i)
		role := "user"
		switch i {
		case 1:
			name = "Preview Customer"
		case 2:
			name = "Preview Administrator"
			role = "admin"
		case 3:
			name = "Preview Staff"
		case 4:
			name = "Preview Stringer"
		case 11:
			name = "Preview Other Customer"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO users(id,phone_number,phone_verified_at,first_name,last_name,platform_role,created_at,updated_at) VALUES($1,$2,now()-interval '30 days',$3,'Fictional', $4,now()-interval '30 days',now()-interval '30 days')`, ID(i), phone(i), name, role); err != nil {
			return err
		}
	}
	services := map[string]m.Service{}
	for _, b := range businesses {
		status := "approved"
		if b.pending {
			status = "pending"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO stringers(id,name,description,area,county,approval_status,accepting_orders,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,now()-interval '30 days',now()-interval '29 days')`, ID(b.id), b.name, fictional, b.area, b.county, status, !b.pending && !b.paused); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO stringer_members(stringer_id,user_id,role) VALUES($1,$2,'owner')`, ID(b.id), ID(b.owner)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO stringer_approval_events(id,stringer_id,actor_id,status,reason,created_at) VALUES($1,$2,$3,$4,$5,now()-interval '29 days')`, ID(90000+b.id), ID(b.id), ID(2), status, fictional); err != nil {
			return err
		}
		for n, sport := range b.sports {
			s := serviceFixture(b, sport, n)
			if err := m.ValidateService(s); err != nil {
				return err
			}
			services[fmt.Sprintf("%d:%s", b.id, sport)] = s
			if _, err := tx.Exec(ctx, `INSERT INTO stringer_services(id,stringer_id,sport,active,labour_cents,own_strings,own_strings_cents,stencil,stencil_cents,two_knots,two_knots_cents,four_knots,four_knots_cents,different_tensions,different_tensions_cents,min_tension_centi_kg,max_tension_centi_kg,step_centi_kg,max_difference_centi_kg,express,express_fee_cents,express_turnaround) VALUES($1,$2,$3,true,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`, s.ID, ID(b.id), sport, s.LabourCents, s.OwnStrings, s.OwnStringsCents, s.Stencil, s.StencilCents, s.TwoKnots, s.TwoKnotsCents, s.FourKnots, s.FourKnotsCents, s.DifferentTensions, s.DifferentTensionsCents, s.MinTension, s.MaxTension, s.Step, s.MaxDifference, s.Express, s.ExpressFeeCents, s.ExpressTurnaround); err != nil {
				return err
			}
			for percent, price := range s.Prestretch {
				if _, err := tx.Exec(ctx, `INSERT INTO service_prestretch_options(service_id,percent,price_cents) VALUES($1,$2,$3)`, s.ID, percent, price); err != nil {
					return err
				}
			}
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO stringer_members(stringer_id,user_id,role) VALUES($1,$2,'staff')`, ID(101), ID(3)); err != nil {
		return err
	}
	rackets := racketFixtures()
	for _, r := range rackets {
		if _, err := tx.Exec(ctx, `INSERT INTO racket_models(id,brand,model,sport,min_tension_centi_kg,max_tension_centi_kg,step_centi_kg,max_difference_centi_kg,allowed_knots,different_tensions_require_four_knots,string_length_mm,technical_source,verified) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,true)`, r.ID, r.Brand, r.Model, r.Sport, r.MinTension, r.MaxTension, r.Step, r.MaxDifference, r.AllowedKnots, r.DifferentRequiresFour, r.StringLengthMM, r.TechnicalSource); err != nil {
			return err
		}
	}
	offerings := map[string]offering{}
	for i := 0; i < 40; i++ {
		v := productFixture(i)
		if _, err := tx.Exec(ctx, `INSERT INTO string_products(id,brand,model,material,sport,description,comfort,control,power,spin,durability,max_prestretch_percent) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, v.ProductID, v.Brand, v.Model, v.Material, v.Sport, v.Description, v.Comfort, v.Control, v.Power, v.Spin, v.Durability, v.MaxPrestretchPercent); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO string_variants(id,product_id,gauge_mm,colour) VALUES($1,$2,$3,$4)`, v.ID, v.ProductID, v.GaugeMM, v.Colour); err != nil {
			return err
		}
		for _, b := range businesses {
			if _, ok := services[fmt.Sprintf("%d:%s", b.id, v.Sport)]; !ok {
				continue
			}
			if b.id == 103 && v.Sport == "tennis" && i >= 16 {
				continue
			}
			if b.id == 105 && i >= 12 {
				continue
			}
			index := (b.id-100)*100 + i
			copy := v
			copy.InventoryID = ID(20000 + index)
			copy.MaterialPriceCents = 800 + i*40 + (b.id-101)*100
			copy.Available = i != 26
			if _, err := tx.Exec(ctx, `INSERT INTO stringer_inventory(id,stringer_id,variant_id,material_price_cents,available) VALUES($1,$2,$3,$4,$5)`, copy.InventoryID, ID(b.id), v.ID, copy.MaterialPriceCents, copy.Available); err != nil {
				return err
			}
			kind := "set"
			quantity := 8
			setLength := 12000
			requiredLength := 12000
			if v.Sport == "badminton" {
				setLength = 10000
				requiredLength = 10000
			}
			if v.Sport == "squash" {
				setLength = 11000
				requiredLength = 11000
			}
			if i%2 == 1 {
				kind = "reel"
				quantity = 200000
				setLength = 0
			}
			if i == 27 {
				quantity = 0
			}
			lotID := ID(30000 + index)
			if _, err := tx.Exec(ctx, `INSERT INTO inventory_lots(id,inventory_id,kind,set_length_mm,quantity,created_at) VALUES($1,$2,$3,$4,$5,now()-interval '20 days')`, lotID, copy.InventoryID, kind, setLength, quantity); err != nil {
				return err
			}
			if quantity > 0 {
				if _, err := tx.Exec(ctx, `INSERT INTO inventory_movements(id,lot_id,actor_id,quantity_delta,reason,created_at) VALUES($1,$2,$3,$4,'Fictional preview stock receipt',now()-interval '20 days')`, ID(60000+index), lotID, ID(b.owner), quantity); err != nil {
					return err
				}
			}
			offerings[fmt.Sprintf("%d:%d", b.id, i)] = offering{copy, lotID, kind, requiredLength}
		}
	}
	for n, status := range []string{"placed", "accepted", "received", "in_progress", "ready", "completed", "cancelled", "placed"} {
		b := businesses[0]
		customer := 1
		if n == 7 {
			b = businesses[1]
			customer = 11
		}
		offer := offerings[fmt.Sprintf("%d:%d", b.id, n)]
		if err := seedOrder(ctx, tx, n, status, b, customer, services[fmt.Sprintf("%d:tennis", b.id)], rackets[0], offer); err != nil {
			return err
		}
	}
	if err := seedOrderCommunication(ctx, tx); err != nil {
		return err
	}
	return nil
}

func seedOrder(ctx context.Context, tx pgx.Tx, n int, status string, b business, customer int, service m.Service, racket m.Racket, offer offering) error {
	booking := m.Booking{RacketModelID: racket.ID, StringSource: "stock", VariantID: offer.variant.ID, MainsTension: 2400, CrossesTension: 2400, Knots: 4, Instructions: "Fictional preview order. No real racket, customer or payment."}
	if n == 3 {
		booking.Express = true
	}
	if n == 0 {
		booking.Stencil = true
		booking.PrestretchPercent = 5
		booking.CrossesTension = 2300
	}
	var variant *m.Variant
	variant = &offer.variant
	if n == 2 {
		booking.StringSource = "customer"
		booking.VariantID = ""
		variant = nil
	}
	quote, err := m.Price(booking, service, racket, variant)
	if err != nil {
		return err
	}
	cfg, err := json.Marshal(quote.Configuration)
	if err != nil {
		return err
	}
	lines, err := json.Marshal(quote.Breakdown)
	if err != nil {
		return err
	}
	created := time.Now().UTC().Add(-time.Duration(n+2) * 24 * time.Hour)
	// Keep one historical completed order outside the current allowance so a
	// fresh preview can demonstrate accepting its fifth booking.
	if n == 5 {
		now := time.Now().UTC()
		created = time.Date(now.Year(), now.Month()-1, 15, 12, 0, 0, 0, time.UTC)
	}
	quoteID := ID(50001 + n)
	orderID := ID(40001 + n)
	if _, err = tx.Exec(ctx, `INSERT INTO quotes(id,customer_id,stringer_id,configuration,breakdown,fingerprint,total_cents,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, quoteID, ID(customer), ID(b.id), cfg, lines, quote.Fingerprint, quote.TotalCents, created, created.Add(10*time.Minute)); err != nil {
		return err
	}
	lastStep := map[string]int{"placed": 0, "accepted": 1, "received": 2, "in_progress": 3, "ready": 4, "completed": 5, "cancelled": 1}[status]
	if _, err = tx.Exec(ctx, `INSERT INTO orders(id,customer_id,stringer_id,quote_id,idempotency_key,configuration,breakdown,total_cents,currency,status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'EUR',$9,$10,$11)`, orderID, ID(customer), ID(b.id), quoteID, fmt.Sprintf("preview-order-key-%03d", n), cfg, lines, quote.TotalCents, status, created, created.Add(time.Duration(lastStep)*time.Hour)); err != nil {
		return err
	}
	path := []string{"placed", "accepted", "received", "in_progress", "ready", "completed"}
	if status == "cancelled" {
		path = []string{"placed", "cancelled"}
	}
	var previous *string
	for step, next := range path {
		actor := b.owner
		if step == 0 || next == "cancelled" {
			actor = customer
		}
		if _, err = tx.Exec(ctx, `INSERT INTO order_status_events(id,order_id,actor_id,from_status,to_status,created_at) VALUES($1,$2,$3,$4,$5,$6)`, ID(70000+n*10+step), orderID, ID(actor), previous, next, created.Add(time.Duration(step)*time.Hour)); err != nil {
			return err
		}
		if next == "accepted" {
			if _, err = tx.Exec(ctx, `INSERT INTO booking_usage(order_id,stringer_id,accepted_at) VALUES($1,$2,$3)`, orderID, ID(b.id), created.Add(time.Duration(step)*time.Hour)); err != nil {
				return err
			}
		}
		if next == status {
			break
		}
		value := next
		previous = &value
	}
	if booking.StringSource == "customer" {
		return nil
	}
	quantity := 1
	if offer.kind == "reel" {
		quantity = offer.length
	}
	state := "reserved"
	if status == "in_progress" || status == "ready" || status == "completed" {
		state = "consumed"
	}
	if status == "cancelled" {
		state = "released"
	}
	if _, err = tx.Exec(ctx, `INSERT INTO inventory_reservations(order_id,lot_id,quantity,state) VALUES($1,$2,$3,$4)`, orderID, offer.lotID, quantity, state); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO inventory_movements(id,lot_id,actor_id,quantity_delta,reserved_delta,reason,order_id,created_at) VALUES($1,$2,$3,0,$4,'Fictional preview reservation',$5,$6)`, ID(80000+n*10), offer.lotID, ID(customer), quantity, orderID, created); err != nil {
		return err
	}
	switch state {
	case "reserved":
		_, err = tx.Exec(ctx, `UPDATE inventory_lots SET reserved=reserved+$2 WHERE id=$1`, offer.lotID, quantity)
	case "consumed":
		if _, err = tx.Exec(ctx, `UPDATE inventory_lots SET quantity=quantity-$2 WHERE id=$1`, offer.lotID, quantity); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO inventory_movements(id,lot_id,actor_id,quantity_delta,reserved_delta,reason,order_id,created_at) VALUES($1,$2,$3,$4,$4,'Fictional preview consumption',$5,$6)`, ID(80001+n*10), offer.lotID, ID(b.owner), -quantity, orderID, created.Add(3*time.Hour))
	case "released":
		_, err = tx.Exec(ctx, `INSERT INTO inventory_movements(id,lot_id,actor_id,quantity_delta,reserved_delta,reason,order_id,created_at) VALUES($1,$2,$3,0,$4,'Fictional preview release',$5,$6)`, ID(80001+n*10), offer.lotID, ID(customer), -quantity, orderID, created.Add(time.Hour))
	}
	return err
}

// Extra fictional examples are added only during the initial seed. Rerunning
// seed preserves all edits, messages and read markers in an existing preview.
func seedOrderCommunication(ctx context.Context, tx pgx.Tx) error {
	now := time.Now().UTC()
	requested := now.AddDate(0, 0, 1).Format("2006-01-02")
	ready := now.Add(-time.Hour)
	state := m.TrackingState{RequestedCollectionDate: &requested, ConfirmedReadyAt: &ready, CollectionInstructions: "Fictional preview: arrange collection in this order conversation.", IssueOpen: true, Version: 1}
	raw, _ := json.Marshal(state)
	if _, err := tx.Exec(ctx, `INSERT INTO order_tracking_updates(id,order_id,actor_id,idempotency_key,kind,request,state,version,note,created_at) VALUES($1,$2,$3,'preview-tracking-update-01','stringer_update','{}',$4,1,'Fictional delay: the machine needs adjustment. Please reply about a later collection.',now()-interval '1 hour')`, ID(91001), ID(40004), ID(4), raw); err != nil {
		return err
	}
	for i, body := range []string{"Fictional preview: could I collect before my next match?", "Fictional preview: there is a delay. I will confirm a new estimate here."} {
		sender := 1
		role := "customer"
		if i == 1 {
			sender = 4
			role = "stringer"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO order_messages(id,order_id,sender_id,sender_role,body,idempotency_key,created_at) VALUES($1,$2,$3,$4,$5,$6,now()-interval '30 minutes')`, ID(92001+i), ID(40004), ID(sender), role, body, fmt.Sprintf("preview-message-key-%03d", i)); err != nil {
			return err
		}
	}
	for _, n := range []struct {
		user, order int
		kind, title string
	}{{1, 40004, "tracking", "Your order needs attention"}, {1, 40005, "status", "Your racket is ready for collection"}, {4, 40001, "new_order", "New restringing booking"}, {3, 40001, "new_order", "New restringing booking"}} {
		if _, err := tx.Exec(ctx, `INSERT INTO notifications(user_id,order_id,event_key,kind,title) VALUES($1,$2,$3,$4,$5)`, ID(n.user), ID(n.order), fmt.Sprintf("preview-notification:%d:%d", n.user, n.order), n.kind, n.title); err != nil {
			return err
		}
	}
	return nil
}
