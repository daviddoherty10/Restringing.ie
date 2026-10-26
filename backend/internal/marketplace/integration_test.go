package marketplace

import (
	"bytes"
	"context"
	"crypto/rand"
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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fixture struct {
	store                         *Store
	customer, other, owner, admin Actor
	shop                          Stringer
	variant                       Variant
	racket                        Racket
	booking                       Booking
	tokens                        map[string]string
	lot                           Lot
}

func integrationStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL for PostgreSQL integration tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("marketplace_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoted); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.MaxConns = 12
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		admin.Exec(context.Background(), `DROP SCHEMA `+quoted+` CASCADE`)
		admin.Close()
	})
	files, err := filepath.Glob("../../migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		raw, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		up := strings.Split(string(raw), "-- +goose Down")[0]
		if _, e = pool.Exec(ctx, up); e != nil {
			t.Fatalf("migration %s: %v", file, e)
		}
	}
	return &Store{Pool: pool}
}
func newFixture(t *testing.T) *fixture {
	t.Helper()
	s := integrationStore(t)
	ctx := context.Background()
	f := &fixture{store: s, tokens: map[string]string{}}
	for i, dest := range []*Actor{&f.customer, &f.other, &f.owner, &f.admin} {
		err := s.Pool.QueryRow(ctx, `INSERT INTO users(phone_number,phone_verified_at,platform_role) VALUES($1,now(),$2) RETURNING id::text`, fmt.Sprintf("+35380000000%d", i), map[bool]string{true: "admin", false: "user"}[i == 3]).Scan(&dest.ID)
		if err != nil {
			t.Fatal(err)
		}
		dest.Admin = i == 3
		var raw [32]byte
		if _, err = rand.Read(raw[:]); err != nil {
			t.Fatal(err)
		}
		token := base64.RawURLEncoding.EncodeToString(raw[:])
		digest := sha256.Sum256([]byte(token))
		if _, err = s.Pool.Exec(ctx, `INSERT INTO sessions(user_id,token_hash,expires_at) VALUES($1,$2,now()+interval '1 hour')`, dest.ID, digest[:]); err != nil {
			t.Fatal(err)
		}
		f.tokens[dest.ID] = token
	}
	shop, err := s.Apply(ctx, f.owner, ProfileInput{Name: "Fictional fixture stringer", AcceptingOrders: true})
	if err != nil {
		t.Fatal(err)
	}
	f.shop = shop
	if _, err = s.Review(ctx, f.admin, shop.ID, "approved", "test fixture"); err != nil {
		t.Fatal(err)
	}
	b, service, r, v := pricingFixture()
	r.ID = ""
	r.Verified = true
	f.racket, err = s.SaveRacket(ctx, "", r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveService(ctx, f.owner, shop.ID, service); err != nil {
		t.Fatal(err)
	}
	p, err := s.SaveProduct(ctx, "", Product{Brand: v.Brand, Model: v.Model, Material: v.Material, Sport: v.Sport, Comfort: v.Comfort, Control: v.Control, Power: v.Power, Spin: v.Spin, Durability: v.Durability, MaxPrestretchPercent: 10, Active: true})
	if err != nil {
		t.Fatal(err)
	}
	variantID, err := s.SaveVariant(ctx, "", VariantInput{ProductID: p.ID, GaugeMM: 1.25, Colour: "silver", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveInventory(ctx, f.owner, shop.ID, variantID, v.MaterialPriceCents, true); err != nil {
		t.Fatal(err)
	}
	f.variant, err = loadVariant(ctx, s.Pool, shop.ID, variantID)
	if err != nil {
		t.Fatal(err)
	}
	f.lot, err = s.AddLot(ctx, f.owner, shop.ID, variantID, "set", 1, 12000)
	if err != nil {
		t.Fatal(err)
	}
	b.RacketModelID = f.racket.ID
	b.VariantID = variantID
	f.booking = b
	return f
}
func (f *fixture) request(t *testing.T, method, path string, body any, a Actor, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	var err error
	if body != nil {
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, "http://api.test/api/v1"+path, bytes.NewReader(raw))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if a.ID != "" {
		req.Header.Set("Authorization", "Bearer "+f.tokens[a.ID])
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	Router(f.store, "http://localhost:5173").ServeHTTP(w, req)
	return w
}
func wantStatus(t *testing.T, r *httptest.ResponseRecorder, status int) {
	t.Helper()
	if r.Code != status {
		t.Fatalf("HTTP %d, expected %d: %s", r.Code, status, r.Body.String())
	}
}
func wantProblem(t *testing.T, err error, code string) {
	t.Helper()
	var p *Problem
	if !errors.As(err, &p) || p.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}

func TestIntegrationAccessControlAndInput(t *testing.T) {
	f := newFixture(t)
	wantStatus(t, f.request(t, "GET", "/strings", nil, Actor{}, nil), 200)
	wantStatus(t, f.request(t, "GET", "/orders", nil, Actor{}, map[string]string{"X-User-ID": f.customer.ID}), 401)
	wantStatus(t, f.request(t, "PUT", "/stringers/"+f.shop.ID+"/inventory/"+f.variant.ID, map[string]any{"material_price_cents": 1, "available": true}, f.customer, nil), 403)
	wantStatus(t, f.request(t, "GET", "/admin/stringer-applications", nil, f.customer, nil), 403)
	wantStatus(t, f.request(t, "GET", "/admin/stringer-applications", nil, f.admin, nil), 200)
	wantStatus(t, f.request(t, "GET", "/stringers/not-a-uuid", nil, Actor{}, nil), 400)
	wantStatus(t, f.request(t, "GET", "/strings?page_size=100000", nil, Actor{}, nil), 400)
	wantStatus(t, f.request(t, "PUT", "/stringers/"+f.shop.ID+"/inventory/"+f.variant.ID, json.RawMessage(`null`), f.owner, nil), 400)
	spoof := map[string]any{"racket_model_id": f.racket.ID, "total_cents": 1}
	wantStatus(t, f.request(t, "POST", "/stringers/"+f.shop.ID+"/quotes", spoof, f.customer, nil), 400)
	wantStatus(t, f.request(t, "GET", "/me", nil, Actor{}, map[string]string{"Cookie": "session_token=" + f.tokens[f.customer.ID]}), 200)
	headers := map[string]string{"Cookie": "session_token=" + f.tokens[f.customer.ID], "Origin": "https://evil.example"}
	wantStatus(t, f.request(t, "POST", "/stringers/"+f.shop.ID+"/quotes", f.booking, Actor{}, headers), 403)
	headers["Origin"] = "http://localhost:5173"
	wantStatus(t, f.request(t, "POST", "/stringers/"+f.shop.ID+"/quotes", f.booking, Actor{}, headers), 201)
	if _, err := f.store.Pool.Exec(context.Background(), `UPDATE sessions SET revoked_at=now() WHERE user_id=$1`, f.customer.ID); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, f.request(t, "GET", "/orders", nil, f.customer, nil), 401)
}
func TestIntegrationOrderLifecycleAndSnapshots(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	s := f.store
	q, err := s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if err != nil {
		t.Fatal(err)
	}
	response := f.request(t, "POST", "/orders", map[string]string{"quote_id": q.ID}, f.customer, map[string]string{"Idempotency-Key": "fixture-order-key-001"})
	wantStatus(t, response, 201)
	var o Order
	if err = json.Unmarshal(response.Body.Bytes(), &o); err != nil {
		t.Fatal(err)
	}
	if o.TotalCents != 2900 || o.PaymentStatus != "unpaid" {
		t.Fatalf("unexpected order %+v", o)
	}
	wantStatus(t, f.request(t, "POST", "/orders", map[string]string{"quote_id": q.ID}, f.customer, map[string]string{"Idempotency-Key": "fixture-order-key-001"}), 200)
	wantStatus(t, f.request(t, "GET", "/orders/"+o.ID, nil, f.other, nil), 404)
	wantStatus(t, f.request(t, "GET", "/stringers/"+f.shop.ID+"/orders", nil, f.other, nil), 403)
	wantStatus(t, f.request(t, "PATCH", "/orders/"+o.ID+"/status", map[string]string{"status": "accepted"}, f.customer, nil), 409)
	if _, err = s.SaveInventory(ctx, f.owner, f.shop.ID, f.variant.ID, 9999, true); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"accepted", "received", "in_progress", "ready", "completed"} {
		if _, err = s.UpdateStatus(ctx, f.owner, o.ID, status); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.GetOrder(ctx, f.customer, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalCents != 2900 || len(got.Events) != 6 || got.Status != "completed" {
		t.Fatalf("snapshot/status lost: %+v", got)
	}
	var quantity, reserved int
	if err = s.Pool.QueryRow(ctx, `SELECT quantity,reserved FROM inventory_lots WHERE id=$1`, f.lot.ID).Scan(&quantity, &reserved); err != nil {
		t.Fatal(err)
	}
	if quantity != 0 || reserved != 0 {
		t.Fatalf("stock not consumed: %d/%d", quantity, reserved)
	}
	history, err := s.ListOrders(ctx, f.customer, "", "", Filters{Page: 1, PageSize: 20})
	if err != nil || history.Total != 1 {
		t.Fatalf("history: %+v %v", history, err)
	}
}
func TestIntegrationConcurrentLastSetAndCancellation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	s := f.store
	q1, err := s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if err != nil {
		t.Fatal(err)
	}
	q2, err := s.CreateQuote(ctx, f.other, f.shop.ID, f.booking)
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		o   Order
		err error
		a   Actor
		q   string
		key string
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for i, pair := range []struct {
		a Actor
		q string
	}{{f.customer, q1.ID}, {f.other, q2.ID}} {
		wg.Add(1)
		go func(i int, a Actor, q string) {
			defer wg.Done()
			key := fmt.Sprintf("last-stock-key-%03d", i)
			o, _, e := s.CreateOrder(ctx, a, q, key)
			results <- result{o, e, a, q, key}
		}(i, pair.a, pair.q)
	}
	wg.Wait()
	close(results)
	var winner, loser result
	success := 0
	for r := range results {
		if r.err == nil {
			winner = r
			success++
		} else {
			wantProblem(t, r.err, "out_of_stock")
			loser = r
		}
	}
	if success != 1 {
		t.Fatalf("expected one successful buyer, got %d", success)
	}
	var count, reserved int
	if err = s.Pool.QueryRow(ctx, `SELECT count(*) FROM orders`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("failed checkout leaked order: %d", count)
	}
	if err = s.Pool.QueryRow(ctx, `SELECT reserved FROM inventory_lots WHERE id=$1`, f.lot.ID).Scan(&reserved); err != nil {
		t.Fatal(err)
	}
	if reserved != 1 {
		t.Fatal("wrong reservation balance")
	}
	_, err = s.AdjustLot(ctx, f.owner, f.shop.ID, f.lot.ID, -1, "cannot remove reserved set")
	wantProblem(t, err, "stock_conflict")
	if _, err = s.UpdateStatus(ctx, winner.a, winner.o.ID, "cancelled"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.CreateOrder(ctx, loser.a, loser.q, loser.key); err != nil {
		t.Fatalf("cancelled stock not released: %v", err)
	}
}
func TestIntegrationConcurrentIdempotency(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	q, err := f.store.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	ids := make(chan string, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o, _, e := f.store.CreateOrder(ctx, f.customer, q.ID, "concurrent-retry-key-001")
			if e != nil {
				errs <- e
			} else {
				ids <- o.ID
			}
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	id := ""
	for v := range ids {
		if id != "" && id != v {
			t.Fatal("duplicate orders created")
		}
		id = v
	}
	var count int
	if err = f.store.Pool.QueryRow(ctx, `SELECT count(*) FROM orders`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("got %d orders", count)
	}
}
func TestIntegrationStaleExpiredAndForeignQuotes(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	s := f.store
	q, err := s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = s.CreateOrder(ctx, f.other, q.ID, "foreign-quote-key-001")
	wantProblem(t, err, "not_found")
	if _, err = s.SaveInventory(ctx, f.owner, f.shop.ID, f.variant.ID, 1300, true); err != nil {
		t.Fatal(err)
	}
	_, _, err = s.CreateOrder(ctx, f.customer, q.ID, "stale-quote-key-001")
	wantProblem(t, err, "quote_changed")
	if _, err = s.Pool.Exec(ctx, `UPDATE quotes SET created_at=now()-interval '20 minutes',expires_at=now()-interval '1 minute' WHERE id=$1`, q.ID); err != nil {
		t.Fatal(err)
	}
	_, _, err = s.CreateOrder(ctx, f.customer, q.ID, "expired-quote-key-001")
	wantProblem(t, err, "quote_expired")
	if _, err = s.Review(ctx, f.admin, f.shop.ID, "suspended", "test"); err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	wantProblem(t, err, "stringer_unavailable")
}
func TestIntegrationCataloguePaginationAndRecommendationScope(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	s := f.store
	for i := 0; i < 28; i++ {
		p, err := s.SaveProduct(ctx, "", Product{Brand: "Demo", Model: fmt.Sprintf("Sample %02d", i), Material: "multifilament", Sport: "tennis", Comfort: 5, Control: 3, Power: 4, Spin: 2, Durability: 2, Active: true})
		if err != nil {
			t.Fatal(err)
		}
		v, err := s.SaveVariant(ctx, "", VariantInput{ProductID: p.ID, GaugeMM: 1.30, Active: true})
		if err != nil {
			t.Fatal(err)
		}
		if i < 26 {
			if _, err = s.SaveInventory(ctx, f.owner, f.shop.ID, v, 1100, i != 25); err != nil {
				t.Fatal(err)
			}
			if i != 24 {
				if _, err = s.AddLot(ctx, f.owner, f.shop.ID, v, "reel", 200000, 0); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	page, err := s.ListVariants(ctx, f.shop.ID, Filters{Page: 1, PageSize: 8, Material: "multifilament"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 25 || len(page.Data) != 8 {
		t.Fatalf("bad pagination total: %+v", page)
	}
	second, err := s.ListVariants(ctx, f.shop.ID, Filters{Page: 2, PageSize: 8, Material: "multifilament"}, true)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, v := range page.Data {
		seen[v.ID] = true
	}
	for _, v := range second.Data {
		if seen[v.ID] {
			t.Fatal("duplicate result across pages")
		}
	}
	request := RecommendationRequest{RacketModelID: f.racket.ID, Experience: "club", SessionsPerWeek: 3, Preferences: map[string]int{"comfort": 5}, PrioritiseArmComfort: true}
	out, err := s.Recommend(ctx, f.shop.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) != 3 {
		t.Fatalf("bad recommendations %+v", out)
	}
	for _, r := range out.Results {
		if r.Variant.InventoryID == "" || !r.Variant.Available {
			t.Fatal("unoffered string recommended")
		}
	}
	wantStatus(t, f.request(t, "POST", "/stringers/"+f.shop.ID+"/recommendations", map[string]any{"racket_model_id": f.racket.ID, "experience": "club", "injury_details": "not accepted"}, Actor{}, nil), 400)
}
func TestIntegrationMigrationsReverse(t *testing.T) {
	s := integrationStore(t)
	files, _ := filepath.Glob("../../migrations/*.sql")
	ctx := context.Background()
	for i := len(files) - 1; i >= 0; i-- {
		raw, err := os.ReadFile(files[i])
		if err != nil {
			t.Fatal(err)
		}
		down := strings.Split(string(raw), "-- +goose Down")[1]
		if _, err = s.Pool.Exec(ctx, down); err != nil {
			t.Fatalf("down %s: %v", files[i], err)
		}
	}
	var count int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema()`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("left %d tables after rollback", count)
	}
}

func TestIntegrationBookingOptionsAndReelLengths(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	options, err := s.BookingOptions(ctx, f.shop.ID, f.racket.ID)
	if err != nil {
		t.Fatal(err)
	}
	if options.MinTension != 1800 || options.MaxTension != 3000 || options.Step != 50 {
		t.Fatalf("incorrect model/service intersection: %+v", options)
	}
	wantStatus(t, f.request(t, "GET", "/stringers/"+f.shop.ID+"/booking-configuration?racket_model_id="+f.racket.ID, nil, Actor{}, nil), 200)
	if _, err = s.Pool.Exec(ctx, `UPDATE inventory_lots SET set_length_mm=11000 WHERE id=$1`, f.lot.ID); err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	wantProblem(t, err, "out_of_stock")
	if _, err = s.AddLot(ctx, f.owner, f.shop.ID, f.variant.ID, "reel", 11999, 0); err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	wantProblem(t, err, "out_of_stock")
	lot, err := s.AddLot(ctx, f.owner, f.shop.ID, f.variant.ID, "reel", 12000, 0)
	if err != nil {
		t.Fatal(err)
	}
	q, err := s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if err != nil {
		t.Fatal(err)
	}
	o, _, err := s.CreateOrder(ctx, f.customer, q.ID, "exact-reel-length-key-001")
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"accepted", "received", "in_progress"} {
		if _, err = s.UpdateStatus(ctx, f.owner, o.ID, status); err != nil {
			t.Fatal(err)
		}
	}
	var quantity, reserved int
	if err = s.Pool.QueryRow(ctx, `SELECT quantity,reserved FROM inventory_lots WHERE id=$1`, lot.ID).Scan(&quantity, &reserved); err != nil {
		t.Fatal(err)
	}
	if quantity != 0 || reserved != 0 {
		t.Fatalf("incorrect reel consumption %d/%d", quantity, reserved)
	}
	page, err := s.ListVariants(ctx, "", Filters{Page: 1, PageSize: 20, GaugeMM: 1.25}, false)
	if err != nil || len(page.Data) != 1 {
		t.Fatalf("gauge filter: %+v %v", page, err)
	}
}

func TestIntegrationCrossStringerCatalogueAndRoles(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	otherShop, err := s.Apply(ctx, f.other, ProfileInput{Name: "Other fixture", AcceptingOrders: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Review(ctx, f.admin, otherShop.ID, "approved", "test")
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.SaveProduct(ctx, "", Product{Brand: "Private stock fixture", Model: "Other only", Material: "polyester", Sport: "tennis", Comfort: 1, Control: 5, Power: 2, Spin: 5, Durability: 5, Active: true})
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.SaveVariant(ctx, "", VariantInput{ProductID: p.ID, GaugeMM: 1.30, Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveInventory(ctx, f.other, otherShop.ID, v, 1000, true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddLot(ctx, f.other, otherShop.ID, v, "set", 1, 12000); err != nil {
		t.Fatal(err)
	}
	b := f.booking
	b.VariantID = v
	_, err = s.CreateQuote(ctx, f.customer, f.shop.ID, b)
	wantProblem(t, err, "not_found")
	out, err := s.Recommend(ctx, f.shop.ID, RecommendationRequest{RacketModelID: f.racket.ID, Experience: "advanced", Preferences: map[string]int{"control": 5}})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range out.Results {
		if m.Variant.ID == v {
			t.Fatal("another stringer's stock was recommended")
		}
	}
	if _, err = s.Pool.Exec(ctx, `INSERT INTO stringer_members(stringer_id,user_id,role) VALUES($1,$2,'staff')`, f.shop.ID, f.customer.ID); err != nil {
		t.Fatal(err)
	}
	_, service, _, _ := pricingFixture()
	_, err = s.SaveService(ctx, f.customer, f.shop.ID, service)
	wantProblem(t, err, "forbidden")
	if _, err = s.SaveInventory(ctx, f.customer, f.shop.ID, f.variant.ID, 1250, true); err != nil {
		t.Fatal("staff should manage stock", err)
	}
}
