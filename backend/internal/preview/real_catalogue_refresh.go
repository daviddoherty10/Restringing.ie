package preview

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// Replace the live fictional catalogue at every preview stringer, while keeping
// old identities, balances, reservations and immutable order snapshots intact.
// Archived seed products cannot appear in public search or recommendations.
func refreshPreviewCatalogue(ctx context.Context, tx pgx.Tx) error {
	var installed bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM preview_seed_extensions WHERE name='all-real-strings-v1')`).Scan(&installed); err != nil {
		return err
	}
	if installed {
		return nil
	}
	type sportProduct struct {
		brand, model, sport, colour, source string
		gauge                               float64
		product, variant                    int
	}
	extra := []sportProduct{
		{"Yonex", "BG65", "badminton", "White", "https://www.yonex.com/bg65", 0.70, 120001, 121001},
		{"Yonex", "BG65", "badminton", "Black", "https://www.yonex.com/bg65", 0.70, 120001, 121002},
		{"Yonex", "BG80", "badminton", "Yellow", "https://www.yonex.com/bg80", 0.68, 120002, 121003},
		{"Tecnifibre", "305 Squash", "squash", "Green", "https://b2b.tecnifibre.com/en/p/06R305G.html", 1.10, 120003, 121004},
		{"Tecnifibre", "305 Squash", "squash", "Green", "https://b2b.tecnifibre.com/en/p/06R305G.html", 1.20, 120003, 121005},
	}
	for _, p := range extra {
		description := fmt.Sprintf("Real product identity. Preview prices and scores are illustrative editorial fixtures. Machine pre-stretch disabled pending verification. Source checked 2026-11-02: %s", p.source)
		if _, err := tx.Exec(ctx, `INSERT INTO string_products(id,brand,model,material,sport,description,comfort,control,power,spin,durability,max_prestretch_percent) VALUES($1,$2,$3,'multifilament',$4,$5,4,3,4,3,3,0) ON CONFLICT DO NOTHING`, ID(p.product), p.brand, p.model, p.sport, description); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO string_variants(id,product_id,gauge_mm,colour) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, ID(p.variant), ID(p.product), p.gauge, p.colour); err != nil {
			return err
		}
	}
	for i := 0; i < 40; i++ {
		if _, err := tx.Exec(ctx, `UPDATE string_products SET active=false WHERE id=$1 AND brand LIKE 'Preview %' AND description=$2`, ID(1001+i), fictional); err != nil {
			return err
		}
	}
	// Copies share canonical real variant IDs and retain independent shop prices.
	// New offers use the ordinary availability-only mode; no old stock is converted.
	for _, b := range businesses {
		for _, sport := range b.sports {
			type item struct {
				id    string
				price int
			}
			items := []item{}
			if sport == "tennis" {
				rows, err := tx.Query(ctx, `SELECT variant_id::text,material_price_cents FROM stringer_inventory WHERE stringer_id=$1 AND variant_id >= $2::uuid AND variant_id <= $3::uuid ORDER BY variant_id`, ID(108), ID(111001), ID(111021))
				if err != nil {
					return err
				}
				for rows.Next() {
					var it item
					if err = rows.Scan(&it.id, &it.price); err != nil {
						rows.Close()
						return err
					}
					items = append(items, it)
				}
				err = rows.Err()
				rows.Close()
				if err != nil {
					return err
				}
			} else {
				for _, p := range extra {
					if p.sport == sport {
						items = append(items, item{ID(p.variant), 1000})
					}
				}
			}
			for i, it := range items {
				if _, err := tx.Exec(ctx, `INSERT INTO stringer_inventory(id,stringer_id,variant_id,material_price_cents,available,track_stock,track_hybrid_stock) VALUES($1,$2,$3,$4,true,false,false) ON CONFLICT DO NOTHING`, ID(130000+b.id*100+i+map[string]int{"tennis": 0, "badminton": 30, "squash": 60}[sport]), ID(b.id), it.id, it.price+(b.id-101)*100); err != nil {
					return err
				}
			}
		}
	}
	_, err := tx.Exec(ctx, `INSERT INTO preview_seed_extensions(name) VALUES('all-real-strings-v1')`)
	return err
}
