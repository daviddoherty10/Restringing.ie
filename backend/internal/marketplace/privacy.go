package marketplace

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

type PrivacyRequest struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Kind      string    `json:"kind"`
	Details   string    `json:"details"`
	Status    string    `json:"status"`
	Outcome   string    `json:"outcome"`
	CreatedAt time.Time `json:"created_at"`
	DueAt     time.Time `json:"due_at"`
	Version   int       `json:"version"`
}
type PrivacyInput struct {
	Kind    string `json:"kind"`
	Details string `json:"details"`
}
type PrivacyReview struct {
	CorrectionFields []string `json:"correction_fields,omitempty"`
	Version          int      `json:"version"`
	Status           string   `json:"status"`
	Outcome          string   `json:"outcome"`
	IdentityVerified bool     `json:"identity_verified"`
	ExportReviewed   bool     `json:"export_reviewed"`
	FirstName        string   `json:"first_name"`
	LastName         string   `json:"last_name"`
	Email            string   `json:"email"`
	RetentionBasis   string   `json:"retention_basis"`
	RetainUntil      string   `json:"retain_until"`
}

const privacyColumns = `id::text,user_id::text,kind,details,status,outcome,created_at,due_at,version`

func scanPrivacy(row pgx.Row) (p PrivacyRequest, e error) {
	e = row.Scan(&p.ID, &p.UserID, &p.Kind, &p.Details, &p.Status, &p.Outcome, &p.CreatedAt, &p.DueAt, &p.Version)
	return
}
func (s *Store) PrivacyList(ctx context.Context, a Actor, admin bool, offset int) ([]PrivacyRequest, error) {
	if admin {
		if e := legalAdmin(a); e != nil {
			return nil, e
		}
	}
	rows, e := s.Pool.Query(ctx, `SELECT `+privacyColumns+` FROM privacy_requests WHERE ($1 OR user_id=$2) ORDER BY created_at DESC,id LIMIT 50 OFFSET $3`, admin, a.ID, offset)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []PrivacyRequest{}
	for rows.Next() {
		p, e := scanPrivacy(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) RequestPrivacy(ctx context.Context, a Actor, in PrivacyInput, key string) (PrivacyRequest, error) {
	if !keyPattern.MatchString(key) || !oneOf(in.Kind, "access", "correction", "erasure") || strings.TrimSpace(in.Details) == "" || len(in.Details) > 2000 {
		return PrivacyRequest{}, invalid(map[string]string{"request": "Select a request type, provide details (up to 2000 bytes) and a retry key."})
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return PrivacyRequest{}, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, a.ID); e != nil {
		return PrivacyRequest{}, e
	}
	p, e := scanPrivacy(tx.QueryRow(ctx, `SELECT `+privacyColumns+` FROM privacy_requests WHERE user_id=$1 AND request_key=$2`, a.ID, key))
	if e == nil {
		if p.Kind != in.Kind || p.Details != in.Details {
			return p, problem(409, "retry_changed", "Retry with the original request.")
		}
		return p, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return p, e
	}
	var n int
	e = tx.QueryRow(ctx, `SELECT count(*) FROM privacy_requests WHERE user_id=$1 AND status NOT IN ('fulfilled','refused')`, a.ID).Scan(&n)
	if e != nil {
		return p, e
	}
	if n >= 5 {
		return p, problem(429, "requests_pending", "You already have open requests. Use their status pages or the contact route for help.")
	}
	p, e = scanPrivacy(tx.QueryRow(ctx, `INSERT INTO privacy_requests(user_id,kind,details,request_key) VALUES($1,$2,$3,$4) RETURNING `+privacyColumns, a.ID, in.Kind, in.Details, key))
	if e != nil {
		return p, e
	}
	if e = auditCompliance(ctx, tx, a, "privacy_request", p.ID, "submitted"); e != nil {
		return p, e
	}
	return p, tx.Commit(ctx)
}

// Deliberately excludes tokens, OTP/provider metadata and other customers' personal details.
func privacyExport(ctx context.Context, db DB, user string) (json.RawMessage, error) {
	var raw []byte
	e := db.QueryRow(ctx, `SELECT jsonb_build_object('generated_at',now(),'account',(SELECT jsonb_build_object('id',id,'phone_number',phone_number,'first_name',first_name,'last_name',last_name,'email',email,'created_at',created_at) FROM users WHERE id=$1),'orders',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',id,'stringer_id',stringer_id,'status',status,'created_at',created_at,'total_cents',total_cents,'payment_status',payment_status)) FROM orders WHERE customer_id=$1),'[]'::jsonb),'requests',COALESCE((SELECT jsonb_agg(jsonb_build_object('kind',kind,'details',details,'outcome',outcome,'created_at',created_at)) FROM privacy_requests WHERE user_id=$1),'[]'::jsonb),'own_messages',COALESCE((SELECT jsonb_agg(jsonb_build_object('order_id',order_id,'body',body,'created_at',created_at)) FROM order_messages WHERE sender_id=$1),'[]'::jsonb),'sessions',COALESCE((SELECT jsonb_agg(jsonb_build_object('created_at',created_at,'expires_at',expires_at,'revoked_at',revoked_at)) FROM sessions WHERE user_id=$1),'[]'::jsonb),'memberships',COALESCE((SELECT jsonb_agg(jsonb_build_object('stringer_id',stringer_id,'role',role)) FROM stringer_members WHERE user_id=$1),'[]'::jsonb),'notifications',COALESCE((SELECT jsonb_agg(jsonb_build_object('order_id',order_id,'title',title,'created_at',created_at)) FROM notifications WHERE user_id=$1),'[]'::jsonb))`, user).Scan(&raw)
	if len(raw) > 2*1024*1024 {
		return nil, problem(409, "manual_export_required", "Prepare a reviewed secure export for this large account.")
	}
	return json.RawMessage(raw), e
}
func (s *Store) PrivacyExport(ctx context.Context, a Actor, id string, preview bool) (json.RawMessage, error) {
	if preview {
		if e := legalAdmin(a); e != nil {
			return nil, e
		}
	}
	p, e := scanPrivacy(s.Pool.QueryRow(ctx, `SELECT `+privacyColumns+` FROM privacy_requests WHERE id=$1 AND ($2 OR user_id=$3)`, id, preview, a.ID))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, problem(404, "not_found", "Request not found.")
	}
	if e != nil {
		return nil, e
	}
	if preview {
		if e = auditCompliance(ctx, s.Pool, a, "privacy_request", id, "export_previewed"); e != nil {
			return nil, e
		}
		return privacyExport(ctx, s.Pool, p.UserID)
	}
	var raw []byte
	e = s.Pool.QueryRow(ctx, `SELECT export FROM privacy_requests WHERE id=$1 AND kind='access' AND status='fulfilled' AND export IS NOT NULL`, id).Scan(&raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, problem(409, "export_unavailable", "Your reviewed export is not yet available.")
	}
	if e == nil {
		e = auditCompliance(ctx, s.Pool, a, "privacy_request", id, "export_downloaded")
	}
	return json.RawMessage(raw), e
}
func (s *Store) ReviewPrivacy(ctx context.Context, a Actor, id string, in PrivacyReview) (PrivacyRequest, error) {
	if e := legalAdmin(a); e != nil {
		return PrivacyRequest{}, e
	}
	if strings.TrimSpace(in.Outcome) == "" || len(in.Outcome) > 2000 || !oneOf(in.Status, "in_review", "awaiting_identity", "extended", "fulfilled", "refused") {
		return PrivacyRequest{}, invalid(map[string]string{"outcome": "Select a status and explain the decision and next steps."})
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return PrivacyRequest{}, e
	}
	defer tx.Rollback(ctx)
	p, e := scanPrivacy(tx.QueryRow(ctx, `SELECT `+privacyColumns+` FROM privacy_requests WHERE id=$1 FOR UPDATE`, id))
	if errors.Is(e, pgx.ErrNoRows) {
		return p, problem(404, "not_found", "Request not found.")
	}
	if e != nil {
		return p, e
	}
	if p.Version != in.Version || p.Status == "fulfilled" || p.Status == "refused" {
		return p, problem(409, "request_changed", "Refresh this request before acting.")
	}
	if in.Status == "extended" {
		var timely bool
		if e = tx.QueryRow(ctx, `SELECT created_at+interval '1 month'>now() AND due_at=created_at+interval '1 month' FROM privacy_requests WHERE id=$1`, id).Scan(&timely); e != nil {
			return p, e
		}
		if !timely {
			return p, problem(409, "extension_requires_review", "Extensions must be explained within the original response period; escalate an overdue request.")
		}
		_, e = tx.Exec(ctx, `UPDATE privacy_requests SET due_at=created_at+interval '3 months' WHERE id=$1`, id)
		if e != nil {
			return p, e
		}
	}
	var export json.RawMessage
	if in.Status == "fulfilled" {
		if !in.IdentityVerified {
			return p, invalid(map[string]string{"identity_verified": "Verify identity proportionately before disclosing, correcting or closing an account."})
		}
		switch p.Kind {
		case "access":
			if !in.ExportReviewed {
				return p, invalid(map[string]string{"export_reviewed": "Preview this export and review supplementary records/redactions before release."})
			}
			export, e = privacyExport(ctx, tx, p.UserID)
		case "correction":
			if len(in.CorrectionFields) == 0 {
				return p, invalid(map[string]string{"correction_fields": "Select the specific fields to correct; unselected fields are preserved."})
			}
			for _, field := range in.CorrectionFields {
				if !oneOf(field, "first_name", "last_name", "email") {
					return p, invalid(map[string]string{"correction_fields": "Only first_name, last_name and email can be corrected here."})
				}
			}
			if in.Email != "" && oneOf("email", in.CorrectionFields...) {
				if _, err := mail.ParseAddress(in.Email); err != nil {
					return p, invalid(map[string]string{"email": "Provide a valid corrected email address."})
				}
			}
			if len(in.FirstName) > 100 || len(in.LastName) > 100 || len(in.Email) > 254 {
				return p, invalid(map[string]string{"correction": "Correction fields are too long."})
			}
			_, e = tx.Exec(ctx, `UPDATE users SET first_name=CASE WHEN 'first_name'=ANY($5::text[]) THEN nullif($2,'') ELSE first_name END,last_name=CASE WHEN 'last_name'=ANY($5::text[]) THEN nullif($3,'') ELSE last_name END,email=CASE WHEN 'email'=ANY($5::text[]) THEN nullif($4,'') ELSE email END,updated_at=now() WHERE id=$1`, p.UserID, in.FirstName, in.LastName, in.Email, in.CorrectionFields)
		case "erasure":
			if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('account-closure-review',0))`); e != nil {
				return p, e
			}
			var lastAdmin bool
			if e = tx.QueryRow(ctx, `SELECT platform_role='admin' AND (SELECT count(*) FROM users WHERE platform_role='admin' AND status='active')<2 FROM users WHERE id=$1 FOR UPDATE`, p.UserID).Scan(&lastAdmin); e != nil {
				return p, e
			}
			if lastAdmin {
				return p, problem(409, "admin_handover_required", "Assign another active administrator before closing the last administrator account.")
			}
			var blocked bool
			e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM stringer_members WHERE user_id=$1) OR EXISTS(SELECT 1 FROM orders WHERE customer_id=$1 AND status NOT IN ('completed','cancelled'))`, p.UserID).Scan(&blocked)
			if e != nil {
				return p, e
			}
			if blocked {
				return p, problem(409, "dependencies_unresolved", "Resolve business ownership/membership, billing and active orders before account closure.")
			}
			var retained bool
			e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM privacy_requests WHERE user_id=$1)`, p.UserID).Scan(&retained)
			if e != nil {
				return p, e
			}
			if retained {
				until, err := time.Parse("2006-01-02", in.RetainUntil)
				if err != nil || !until.After(time.Now()) || strings.TrimSpace(in.RetentionBasis) == "" {
					return p, invalid(map[string]string{"retention_basis": "Review retained requests, orders, messages and contracts, their legal basis, scope and future deletion date. Explain the partial erasure in the outcome."})
				}
			}
			_, e = tx.Exec(ctx, `DELETE FROM auth_challenges WHERE phone_number=(SELECT phone_number FROM users WHERE id=$1)`, p.UserID)
			if e == nil {
				_, e = tx.Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, p.UserID)
			}
			if e == nil {
				_, e = tx.Exec(ctx, `DELETE FROM notifications WHERE user_id=$1`, p.UserID)
			}
			if e == nil {
				_, e = tx.Exec(ctx, `UPDATE users SET phone_number='closed:'||id::text,first_name=NULL,last_name=NULL,email=NULL,status='closed',updated_at=now() WHERE id=$1`, p.UserID)
			}
		}
		if e != nil {
			return p, e
		}
	}
	p, e = scanPrivacy(tx.QueryRow(ctx, `UPDATE privacy_requests SET status=$2,outcome=$3,version=version+1,updated_at=now(),identity_verified_by=CASE WHEN $4 THEN $5::uuid ELSE identity_verified_by END,export=$6,retention_basis=$7,retain_until=nullif($8,'')::date WHERE id=$1 RETURNING `+privacyColumns, id, in.Status, in.Outcome, in.IdentityVerified, a.ID, export, in.RetentionBasis, in.RetainUntil))
	if e != nil {
		return p, e
	}
	history, _ := json.Marshal(map[string]any{"correction_fields": in.CorrectionFields, "version": p.Version, "due_at": p.DueAt, "identity_verified": in.IdentityVerified, "export_reviewed": in.ExportReviewed, "retention_basis": in.RetentionBasis, "retain_until": in.RetainUntil})
	if _, e = tx.Exec(ctx, `INSERT INTO privacy_decision_events(request_id,actor_id,status,reason,details) VALUES($1,$2,$3,$4,$5)`, id, a.ID, in.Status, in.Outcome, history); e != nil {
		return p, e
	}
	if e = auditCompliance(ctx, tx, a, "privacy_request", id, in.Status); e != nil {
		return p, e
	}
	return p, tx.Commit(ctx)
}
func registerPrivacy(r chi.Router, s *Store) {
	for _, admin := range []bool{false, true} {
		prefix := "/me/privacy-requests"
		if admin {
			prefix = "/admin/privacy-requests"
		}
		r.Get(prefix, wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
			page, e := filters(r)
			if e != nil {
				return nil, e
			}
			return s.PrivacyList(r.Context(), actor(r), admin, (page.Page-1)*50)
		}))
	}
	r.Post("/me/privacy-requests", wrap(201, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in PrivacyInput
		if e := decode(w, r, &in); e != nil {
			return nil, e
		}
		return s.RequestPrivacy(r.Context(), actor(r), in, r.Header.Get("Idempotency-Key"))
	}))
	r.Patch("/admin/privacy-requests/{requestID}", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in PrivacyReview
		if e := decode(w, r, &in); e != nil {
			return nil, e
		}
		return s.ReviewPrivacy(r.Context(), actor(r), chi.URLParam(r, "requestID"), in)
	}))
	for _, preview := range []bool{false, true} {
		path := "/me/privacy-requests/{requestID}/export"
		if preview {
			path = "/admin/privacy-requests/{requestID}/export-preview"
		}
		r.Get(path, wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
			return s.PrivacyExport(r.Context(), actor(r), chi.URLParam(r, "requestID"), preview)
		}))
	}
}
