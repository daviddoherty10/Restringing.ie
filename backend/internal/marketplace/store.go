package marketplace

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ Pool *pgxpool.Pool }
type DB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type scanner interface{ Scan(...any) error }

func missing(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return problem(404, "not_found", "The requested resource was not found.")
	}
	return err
}

func authorize(ctx context.Context, db DB, a Actor, stringerID string, owner bool) error {
	if a.Admin {
		return nil
	}
	var ok bool
	err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM stringer_members WHERE stringer_id=$1 AND user_id=$2 AND (NOT $3 OR role='owner'))`, stringerID, a.ID, owner).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return problem(403, "forbidden", "You do not have access to this stringer.")
	}
	return nil
}

const stringerCols = `id::text,name,description,area,county,approval_status,accepting_orders`

func scanStringer(row scanner) (Stringer, error) {
	var s Stringer
	err := row.Scan(&s.ID, &s.Name, &s.Description, &s.Area, &s.County, &s.ApprovalStatus, &s.AcceptingOrders)
	return s, missing(err)
}
func lockStringer(ctx context.Context, db DB, id string) (Stringer, error) {
	return scanStringer(db.QueryRow(ctx, `SELECT `+stringerCols+` FROM stringers WHERE id=$1 FOR UPDATE`, id))
}
func requireOpen(s Stringer) error {
	if s.ApprovalStatus != "approved" || !s.AcceptingOrders {
		return problem(409, "stringer_unavailable", "This stringer is not accepting bookings.")
	}
	return nil
}
func (s *Store) ListStringers(ctx context.Context, f Filters) (Page[Stringer], error) {
	out := Page[Stringer]{Data: []Stringer{}, Page: f.Page, PageSize: f.PageSize}
	base := ` FROM stringers s WHERE approval_status='approved' AND ($1='' OR name ILIKE '%'||$1||'%' OR area ILIKE '%'||$1||'%') AND ($2='' OR lower(county)=lower($2)) AND ($3='' OR EXISTS(SELECT 1 FROM stringer_services ss WHERE ss.stringer_id=s.id AND ss.sport=$3 AND ss.active))`
	args := []any{f.Query, f.County, f.Sport}
	if err := s.Pool.QueryRow(ctx, `SELECT count(*)`+base, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+stringerCols+base+` ORDER BY name,id LIMIT $4 OFFSET $5`, append(args, f.PageSize, (f.Page-1)*f.PageSize)...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		v, err := scanStringer(rows)
		if err != nil {
			return out, err
		}
		out.Data = append(out.Data, v)
	}
	return out, rows.Err()
}
func (s *Store) GetStringer(ctx context.Context, id string) (Stringer, error) {
	return scanStringer(s.Pool.QueryRow(ctx, `SELECT `+stringerCols+` FROM stringers WHERE id=$1 AND approval_status='approved'`, id))
}

const variantCols = `v.id::text,p.id::text,p.brand,p.model,p.description,p.material,p.sport,v.gauge_mm::float8,v.colour,p.comfort,p.control,p.power,p.spin,p.durability,p.max_prestretch_percent,COALESCE(i.id::text,''),COALESCE(i.material_price_cents,0),COALESCE(i.available,false)`
const variantJoin = ` FROM string_variants v JOIN string_products p ON p.id=v.product_id LEFT JOIN stringer_inventory i ON i.variant_id=v.id AND i.stringer_id=NULLIF($1,'')::uuid `

func scanVariant(row scanner) (Variant, error) {
	var v Variant
	err := row.Scan(&v.ID, &v.ProductID, &v.Brand, &v.Model, &v.Description, &v.Material, &v.Sport, &v.GaugeMM, &v.Colour, &v.Comfort, &v.Control, &v.Power, &v.Spin, &v.Durability, &v.MaxPrestretchPercent, &v.InventoryID, &v.MaterialPriceCents, &v.Available)
	return v, missing(err)
}
func loadVariant(ctx context.Context, db DB, stringerID, id string) (Variant, error) {
	return scanVariant(db.QueryRow(ctx, `SELECT `+variantCols+variantJoin+`WHERE v.id=$2 AND v.active AND p.active AND ($1='' OR i.id IS NOT NULL) FOR SHARE OF v,p`, stringerID, id))
}
func (s *Store) ListVariants(ctx context.Context, stringerID string, f Filters, availableOnly bool) (Page[Variant], error) {
	out := Page[Variant]{Data: []Variant{}, Page: f.Page, PageSize: f.PageSize}
	base := variantJoin + `WHERE v.active AND p.active AND ($1='' OR i.id IS NOT NULL) AND ($2='' OR p.brand ILIKE '%'||$2||'%' OR p.model ILIKE '%'||$2||'%' OR v.gauge_mm::text ILIKE '%'||$2||'%' OR v.colour ILIKE '%'||$2||'%') AND ($3='' OR p.material=$3) AND ($4='' OR lower(p.brand)=lower($4)) AND ($5='' OR p.sport=$5) AND (NOT $6 OR (i.available AND EXISTS(SELECT 1 FROM inventory_lots l WHERE l.inventory_id=i.id AND l.quantity>l.reserved))) AND ($7::float8=0 OR v.gauge_mm::float8=$7::float8) AND ($8='' OR v.id=NULLIF($8,'')::uuid)`
	args := []any{stringerID, f.Query, f.Material, f.Brand, f.Sport, availableOnly, f.GaugeMM, f.VariantID}
	if err := s.Pool.QueryRow(ctx, `SELECT count(*)`+base, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+variantCols+base+` ORDER BY p.brand,p.model,v.gauge_mm,v.id LIMIT $9 OFFSET $10`, append(args, f.PageSize, (f.Page-1)*f.PageSize)...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		v, err := scanVariant(rows)
		if err != nil {
			return out, err
		}
		out.Data = append(out.Data, v)
	}
	return out, rows.Err()
}

const racketCols = `id::text,brand,model,sport,min_tension_centi_kg,max_tension_centi_kg,step_centi_kg,max_difference_centi_kg,allowed_knots,different_tensions_require_four_knots,string_length_mm,technical_source,verified`

func scanRacket(row scanner) (Racket, error) {
	var r Racket
	err := row.Scan(&r.ID, &r.Brand, &r.Model, &r.Sport, &r.MinTension, &r.MaxTension, &r.Step, &r.MaxDifference, &r.AllowedKnots, &r.DifferentRequiresFour, &r.StringLengthMM, &r.TechnicalSource, &r.Verified)
	return r, missing(err)
}
func loadRacket(ctx context.Context, db DB, id string) (Racket, error) {
	return scanRacket(db.QueryRow(ctx, `SELECT `+racketCols+` FROM racket_models WHERE id=$1 AND verified FOR SHARE`, id))
}
func (s *Store) ListRackets(ctx context.Context, f Filters) (Page[Racket], error) {
	out := Page[Racket]{Data: []Racket{}, Page: f.Page, PageSize: f.PageSize}
	base := ` FROM racket_models WHERE verified AND ($1='' OR sport=$1) AND ($2='' OR brand ILIKE '%'||$2||'%' OR model ILIKE '%'||$2||'%')`
	if err := s.Pool.QueryRow(ctx, `SELECT count(*)`+base, f.Sport, f.Query).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+racketCols+base+` ORDER BY brand,model,id LIMIT $3 OFFSET $4`, f.Sport, f.Query, f.PageSize, (f.Page-1)*f.PageSize)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		r, e := scanRacket(rows)
		if e != nil {
			return out, e
		}
		out.Data = append(out.Data, r)
	}
	return out, rows.Err()
}

const serviceCols = `ss.id::text,ss.sport,ss.active,labour_cents,own_strings,own_strings_cents,stencil,stencil_cents,two_knots,two_knots_cents,four_knots,four_knots_cents,different_tensions,different_tensions_cents,min_tension_centi_kg,max_tension_centi_kg,step_centi_kg,max_difference_centi_kg,COALESCE((SELECT jsonb_object_agg(percent,price_cents) FROM service_prestretch_options po WHERE po.service_id=ss.id),'{}'::jsonb)`

func scanService(row scanner) (Service, error) {
	var s Service
	var options []byte
	err := row.Scan(&s.ID, &s.Sport, &s.Active, &s.LabourCents, &s.OwnStrings, &s.OwnStringsCents, &s.Stencil, &s.StencilCents, &s.TwoKnots, &s.TwoKnotsCents, &s.FourKnots, &s.FourKnotsCents, &s.DifferentTensions, &s.DifferentTensionsCents, &s.MinTension, &s.MaxTension, &s.Step, &s.MaxDifference, &options)
	if err != nil {
		return s, missing(err)
	}
	err = json.Unmarshal(options, &s.Prestretch)
	return s, err
}
func loadService(ctx context.Context, db DB, stringerID, sport string) (Service, error) {
	return scanService(db.QueryRow(ctx, `SELECT `+serviceCols+` FROM stringer_services ss WHERE ss.stringer_id=$1 AND ss.sport=$2`, stringerID, sport))
}
func (s *Store) Services(ctx context.Context, id string, includeInactive bool) ([]Service, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+serviceCols+` FROM stringer_services ss WHERE ss.stringer_id=$1 AND ($2 OR ss.active) ORDER BY ss.sport`, id, includeInactive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Service{}
	for rows.Next() {
		v, e := scanService(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) BookingOptions(ctx context.Context, id, racketID string) (BookingOptions, error) {
	out := BookingOptions{StringerID: id, AllowedKnots: []int{}, Notice: "Catalogue pre-stretch limits also apply to the selected string. Request a server quote before ordering."}
	shop, err := s.GetStringer(ctx, id)
	if err != nil {
		return out, err
	}
	if err = requireOpen(shop); err != nil {
		return out, err
	}
	out.Racket, err = loadRacket(ctx, s.Pool, racketID)
	if err != nil {
		return out, err
	}
	out.Service, err = loadService(ctx, s.Pool, id, out.Racket.Sport)
	if err != nil {
		return out, err
	}
	if !out.Service.Active {
		return out, problem(409, "service_unavailable", "This sport is not currently offered.")
	}
	a, b := out.Service.Step, out.Racket.Step
	for b != 0 {
		a, b = b, a%b
	}
	out.Step = out.Service.Step / a * out.Racket.Step
	out.MinTension = (max(out.Service.MinTension, out.Racket.MinTension) + out.Step - 1) / out.Step * out.Step
	out.MaxTension = min(out.Service.MaxTension, out.Racket.MaxTension) / out.Step * out.Step
	out.MaxDifference = min(out.Service.MaxDifference, out.Racket.MaxDifference)
	for _, k := range []int{2, 4} {
		if slices.Contains(out.Racket.AllowedKnots, k) && ((k == 2 && out.Service.TwoKnots) || (k == 4 && out.Service.FourKnots)) {
			out.AllowedKnots = append(out.AllowedKnots, k)
		}
	}
	if out.MinTension > out.MaxTension || len(out.AllowedKnots) == 0 {
		return out, problem(409, "incompatible_service", "This stringer's service constraints do not support this racket model.")
	}
	return out, nil
}

// Called inside a transaction after acquiring the stringer lock. Shared model /
// catalogue locks prevent administrative edits during repricing and reservation.
func calculate(ctx context.Context, db DB, stringerID string, b Booking) (Quote, Racket, *Variant, error) {
	r, err := loadRacket(ctx, db, b.RacketModelID)
	if err != nil {
		return Quote{}, r, nil, err
	}
	service, err := loadService(ctx, db, stringerID, r.Sport)
	if err != nil {
		return Quote{}, r, nil, err
	}
	var v *Variant
	if b.VariantID != "" {
		scope := stringerID
		if b.StringSource == "customer" {
			scope = ""
		}
		value, e := loadVariant(ctx, db, scope, b.VariantID)
		if e != nil {
			return Quote{}, r, nil, e
		}
		v = &value
	}
	quote, err := Price(b, service, r, v)
	return quote, r, v, err
}
func (s *Store) CreateQuote(ctx context.Context, a Actor, stringerID string, b Booking) (Quote, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Quote{}, err
	}
	defer tx.Rollback(ctx)
	shop, err := lockStringer(ctx, tx, stringerID)
	if err != nil {
		return Quote{}, err
	}
	if err = requireOpen(shop); err != nil {
		return Quote{}, err
	}
	var count int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM quotes WHERE customer_id=$1 AND created_at>now()-interval '1 minute'`, a.ID).Scan(&count)
	if err != nil {
		return Quote{}, err
	}
	if count >= 30 {
		return Quote{}, problem(429, "quote_rate_limit", "Please wait before requesting more quotes.")
	}
	q, r, v, err := calculate(ctx, tx, stringerID, b)
	if err != nil {
		return Quote{}, err
	}
	if b.StringSource == "stock" {
		if _, _, err = chooseLot(ctx, tx, v.InventoryID, r.StringLengthMM, false); err != nil {
			return Quote{}, err
		}
	}
	cfg, _ := json.Marshal(q.Configuration)
	lines, _ := json.Marshal(q.Breakdown)
	err = tx.QueryRow(ctx, `INSERT INTO quotes(customer_id,stringer_id,configuration,breakdown,fingerprint,total_cents,expires_at) VALUES($1,$2,$3,$4,$5,$6,now()+interval '10 minutes') RETURNING id::text,expires_at`, a.ID, stringerID, cfg, lines, q.Fingerprint, q.TotalCents).Scan(&q.ID, &q.ExpiresAt)
	if err != nil {
		return Quote{}, err
	}
	q.StringerID = stringerID
	return q, tx.Commit(ctx)
}

func (s *Store) Recommend(ctx context.Context, stringerID string, req RecommendationRequest) (Recommendations, error) {
	if err := ValidateRecommendation(req); err != nil {
		return Recommendations{}, err
	}
	shop, err := s.GetStringer(ctx, stringerID)
	if err != nil {
		return Recommendations{}, err
	}
	if err = requireOpen(shop); err != nil {
		return Recommendations{}, err
	}
	r, err := loadRacket(ctx, s.Pool, req.RacketModelID)
	if err != nil {
		return Recommendations{}, err
	}
	service, err := loadService(ctx, s.Pool, stringerID, r.Sport)
	if err != nil {
		return Recommendations{}, err
	}
	if !service.Active {
		return Recommendations{}, problem(409, "service_unavailable", "This sport is not currently offered.")
	}
	var current *Variant
	if req.CurrentVariantID != "" {
		v, e := loadVariant(ctx, s.Pool, "", req.CurrentVariantID)
		if e != nil {
			return Recommendations{}, e
		}
		if v.Sport != r.Sport {
			return Recommendations{}, invalid(map[string]string{"current_variant_id": "Current setup is for a different sport."})
		}
		current = &v
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+variantCols+variantJoin+`WHERE p.active AND v.active AND p.sport=$2 AND i.available AND EXISTS(SELECT 1 FROM inventory_lots l WHERE l.inventory_id=i.id AND ((l.kind='set' AND l.quantity-l.reserved>=1 AND l.set_length_mm >= $3) OR (l.kind='reel' AND l.quantity-l.reserved >= $3))) ORDER BY v.id`, stringerID, r.Sport, r.StringLengthMM)
	if err != nil {
		return Recommendations{}, err
	}
	defer rows.Close()
	candidates := []Variant{}
	for rows.Next() {
		v, e := scanVariant(rows)
		if e != nil {
			return Recommendations{}, e
		}
		candidates = append(candidates, v)
	}
	if err = rows.Err(); err != nil {
		return Recommendations{}, err
	}
	return Rank(req, candidates, current), nil
}

func chooseLot(ctx context.Context, db DB, inventoryID string, lengthMM int, lock bool) (string, int, error) {
	sql := `SELECT id::text,CASE WHEN kind='set' THEN 1 ELSE $2 END FROM inventory_lots WHERE inventory_id=$1 AND ((kind='set' AND quantity-reserved>=1 AND set_length_mm >= $2) OR (kind='reel' AND quantity-reserved >= $2)) ORDER BY kind DESC,created_at,id LIMIT 1`
	if lock {
		sql += ` FOR UPDATE`
	}
	var id string
	var quantity int
	err := db.QueryRow(ctx, sql, inventoryID, lengthMM).Scan(&id, &quantity)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, problem(409, "out_of_stock", "No single set or reel has enough unreserved string for this racket.")
	}
	return id, quantity, err
}

func jsonOrder(row scanner) (Order, error) {
	var o Order
	var cfg, lines []byte
	err := row.Scan(&o.ID, &o.CustomerID, &o.StringerID, &o.QuoteID, &cfg, &lines, &o.TotalCents, &o.Currency, &o.PaymentStatus, &o.Status, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return o, missing(err)
	}
	if err = json.Unmarshal(cfg, &o.Configuration); err != nil {
		return o, err
	}
	err = json.Unmarshal(lines, &o.Breakdown)
	return o, err
}

const orderCols = `id::text,customer_id::text,stringer_id::text,quote_id::text,configuration,breakdown,total_cents,currency,payment_status,status,created_at,updated_at`

// Expired quotes are never used as an authority for price or stock.
func expired(t time.Time) bool { return !t.After(time.Now()) }
