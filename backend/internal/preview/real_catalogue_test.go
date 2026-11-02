package preview

import (
	"context"
	m "github.com/daviddoherty10/Restringing.ie/backend/internal/marketplace"
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
	if first.Stringers != 8 || first.Products != 48 || first.Offerings != 185 || first.Orders != 8 || first.FinderURL == "" {
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
