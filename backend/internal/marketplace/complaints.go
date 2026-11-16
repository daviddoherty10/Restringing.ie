package marketplace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

type CaseInput struct {
	Kind       string `json:"kind"`
	StringerID string `json:"stringer_id"`
	OrderID    string `json:"order_id"`
	Location   string `json:"location"`
	Details    string `json:"details"`
	Contact    string `json:"contact"`
	GoodFaith  bool   `json:"good_faith"`
}
type ComplianceCase struct {
	ID         string    `json:"id"`
	ReporterID string    `json:"reporter_id"`
	StringerID string    `json:"stringer_id"`
	OrderID    string    `json:"order_id"`
	Kind       string    `json:"kind"`
	Location   string    `json:"location"`
	Details    string    `json:"details"`
	Contact    string    `json:"contact"`
	Status     string    `json:"status"`
	Outcome    string    `json:"outcome"`
	Appeal     string    `json:"appeal"`
	Version    int       `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
}
type CaseAction struct {
	Version int    `json:"version"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
}
type CaseDecision struct {
	ID         string `json:"id"`
	StringerID string `json:"stringer_id"`
	Status     string `json:"status"`
	Outcome    string `json:"outcome"`
	Version    int    `json:"version"`
}
type CaseReceipt struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}
type ComplianceNotification struct {
	ID        string     `json:"id"`
	CaseID    string     `json:"case_id"`
	Title     string     `json:"title"`
	CreatedAt time.Time  `json:"created_at"`
	ReadAt    *time.Time `json:"read_at"`
}

const caseColumns = `id::text,COALESCE(reporter_id::text,''),COALESCE(stringer_id::text,''),COALESCE(order_id::text,''),kind,location,details,contact,status,outcome,appeal,version,created_at`

func scanCase(r pgx.Row) (out ComplianceCase, e error) {
	e = r.Scan(&out.ID, &out.ReporterID, &out.StringerID, &out.OrderID, &out.Kind, &out.Location, &out.Details, &out.Contact, &out.Status, &out.Outcome, &out.Appeal, &out.Version, &out.CreatedAt)
	return out, missing(e)
}
func (s *Store) SubmitCase(ctx context.Context, a Actor, in CaseInput, key string, public bool) (CaseReceipt, error) {
	if !keyPattern.MatchString(key) || !oneOf(in.Kind, "listing", "order", "unlawful_content") || public && in.Kind != "unlawful_content" || strings.TrimSpace(in.Details) == "" || len(in.Details) > 4000 || strings.TrimSpace(in.Location) == "" || len(in.Location) > 1000 || !in.GoodFaith {
		return CaseReceipt{}, invalid(map[string]string{"report": "Provide the exact content location, reasons and good-faith accuracy declaration."})
	}
	if public {
		if _, e := mail.ParseAddress(in.Contact); e != nil {
			return CaseReceipt{}, invalid(map[string]string{"contact": "Provide a contact email for human follow-up."})
		}
	}
	if len(in.Contact) > 254 {
		return CaseReceipt{}, invalid(map[string]string{"contact": "Contact is too long."})
	}
	if in.StringerID != "" {
		if e := identifier(in.StringerID); e != nil {
			return CaseReceipt{}, e
		}
	}
	if in.OrderID != "" {
		if e := identifier(in.OrderID); e != nil {
			return CaseReceipt{}, e
		}
	}
	if in.Kind != "order" {
		in.OrderID = ""
	}
	if in.Kind == "order" {
		o, e := s.GetOrder(ctx, a, in.OrderID)
		if e != nil {
			return CaseReceipt{}, e
		}
		if o.CustomerID != a.ID && !a.Admin {
			return CaseReceipt{}, problem(403, "forbidden", "Use an order you placed.")
		}
		in.StringerID = o.StringerID
	}
	if in.Kind == "listing" && in.StringerID == "" {
		return CaseReceipt{}, invalid(map[string]string{"stringer_id": "Choose the listing concerned."})
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return CaseReceipt{}, e
	}
	defer tx.Rollback(ctx)
	// One small public intake budget; production must additionally enforce edge abuse controls.
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('compliance-case-intake',0))`); e != nil {
		return CaseReceipt{}, e
	}
	raw, _ := json.Marshal(in)
	digest := sha256.Sum256(raw)
	hash := hex.EncodeToString(digest[:])
	user := a.ID
	if public {
		user = ""
	}
	var out CaseReceipt
	var oldHash string
	e = tx.QueryRow(ctx, `SELECT id::text,status,request_hash FROM compliance_cases WHERE COALESCE(reporter_id::text,'')=$1 AND request_key=$2`, user, key).Scan(&out.ID, &out.Status, &oldHash)
	if e == nil {
		if oldHash != hash {
			return out, problem(409, "retry_changed", "Retry the original notice.")
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return out, e
	}
	var n int
	e = tx.QueryRow(ctx, `SELECT count(*) FROM compliance_cases WHERE created_at>now()-interval '1 hour' AND (CASE WHEN $1='' THEN reporter_id IS NULL ELSE reporter_id::text=$1 END)`, user).Scan(&n)
	if e != nil {
		return out, e
	}
	limit := 10
	if public {
		limit = 100
	}
	if n >= limit {
		return out, problem(429, "notice_rate_limit", "Too many notices. Please retry later or use the contact route for urgent help.")
	}
	e = tx.QueryRow(ctx, `INSERT INTO compliance_cases(reporter_id,stringer_id,order_id,kind,location,details,contact,request_key,request_hash) VALUES(nullif($1,'')::uuid,nullif($2,'')::uuid,nullif($3,'')::uuid,$4,$5,$6,$7,$8,$9) RETURNING id::text,status`, user, in.StringerID, in.OrderID, in.Kind, in.Location, in.Details, in.Contact, key, hash).Scan(&out.ID, &out.Status)
	if e != nil {
		return out, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO compliance_case_events(case_id,actor_id,action,reason) VALUES($1,nullif($2,'')::uuid,'submitted','Notice received for human review')`, out.ID, user)
	if e != nil {
		return out, e
	}
	return out, tx.Commit(ctx)
}
func (s *Store) CaseList(ctx context.Context, a Actor, admin bool, page int) ([]ComplianceCase, error) {
	if admin {
		if e := legalAdmin(a); e != nil {
			return nil, e
		}
	}
	rows, e := s.Pool.Query(ctx, `SELECT `+caseColumns+` FROM compliance_cases WHERE $1 OR reporter_id=$2 ORDER BY created_at DESC,id LIMIT 50 OFFSET $3`, admin, a.ID, (page-1)*50)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []ComplianceCase{}
	for rows.Next() {
		r, e := scanCase(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Store) CaseDecisions(ctx context.Context, a Actor, business string) ([]CaseDecision, error) {
	if e := authorize(ctx, s.Pool, a, business, true); e != nil {
		return nil, e
	}
	rows, e := s.Pool.Query(ctx, `SELECT id::text,stringer_id::text,status,outcome,version FROM compliance_cases WHERE stringer_id=$1 AND outcome<>'' ORDER BY updated_at DESC LIMIT 50`, business)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []CaseDecision{}
	for rows.Next() {
		var d CaseDecision
		if e = rows.Scan(&d.ID, &d.StringerID, &d.Status, &d.Outcome, &d.Version); e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) ActCase(ctx context.Context, a Actor, id string, in CaseAction, appeal bool) (CaseDecision, error) {
	if !appeal {
		if e := legalAdmin(a); e != nil {
			return CaseDecision{}, e
		}
	}
	if strings.TrimSpace(in.Reason) == "" || len(in.Reason) > 2000 {
		return CaseDecision{}, invalid(map[string]string{"reason": "Provide a reason and next steps (up to 2000 bytes)."})
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return CaseDecision{}, e
	}
	defer tx.Rollback(ctx)
	c, e := scanCase(tx.QueryRow(ctx, `SELECT `+caseColumns+` FROM compliance_cases WHERE id=$1 FOR UPDATE`, id))
	if e != nil {
		return CaseDecision{}, e
	}
	if c.Version != in.Version {
		return CaseDecision{}, problem(409, "case_changed", "Refresh the case before acting.")
	}
	if appeal {
		if c.ReporterID != a.ID {
			if c.StringerID == "" {
				return CaseDecision{}, problem(404, "not_found", "Case not found.")
			}
			if e = authorize(ctx, tx, a, c.StringerID, true); e != nil {
				return CaseDecision{}, e
			}
		}
		var used bool
		if e = tx.QueryRow(ctx, `SELECT appealed FROM compliance_cases WHERE id=$1`, id).Scan(&used); e != nil {
			return CaseDecision{}, e
		}
		if used || (c.Status != "resolved" && c.Status != "dismissed") {
			return CaseDecision{}, problem(409, "review_unavailable", "Review is available once after a decision. Further escalation uses the contact route.")
		}
		_, e = tx.Exec(ctx, `UPDATE compliance_cases SET status='appealed',appeal=$2,appealed=true,version=version+1,updated_at=now() WHERE id=$1`, id, in.Reason)
		in.Status = "appealed"
	} else {
		if !oneOf(in.Status, "in_review", "resolved", "dismissed") {
			return CaseDecision{}, invalid(map[string]string{"status": "Choose in_review, resolved or dismissed."})
		}
		var same bool
		if e = tx.QueryRow(ctx, `SELECT appealed AND COALESCE(initial_reviewer_id=$2::uuid,false) FROM compliance_cases WHERE id=$1`, id, a.ID).Scan(&same); e != nil {
			return CaseDecision{}, e
		}
		if same {
			return CaseDecision{}, problem(409, "independent_review_required", "Assign the appeal to a different administrator.")
		}
		_, e = tx.Exec(ctx, `UPDATE compliance_cases SET status=$2,outcome=$3,reviewer_id=$4,initial_reviewer_id=CASE WHEN $2 IN ('resolved','dismissed') THEN COALESCE(initial_reviewer_id,$4) ELSE initial_reviewer_id END,version=version+1,updated_at=now() WHERE id=$1`, id, in.Status, in.Reason, a.ID)
	}
	if e != nil {
		return CaseDecision{}, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO compliance_case_events(case_id,actor_id,action,reason) VALUES($1,$2,$3,$4)`, id, a.ID, in.Status, in.Reason); e != nil {
		return CaseDecision{}, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO compliance_notifications(user_id,case_id,title) SELECT id,$1,'A case has a new decision or review update' FROM users WHERE id=$2::uuid OR id IN(SELECT user_id FROM stringer_members WHERE stringer_id=nullif($3,'')::uuid AND role='owner')`, id, nullUUID(c.ReporterID), c.StringerID); e != nil {
		return CaseDecision{}, e
	}
	c, e = scanCase(tx.QueryRow(ctx, `SELECT `+caseColumns+` FROM compliance_cases WHERE id=$1`, id))
	if e != nil {
		return CaseDecision{}, e
	}
	return CaseDecision{c.ID, c.StringerID, c.Status, c.Outcome, c.Version}, tx.Commit(ctx)
}
func nullUUID(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func (s *Store) CaseInbox(ctx context.Context, a Actor) ([]ComplianceNotification, error) {
	rows, e := s.Pool.Query(ctx, `SELECT id::text,case_id::text,title,created_at,read_at FROM compliance_notifications WHERE user_id=$1 ORDER BY created_at DESC LIMIT 50`, a.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []ComplianceNotification{}
	for rows.Next() {
		var v ComplianceNotification
		if e = rows.Scan(&v.ID, &v.CaseID, &v.Title, &v.CreatedAt, &v.ReadAt); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func registerCasesPublic(r chi.Router, s *Store) {
	r.Post("/content-notices", wrap(201, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in CaseInput
		if e := decode(w, r, &in); e != nil {
			return nil, e
		}
		return s.SubmitCase(r.Context(), Actor{}, in, r.Header.Get("Idempotency-Key"), true)
	}))
}
func registerCasesPrivate(r chi.Router, s *Store) {
	for _, admin := range []bool{false, true} {
		path := "/me/cases"
		if admin {
			path = "/admin/cases"
		}
		r.Get(path, wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
			f, e := filters(r)
			if e != nil {
				return nil, e
			}
			return s.CaseList(r.Context(), actor(r), admin, f.Page)
		}))
	}
	r.Post("/me/cases", wrap(201, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in CaseInput
		if e := decode(w, r, &in); e != nil {
			return nil, e
		}
		return s.SubmitCase(r.Context(), actor(r), in, r.Header.Get("Idempotency-Key"), false)
	}))
	for _, appeal := range []bool{false, true} {
		path := "/admin/cases/{caseID}"
		method := "PATCH"
		if appeal {
			path = "/cases/{caseID}/review"
			method = "POST"
		}
		r.MethodFunc(method, path, wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
			var in CaseAction
			if e := decode(w, r, &in); e != nil {
				return nil, e
			}
			return s.ActCase(r.Context(), actor(r), chi.URLParam(r, "caseID"), in, appeal)
		}))
	}
	r.Get("/stringers/{stringerID}/decisions", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.CaseDecisions(r.Context(), actor(r), chi.URLParam(r, "stringerID"))
	}))
	r.Get("/me/case-notifications", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) { return s.CaseInbox(r.Context(), actor(r)) }))
}
