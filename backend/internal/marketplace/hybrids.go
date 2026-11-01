package marketplace

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/go-chi/chi/v5"
)

func variantLabel(v Variant) string {
	return fmt.Sprintf("%s %s %.2f mm %s", v.Brand, v.Model, v.GaugeMM, v.Colour)
}

type hybridAllocation struct {
	component, lot string
	quantity       int
}

// Reserve one whole half-racket portion per component. Provisional demand is
// subtracted locally before the second selection, including identical variants.
// The caller holds the business lock; stock writes and transitions share it.
func hybridAllocations(ctx context.Context, db DB, m, c *Variant, lock bool) ([]hybridAllocation, error) {
	if m == nil || c == nil {
		return nil, problem(422, "invalid_hybrid", "Choose both stock strings.")
	}
	used := map[string]int{}
	out := []hybridAllocation{}
	for _, component := range []struct {
		name string
		v    *Variant
	}{{"mains", m}, {"crosses", c}} {
		if !component.v.TrackHybridStock {
			continue
		}
		query := `SELECT id::text,quantity-reserved FROM inventory_lots WHERE inventory_id=$1 AND kind='half_racket' AND quantity>reserved ORDER BY created_at,id LIMIT 2`
		if lock {
			query += ` FOR UPDATE`
		}
		rows, e := db.Query(ctx, query, component.v.InventoryID)
		if e != nil {
			return nil, e
		}
		lot := ""
		for rows.Next() {
			var id string
			var free int
			if e = rows.Scan(&id, &free); e != nil {
				rows.Close()
				return nil, e
			}
			if lot == "" && free-used[id] >= 1 {
				lot = id
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
		if lot == "" {
			return nil, problem(409, "hybrid_out_of_stock", "Not enough unreserved half-racket portions for both selected strings. Choose another setup.")
		}
		used[lot]++
		out = append(out, hybridAllocation{component.name, lot, 1})
	}
	return out, nil
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
	if !service.Active || !service.Hybrid || !service.FourKnots || !slices.Contains(r.AllowedKnots, 4) {
		return out, problem(422, "hybrid_unavailable", "Hybrid stringing is not configured for this stringer and racket.")
	}
	base := variantJoin + `WHERE v.active AND p.active AND i.available AND p.sport=$2 AND p.material<>'hybrid' AND (NOT i.track_hybrid_stock OR EXISTS(SELECT 1 FROM inventory_lots l WHERE l.inventory_id=i.id AND l.kind='half_racket' AND l.quantity-l.reserved >= 1)) AND ($3='' OR p.brand ILIKE '%'||$3||'%' OR p.model ILIKE '%'||$3||'%' OR v.colour ILIKE '%'||$3||'%') AND ($4='' OR p.material=$4) AND ($5='' OR lower(p.brand)=lower($5)) AND ($6='' OR v.id=NULLIF($6,'')::uuid) AND ($7::float8=0 OR v.gauge_mm::float8=$7::float8)`
	return cataloguePage(ctx, s.Pool, out, `SELECT `+variantCols, base, []any{business, r.Sport, f.Query, f.Material, f.Brand, f.VariantID, f.GaugeMM}, `p.brand,p.model,v.gauge_mm,v.id`, scanVariant)
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
