// Package preview provisions fictional fixtures in a dedicated local database.
// It is used by cmd/preview only, never by the production HTTP server.
package preview

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const DatabaseName = "restringing_preview"
const DefaultURL = "postgres://restringing_preview:preview_local_only@127.0.0.1:5544/restringing_preview?sslmode=disable"
const datasetName = "restringing-fictional-preview"
const datasetVersion = 1

var ErrUnsafeTarget = errors.New("preview tools require a loopback PostgreSQL URL with database name restringing_preview; normal DATABASE_URL is not used")

func ValidateTarget(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Fragment != "" || u.Path != "/"+DatabaseName || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1") {
		return ErrUnsafeTarget
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return ErrUnsafeTarget
	}
	// Connection-override parameters must not redirect an apparently safe URL.
	for key := range q {
		if key != "sslmode" && key != "connect_timeout" && key != "application_name" && key != "search_path" {
			return ErrUnsafeTarget
		}
	}
	return nil
}

func Open(ctx context.Context, raw string) (*pgxpool.Pool, error) {
	if err := ValidateTarget(raw); err != nil {
		return nil, err
	}
	cfg, err := pgxpool.ParseConfig(raw)
	if err != nil {
		return nil, errors.New("invalid preview connection configuration")
	}
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err = checkDatabase(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func checkDatabase(ctx context.Context, db queryer) error {
	var name string
	if err := db.QueryRow(ctx, `SELECT current_database()`).Scan(&name); err != nil {
		return err
	}
	if name != DatabaseName {
		return ErrUnsafeTarget
	}
	return nil
}

func ID(n int) string    { return fmt.Sprintf("90000000-0000-4000-8000-%012d", n) }
func phone(n int) string { return fmt.Sprintf("+447700900%03d", n) }

type Summary struct {
	Dataset       string `json:"dataset"`
	AlreadySeeded bool   `json:"already_seeded"`
	Stringers     int    `json:"stringers"`
	Products      int    `json:"products"`
	Offerings     int    `json:"offerings"`
	Orders        int    `json:"orders"`
	ProfileURL    string `json:"profile_url"`
	BookingURL    string `json:"booking_url"`
}

func summary(ctx context.Context, db queryer, already bool) (Summary, error) {
	s := Summary{Dataset: datasetName, AlreadySeeded: already, ProfileURL: "http://127.0.0.1:5174/stringers/" + ID(101), BookingURL: "http://127.0.0.1:5174/stringers/" + ID(101) + "/book"}
	err := db.QueryRow(ctx, `SELECT (SELECT count(*) FROM stringers),(SELECT count(*) FROM string_products),(SELECT count(*) FROM stringer_inventory),(SELECT count(*) FROM orders)`).Scan(&s.Stringers, &s.Products, &s.Offerings, &s.Orders)
	return s, err
}

func requireDataset(ctx context.Context, db queryer) error {
	var name string
	var version int
	err := db.QueryRow(ctx, `SELECT dataset,version FROM preview_seed_metadata WHERE singleton=true`).Scan(&name, &version)
	if err != nil {
		return errors.New("preview data is not installed; run scripts/preview.sh seed first")
	}
	if name != datasetName || version != datasetVersion {
		return errors.New("preview dataset version mismatch; use a fresh dedicated preview database")
	}
	return nil
}

func Status(ctx context.Context, pool *pgxpool.Pool) (Summary, error) {
	if err := checkDatabase(ctx, pool); err != nil {
		return Summary{}, err
	}
	if err := requireDataset(ctx, pool); err != nil {
		return Summary{}, err
	}
	return summary(ctx, pool, true)
}

func Seed(ctx context.Context, pool *pgxpool.Pool) (Summary, error) {
	if err := checkDatabase(ctx, pool); err != nil {
		return Summary{}, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Summary{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('restringing-preview-seed',0))`); err != nil {
		return Summary{}, err
	}
	// Metadata is deliberately not a production goose migration.
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS preview_seed_metadata(singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK(singleton),dataset TEXT NOT NULL,version INTEGER NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return Summary{}, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM preview_seed_metadata)`).Scan(&exists); err != nil {
		return Summary{}, err
	}
	if exists {
		if err = requireDataset(ctx, tx); err != nil {
			return Summary{}, err
		}
		s, e := summary(ctx, tx, true)
		if e != nil {
			return s, e
		}
		return s, tx.Commit(ctx)
	}
	// Prevent another local process inserting application rows between the
	// empty-database check and the fixture transaction committing.
	if _, err = tx.Exec(ctx, `LOCK TABLE users,stringers,string_products,racket_models,orders IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return Summary{}, fmt.Errorf("apply the goose migrations to the preview database before seeding: %w", err)
	}
	var populated bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users UNION ALL SELECT 1 FROM stringers UNION ALL SELECT 1 FROM string_products UNION ALL SELECT 1 FROM racket_models UNION ALL SELECT 1 FROM orders)`).Scan(&populated)
	if err != nil {
		return Summary{}, fmt.Errorf("apply the goose migrations to the preview database before seeding: %w", err)
	}
	if populated {
		return Summary{}, errors.New("refusing to mix preview fixtures with an unmarked nonempty database")
	}
	if err = seedDataset(ctx, tx); err != nil {
		return Summary{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO preview_seed_metadata(singleton,dataset,version) VALUES(true,$1,$2)`, datasetName, datasetVersion); err != nil {
		return Summary{}, err
	}
	s, err := summary(ctx, tx, false)
	if err != nil {
		return s, err
	}
	return s, tx.Commit(ctx)
}

type Session struct {
	Role           string    `json:"role"`
	UserID         string    `json:"user_id"`
	Token          string    `json:"token"`
	ExpiresAt      time.Time `json:"expires_at"`
	BrowserCommand string    `json:"browser_command"`
}

func SessionForRole(ctx context.Context, pool *pgxpool.Pool, role string) (Session, error) {
	users := map[string]int{"customer": 1, "admin": 2, "staff": 3, "stringer": 4}
	n, ok := users[role]
	if !ok {
		return Session{}, errors.New("choose preview role customer, stringer, staff or admin")
	}
	if err := checkDatabase(ctx, pool); err != nil {
		return Session{}, err
	}
	if err := requireDataset(ctx, pool); err != nil {
		return Session{}, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)
	var roleInDB, status, number string
	err = tx.QueryRow(ctx, `SELECT platform_role,status,phone_number FROM users WHERE id=$1 FOR SHARE`, ID(n)).Scan(&roleInDB, &status, &number)
	if err != nil {
		return Session{}, err
	}
	if status != "active" || number != phone(n) || (role == "admin") != (roleInDB == "admin") {
		return Session{}, errors.New("preview account was changed; refusing to issue a session for an unexpected identity")
	}
	if role == "staff" || role == "stringer" {
		var member string
		err = tx.QueryRow(ctx, `SELECT role FROM stringer_members WHERE stringer_id=$1 AND user_id=$2`, ID(101), ID(n)).Scan(&member)
		if err != nil || (role == "staff" && member != "staff") || (role == "stringer" && member != "owner") {
			return Session{}, errors.New("preview membership was changed")
		}
	}
	var raw [32]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return Session{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	digest := sha256.Sum256([]byte(token))
	out := Session{Role: role, UserID: ID(n), Token: token}
	err = tx.QueryRow(ctx, `INSERT INTO sessions(user_id,token_hash,expires_at) VALUES($1,$2,now()+interval '1 hour') RETURNING expires_at`, out.UserID, digest[:]).Scan(&out.ExpiresAt)
	if err != nil {
		return Session{}, err
	}
	// Deliberate local-only browser fixture. The production API still requires
	// real OTP-issued HttpOnly sessions; it never imports this package.
	out.BrowserCommand = `if (location.origin !== "http://127.0.0.1:5174") { throw new Error("Open the local preview at http://127.0.0.1:5174 first."); } document.cookie = "session_token=` + token + `; Path=/; Max-Age=3600; SameSite=Lax"; location.reload();`
	return out, tx.Commit(ctx)
}
