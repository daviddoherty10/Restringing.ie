package preview

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	m "github.com/daviddoherty10/Restringing.ie/backend/internal/marketplace"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestValidateTarget(t *testing.T) {
	for _, u := range []string{DefaultURL, "postgresql://u:p@localhost:5555/restringing_preview?sslmode=disable", "postgres://u:p@[::1]:5432/restringing_preview?search_path=preview_test"} {
		if err := ValidateTarget(u); err != nil {
			t.Errorf("rejected local target %s: %v", u, err)
		}
	}
	for _, u := range []string{"", "postgres://u:p@127.0.0.1:5432/restringing", "postgres://u:p@database.example/restringing_preview", "postgres://u:p@localhost/restringing_preview?host=production.example", "postgres://u:p@localhost/restringing_preview?dbname=production", "postgres://u:p@localhost/restringing_preview?service=production", "postgres://u:p@localhost/restringing_preview?options=-c%20search_path=public", "postgres://u:p@localhost/restringing_preview#fragment", "https://localhost/restringing_preview", "postgres://u:p@127.0.0.1.example/restringing_preview"} {
		if !errors.Is(ValidateTarget(u), ErrUnsafeTarget) {
			t.Errorf("accepted unsafe target %s", u)
		}
	}
}

func TestIntegrationConnectedDatabaseIsChecked(t *testing.T) {
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("set TEST_DATABASE_URL for PostgreSQL integration tests")
	}
	pool, err := pgxpool.New(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var name string
	if err = pool.QueryRow(context.Background(), `SELECT current_database()`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name == DatabaseName {
		t.Skip("this check requires a non-preview disposable test database")
	}
	if _, err = Seed(context.Background(), pool); !errors.Is(err, ErrUnsafeTarget) {
		t.Fatalf("seed did not check actual database: %v", err)
	}
	if _, err = SessionForRole(context.Background(), pool, "admin"); !errors.Is(err, ErrUnsafeTarget) {
		t.Fatalf("session did not check actual database: %v", err)
	}
}

func previewPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("set TEST_DATABASE_URL on a disposable PostgreSQL server with CREATE DATABASE permission")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	_, err = admin.Exec(ctx, `CREATE DATABASE restringing_preview`)
	if err != nil {
		var p *pgconn.PgError
		if !errors.As(err, &p) || p.Code != "42P04" {
			admin.Close()
			t.Fatal(err)
		}
	}
	cfg, err := pgxpool.ParseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = DatabaseName
	cfg.MaxConns = 8
	base, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("preview_seed_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = base.Exec(ctx, `CREATE SCHEMA `+quoted); err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		base.Exec(context.Background(), `DROP SCHEMA `+quoted+` CASCADE`)
		base.Close()
		admin.Close()
	})
	files, _ := filepath.Glob("../../migrations/*.sql")
	for _, file := range files {
		raw, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, strings.Split(string(raw), "-- +goose Down")[0]); e != nil {
			t.Fatal(e)
		}
	}
	return pool
}

func TestIntegrationSeedIsRepeatableAndPreservesEdits(t *testing.T) {
	pool := previewPool(t)
	ctx := context.Background()
	first, err := Seed(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if first.AlreadySeeded || first.Stringers != 7 || first.Products != 40 || first.Offerings != 164 || first.Orders != 8 {
		t.Fatalf("unexpected dataset: %+v", first)
	}
	if _, err = pool.Exec(ctx, `UPDATE stringers SET name='Edited preview business' WHERE id=$1`, ID(101)); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE inventory_lots SET quantity=quantity-1 WHERE id=$1`, ID(30100)); err != nil {
		t.Fatal(err)
	}
	second, err := Seed(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if !second.AlreadySeeded || second.Products != first.Products || second.Offerings != first.Offerings || second.Orders != first.Orders {
		t.Fatalf("reseed changed counts: %+v", second)
	}
	var name string
	var qty int
	if err = pool.QueryRow(ctx, `SELECT name FROM stringers WHERE id=$1`, ID(101)).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "Edited preview business" {
		t.Fatal("reseed overwrote preview edits")
	}
	if err = pool.QueryRow(ctx, `SELECT quantity FROM inventory_lots WHERE id=$1`, ID(30100)).Scan(&qty); err != nil {
		t.Fatal(err)
	}
	if qty != 7 {
		t.Fatal("reseed replenished stock")
	}
}

func TestIntegrationPreviewFeedsActualAPI(t *testing.T) {
	pool := previewPool(t)
	ctx := context.Background()
	if _, err := Seed(ctx, pool); err != nil {
		t.Fatal(err)
	}
	store := &m.Store{Pool: pool}
	router := m.Router(store, "http://127.0.0.1:5174")
	request := func(method, path, body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:8082/api/v1"+path, strings.NewReader(body))
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			r.Header.Set("Cookie", "session_token="+token)
			r.Header.Set("Origin", "http://127.0.0.1:5174")
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	for role, wantAdmin := range map[string]bool{"customer": false, "admin": true, "stringer": false, "staff": false} {
		s, err := SessionForRole(ctx, pool, role)
		if err != nil {
			t.Fatal(err)
		}
		w := request("GET", "/me", "", s.Token)
		if w.Code != 200 {
			t.Fatalf("%s identity: %d %s", role, w.Code, w.Body.String())
		}
		var me struct {
			ID    string `json:"id"`
			Admin bool   `json:"admin"`
		}
		if err = json.Unmarshal(w.Body.Bytes(), &me); err != nil {
			t.Fatal(err)
		}
		if me.ID != s.UserID || me.Admin != wantAdmin {
			t.Fatalf("wrong identity for %s: %+v", role, me)
		}
	}
	customer, err := SessionForRole(ctx, pool, "customer")
	if err != nil {
		t.Fatal(err)
	}
	w := request("GET", "/orders", "", customer.Token)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var orders m.Page[m.Order]
	if err = json.Unmarshal(w.Body.Bytes(), &orders); err != nil {
		t.Fatal(err)
	}
	if orders.Total != 7 {
		t.Fatalf("customer history has %d orders", orders.Total)
	}
	for _, o := range orders.Data {
		if o.PaymentStatus != "unpaid" {
			t.Fatal("preview order claims a payment was taken")
		}
		if o.CustomerID != customer.UserID {
			t.Fatal("another customer's order exposed")
		}
	}

	for _, path := range []string{"/orders/" + ID(40004) + "/tracking", "/orders/" + ID(40004) + "/messages", "/notifications"} {
		response := request("GET", path, "", customer.Token)
		if response.Code != 200 {
			t.Fatalf("preview communication %s: %d %s", path, response.Code, response.Body.String())
		}
	}
	admin, err := SessionForRole(ctx, pool, "admin")
	if err != nil {
		t.Fatal(err)
	}
	w = request("GET", "/admin/stringer-applications", "", admin.Token)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var pending m.Page[m.Stringer]
	if err = json.Unmarshal(w.Body.Bytes(), &pending); err != nil {
		t.Fatal(err)
	}
	if pending.Total != 1 {
		t.Fatal("pending approval fixture missing")
	}
	if w = request("GET", "/admin/stringer-applications", "", customer.Token); w.Code != 403 {
		t.Fatal("preview customer acquired administrator access")
	}
	w = request("GET", "/stringers?county=Dublin", "", "")
	var profiles m.Page[m.Stringer]
	if err = json.Unmarshal(w.Body.Bytes(), &profiles); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || profiles.Total != 3 {
		t.Fatalf("county filter: %d %+v", w.Code, profiles)
	}
	if w = request("GET", "/stringers/"+ID(107), "", ""); w.Code != 404 {
		t.Fatal("pending business was public")
	}
	page, err := store.ListVariants(ctx, ID(101), m.Filters{Page: 1, PageSize: 10}, true)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 26 || len(page.Data) != 10 {
		t.Fatalf("not enough stock for pagination: %+v", page)
	}
	b := m.Booking{RacketModelID: ID(3001), StringSource: "stock", VariantID: ID(2001), MainsTension: 2400, CrossesTension: 2300, Knots: 4, Stencil: true, PrestretchPercent: 5, Instructions: "Another local preview booking"}
	body, _ := json.Marshal(b)
	w = request("POST", "/stringers/"+ID(101)+"/quotes", string(body), customer.Token)
	if w.Code != 201 {
		t.Fatalf("preview quote rejected: %d %s", w.Code, w.Body.String())
	}
	var q m.Quote
	if err = json.Unmarshal(w.Body.Bytes(), &q); err != nil {
		t.Fatal(err)
	}
	if q.TotalCents != 3100 {
		t.Fatalf("unexpected server-calculated price: %+v", q)
	}
	if _, _, err = store.CreateOrder(ctx, m.Actor{ID: customer.UserID}, q.ID, "preview-new-booking-001"); err != nil {
		t.Fatal(err)
	}
	recommendations, err := store.Recommend(ctx, ID(101), m.RecommendationRequest{RacketModelID: ID(3001), Experience: "club", Preferences: map[string]int{"comfort": 5}})
	if err != nil {
		t.Fatal(err)
	}
	if len(recommendations.Results) != 3 {
		t.Fatal("recommendation fixtures missing")
	}
}

func TestIntegrationSeedStockLedgerIsConsistent(t *testing.T) {
	pool := previewPool(t)
	ctx := context.Background()
	if _, err := Seed(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var invalid int
	err := pool.QueryRow(ctx, `SELECT count(*) FROM inventory_lots l WHERE l.quantity<>(SELECT COALESCE(sum(quantity_delta),0) FROM inventory_movements WHERE lot_id=l.id) OR l.reserved<>(SELECT COALESCE(sum(reserved_delta),0) FROM inventory_movements WHERE lot_id=l.id) OR l.reserved<>(SELECT COALESCE(sum(quantity),0) FROM inventory_reservations WHERE lot_id=l.id AND state='reserved')`).Scan(&invalid)
	if err != nil {
		t.Fatal(err)
	}
	if invalid != 0 {
		t.Fatalf("%d inconsistent stock balances", invalid)
	}
	err = pool.QueryRow(ctx, `SELECT count(*) FROM orders o WHERE updated_at<(SELECT max(created_at) FROM order_status_events WHERE order_id=o.id)`).Scan(&invalid)
	if err != nil {
		t.Fatal(err)
	}
	if invalid != 0 {
		t.Fatal("inconsistent order timelines")
	}
	var a, b int
	if err = pool.QueryRow(ctx, `SELECT material_price_cents FROM stringer_inventory WHERE stringer_id=$1 AND variant_id=$2`, ID(101), ID(2001)).Scan(&a); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT material_price_cents FROM stringer_inventory WHERE stringer_id=$1 AND variant_id=$2`, ID(102), ID(2001)).Scan(&b); err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("shared string has no per-stringer price variation")
	}
}

func TestIntegrationSeedRefusesMixedDataAndRollsBack(t *testing.T) {
	ctx := context.Background()
	t.Run("nonempty", func(t *testing.T) {
		pool := previewPool(t)
		if _, err := pool.Exec(ctx, `INSERT INTO users(phone_number,phone_verified_at) VALUES('+447700900999',now())`); err != nil {
			t.Fatal(err)
		}
		if _, err := Seed(ctx, pool); err == nil {
			t.Fatal("mixed fixtures into preexisting data")
		}
		var n int
		pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
		if n != 1 {
			t.Fatal("modified preexisting data")
		}
	})
	t.Run("atomic", func(t *testing.T) {
		pool := previewPool(t)
		if _, err := pool.Exec(ctx, `ALTER TABLE string_products ADD CONSTRAINT preview_injected_failure CHECK (brand<>'Preview Strings')`); err != nil {
			t.Fatal(err)
		}
		if _, err := Seed(ctx, pool); err == nil {
			t.Fatal("expected injected seed failure")
		}
		var n int
		var noMarker bool
		if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM users)+(SELECT count(*) FROM stringers),to_regclass('preview_seed_metadata') IS NULL`).Scan(&n, &noMarker); err != nil {
			t.Fatal(err)
		}
		if n != 0 || !noMarker {
			t.Fatal("failed seed left partial records")
		}
	})
}

func TestIntegrationSeedConcurrentAndSessionIsolation(t *testing.T) {
	pool := previewPool(t)
	ctx := context.Background()
	if _, err := SessionForRole(ctx, pool, "admin"); err == nil {
		t.Fatal("issued a session before fixtures existed")
	}
	var wg sync.WaitGroup
	results := make(chan Summary, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); s, e := Seed(ctx, pool); results <- s; errs <- e }()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	initial := 0
	for s := range results {
		if !s.AlreadySeeded {
			initial++
		}
	}
	if initial != 1 {
		t.Fatal("concurrent seed did not serialize")
	}
	s, err := SessionForRole(ctx, pool, "customer")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(s.Token)
	if err != nil || len(raw) != 32 {
		t.Fatal("weak preview token")
	}
	if time.Until(s.ExpiresAt) > time.Hour+time.Second || time.Until(s.ExpiresAt) < 59*time.Minute {
		t.Fatal("unexpected preview session lifetime")
	}
	digest := sha256.Sum256([]byte(s.Token))
	var stored []byte
	if err = pool.QueryRow(ctx, `SELECT token_hash FROM sessions WHERE user_id=$1`, s.UserID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if string(stored) != string(digest[:]) {
		t.Fatal("preview session bypassed hashed token storage")
	}
	if !strings.Contains(s.BrowserCommand, `location.origin !== "http://127.0.0.1:5174"`) {
		t.Fatal("browser command missing origin guard")
	}
	if _, err = SessionForRole(ctx, pool, "arbitrary-user-id"); err == nil {
		t.Fatal("accepted an arbitrary login identity")
	}
	if _, err = pool.Exec(ctx, `UPDATE users SET phone_number='+447700900999' WHERE id=$1`, ID(2)); err != nil {
		t.Fatal(err)
	}
	if _, err = SessionForRole(ctx, pool, "admin"); err == nil {
		t.Fatal("issued session for a replaced fixture identity")
	}
}
