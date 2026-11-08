package marketplace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type PublicTraderIdentity struct {
	Name         string `json:"name"`
	Address      string `json:"address"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	TraderStatus string `json:"trader_status"`
}
type LegalDocument struct {
	Version                   string               `json:"version"`
	Trader                    PublicTraderIdentity `json:"trader"`
	Terms                     string               `json:"terms"`
	CancellationNotice        string               `json:"cancellation_notice"`
	PriceNotice               string               `json:"price_notice"`
	Turnaround                string               `json:"turnaround"`
	AcceptanceLabel           string               `json:"acceptance_label"`
	EarlyRequestLabel         string               `json:"early_request_label"`
	EarlyAcknowledgementLabel string               `json:"early_acknowledgement_label"`
	SubmitLabel               string               `json:"submit_label"`
	WithdrawalDays            int                  `json:"withdrawal_days"`
	ReviewReference           string               `json:"review_reference"`
	Reviewed                  bool                 `json:"reviewed"`
}
type BookingAcceptance struct {
	VersionID            string `json:"version_id"`
	Accepted             bool   `json:"accepted"`
	EarlyRequest         bool   `json:"early_request"`
	EarlyAcknowledgement bool   `json:"early_acknowledgement"`
}
type CheckoutDisclosure struct {
	Enabled   bool           `json:"enabled"`
	VersionID string         `json:"version_id,omitempty"`
	Document  *LegalDocument `json:"document,omitempty"`
}
type BookingConfirmation struct {
	OrderID    string          `json:"order_id"`
	RecordedAt time.Time       `json:"recorded_at"`
	Snapshot   json.RawMessage `json:"snapshot"`
}

func auditCompliance(ctx context.Context, db DB, a Actor, kind, id, action string) error {
	_, e := db.Exec(ctx, `INSERT INTO compliance_audit(actor_id,resource_type,resource_id,action) VALUES($1,$2,$3,$4)`, a.ID, kind, id, action)
	return e
}
func legalAdmin(a Actor) error {
	if !a.Admin {
		return problem(403, "forbidden", "Administrator access is required.")
	}
	return nil
}
func validateLegalDocument(d LegalDocument) error {
	if !d.Reviewed || strings.TrimSpace(d.ReviewReference) == "" || d.WithdrawalDays < 1 || d.WithdrawalDays > 60 {
		return invalid(map[string]string{"reviewed": "Supply approved wording, a review reference and the reviewed withdrawal period before publishing."})
	}
	for field, v := range map[string]string{"version": d.Version, "name": d.Trader.Name, "address": d.Trader.Address, "phone": d.Trader.Phone, "trader_status": d.Trader.TraderStatus, "terms": d.Terms, "cancellation_notice": d.CancellationNotice, "price_notice": d.PriceNotice, "turnaround": d.Turnaround, "acceptance_label": d.AcceptanceLabel, "early_request_label": d.EarlyRequestLabel, "early_acknowledgement_label": d.EarlyAcknowledgementLabel, "submit_label": d.SubmitLabel} {
		if strings.TrimSpace(v) == "" || len(v) > 4000 {
			return invalid(map[string]string{field: "Approved information is required (maximum 4000 bytes)."})
		}
	}
	if _, e := mail.ParseAddress(d.Trader.Email); e != nil {
		return invalid(map[string]string{"email": "A valid public business contact is required."})
	}
	return nil
}
func (s *Store) PublishLegal(ctx context.Context, a Actor, business string, d LegalDocument) (CheckoutDisclosure, error) {
	if e := legalAdmin(a); e != nil {
		return CheckoutDisclosure{}, e
	}
	if e := validateLegalDocument(d); e != nil {
		return CheckoutDisclosure{}, e
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return CheckoutDisclosure{}, e
	}
	defer tx.Rollback(ctx)
	if _, e = lockStringer(ctx, tx, business); e != nil {
		return CheckoutDisclosure{}, e
	}
	if s.EnforceTrader {
		if e = s.requireTrader(ctx, tx, business); e != nil {
			return CheckoutDisclosure{}, e
		}
		var rawIdentity []byte
		if e = tx.QueryRow(ctx, `SELECT document->'public' FROM trader_verifications WHERE stringer_id=$1`, business).Scan(&rawIdentity); e != nil {
			return CheckoutDisclosure{}, e
		}
		var identity PublicTraderIdentity
		if e = json.Unmarshal(rawIdentity, &identity); e != nil {
			return CheckoutDisclosure{}, e
		}
		if identity != d.Trader {
			return CheckoutDisclosure{}, invalid(map[string]string{"trader": "Use the verified public identity in the reviewed terms."})
		}
	}
	raw, _ := json.Marshal(d)
	if _, e = tx.Exec(ctx, `UPDATE booking_legal_versions SET active=false WHERE stringer_id=$1 AND active`, business); e != nil {
		return CheckoutDisclosure{}, e
	}
	out := CheckoutDisclosure{Enabled: s.EnforceLegal, Document: &d}
	e = tx.QueryRow(ctx, `INSERT INTO booking_legal_versions(stringer_id,document,approved_by) VALUES($1,$2,$3) RETURNING id::text`, business, raw, a.ID).Scan(&out.VersionID)
	if e != nil {
		return out, e
	}
	if e = auditCompliance(ctx, tx, a, "legal_version", out.VersionID, "published_reviewed_wording"); e != nil {
		return out, e
	}
	return out, tx.Commit(ctx)
}
func currentLegal(ctx context.Context, db DB, business string) (CheckoutDisclosure, error) {
	var raw []byte
	out := CheckoutDisclosure{Enabled: true}
	e := db.QueryRow(ctx, `SELECT id::text,document FROM booking_legal_versions WHERE stringer_id=$1 AND active FOR SHARE`, business).Scan(&out.VersionID, &raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, problem(409, "legal_information_unavailable", "Approved booking disclosures are not yet available for this stringer.")
	}
	if e != nil {
		return out, e
	}
	e = json.Unmarshal(raw, &out.Document)
	return out, e
}
func (s *Store) CheckoutDisclosure(ctx context.Context, business string) (CheckoutDisclosure, error) {
	if _, e := s.GetStringer(ctx, business); e != nil {
		return CheckoutDisclosure{}, e
	}
	if !s.EnforceLegal {
		return CheckoutDisclosure{Enabled: false}, nil
	}
	return currentLegal(ctx, s.Pool, business)
}
func acceptanceHash(a *BookingAcceptance) string {
	raw, _ := json.Marshal(a)
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}
func validateAcceptance(a *BookingAcceptance, version string) error {
	if a == nil || !a.Accepted || a.VersionID != version {
		return problem(422, "legal_acceptance_required", "Review and accept the disclosed version before booking.")
	}
	if a.EarlyRequest != a.EarlyAcknowledgement {
		return problem(422, "early_performance_acknowledgement_required", "An early-performance request and its separate acknowledgement must be chosen together, or both left unselected.")
	}
	return nil
}
func legalRetry(ctx context.Context, db DB, order string, a *BookingAcceptance) error {
	var hash string
	e := db.QueryRow(ctx, `SELECT acceptance_hash FROM booking_confirmations WHERE order_id=$1`, order).Scan(&hash)
	if errors.Is(e, pgx.ErrNoRows) && a == nil {
		return nil
	}
	if e != nil {
		return missing(e)
	}
	if hash != acceptanceHash(a) {
		return problem(409, "idempotency_conflict", "This retry has different contractual acknowledgements.")
	}
	return nil
}
func (s *Store) recordConfirmation(ctx context.Context, tx pgx.Tx, a Actor, o Order, version string, acceptance *BookingAcceptance) error {
	now, e := databaseTime(ctx, tx)
	if e != nil {
		return e
	}
	notBefore := now
	snapshot := map[string]any{"order": o, "recorded_at": now, "contractual_workflow_enabled": s.EnforceLegal, "payment_method": "cash"}
	if s.EnforceLegal {
		legal, e := currentLegal(ctx, tx, o.StringerID)
		if e != nil {
			return e
		}
		if legal.VersionID != version {
			return problem(409, "legal_terms_changed", "The contractual information changed. Request a fresh quotation and review it.")
		}
		if e = validateAcceptance(acceptance, version); e != nil {
			return e
		}
		snapshot["legal"] = legal
		snapshot["acceptance"] = acceptance
		if !acceptance.EarlyRequest {
			loc, e := time.LoadLocation("Europe/Dublin")
			if e != nil {
				return e
			}
			local := now.In(loc)
			notBefore = time.Date(local.Year(), local.Month(), local.Day()+legal.Document.WithdrawalDays+1, 0, 0, 0, 0, loc)
		}
	} else if acceptance != nil {
		return problem(409, "legal_workflow_disabled", "The reviewed contractual workflow is not active.")
	}
	snapshot["work_not_before"] = notBefore
	raw, _ := json.Marshal(snapshot)
	_, e = tx.Exec(ctx, `INSERT INTO booking_confirmations(order_id,customer_id,legal_version_id,snapshot,acceptance_hash,early_requested,work_not_before) VALUES($1,$2,NULLIF($3,'')::uuid,$4,$5,$6,$7)`, o.ID, a.ID, version, raw, acceptanceHash(acceptance), acceptance != nil && acceptance.EarlyRequest, notBefore)
	return e
}
func (s *Store) Confirmation(ctx context.Context, a Actor, id string) (BookingConfirmation, error) {
	if _, e := s.GetOrder(ctx, a, id); e != nil {
		return BookingConfirmation{}, e
	}
	var out BookingConfirmation
	e := s.Pool.QueryRow(ctx, `SELECT order_id::text,recorded_at,snapshot FROM booking_confirmations WHERE order_id=$1`, id).Scan(&out.OrderID, &out.RecordedAt, &out.Snapshot)
	return out, missing(e)
}
func (s *Store) legalStartAllowed(ctx context.Context, tx pgx.Tx, order string) error {
	if !s.EnforceLegal {
		return nil
	}
	var allowed bool
	e := tx.QueryRow(ctx, `SELECT early_requested OR work_not_before<=now() FROM booking_confirmations WHERE order_id=$1 AND legal_version_id IS NOT NULL`, order).Scan(&allowed)
	if errors.Is(e, pgx.ErrNoRows) || e == nil && !allowed {
		return problem(409, "early_performance_not_authorised", "Work cannot start before the recorded withdrawal period expires without the customer's recorded express request and separate acknowledgement.")
	}
	return e
}
func registerLegalPublic(r chi.Router, s *Store) {
	r.Get("/stringers/{stringerID}/checkout-disclosure", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.CheckoutDisclosure(r.Context(), chi.URLParam(r, "stringerID"))
	}))
}
func registerLegalPrivate(r chi.Router, s *Store) {
	r.Post("/admin/stringers/{stringerID}/legal-versions", wrap(201, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var d LegalDocument
		if e := decode(w, r, &d); e != nil {
			return nil, e
		}
		return s.PublishLegal(r.Context(), actor(r), chi.URLParam(r, "stringerID"), d)
	}))
	r.Get("/orders/{orderID}/confirmation", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.Confirmation(r.Context(), actor(r), chi.URLParam(r, "orderID"))
	}))
}
func legalSummary(d CheckoutDisclosure) string {
	return fmt.Sprintf("%s: %s", d.VersionID, d.Document.Version)
}
