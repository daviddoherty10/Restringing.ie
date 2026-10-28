package marketplace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type GripInput struct {
	Brand       string `json:"brand"`
	Model       string `json:"model"`
	Kind        string `json:"kind"`
	Colour      string `json:"colour"`
	Description string `json:"description"`
	Active      bool   `json:"active"`
}
type GripProduct struct {
	ID string `json:"id"`
	GripInput
}
type GripOffering struct {
	GripProduct
	InventoryID      string `json:"inventory_id"`
	FittedPriceCents int    `json:"fitted_price_cents"`
	Available        bool   `json:"available"`
}
type GripStock struct {
	GripOffering
	Quantity int `json:"quantity"`
	Reserved int `json:"reserved"`
}
type GripOfferingInput struct {
	FittedPriceCents int  `json:"fitted_price_cents"`
	Available        bool `json:"available"`
}
type GripStockInput struct {
	QuantityDelta int    `json:"quantity_delta"`
	Reason        string `json:"reason"`
}
type GripMovement struct {
	ID            string    `json:"id"`
	InventoryID   string    `json:"inventory_id"`
	QuantityDelta int       `json:"quantity_delta"`
	ReservedDelta int       `json:"reserved_delta"`
	Reason        string    `json:"reason"`
	OrderID       *string   `json:"order_id"`
	CreatedAt     time.Time `json:"created_at"`
}
type GripFilters struct {
	CatalogueFilters
	Kind          string
	GripProductID string
}

func gripFilters(r *http.Request) (GripFilters, error) {
	f, e := catalogueFilters(r)
	out := GripFilters{CatalogueFilters: f, Kind: r.URL.Query().Get("kind"), GripProductID: r.URL.Query().Get("grip_product_id")}
	if e != nil {
		return out, e
	}
	if out.Kind != "" && out.Kind != "overgrip" && out.Kind != "replacement_grip" {
		return out, problem(400, "invalid_filters", "Choose overgrip or replacement_grip.")
	}
	if out.GripProductID != "" {
		e = identifier(out.GripProductID)
	}
	return out, e
}

const gripCols = `p.id::text,p.brand,p.model,p.kind,p.colour,p.description,p.active`

func scanGrip(row scanner) (GripProduct, error) {
	var p GripProduct
	e := row.Scan(&p.ID, &p.Brand, &p.Model, &p.Kind, &p.Colour, &p.Description, &p.Active)
	return p, missing(e)
}
func (s *Store) ListGripProducts(ctx context.Context, a Actor, f GripFilters, all bool) (Page[GripProduct], error) {
	out := Page[GripProduct]{Data: []GripProduct{}, Page: f.Page, PageSize: f.PageSize}
	active := "true"
	if all {
		if e := adminOnly(a); e != nil {
			return out, e
		}
		active = f.Active
	}
	base := ` FROM grip_products p WHERE ($1='' OR p.brand ILIKE '%'||$1||'%' OR p.model ILIKE '%'||$1||'%' OR p.colour ILIKE '%'||$1||'%') AND ($2='' OR p.kind=$2) AND ($3='' OR p.active=NULLIF($3,'')::boolean) AND ($4='' OR lower(p.brand)=lower($4)) AND ($5='' OR p.id=NULLIF($5,'')::uuid)`
	return cataloguePage(ctx, s.Pool, out, `SELECT `+gripCols, base, []any{f.Query, f.Kind, active, f.Brand, f.GripProductID}, `p.brand,p.model,p.colour,p.id`, scanGrip)
}
func normalizeGrip(p GripInput) (GripInput, error) {
	p.Brand = strings.TrimSpace(p.Brand)
	p.Model = strings.TrimSpace(p.Model)
	p.Colour = strings.TrimSpace(p.Colour)
	p.Description = strings.TrimSpace(p.Description)
	fields := map[string]string{}
	if p.Brand == "" || !validText(p.Brand, 100) || strings.ContainsAny(p.Brand, "\r\n\t") {
		fields["brand"] = "Use 1–100 characters on one line."
	}
	if p.Model == "" || !validText(p.Model, 120) || strings.ContainsAny(p.Model, "\r\n\t") {
		fields["model"] = "Use 1–120 characters on one line."
	}
	if p.Kind != "overgrip" && p.Kind != "replacement_grip" {
		fields["kind"] = "Choose overgrip or replacement_grip."
	}
	if !validText(p.Colour, 80) || strings.ContainsAny(p.Colour, "\r\n\t") {
		fields["colour"] = "Use up to 80 characters on one line."
	}
	if !validText(p.Description, 1000) {
		fields["description"] = "Use up to 1000 characters."
	}
	if len(fields) > 0 {
		return p, invalid(fields)
	}
	return p, nil
}
func (s *Store) SaveGripProduct(ctx context.Context, a Actor, id, key string, input GripInput) (GripProduct, error) {
	if e := adminOnly(a); e != nil {
		return GripProduct{}, e
	}
	p, e := normalizeGrip(input)
	if e != nil {
		return GripProduct{}, e
	}
	if id != "" {
		return scanGrip(s.Pool.QueryRow(ctx, `UPDATE grip_products p SET brand=$2,model=$3,kind=$4,colour=$5,description=$6,active=$7 WHERE id=$1 RETURNING `+gripCols, id, p.Brand, p.Model, p.Kind, p.Colour, p.Description, p.Active))
	}
	if !keyPattern.MatchString(key) {
		return GripProduct{}, problem(400, "idempotency_key_required", "Supply a valid Idempotency-Key.")
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return GripProduct{}, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,8))`, a.ID+":"+key); e != nil {
		return GripProduct{}, e
	}
	raw, _ := json.Marshal(p)
	var existing string
	var same bool
	e = tx.QueryRow(ctx, `SELECT id::text,creation_request=$3::jsonb FROM grip_products WHERE created_by=$1 AND creation_key=$2`, a.ID, key, raw).Scan(&existing, &same)
	if e == nil {
		if !same {
			return GripProduct{}, problem(409, "idempotency_conflict", "This key was used for another catalogue creation.")
		}
		out, e := scanGrip(tx.QueryRow(ctx, `SELECT `+gripCols+` FROM grip_products p WHERE p.id=$1`, existing))
		if e != nil {
			return out, e
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return GripProduct{}, e
	}
	out, e := scanGrip(tx.QueryRow(ctx, `INSERT INTO grip_products AS p(brand,model,kind,colour,description,active,created_by,creation_key,creation_request) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+gripCols, p.Brand, p.Model, p.Kind, p.Colour, p.Description, p.Active, a.ID, key, raw))
	if e != nil {
		return out, e
	}
	return out, tx.Commit(ctx)
}

const gripStockCols = gripCols + `,i.id::text,i.fitted_price_cents,i.available,i.quantity,i.reserved`
const gripJoin = ` FROM grip_products p JOIN grip_inventory i ON i.grip_product_id=p.id `

func scanGripStock(row scanner) (GripStock, error) {
	var g GripStock
	e := row.Scan(&g.ID, &g.Brand, &g.Model, &g.Kind, &g.Colour, &g.Description, &g.Active, &g.InventoryID, &g.FittedPriceCents, &g.Available, &g.Quantity, &g.Reserved)
	return g, missing(e)
}
func loadGrip(ctx context.Context, db DB, business, product string) (GripOffering, error) {
	g, e := scanGripStock(db.QueryRow(ctx, `SELECT `+gripStockCols+gripJoin+`WHERE i.stringer_id=$1 AND p.id=$2 AND p.active FOR SHARE OF p`, business, product))
	return g.GripOffering, e
}
func (s *Store) gripStock(ctx context.Context, id string, f GripFilters, bookable bool) (Page[GripStock], error) {
	out := Page[GripStock]{Data: []GripStock{}, Page: f.Page, PageSize: f.PageSize}
	base := gripJoin + `WHERE i.stringer_id=$1 AND ($2='' OR p.brand ILIKE '%'||$2||'%' OR p.model ILIKE '%'||$2||'%' OR p.colour ILIKE '%'||$2||'%') AND ($3='' OR p.kind=$3) AND ($4='' OR p.id=NULLIF($4,'')::uuid) AND (NOT $5 OR (p.active AND i.available AND i.quantity-i.reserved>=1))`
	return cataloguePage(ctx, s.Pool, out, `SELECT `+gripStockCols, base, []any{id, f.Query, f.Kind, f.GripProductID, bookable}, `p.brand,p.model,p.colour,p.id`, scanGripStock)
}
func (s *Store) ManageGrips(ctx context.Context, a Actor, id string, f GripFilters) (Page[GripStock], error) {
	if e := authorize(ctx, s.Pool, a, id, false); e != nil {
		return Page[GripStock]{}, e
	}
	return s.gripStock(ctx, id, f, false)
}
func (s *Store) OfferedGrips(ctx context.Context, id string, f GripFilters) (Page[GripOffering], error) {
	out := Page[GripOffering]{Data: []GripOffering{}, Page: f.Page, PageSize: f.PageSize}
	if _, e := s.GetStringer(ctx, id); e != nil {
		return out, e
	}
	stock, e := s.gripStock(ctx, id, f, true)
	if e != nil {
		return out, e
	}
	out.Total = stock.Total
	for _, g := range stock.Data {
		out.Data = append(out.Data, g.GripOffering)
	}
	return out, nil
}
func (s *Store) SaveGripOffering(ctx context.Context, a Actor, id, product string, p GripOfferingInput) (GripStock, error) {
	if p.FittedPriceCents < 0 || p.FittedPriceCents > 1000000 {
		return GripStock{}, invalid(map[string]string{"fitted_price_cents": "Use 0–1000000 cents including fitting."})
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return GripStock{}, e
	}
	defer tx.Rollback(ctx)
	if _, e = lockStringer(ctx, tx, id); e != nil {
		return GripStock{}, e
	}
	if e = authorize(ctx, tx, a, id, false); e != nil {
		return GripStock{}, e
	}
	// Disabled catalogue products can be managed if already stocked, but cannot
	// become a new offering until an administrator activates them.
	var active, exists bool
	e = tx.QueryRow(ctx, `SELECT p.active,EXISTS(SELECT 1 FROM grip_inventory WHERE stringer_id=$1 AND grip_product_id=p.id) FROM grip_products p WHERE p.id=$2 FOR SHARE`, id, product).Scan(&active, &exists)
	if e != nil {
		return GripStock{}, missing(e)
	}
	if !active && !exists {
		return GripStock{}, problem(409, "grip_inactive", "This grip is not active in the shared catalogue.")
	}
	if _, e = tx.Exec(ctx, `INSERT INTO grip_inventory(stringer_id,grip_product_id,fitted_price_cents,available) VALUES($1,$2,$3,$4) ON CONFLICT(stringer_id,grip_product_id) DO UPDATE SET fitted_price_cents=EXCLUDED.fitted_price_cents,available=EXCLUDED.available`, id, product, p.FittedPriceCents, p.Available); e != nil {
		return GripStock{}, e
	}
	out, e := scanGripStock(tx.QueryRow(ctx, `SELECT `+gripStockCols+gripJoin+`WHERE i.stringer_id=$1 AND p.id=$2`, id, product))
	if e != nil {
		return out, e
	}
	return out, tx.Commit(ctx)
}

const gripMovementCols = `id::text,inventory_id::text,quantity_delta,reserved_delta,reason,order_id::text,created_at`

func scanGripMovement(row scanner) (GripMovement, error) {
	var m GripMovement
	e := row.Scan(&m.ID, &m.InventoryID, &m.QuantityDelta, &m.ReservedDelta, &m.Reason, &m.OrderID, &m.CreatedAt)
	return m, missing(e)
}
func (s *Store) AdjustGripStock(ctx context.Context, a Actor, id, product, key string, p GripStockInput) (GripMovement, error) {
	if !keyPattern.MatchString(key) {
		return GripMovement{}, problem(400, "idempotency_key_required", "Supply a valid Idempotency-Key.")
	}
	p.Reason = strings.TrimSpace(p.Reason)
	if p.QuantityDelta == 0 || p.QuantityDelta < -1000000 || p.QuantityDelta > 1000000 || p.Reason == "" || !validText(p.Reason, 500) {
		return GripMovement{}, invalid(map[string]string{"stock": "Use a nonzero whole-unit correction up to 1000000 and a reason of 1–500 characters."})
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return GripMovement{}, e
	}
	defer tx.Rollback(ctx)
	if _, e = lockStringer(ctx, tx, id); e != nil {
		return GripMovement{}, e
	}
	if e = authorize(ctx, tx, a, id, false); e != nil {
		return GripMovement{}, e
	}
	var inventory string
	var quantity, reserved int
	if e = tx.QueryRow(ctx, `SELECT id::text,quantity,reserved FROM grip_inventory WHERE stringer_id=$1 AND grip_product_id=$2 FOR UPDATE`, id, product).Scan(&inventory, &quantity, &reserved); e != nil {
		return GripMovement{}, missing(e)
	}
	// Serialize the same actor/key across businesses as well as the stock row.
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,9))`, a.ID+":"+key); e != nil {
		return GripMovement{}, e
	}
	previous, e := scanGripMovement(tx.QueryRow(ctx, `SELECT `+gripMovementCols+` FROM grip_inventory_movements WHERE actor_id=$1 AND idempotency_key=$2`, a.ID, key))
	if e == nil {
		if previous.InventoryID != inventory || previous.QuantityDelta != p.QuantityDelta || previous.Reason != p.Reason {
			return previous, problem(409, "idempotency_conflict", "This key was already used for another stock correction.")
		}
		return previous, tx.Commit(ctx)
	}
	var problemValue *Problem
	if !errors.As(e, &problemValue) || problemValue.Status != 404 {
		return GripMovement{}, e
	}
	if quantity+p.QuantityDelta < reserved || quantity+p.QuantityDelta > 1000000 {
		return GripMovement{}, problem(409, "stock_conflict", "The correction would remove reserved units, make stock negative or exceed the stock limit.")
	}
	if _, e = tx.Exec(ctx, `UPDATE grip_inventory SET quantity=quantity+$2 WHERE id=$1`, inventory, p.QuantityDelta); e != nil {
		return GripMovement{}, e
	}
	out, e := scanGripMovement(tx.QueryRow(ctx, `INSERT INTO grip_inventory_movements(inventory_id,actor_id,quantity_delta,reason,idempotency_key) VALUES($1,$2,$3,$4,$5) RETURNING `+gripMovementCols, inventory, a.ID, p.QuantityDelta, p.Reason, key))
	if e != nil {
		return out, e
	}
	return out, tx.Commit(ctx)
}
func (s *Store) GripMovements(ctx context.Context, a Actor, id, product string, f Filters) (Page[GripMovement], error) {
	out := Page[GripMovement]{Data: []GripMovement{}, Page: f.Page, PageSize: f.PageSize}
	if e := authorize(ctx, s.Pool, a, id, false); e != nil {
		return out, e
	}
	var inventory string
	if e := s.Pool.QueryRow(ctx, `SELECT id::text FROM grip_inventory WHERE stringer_id=$1 AND grip_product_id=$2`, id, product).Scan(&inventory); e != nil {
		return out, missing(e)
	}
	return cataloguePage(ctx, s.Pool, out, `SELECT `+gripMovementCols, ` FROM grip_inventory_movements WHERE inventory_id=$1`, []any{inventory}, `created_at DESC,id DESC`, scanGripMovement)
}
func checkGripStock(ctx context.Context, db DB, id, product string) error {
	var enough bool
	e := db.QueryRow(ctx, `SELECT quantity-reserved>=1 FROM grip_inventory WHERE stringer_id=$1 AND grip_product_id=$2`, id, product).Scan(&enough)
	if e != nil {
		return missing(e)
	}
	if !enough {
		return problem(409, "grip_out_of_stock", "This grip has no unreserved units. Choose another grip or continue without one.")
	}
	return nil
}
func gripLabel(g GripOffering) string {
	return fmt.Sprintf("%s %s · %s · %s", g.Brand, g.Model, strings.ReplaceAll(g.Kind, "_", " "), g.Colour)
}
