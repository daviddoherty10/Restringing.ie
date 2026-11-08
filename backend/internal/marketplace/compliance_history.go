package marketplace

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
	"time"
)

type ComplianceHistoryEvent struct {
	ID        int64           `json:"id"`
	ActorID   string          `json:"actor_id"`
	Action    string          `json:"action"`
	Reason    string          `json:"reason"`
	Details   json.RawMessage `json:"details"`
	CreatedAt time.Time       `json:"created_at"`
}

func (s *Store) ComplianceHistory(ctx context.Context, a Actor, kind, id string, page int) ([]ComplianceHistoryEvent, error) {
	if e := legalAdmin(a); e != nil {
		return nil, e
	}
	var query string
	switch kind {
	case "privacy_request":
		query = `SELECT id,actor_id::text,status,reason,details,created_at FROM privacy_decision_events WHERE request_id=$1 ORDER BY id DESC LIMIT 50 OFFSET $2`
	case "trader":
		query = `SELECT id,actor_id::text,status,reason,document,created_at FROM trader_verification_events WHERE stringer_id=$1 ORDER BY id DESC LIMIT 50 OFFSET $2`
	case "case":
		query = `SELECT id,COALESCE(actor_id::text,''),action,reason,'{}'::jsonb,created_at FROM compliance_case_events WHERE case_id=$1 ORDER BY id DESC LIMIT 50 OFFSET $2`
	default:
		return nil, invalid(map[string]string{"kind": "Use privacy_request, trader or case."})
	}
	rows, e := s.Pool.Query(ctx, query, id, (page-1)*50)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []ComplianceHistoryEvent{}
	for rows.Next() {
		var event ComplianceHistoryEvent
		if e = rows.Scan(&event.ID, &event.ActorID, &event.Action, &event.Reason, &event.Details, &event.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, event)
	}
	return out, rows.Err()
}
func registerComplianceHistory(r chi.Router, s *Store) {
	r.Get("/admin/compliance-history/{resourceID}", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		f, e := filters(r)
		if e != nil {
			return nil, e
		}
		return s.ComplianceHistory(r.Context(), actor(r), r.URL.Query().Get("kind"), chi.URLParam(r, "resourceID"), f.Page)
	}))
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if item == value {
			return true
		}
	}
	return false
}
