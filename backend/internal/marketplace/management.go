package marketplace

import (
	"context"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

type ProfileInput struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	Area            string `json:"area"`
	County          string `json:"county"`
	AcceptingOrders bool   `json:"accepting_orders"`
}

func validateProfile(p ProfileInput) error {
	if strings.TrimSpace(p.Name) == "" || utf8.RuneCountInString(p.Name) > 120 || utf8.RuneCountInString(p.Description) > 2000 || utf8.RuneCountInString(p.Area) > 120 || utf8.RuneCountInString(p.County) > 120 {
		return invalid(map[string]string{"profile": "Name is required (max 120); description max 2000; area/county max 120 characters."})
	}
	return nil
}
func (s *Store) Apply(ctx context.Context, a Actor, p ProfileInput) (Stringer, error) {
	if err := validateProfile(p); err != nil {
		return Stringer{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Stringer{}, err
	}
	defer tx.Rollback(ctx)
	shop, err := scanStringer(tx.QueryRow(ctx, `INSERT INTO stringers(name,description,area,county,accepting_orders) VALUES($1,$2,$3,$4,$5) RETURNING `+stringerCols, strings.TrimSpace(p.Name), p.Description, p.Area, p.County, p.AcceptingOrders))
	if err != nil {
		return shop, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO stringer_members(stringer_id,user_id,role) VALUES($1,$2,'owner')`, shop.ID, a.ID); err != nil {
		return shop, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO stringer_approval_events(stringer_id,actor_id,status) VALUES($1,$2,'pending')`, shop.ID, a.ID); err != nil {
		return shop, err
	}
	return shop, tx.Commit(ctx)
}
func (s *Store) UpdateProfile(ctx context.Context, a Actor, id string, p ProfileInput) (Stringer, error) {
	if err := validateProfile(p); err != nil {
		return Stringer{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Stringer{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = lockStringer(ctx, tx, id); err != nil {
		return Stringer{}, err
	}
	if err = authorize(ctx, tx, a, id, true); err != nil {
		return Stringer{}, err
	}
	shop, err := scanStringer(tx.QueryRow(ctx, `UPDATE stringers SET name=$2,description=$3,area=$4,county=$5,accepting_orders=$6,updated_at=now() WHERE id=$1 RETURNING `+stringerCols, id, strings.TrimSpace(p.Name), p.Description, p.Area, p.County, p.AcceptingOrders))
	if err != nil {
		return shop, err
	}
	return shop, tx.Commit(ctx)
}
func (s *Store) Review(ctx context.Context, a Actor, id, status, reason string) (Stringer, error) {
	if !a.Admin {
		return Stringer{}, problem(403, "forbidden", "Administrator access is required.")
	}
	if status != "approved" && status != "rejected" && status != "suspended" {
		return Stringer{}, invalid(map[string]string{"status": "Choose approved, rejected or suspended."})
	}
	if utf8.RuneCountInString(reason) > 1000 {
		return Stringer{}, invalid(map[string]string{"reason": "Use at most 1000 characters."})
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Stringer{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = lockStringer(ctx, tx, id); err != nil {
		return Stringer{}, err
	}
	shop, err := scanStringer(tx.QueryRow(ctx, `UPDATE stringers SET approval_status=$2,updated_at=now() WHERE id=$1 RETURNING `+stringerCols, id, status))
	if err != nil {
		return shop, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO stringer_approval_events(stringer_id,actor_id,status,reason) VALUES($1,$2,$3,$4)`, id, a.ID, status, reason); err != nil {
		return shop, err
	}
	return shop, tx.Commit(ctx)
}
func (s *Store) MyStringers(ctx context.Context, a Actor) ([]Stringer, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+stringerCols+` FROM stringers WHERE id IN (SELECT stringer_id FROM stringer_members WHERE user_id=$1) ORDER BY name,id`, a.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Stringer{}
	for rows.Next() {
		v, e := scanStringer(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) Applications(ctx context.Context, a Actor, f Filters) (Page[Stringer], error) {
	out := Page[Stringer]{Data: []Stringer{}, Page: f.Page, PageSize: f.PageSize}
	if !a.Admin {
		return out, problem(403, "forbidden", "Administrator access is required.")
	}
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM stringers WHERE approval_status='pending'`).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+stringerCols+` FROM stringers WHERE approval_status='pending' ORDER BY created_at,id LIMIT $1 OFFSET $2`, f.PageSize, (f.Page-1)*f.PageSize)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		v, e := scanStringer(rows)
		if e != nil {
			return out, e
		}
		out.Data = append(out.Data, v)
	}
	return out, rows.Err()
}

func (s *Store) SaveService(ctx context.Context, a Actor, id string, v Service) (Service, error) {
	if err := ValidateService(v); err != nil {
		return v, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return v, err
	}
	defer tx.Rollback(ctx)
	if _, err = lockStringer(ctx, tx, id); err != nil {
		return v, err
	}
	if err = authorize(ctx, tx, a, id, true); err != nil {
		return v, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO stringer_services(stringer_id,sport,active,labour_cents,own_strings,own_strings_cents,stencil,stencil_cents,two_knots,two_knots_cents,four_knots,four_knots_cents,different_tensions,different_tensions_cents,min_tension_centi_kg,max_tension_centi_kg,step_centi_kg,max_difference_centi_kg) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18) ON CONFLICT(stringer_id,sport) DO UPDATE SET active=EXCLUDED.active,labour_cents=EXCLUDED.labour_cents,own_strings=EXCLUDED.own_strings,own_strings_cents=EXCLUDED.own_strings_cents,stencil=EXCLUDED.stencil,stencil_cents=EXCLUDED.stencil_cents,two_knots=EXCLUDED.two_knots,two_knots_cents=EXCLUDED.two_knots_cents,four_knots=EXCLUDED.four_knots,four_knots_cents=EXCLUDED.four_knots_cents,different_tensions=EXCLUDED.different_tensions,different_tensions_cents=EXCLUDED.different_tensions_cents,min_tension_centi_kg=EXCLUDED.min_tension_centi_kg,max_tension_centi_kg=EXCLUDED.max_tension_centi_kg,step_centi_kg=EXCLUDED.step_centi_kg,max_difference_centi_kg=EXCLUDED.max_difference_centi_kg RETURNING id::text`, id, v.Sport, v.Active, v.LabourCents, v.OwnStrings, v.OwnStringsCents, v.Stencil, v.StencilCents, v.TwoKnots, v.TwoKnotsCents, v.FourKnots, v.FourKnotsCents, v.DifferentTensions, v.DifferentTensionsCents, v.MinTension, v.MaxTension, v.Step, v.MaxDifference).Scan(&v.ID)
	if err != nil {
		return v, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM service_prestretch_options WHERE service_id=$1`, v.ID); err != nil {
		return v, err
	}
	for percent, price := range v.Prestretch {
		if _, err = tx.Exec(ctx, `INSERT INTO service_prestretch_options(service_id,percent,price_cents) VALUES($1,$2,$3)`, v.ID, percent, price); err != nil {
			return v, err
		}
	}
	if v.Prestretch == nil {
		v.Prestretch = map[int]int{}
	}
	return v, tx.Commit(ctx)
}

func (s *Store) SaveInventory(ctx context.Context, a Actor, id, variantID string, price int, available bool) (string, error) {
	if price < 0 || price > 1000000 {
		return "", invalid(map[string]string{"material_price_cents": "Use 0–1000000 cents."})
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if _, err = lockStringer(ctx, tx, id); err != nil {
		return "", err
	}
	if err = authorize(ctx, tx, a, id, false); err != nil {
		return "", err
	}
	if _, err = loadVariant(ctx, tx, "", variantID); err != nil {
		return "", err
	}
	var inventoryID string
	err = tx.QueryRow(ctx, `INSERT INTO stringer_inventory(stringer_id,variant_id,material_price_cents,available) VALUES($1,$2,$3,$4) ON CONFLICT(stringer_id,variant_id) DO UPDATE SET material_price_cents=EXCLUDED.material_price_cents,available=EXCLUDED.available RETURNING id::text`, id, variantID, price, available).Scan(&inventoryID)
	if err != nil {
		return "", err
	}
	return inventoryID, tx.Commit(ctx)
}

type Lot struct {
	ID          string `json:"id"`
	InventoryID string `json:"inventory_id"`
	Kind        string `json:"kind"`
	SetLengthMM int    `json:"set_length_mm"`
	Quantity    int    `json:"quantity"`
	Reserved    int    `json:"reserved"`
}

func (s *Store) AddLot(ctx context.Context, a Actor, id, variantID, kind string, quantity, setLengthMM int) (Lot, error) {
	if (kind != "set" && kind != "reel") || quantity < 1 || quantity > 100000000 {
		return Lot{}, invalid(map[string]string{"stock": "Use kind set/reel and quantity 1–100000000 (sets or millimetres)."})
	}
	if (kind == "set" && (setLengthMM < 1000 || setLengthMM > 30000)) || (kind == "reel" && setLengthMM != 0) {
		return Lot{}, invalid(map[string]string{"set_length_mm": "Sets require a verified length of 1000–30000 mm; reels use 0."})
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Lot{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = lockStringer(ctx, tx, id); err != nil {
		return Lot{}, err
	}
	if err = authorize(ctx, tx, a, id, false); err != nil {
		return Lot{}, err
	}
	var inventoryID string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM stringer_inventory WHERE stringer_id=$1 AND variant_id=$2`, id, variantID).Scan(&inventoryID); err != nil {
		return Lot{}, missing(err)
	}
	l := Lot{InventoryID: inventoryID, Kind: kind, Quantity: quantity, SetLengthMM: setLengthMM}
	err = tx.QueryRow(ctx, `INSERT INTO inventory_lots(inventory_id,kind,quantity,set_length_mm) VALUES($1,$2,$3,$4) RETURNING id::text`, inventoryID, kind, quantity, setLengthMM).Scan(&l.ID)
	if err != nil {
		return l, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO inventory_movements(lot_id,actor_id,quantity_delta,reason) VALUES($1,$2,$3,'stock received')`, l.ID, a.ID, quantity); err != nil {
		return l, err
	}
	return l, tx.Commit(ctx)
}
func (s *Store) AdjustLot(ctx context.Context, a Actor, id, lotID string, delta int, reason string) (Lot, error) {
	if delta == 0 || delta < -100000000 || delta > 100000000 || strings.TrimSpace(reason) == "" || utf8.RuneCountInString(reason) > 200 {
		return Lot{}, invalid(map[string]string{"adjustment": "Supply a nonzero quantity delta (max magnitude 100000000) and a reason of 1–200 characters."})
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Lot{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = lockStringer(ctx, tx, id); err != nil {
		return Lot{}, err
	}
	if err = authorize(ctx, tx, a, id, false); err != nil {
		return Lot{}, err
	}
	var l Lot
	err = tx.QueryRow(ctx, `UPDATE inventory_lots l SET quantity=l.quantity+$3 FROM stringer_inventory i WHERE l.id=$2 AND l.inventory_id=i.id AND i.stringer_id=$1 AND l.quantity+$3 BETWEEN l.reserved AND 100000000 RETURNING l.id::text,l.inventory_id::text,l.kind,l.quantity,l.reserved,l.set_length_mm`, id, lotID, delta).Scan(&l.ID, &l.InventoryID, &l.Kind, &l.Quantity, &l.Reserved, &l.SetLengthMM)
	if err == pgx.ErrNoRows {
		return l, problem(409, "stock_conflict", "Lot not found in this business, or adjustment would remove reserved stock or exceed bounds.")
	}
	if err != nil {
		return l, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO inventory_movements(lot_id,actor_id,quantity_delta,reason) VALUES($1,$2,$3,$4)`, lotID, a.ID, delta, reason); err != nil {
		return l, err
	}
	return l, tx.Commit(ctx)
}
func (s *Store) Lots(ctx context.Context, a Actor, id string, f Filters) (Page[Lot], error) {
	out := Page[Lot]{Data: []Lot{}, Page: f.Page, PageSize: f.PageSize}
	if err := authorize(ctx, s.Pool, a, id, false); err != nil {
		return out, err
	}
	base := ` FROM inventory_lots l JOIN stringer_inventory i ON i.id=l.inventory_id WHERE i.stringer_id=$1`
	if err := s.Pool.QueryRow(ctx, `SELECT count(*)`+base, id).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT l.id::text,l.inventory_id::text,l.kind,l.quantity,l.reserved,l.set_length_mm`+base+` ORDER BY l.created_at,l.id LIMIT $2 OFFSET $3`, id, f.PageSize, (f.Page-1)*f.PageSize)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var l Lot
		if err = rows.Scan(&l.ID, &l.InventoryID, &l.Kind, &l.Quantity, &l.Reserved, &l.SetLengthMM); err != nil {
			return out, err
		}
		out.Data = append(out.Data, l)
	}
	return out, rows.Err()
}

func (s *Store) SaveProduct(ctx context.Context, id string, p Product) (Product, error) {
	f := map[string]string{}
	if strings.TrimSpace(p.Brand) == "" || utf8.RuneCountInString(p.Brand) > 100 || strings.TrimSpace(p.Model) == "" || utf8.RuneCountInString(p.Model) > 120 || utf8.RuneCountInString(p.Description) > 2000 {
		f["product"] = "Supply a brand (max 100), model (max 120) and description (max 2000)."
	}
	if !validSport(p.Sport) || !validMaterial(p.Material) {
		f["classification"] = "Supply a supported sport and material."
	}
	for _, n := range []int{p.Comfort, p.Control, p.Power, p.Spin, p.Durability} {
		if n < 1 || n > 5 {
			f["ratings"] = "Editorial ratings must be 1–5."
		}
	}
	if p.MaxPrestretchPercent < 0 || p.MaxPrestretchPercent > 30 {
		f["max_prestretch_percent"] = "Use 0–30 based on verified product information."
	}
	if len(f) > 0 {
		return p, invalid(f)
	}
	args := []any{p.Brand, p.Model, p.Material, p.Sport, p.Description, p.Comfort, p.Control, p.Power, p.Spin, p.Durability, p.MaxPrestretchPercent, p.Active}
	query := `INSERT INTO string_products(brand,model,material,sport,description,comfort,control,power,spin,durability,max_prestretch_percent,active) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id::text`
	if id != "" {
		query = `UPDATE string_products SET brand=$1,model=$2,material=$3,sport=$4,description=$5,comfort=$6,control=$7,power=$8,spin=$9,durability=$10,max_prestretch_percent=$11,active=$12 WHERE id=$13 RETURNING id::text`
		args = append(args, id)
	}
	err := s.Pool.QueryRow(ctx, query, args...).Scan(&p.ID)
	return p, missing(err)
}

type VariantInput struct {
	ProductID string  `json:"product_id"`
	GaugeMM   float64 `json:"gauge_mm"`
	Colour    string  `json:"colour"`
	Active    bool    `json:"active"`
}

func (s *Store) SaveVariant(ctx context.Context, id string, v VariantInput) (string, error) {
	if math.IsNaN(v.GaugeMM) || math.IsInf(v.GaugeMM, 0) || v.GaugeMM < 0.4 || v.GaugeMM > 2 || math.Abs(v.GaugeMM*100-math.Round(v.GaugeMM*100)) > 0.000001 || utf8.RuneCountInString(v.Colour) > 60 {
		return "", invalid(map[string]string{"variant": "Gauge must be 0.40–2.00 mm with at most 2 decimal places; colour max 60 characters."})
	}
	query := `INSERT INTO string_variants(product_id,gauge_mm,colour,active) VALUES($1,$2,$3,$4) RETURNING id::text`
	args := []any{v.ProductID, v.GaugeMM, v.Colour, v.Active}
	if id != "" {
		query = `UPDATE string_variants SET product_id=$1,gauge_mm=$2,colour=$3,active=$4 WHERE id=$5 RETURNING id::text`
		args = append(args, id)
	}
	var result string
	err := s.Pool.QueryRow(ctx, query, args...).Scan(&result)
	return result, missing(err)
}
func (s *Store) SaveRacket(ctx context.Context, id string, r Racket) (Racket, error) {
	validKnots := len(r.AllowedKnots) > 0
	for _, k := range r.AllowedKnots {
		if k != 2 && k != 4 {
			validKnots = false
		}
	}
	if strings.TrimSpace(r.Brand) == "" || utf8.RuneCountInString(r.Brand) > 100 || strings.TrimSpace(r.Model) == "" || utf8.RuneCountInString(r.Model) > 120 || !validSport(r.Sport) || r.MinTension <= 0 || r.MaxTension < r.MinTension || r.MaxTension > 10000 || r.Step < 1 || r.Step > 100 || r.MaxDifference < 0 || r.MaxDifference > 10000 || !validKnots || r.StringLengthMM < 1000 || r.StringLengthMM > 30000 || strings.TrimSpace(r.TechnicalSource) == "" || utf8.RuneCountInString(r.TechnicalSource) > 1000 {
		return r, invalid(map[string]string{"racket": "Supply verified model-specific tension limits, increments, knot patterns, string length, and a technical source; see the API contract for bounds."})
	}
	args := []any{r.Brand, r.Model, r.Sport, r.MinTension, r.MaxTension, r.Step, r.MaxDifference, r.AllowedKnots, r.DifferentRequiresFour, r.StringLengthMM, r.TechnicalSource, r.Verified}
	query := `INSERT INTO racket_models(brand,model,sport,min_tension_centi_kg,max_tension_centi_kg,step_centi_kg,max_difference_centi_kg,allowed_knots,different_tensions_require_four_knots,string_length_mm,technical_source,verified) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id::text`
	if id != "" {
		query = `UPDATE racket_models SET brand=$1,model=$2,sport=$3,min_tension_centi_kg=$4,max_tension_centi_kg=$5,step_centi_kg=$6,max_difference_centi_kg=$7,allowed_knots=$8,different_tensions_require_four_knots=$9,string_length_mm=$10,technical_source=$11,verified=$12 WHERE id=$13 RETURNING id::text`
		args = append(args, id)
	}
	err := s.Pool.QueryRow(ctx, query, args...).Scan(&r.ID)
	return r, missing(err)
}
