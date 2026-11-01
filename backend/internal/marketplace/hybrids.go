package marketplace

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func variantLabel(v Variant) string {
	return fmt.Sprintf("%s %s %.2f mm %s", v.Brand, v.Model, v.GaugeMM, v.Colour)
}
func validHybridLengths(r Racket) bool {
	if r.MainsLengthMM == 0 && r.CrossesLengthMM == 0 && r.HybridTechnicalSource == "" {
		return true
	}
	return r.MainsLengthMM >= 1000 && r.MainsLengthMM <= 30000 && r.CrossesLengthMM >= 1000 && r.CrossesLengthMM <= 30000 && r.MainsLengthMM+r.CrossesLengthMM <= 30000 && slices.Contains(r.AllowedKnots, 4) && strings.TrimSpace(r.HybridTechnicalSource) != "" && validText(r.HybridTechnicalSource, 1000)
}

type HybridPrices struct {
	Mains   *int `json:"mains_price_cents"`
	Crosses *int `json:"crosses_price_cents"`
}

// A repeatable replacement, independent of full-bed price and physical stock.
// NULL disables that component; zero is a deliberately free component.
func (s *Store) SaveHybridPrices(ctx context.Context, a Actor, business, variant string, p HybridPrices) (HybridPrices, error) {
	for _, price := range []*int{p.Mains, p.Crosses} {
		if price != nil && (*price < 0 || *price > 1000000) {
			return p, invalid(map[string]string{"prices": "Use null to disable a component, or 0–1000000 cents."})
		}
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return p, e
	}
	defer tx.Rollback(ctx)
	if _, e = lockStringer(ctx, tx, business); e != nil {
		return p, e
	}
	if e = authorize(ctx, tx, a, business, false); e != nil {
		return p, e
	}
	result, e := tx.Exec(ctx, `UPDATE stringer_inventory SET mains_price_cents=$3,crosses_price_cents=$4 WHERE stringer_id=$1 AND variant_id=$2`, business, variant, p.Mains, p.Crosses)
	if e != nil {
		return p, e
	}
	if result.RowsAffected() != 1 {
		return p, problem(404, "not_found", "Save this string offering before configuring hybrid component prices.")
	}
	return p, tx.Commit(ctx)
}
func registerHybridRoutes(r chi.Router, s *Store) {
	r.Put("/stringers/{stringerID}/inventory/{variantID}/hybrid-prices", wrap(http.StatusOK, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var p HybridPrices
		if e := decode(w, r, &p); e != nil {
			return nil, e
		}
		return s.SaveHybridPrices(r.Context(), actor(r), chi.URLParam(r, "stringerID"), chi.URLParam(r, "variantID"), p)
	}))
}

type hybridAllocation struct {
	component, lot string
	quantity       int
}

// The business lock serializes stock corrections, other bookings and transitions.
// Each component comes from a single reel. Identical variants require one reel
// sufficient for the combined demand, so the same free length is never counted twice.
func hybridAllocations(ctx context.Context, db DB, r Racket, m, c *Variant, lock bool) ([]hybridAllocation, error) {
	if m == nil || c == nil || r.MainsLengthMM < 1000 || r.CrossesLengthMM < 1000 {
		return nil, problem(422, "invalid_hybrid", "Choose both strings and a model with verified component lengths.")
	}
	choose := func(inventory string, length int) (string, error) {
		query := `SELECT id::text FROM inventory_lots WHERE inventory_id=$1 AND kind='reel' AND quantity-reserved >= $2 ORDER BY created_at,id LIMIT 1`
		if lock {
			query += ` FOR UPDATE`
		}
		var id string
		e := db.QueryRow(ctx, query, inventory, length).Scan(&id)
		if e == pgx.ErrNoRows {
			return "", problem(409, "hybrid_out_of_stock", "Each hybrid component needs a sufficient unreserved reel. Identical variants need one reel for the combined length; whole sets cannot be split automatically.")
		}
		return id, e
	}
	var ml, cl string
	var e error
	if m.InventoryID == c.InventoryID {
		ml, e = choose(m.InventoryID, r.MainsLengthMM+r.CrossesLengthMM)
		cl = ml
	} else {
		ml, e = choose(m.InventoryID, r.MainsLengthMM)
		if e == nil {
			cl, e = choose(c.InventoryID, r.CrossesLengthMM)
		}
	}
	if e != nil {
		return nil, e
	}
	return []hybridAllocation{{"mains", ml, r.MainsLengthMM}, {"crosses", cl, r.CrossesLengthMM}}, nil
}

func (s *Store) HybridInventory(ctx context.Context, business, racket, component string, f Filters) (Page[Variant], error) {
	out := Page[Variant]{Data: []Variant{}, Page: f.Page, PageSize: f.PageSize}
	if component != "mains" && component != "crosses" {
		return out, problem(400, "invalid_component", "Choose mains or crosses.")
	}
	shop, e := s.GetStringer(ctx, business)
	if e != nil {
		return out, e
	}
	if e = requireOpen(shop); e != nil {
		return out, e
	}
	r, e := loadRacket(ctx, s.Pool, racket)
	if e != nil {
		return out, e
	}
	service, e := loadService(ctx, s.Pool, business, r.Sport)
	if e != nil {
		return out, e
	}
	if !service.Active || !service.Hybrid || !service.FourKnots || r.MainsLengthMM == 0 || r.CrossesLengthMM == 0 || !slices.Contains(r.AllowedKnots, 4) {
		return out, problem(422, "hybrid_unavailable", "Hybrid stringing is not configured for this stringer and racket.")
	}
	length := r.MainsLengthMM
	price := "i.mains_price_cents"
	if component == "crosses" {
		length = r.CrossesLengthMM
		price = "i.crosses_price_cents"
	}
	base := variantJoin + `WHERE v.active AND p.active AND i.available AND p.sport=$2 AND p.material<>'hybrid' AND ` + price + ` IS NOT NULL AND EXISTS(SELECT 1 FROM inventory_lots l WHERE l.inventory_id=i.id AND l.kind='reel' AND l.quantity-l.reserved >= $3) AND ($4='' OR p.brand ILIKE '%'||$4||'%' OR p.model ILIKE '%'||$4||'%' OR v.colour ILIKE '%'||$4||'%') AND ($5='' OR p.material=$5) AND ($6='' OR lower(p.brand)=lower($6)) AND ($7='' OR v.id=NULLIF($7,'')::uuid) AND ($8::float8=0 OR v.gauge_mm::float8=$8::float8)`
	return cataloguePage(ctx, s.Pool, out, `SELECT `+variantCols, base, []any{business, r.Sport, length, f.Query, f.Material, f.Brand, f.VariantID, f.GaugeMM}, `p.brand,p.model,v.gauge_mm,v.id`, scanVariant)
}
func registerHybridPublicRoutes(r chi.Router, s *Store) {
	r.Get("/stringers/{stringerID}/hybrid-inventory", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		id := r.URL.Query().Get("racket_model_id")
		if e := identifier(id); e != nil {
			return nil, e
		}
		f, e := filters(r)
		if e != nil {
			return nil, e
		}
		return s.HybridInventory(r.Context(), chi.URLParam(r, "stringerID"), id, r.URL.Query().Get("component"), f)
	}))
}
