package marketplace

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
	"time"
)

type TraderDocument struct {
	LegalName    string               `json:"legal_name"`
	BusinessType string               `json:"business_type"`
	Address      string               `json:"address"`
	Email        string               `json:"email"`
	Phone        string               `json:"phone"`
	Registration string               `json:"registration"`
	Public       PublicTraderIdentity `json:"public"`
	Declaration  bool                 `json:"declaration"`
}
type TraderRecord struct {
	StringerID string         `json:"stringer_id"`
	Document   TraderDocument `json:"document"`
	Status     string         `json:"status"`
	Version    int            `json:"version"`
	Reason     string         `json:"reason"`
	UpdatedAt  time.Time      `json:"updated_at"`
}
type TraderSubmission struct {
	Version  int            `json:"version"`
	Document TraderDocument `json:"document"`
}
type TraderReview struct {
	Version             int    `json:"version"`
	Status              string `json:"status"`
	Reason              string `json:"reason"`
	IdentityChecked     bool   `json:"identity_checked"`
	ContactChecked      bool   `json:"contact_checked"`
	RegistrationChecked bool   `json:"registration_checked"`
}

const traderColumns = `stringer_id::text,document,status,version,reason,updated_at`

func scanTrader(r pgx.Row) (out TraderRecord, e error) {
	var raw []byte
	e = r.Scan(&out.StringerID, &raw, &out.Status, &out.Version, &out.Reason, &out.UpdatedAt)
	if e == nil {
		e = json.Unmarshal(raw, &out.Document)
	}
	return out, missing(e)
}
func (s *Store) Trader(ctx context.Context, a Actor, id string) (TraderRecord, error) {
	if e := authorize(ctx, s.Pool, a, id, true); e != nil {
		return TraderRecord{}, e
	}
	return scanTrader(s.Pool.QueryRow(ctx, `SELECT `+traderColumns+` FROM trader_verifications WHERE stringer_id=$1`, id))
}
func (s *Store) TraderQueue(ctx context.Context, a Actor, page int) ([]TraderRecord, error) {
	if e := legalAdmin(a); e != nil {
		return nil, e
	}
	rows, e := s.Pool.Query(ctx, `SELECT `+traderColumns+` FROM trader_verifications ORDER BY updated_at DESC,stringer_id LIMIT 50 OFFSET $1`, (page-1)*50)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []TraderRecord{}
	for rows.Next() {
		v, e := scanTrader(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) SubmitTrader(ctx context.Context, a Actor, id string, in TraderSubmission) (TraderRecord, error) {
	d := in.Document
	if !d.Declaration || !oneOf(d.BusinessType, "sole_trader", "registered_business", "non_trader") {
		return TraderRecord{}, invalid(map[string]string{"declaration": "Choose a business type and confirm accuracy."})
	}
	for k, v := range map[string]string{"legal_name": d.LegalName, "address": d.Address, "email": d.Email, "phone": d.Phone, "public_name": d.Public.Name, "public_address": d.Public.Address, "public_email": d.Public.Email, "public_phone": d.Public.Phone, "trader_status": d.Public.TraderStatus} {
		if strings.TrimSpace(v) == "" || len(v) > 500 {
			return TraderRecord{}, invalid(map[string]string{k: "This business identity field is required (maximum 500 bytes)."})
		}
	}
	if len(d.Registration) > 200 || d.BusinessType == "registered_business" && strings.TrimSpace(d.Registration) == "" {
		return TraderRecord{}, invalid(map[string]string{"registration": "Provide the applicable registration reference (no ID documents or tax identifiers)."})
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return TraderRecord{}, e
	}
	defer tx.Rollback(ctx)
	if _, e = lockStringer(ctx, tx, id); e != nil {
		return TraderRecord{}, e
	}
	if e = authorize(ctx, tx, a, id, true); e != nil {
		return TraderRecord{}, e
	}
	old, e := scanTrader(tx.QueryRow(ctx, `SELECT `+traderColumns+` FROM trader_verifications WHERE stringer_id=$1`, id))
	var p *Problem
	if e != nil && (!errors.As(e, &p) || p.Status != 404) {
		return old, e
	}
	if old.Version != in.Version {
		return old, problem(409, "trader_changed", "Refresh the verification record before submitting.")
	}
	raw, _ := json.Marshal(d)
	out, e := scanTrader(tx.QueryRow(ctx, `INSERT INTO trader_verifications(stringer_id,document,status) VALUES($1,$2,'submitted') ON CONFLICT(stringer_id) DO UPDATE SET document=$2,status='submitted',version=trader_verifications.version+1,reason='',reviewed_by=NULL,reviewed_at=NULL,updated_at=now() RETURNING `+traderColumns, id, raw))
	if e != nil {
		return out, e
	}
	// A changed identity needs fresh reviewed contractual disclosures too.
	if _, e = tx.Exec(ctx, `UPDATE booking_legal_versions SET active=false WHERE stringer_id=$1 AND active`, id); e != nil {
		return out, e
	}
	if e = traderEvent(ctx, tx, a, out, "Owner submitted identity details"); e != nil {
		return out, e
	}
	return out, tx.Commit(ctx)
}
func traderEvent(ctx context.Context, db DB, a Actor, r TraderRecord, reason string) error {
	raw, _ := json.Marshal(r.Document)
	_, e := db.Exec(ctx, `INSERT INTO trader_verification_events(stringer_id,actor_id,status,document,reason) VALUES($1,$2,$3,$4,$5)`, r.StringerID, a.ID, r.Status, raw, reason)
	if e != nil {
		return e
	}
	return auditCompliance(ctx, db, a, "trader", r.StringerID, r.Status)
}
func (s *Store) ReviewTrader(ctx context.Context, a Actor, id string, in TraderReview) (TraderRecord, error) {
	if e := legalAdmin(a); e != nil {
		return TraderRecord{}, e
	}
	if !oneOf(in.Status, "needs_info", "verified", "rejected", "suspended") || strings.TrimSpace(in.Reason) == "" || len(in.Reason) > 1000 {
		return TraderRecord{}, invalid(map[string]string{"decision": "Select a decision and record the reviewed evidence reference and reason (no sensitive document content)."})
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return TraderRecord{}, e
	}
	defer tx.Rollback(ctx)
	if _, e = lockStringer(ctx, tx, id); e != nil {
		return TraderRecord{}, e
	}
	out, e := scanTrader(tx.QueryRow(ctx, `SELECT `+traderColumns+` FROM trader_verifications WHERE stringer_id=$1 FOR UPDATE`, id))
	if e != nil {
		return out, e
	}
	if out.Version != in.Version {
		return out, problem(409, "trader_changed", "Refresh the verification record.")
	}
	if in.Status == "verified" && (!in.IdentityChecked || !in.ContactChecked || (out.Document.Registration != "" && !in.RegistrationChecked)) {
		return out, invalid(map[string]string{"checks": "Complete applicable identity, contact/address and registration checks before verification."})
	}
	out, e = scanTrader(tx.QueryRow(ctx, `UPDATE trader_verifications SET status=$2,reason=$3,version=version+1,reviewed_by=$4,reviewed_at=now(),updated_at=now() WHERE stringer_id=$1 RETURNING `+traderColumns, id, in.Status, in.Reason, a.ID))
	if e != nil {
		return out, e
	}
	if e = traderEvent(ctx, tx, a, out, in.Reason); e != nil {
		return out, e
	}
	return out, tx.Commit(ctx)
}
func (s *Store) requireTrader(ctx context.Context, db DB, id string) error {
	if !s.EnforceTrader {
		return nil
	}
	var ok bool
	e := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM trader_verifications WHERE stringer_id=$1 AND status='verified')`, id).Scan(&ok)
	if e != nil {
		return e
	}
	if !ok {
		return problem(409, "trader_verification_required", "Reviewed trader verification is required before new bookings or approval.")
	}
	return nil
}
func (s *Store) PublicTrader(ctx context.Context, id string) (*PublicTraderIdentity, error) {
	if _, e := s.GetStringer(ctx, id); e != nil {
		return nil, e
	}
	var raw []byte
	e := s.Pool.QueryRow(ctx, `SELECT document->'public' FROM trader_verifications WHERE stringer_id=$1 AND status='verified'`, id).Scan(&raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var out PublicTraderIdentity
	e = json.Unmarshal(raw, &out)
	return &out, e
}
func registerTraderPrivate(r chi.Router, s *Store) {
	r.Get("/admin/traders", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		f, e := filters(r)
		if e != nil {
			return nil, e
		}
		return s.TraderQueue(r.Context(), actor(r), f.Page)
	}))
	r.Get("/stringers/{stringerID}/trader", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.Trader(r.Context(), actor(r), chi.URLParam(r, "stringerID"))
	}))
	r.Put("/stringers/{stringerID}/trader", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in TraderSubmission
		if e := decode(w, r, &in); e != nil {
			return nil, e
		}
		return s.SubmitTrader(r.Context(), actor(r), chi.URLParam(r, "stringerID"), in)
	}))
	r.Patch("/admin/stringers/{stringerID}/trader", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in TraderReview
		if e := decode(w, r, &in); e != nil {
			return nil, e
		}
		return s.ReviewTrader(r.Context(), actor(r), chi.URLParam(r, "stringerID"), in)
	}))
}
