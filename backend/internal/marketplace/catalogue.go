package marketplace

import (
	"context"
	"net/http"
	"strconv"
)

// Private curation reads deliberately include disabled products/variants and
// unverified rackets. They never replace the active-only public queries.
type CatalogueFilters struct {
	Filters
	ProductID string
	Active    string
	Verified  string
}
type CatalogueVariant struct {
	ID            string  `json:"id"`
	ProductID     string  `json:"product_id"`
	Brand         string  `json:"brand"`
	Model         string  `json:"model"`
	Material      string  `json:"material"`
	Sport         string  `json:"sport"`
	GaugeMM       float64 `json:"gauge_mm"`
	Colour        string  `json:"colour"`
	Active        bool    `json:"active"`
	ProductActive bool    `json:"product_active"`
}

func adminOnly(a Actor) error {
	if !a.Admin {
		return problem(403, "forbidden", "Administrator access is required.")
	}
	return nil
}
func catalogueFilters(r *http.Request) (CatalogueFilters, error) {
	f, e := filters(r)
	out := CatalogueFilters{Filters: f, ProductID: r.URL.Query().Get("product_id"), Active: r.URL.Query().Get("active"), Verified: r.URL.Query().Get("verified")}
	if e != nil {
		return out, e
	}
	if out.ProductID != "" {
		if e = identifier(out.ProductID); e != nil {
			return out, e
		}
	}
	for _, v := range []string{out.Active, out.Verified} {
		if v != "" && v != "true" && v != "false" {
			return out, problem(400, "invalid_filters", "Use true or false for active and verified filters.")
		}
	}
	return out, nil
}

const productCols = `id::text,brand,model,material,sport,description,comfort,control,power,spin,durability,max_prestretch_percent,active`

func scanProduct(row scanner) (Product, error) {
	var p Product
	err := row.Scan(&p.ID, &p.Brand, &p.Model, &p.Material, &p.Sport, &p.Description, &p.Comfort, &p.Control, &p.Power, &p.Spin, &p.Durability, &p.MaxPrestretchPercent, &p.Active)
	return p, missing(err)
}
func (s *Store) CatalogueProduct(ctx context.Context, a Actor, id string) (Product, error) {
	if e := adminOnly(a); e != nil {
		return Product{}, e
	}
	return scanProduct(s.Pool.QueryRow(ctx, `SELECT `+productCols+` FROM string_products WHERE id=$1`, id))
}
func (s *Store) CatalogueProducts(ctx context.Context, a Actor, f CatalogueFilters) (Page[Product], error) {
	out := Page[Product]{Data: []Product{}, Page: f.Page, PageSize: f.PageSize}
	if e := adminOnly(a); e != nil {
		return out, e
	}
	base := ` FROM string_products WHERE ($1='' OR brand ILIKE '%'||$1||'%' OR model ILIKE '%'||$1||'%') AND ($2='' OR sport=$2) AND ($3='' OR material=$3) AND ($4='' OR active=NULLIF($4,'')::boolean) AND ($5='' OR lower(brand)=lower($5))`
	args := []any{f.Query, f.Sport, f.Material, f.Active, f.Brand}
	return cataloguePage(ctx, s.Pool, out, `SELECT `+productCols, base, args, `brand,model,id`, scanProduct)
}

const catalogueVariantCols = `v.id::text,v.product_id::text,p.brand,p.model,p.material,p.sport,v.gauge_mm::float8,v.colour,v.active,p.active`
const catalogueVariantJoin = ` FROM string_variants v JOIN string_products p ON p.id=v.product_id `

func scanCatalogueVariant(row scanner) (CatalogueVariant, error) {
	var v CatalogueVariant
	err := row.Scan(&v.ID, &v.ProductID, &v.Brand, &v.Model, &v.Material, &v.Sport, &v.GaugeMM, &v.Colour, &v.Active, &v.ProductActive)
	return v, missing(err)
}
func (s *Store) CatalogueVariant(ctx context.Context, a Actor, id string) (CatalogueVariant, error) {
	if e := adminOnly(a); e != nil {
		return CatalogueVariant{}, e
	}
	return scanCatalogueVariant(s.Pool.QueryRow(ctx, `SELECT `+catalogueVariantCols+catalogueVariantJoin+`WHERE v.id=$1`, id))
}
func (s *Store) CatalogueVariants(ctx context.Context, a Actor, f CatalogueFilters) (Page[CatalogueVariant], error) {
	out := Page[CatalogueVariant]{Data: []CatalogueVariant{}, Page: f.Page, PageSize: f.PageSize}
	if e := adminOnly(a); e != nil {
		return out, e
	}
	base := catalogueVariantJoin + `WHERE ($1='' OR p.brand ILIKE '%'||$1||'%' OR p.model ILIKE '%'||$1||'%' OR v.gauge_mm::text ILIKE '%'||$1||'%' OR v.colour ILIKE '%'||$1||'%') AND ($2='' OR p.sport=$2) AND ($3='' OR p.material=$3) AND ($4='' OR v.active=NULLIF($4,'')::boolean) AND ($5='' OR v.product_id=NULLIF($5,'')::uuid) AND ($6::float8=0 OR v.gauge_mm::float8=$6::float8) AND ($7='' OR lower(p.brand)=lower($7))`
	args := []any{f.Query, f.Sport, f.Material, f.Active, f.ProductID, f.GaugeMM, f.Brand}
	return cataloguePage(ctx, s.Pool, out, `SELECT `+catalogueVariantCols, base, args, `p.brand,p.model,v.gauge_mm,v.id`, scanCatalogueVariant)
}
func (s *Store) CatalogueRacket(ctx context.Context, a Actor, id string) (Racket, error) {
	if e := adminOnly(a); e != nil {
		return Racket{}, e
	}
	return scanRacket(s.Pool.QueryRow(ctx, `SELECT `+racketCols+` FROM racket_models WHERE id=$1`, id))
}
func (s *Store) CatalogueRackets(ctx context.Context, a Actor, f CatalogueFilters) (Page[Racket], error) {
	out := Page[Racket]{Data: []Racket{}, Page: f.Page, PageSize: f.PageSize}
	if e := adminOnly(a); e != nil {
		return out, e
	}
	base := ` FROM racket_models WHERE ($1='' OR brand ILIKE '%'||$1||'%' OR model ILIKE '%'||$1||'%') AND ($2='' OR sport=$2) AND ($3='' OR verified=NULLIF($3,'')::boolean) AND ($4='' OR lower(brand)=lower($4))`
	args := []any{f.Query, f.Sport, f.Verified, f.Brand}
	return cataloguePage(ctx, s.Pool, out, `SELECT `+racketCols, base, args, `brand,model,id`, scanRacket)
}

// SQL fragments are fixed by the callers, never taken from query parameters.
func cataloguePage[T any](ctx context.Context, db DB, out Page[T], selectSQL, base string, args []any, order string, scan func(scanner) (T, error)) (Page[T], error) {
	if err := db.QueryRow(ctx, `SELECT count(*)`+base, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := db.Query(ctx, selectSQL+base+` ORDER BY `+order+` LIMIT `+parameter(len(args)+1)+` OFFSET `+parameter(len(args)+2), append(args, out.PageSize, (out.Page-1)*out.PageSize)...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		v, e := scan(rows)
		if e != nil {
			return out, e
		}
		out.Data = append(out.Data, v)
	}
	return out, rows.Err()
}

func parameter(n int) string { return "$" + strconv.Itoa(n) }
