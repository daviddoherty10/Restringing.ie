package preview

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Product identities/gauges/materials are sourced; prices and 1–5 scores are
// illustrative editorial fixtures, not manufacturer ratings or measured data.
// Do not infer a machine pre-stretch limit from a product's factory treatment.
type realProduct struct {
	brand, model, material, colour, source           string
	gauges                                           []float64
	comfort, control, power, spin, durability, price int
}

var realProducts = []realProduct{
	{"HEAD", "Velocity MLT", "multifilament", "Natural", "https://www.head.com/en_US/product/velocity-mlt-4", []float64{1.25, 1.30}, 4, 4, 3, 3, 3, 1000},
	{"Wilson", "NXT", "multifilament", "Natural", "https://www.wilson.com/en-gb/products/nxt-16-tennis-string-set-rz94270", []float64{1.30}, 5, 3, 4, 2, 3, 2000},
	{"Tecnifibre", "X-One Biphase", "multifilament", "Black", "https://www.tecnifibre.com/en/products/garniture-x-one-biphase-black", []float64{1.24, 1.30}, 5, 3, 5, 2, 2, 2400},
	{"Tecnifibre", "Multifeel", "multifilament", "Natural", "https://b2b.tecnifibre.com/en/p/02GMU135XN.html", []float64{1.25, 1.30, 1.35}, 4, 3, 4, 3, 4, 1200},
	{"Solinco", "Hyper-G", "polyester", "Green", "https://solincosports.com/equipment/hyper-g/", []float64{1.15, 1.20, 1.25, 1.30}, 2, 5, 2, 5, 5, 1000},
	{"Solinco", "Hyper-G Soft", "polyester", "Green", "https://shop.solincosports.com/products/hyper-g-soft", []float64{1.20, 1.25, 1.30}, 3, 4, 3, 5, 4, 1100},
	{"Solinco", "Tour Bite", "polyester", "Silver", "https://solincosports.com/equipment/tour-bite/", []float64{1.20, 1.25, 1.30}, 1, 5, 2, 5, 5, 1000},
	{"Luxilon", "ALU Power", "polyester", "Silver", "https://au.wilson.com/products/luxilon-alu-power-string-set", []float64{1.25}, 2, 5, 3, 4, 4, 1800},
}

// The extension runs once on fresh OR previously seeded, marked preview DBs.
// It adds a separate demo business instead of overwriting old fixtures or edits.
func seedRealCatalogue(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS preview_seed_extensions(name TEXT PRIMARY KEY,created_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	var installed bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM preview_seed_extensions WHERE name='real-strings-v1')`).Scan(&installed); err != nil {
		return err
	}
	if installed {
		return nil
	}
	description := "Preview business only. Real string names with illustrative prices and editorial scores; not a real shop or manufacturer recommendation."
	if _, err := tx.Exec(ctx, `INSERT INTO stringers(id,name,description,area,county,approval_status,accepting_orders) VALUES($1,'Preview — Real String Demo',$2,'Dún Laoghaire','Dublin','approved',true) ON CONFLICT(id) DO NOTHING`, ID(108), description); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO stringer_members(stringer_id,user_id,role) VALUES($1,$2,'owner'),($1,$3,'staff') ON CONFLICT DO NOTHING`, ID(108), ID(4), ID(3)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO stringer_approval_events(stringer_id,actor_id,status,reason) VALUES($1,$2,'approved','Preview business for real string identities; illustrative prices only')`, ID(108), ID(2)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO stringer_services(id,stringer_id,sport,active,labour_cents,own_strings,two_knots,four_knots,hybrid,different_tensions,min_tension_centi_kg,max_tension_centi_kg,step_centi_kg,max_difference_centi_kg) VALUES($1,$2,'tennis',true,1800,true,true,true,true,true,1800,3000,50,200) ON CONFLICT DO NOTHING`, ID(10080), ID(108)); err != nil {
		return err
	}
	variantIndex := 0
	for i, p := range realProducts {
		product := ID(110001 + i)
		detail := fmt.Sprintf("Real product identity. Preview price and 1–5 attribute scores are illustrative editorial data, not measured manufacturer ratings. Machine pre-stretch is disabled pending verification. Source (checked 2026-11-02): %s", p.source)
		if _, err := tx.Exec(ctx, `INSERT INTO string_products(id,brand,model,material,sport,description,comfort,control,power,spin,durability,max_prestretch_percent) VALUES($1,$2,$3,$4,'tennis',$5,$6,$7,$8,$9,$10,0) ON CONFLICT(id) DO NOTHING`, product, p.brand, p.model, p.material, detail, p.comfort, p.control, p.power, p.spin, p.durability); err != nil {
			return err
		}
		colours := []string{p.colour}
		if i == 0 {
			colours = append(colours, "Black")
		}
		for _, colour := range colours {
			for _, gauge := range p.gauges {
				variant := ID(111001 + variantIndex)
				if _, err := tx.Exec(ctx, `INSERT INTO string_variants(id,product_id,gauge_mm,colour) VALUES($1,$2,$3,$4) ON CONFLICT(id) DO NOTHING`, variant, product, gauge, colour); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `INSERT INTO stringer_inventory(id,stringer_id,variant_id,material_price_cents,available,track_stock,track_hybrid_stock) VALUES($1,$2,$3,$4,true,false,false) ON CONFLICT DO NOTHING`, ID(112001+variantIndex), ID(108), variant, p.price); err != nil {
					return err
				}
				variantIndex++
			}
		}
	}
	_, err := tx.Exec(ctx, `INSERT INTO preview_seed_extensions(name) VALUES('real-strings-v1')`)
	return err
}
