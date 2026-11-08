package preview

import (
	"context"
	m "github.com/daviddoherty10/Restringing.ie/backend/internal/marketplace"
	"strings"
	"testing"
)

func TestIntegrationRealCatalogueUpgradePreservesExistingPreview(t *testing.T) {
	pool := previewPool(t)
	ctx := context.Background()
	// Install exactly the old dataset to exercise the additive upgrade path.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = seedDataset(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE preview_seed_metadata(singleton BOOLEAN PRIMARY KEY DEFAULT true,dataset TEXT NOT NULL,version INTEGER NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now());INSERT INTO preview_seed_metadata(singleton,dataset,version) VALUES(true,'restringing-fictional-preview',1);UPDATE stringers SET name='My old preview edits' WHERE id='90000000-0000-4000-8000-000000000101'`); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	first, err := Seed(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if first.Stringers != 8 || first.Products != 51 || first.Offerings != 321 || first.Orders != 8 || first.FinderURL == "" {
		t.Fatal("real catalogue not added", first)
	}
	store := &m.Store{Pool: pool}
	recs, err := store.Recommend(ctx, ID(108), m.RecommendationRequest{RacketModelID: ID(3001), Experience: "new"})
	if err != nil || len(recs.Results) == 0 {
		t.Fatal("real preview cannot recommend", err)
	}
	for _, rec := range recs.Results {
		if rec.Variant.Material != "multifilament" || rec.Setup == nil || rec.Variant.Brand == "Preview Strings" {
			t.Fatal("beginner did not receive a real soft-string setup", rec)
		}
	}

	profiles := []m.RecommendationRequest{
		{RacketModelID: ID(3001), Experience: "new", SessionsPerWeek: 1, Preferences: map[string]int{"comfort": 4, "control": 3, "power": 3, "spin": 2, "durability": 3}},
		{RacketModelID: ID(3001), Experience: "new", SessionsPerWeek: 1, Preferences: map[string]int{"comfort": 3, "control": 5, "power": 1, "spin": 3, "durability": 3}},
		{RacketModelID: ID(3001), Experience: "advanced", SessionsPerWeek: 4, Preferences: map[string]int{"comfort": 3, "control": 4, "power": 2, "spin": 5, "durability": 3}},
	}
	models := map[string]bool{}
	for _, profile := range profiles {
		result, e := store.Recommend(ctx, ID(108), profile)
		if e != nil || len(result.Results) == 0 {
			t.Fatal("real profile has no match", e)
		}
		rec := result.Results[0]
		models[rec.Variant.Model] = true
		t.Logf("%s: %s %s %.2f; hybrid=%t, tension=%d, estimate=%d cents", profile.Experience, rec.Variant.Brand, rec.Variant.Model, rec.Variant.GaugeMM, rec.Setup.Booking.Hybrid, rec.Setup.Booking.MainsTension, rec.Setup.EstimatedTotalCents)
	}
	if len(models) < 2 {
		t.Fatal("different goals do not change the real-catalogue suggestions")
	}
	if _, err = pool.Exec(ctx, `UPDATE stringer_inventory SET available=false,material_price_cents=1234 WHERE variant_id=$1 AND stringer_id=$2`, ID(111001), ID(108)); err != nil {
		t.Fatal(err)
	}
	second, err := Seed(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	var name string
	var available bool
	var price int
	if err = pool.QueryRow(ctx, `SELECT name FROM stringers WHERE id=$1`, ID(101)).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT available,material_price_cents FROM stringer_inventory WHERE stringer_id=$1 AND variant_id=$2`, ID(108), ID(111001)).Scan(&available, &price); err != nil {
		t.Fatal(err)
	}
	if name != "My old preview edits" || available || price != 1234 || second.Products != first.Products || second.Offerings != first.Offerings {
		t.Fatal("upgrade or reseed overwrote edits")
	}
}

func TestIntegrationRealRefreshUpgradesV1WithoutChangingOrderOrStockHistory(t *testing.T) {
	pool := previewPool(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = seedDataset(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if err = seedRealCatalogue(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE preview_seed_metadata(singleton BOOLEAN PRIMARY KEY DEFAULT true,dataset TEXT NOT NULL,version INTEGER NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now());INSERT INTO preview_seed_metadata VALUES(true,'restringing-fictional-preview',1,now());UPDATE stringer_inventory SET available=false,material_price_cents=1234 WHERE stringer_id='90000000-0000-4000-8000-000000000108' AND variant_id='90000000-0000-4000-8000-000000111001'`); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	history := func() (string, int, int) {
		var hash string
		var quantity, reserved int
		if e := pool.QueryRow(ctx, `SELECT md5(string_agg(configuration::text||breakdown::text,'|' ORDER BY id)) FROM orders`).Scan(&hash); e != nil {
			t.Fatal(e)
		}
		if e := pool.QueryRow(ctx, `SELECT sum(quantity),sum(reserved) FROM inventory_lots`).Scan(&quantity, &reserved); e != nil {
			t.Fatal(e)
		}
		return hash, quantity, reserved
	}
	h, q, r := history()
	if _, err = Seed(ctx, pool); err != nil {
		t.Fatal(err)
	}
	h2, q2, r2 := history()
	if h != h2 || q != q2 || r != r2 {
		t.Fatal("history or stock was rewritten")
	}
	store := &m.Store{Pool: pool}
	for _, business := range businesses {
		if business.pending {
			continue
		}
		for _, sport := range business.sports {
			page, e := store.ListVariants(ctx, ID(business.id), m.Filters{Sport: sport, Page: 1, PageSize: 100}, true)
			if e != nil || page.Total == 0 {
				t.Fatal("real offerings missing", business.id, sport, e)
			}
			for _, v := range page.Data {
				if strings.HasPrefix(v.Brand, "Preview") {
					t.Fatal("fictional string remains live", v)
				}
			}
		}
	}
	var active int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM string_products WHERE active`).Scan(&active); err != nil || active != 11 {
		t.Fatal("unexpected live product catalogue", active, err)
	}
	var price int
	var available bool
	if err = pool.QueryRow(ctx, `SELECT material_price_cents,available FROM stringer_inventory WHERE stringer_id=$1 AND variant_id=$2`, ID(108), ID(111001)).Scan(&price, &available); err != nil || price != 1234 || available {
		t.Fatal("existing real offering was overwritten", err)
	}
}
