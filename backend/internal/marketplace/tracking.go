package marketplace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

type TrackingState struct {
	RequestedCollectionDate *string    `json:"requested_collection_date"`
	ConfirmedReadyAt        *time.Time `json:"confirmed_ready_at"`
	CollectionInstructions  string     `json:"collection_instructions"`
	IssueOpen               bool       `json:"issue_open"`
	Version                 int        `json:"version"`
}
type TrackingUpdate struct {
	ID        string        `json:"id"`
	Kind      string        `json:"kind"`
	Note      string        `json:"note"`
	State     TrackingState `json:"state"`
	CreatedAt time.Time     `json:"created_at"`
}
type OrderTracking struct {
	TrackingState
	StringerName string          `json:"stringer_name"`
	Area         string          `json:"area"`
	County       string          `json:"county"`
	CanManage    bool            `json:"can_manage"`
	IsCustomer   bool            `json:"is_customer"`
	Timezone     string          `json:"timezone"`
	LatestUpdate *TrackingUpdate `json:"latest_update,omitempty"`
}
type CollectionRequest struct {
	Date            string `json:"date"`
	ExpectedVersion int    `json:"expected_version"`
}
type TrackingInput struct {
	ConfirmedReadyAt       *time.Time `json:"confirmed_ready_at"`
	CollectionInstructions string     `json:"collection_instructions"`
	IssueOpen              bool       `json:"issue_open"`
	Note                   string     `json:"note"`
	ExpectedVersion        int        `json:"expected_version"`
}

func validText(value string, limit int) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > limit {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' && r != '\t' && r != '\r' {
			return false
		}
	}
	return true
}
func trackingState(ctx context.Context, db DB, id string) (TrackingState, error) {
	var state TrackingState
	var raw []byte
	err := db.QueryRow(ctx, `SELECT state FROM order_tracking_updates WHERE order_id=$1 ORDER BY version DESC LIMIT 1`, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return state, nil
	}
	if err == nil {
		err = json.Unmarshal(raw, &state)
	}
	return state, err
}
func (s *Store) Tracking(ctx context.Context, a Actor, id string) (OrderTracking, error) {
	var out OrderTracking
	o, err := jsonOrder(s.Pool.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id=$1`, id))
	if err != nil {
		return out, err
	}
	if err = canReadOrder(ctx, s.Pool, a, o); err != nil {
		return out, err
	}
	var latest TrackingUpdate
	var raw []byte
	err = s.Pool.QueryRow(ctx, `SELECT id::text,kind,note,state,created_at FROM order_tracking_updates WHERE order_id=$1 ORDER BY version DESC LIMIT 1`, id).Scan(&latest.ID, &latest.Kind, &latest.Note, &raw, &latest.CreatedAt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if err == nil {
		if err = json.Unmarshal(raw, &out.TrackingState); err != nil {
			return out, err
		}
		latest.State = out.TrackingState
		out.LatestUpdate = &latest
	}
	out.IsCustomer = a.ID == o.CustomerID
	out.Timezone = "Europe/Dublin"
	err = authorize(ctx, s.Pool, a, o.StringerID, false)
	if err == nil {
		out.CanManage = true
	} else {
		var p *Problem
		if !errors.As(err, &p) || p.Status != 403 {
			return out, err
		}
	}
	err = s.Pool.QueryRow(ctx, `SELECT name,area,county FROM stringers WHERE id=$1`, o.StringerID).Scan(&out.StringerName, &out.Area, &out.County)
	return out, err
}

// Lock ordering matches fulfilment and stock writes. Authorization is evaluated
// again inside the transaction, including for idempotent replays.
func lockAccessibleOrder(ctx context.Context, tx pgx.Tx, a Actor, id string) (Order, error) {
	var business string
	if err := tx.QueryRow(ctx, `SELECT stringer_id::text FROM orders WHERE id=$1`, id).Scan(&business); err != nil {
		return Order{}, missing(err)
	}
	if _, err := lockStringer(ctx, tx, business); err != nil {
		return Order{}, err
	}
	o, err := jsonOrder(tx.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id=$1 FOR UPDATE`, id))
	if err == nil {
		err = canReadOrder(ctx, tx, a, o)
	}
	return o, err
}

func (s *Store) ChangeTracking(ctx context.Context, a Actor, id, key string, request *CollectionRequest, input *TrackingInput) (TrackingUpdate, error) {
	var out TrackingUpdate
	if !keyPattern.MatchString(key) {
		return out, problem(400, "idempotency_key_required", "Supply a valid Idempotency-Key.")
	}
	kind := "collection_request"
	var payload any = request
	if input != nil {
		kind = "stringer_update"
		payload = input
	}
	if (request == nil) == (input == nil) {
		return out, problem(400, "invalid_request", "Choose one tracking update.")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return out, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	o, err := lockAccessibleOrder(ctx, tx, a, id)
	if err != nil {
		return out, err
	}
	if request != nil && a.ID != o.CustomerID {
		return out, problem(403, "forbidden", "Only this order's customer can request a collection date.")
	}
	if input != nil {
		if err = authorize(ctx, tx, a, o.StringerID, false); err != nil {
			return out, err
		}
	}
	var replay bool
	err = tx.QueryRow(ctx, `SELECT id::text,kind,note,state,created_at,(request=$4::jsonb AND kind=$5) FROM order_tracking_updates WHERE order_id=$1 AND actor_id=$2 AND idempotency_key=$3`, id, a.ID, key, raw, kind).Scan(&out.ID, &out.Kind, &out.Note, &outStateBytes{target: &out.State}, &out.CreatedAt, &replay)
	if err == nil {
		if !replay {
			return out, problem(409, "idempotency_conflict", "This key was already used for a different update.")
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if o.Status == "completed" || o.Status == "cancelled" {
		return out, problem(409, "order_closed", "Collection details cannot be changed on a closed order.")
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM order_tracking_updates WHERE order_id=$1 AND actor_id=$2 AND created_at>clock_timestamp()-interval '1 minute'`, id, a.ID).Scan(&count); err != nil {
		return out, err
	}
	if count >= 20 {
		return out, problem(429, "tracking_rate_limit", "Please wait before updating collection details again.")
	}
	state, err := trackingState(ctx, tx, id)
	if err != nil {
		return out, err
	}
	expected := 0
	now, err := databaseTime(ctx, tx)
	if err != nil {
		return out, err
	}
	if request != nil {
		expected = request.ExpectedVersion
		if err = validateCollectionDate(request.Date, now); err != nil {
			return out, err
		}
		state.RequestedCollectionDate = &request.Date
		out.Note = "Customer requested collection on " + request.Date + ". Awaiting the stringer's agreement."
	} else {
		expected = input.ExpectedVersion
		input.Note = strings.TrimSpace(input.Note)
		if input.Note == "" || !validText(input.Note, 1000) || !validText(input.CollectionInstructions, 1000) {
			return out, invalid(map[string]string{"note": "Add a clear update of up to 1000 characters; collection instructions also allow up to 1000."})
		}
		if o.Status == "placed" {
			return out, problem(409, "order_not_accepted", "Accept the order before confirming collection details.")
		}
		if input.ConfirmedReadyAt != nil && ((!input.ConfirmedReadyAt.After(now) && (state.ConfirmedReadyAt == nil || !input.ConfirmedReadyAt.Equal(*state.ConfirmedReadyAt))) || input.ConfirmedReadyAt.After(now.AddDate(1, 0, 0))) {
			return out, invalid(map[string]string{"confirmed_ready_at": "Choose a future estimate within the next year, including its timezone."})
		}
		state.ConfirmedReadyAt = input.ConfirmedReadyAt
		state.CollectionInstructions = strings.TrimSpace(input.CollectionInstructions)
		state.IssueOpen = input.IssueOpen
		out.Note = input.Note
	}
	if expected < 0 || state.Version != expected {
		return out, problem(409, "tracking_changed", "Collection details changed. Refresh them before saving again.")
	}
	state.Version++
	stateRaw, _ := json.Marshal(state)
	err = tx.QueryRow(ctx, `INSERT INTO order_tracking_updates(order_id,actor_id,idempotency_key,kind,request,state,version,note) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id::text,created_at`, id, a.ID, key, kind, raw, stateRaw, state.Version, out.Note).Scan(&out.ID, &out.CreatedAt)
	if err != nil {
		return out, err
	}
	out.Kind = kind
	out.State = state
	title := "Collection request updated"
	if input != nil {
		title = "Collection details updated"
		if input.IssueOpen {
			title = "Your order needs attention"
		}
	}
	if err = notifyOrder(ctx, tx, o, a.ID, "tracking:"+out.ID, "tracking", title); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

// pgx scans JSON bytes through this helper while preserving the common scanner.
type outStateBytes struct{ target *TrackingState }

func (v *outStateBytes) Scan(src any) error {
	switch raw := src.(type) {
	case []byte:
		return json.Unmarshal(raw, v.target)
	case string:
		return json.Unmarshal([]byte(raw), v.target)
	default:
		return fmt.Errorf("invalid tracking state")
	}
}

func (s *Store) TrackingUpdates(ctx context.Context, a Actor, id string, f Filters) (Page[TrackingUpdate], error) {
	out := Page[TrackingUpdate]{Data: []TrackingUpdate{}, Page: f.Page, PageSize: f.PageSize}
	o, err := jsonOrder(s.Pool.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id=$1`, id))
	if err != nil {
		return out, err
	}
	if err = canReadOrder(ctx, s.Pool, a, o); err != nil {
		return out, err
	}
	if err = s.Pool.QueryRow(ctx, `SELECT count(*) FROM order_tracking_updates WHERE order_id=$1`, id).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT id::text,kind,note,state,created_at FROM order_tracking_updates WHERE order_id=$1 ORDER BY version DESC LIMIT $2 OFFSET $3`, id, f.PageSize, (f.Page-1)*f.PageSize)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var u TrackingUpdate
		var raw []byte
		if err = rows.Scan(&u.ID, &u.Kind, &u.Note, &raw, &u.CreatedAt); err != nil {
			return out, err
		}
		if err = json.Unmarshal(raw, &u.State); err != nil {
			return out, err
		}
		out.Data = append(out.Data, u)
	}
	return out, rows.Err()
}

func validateCollectionDate(value string, now time.Time) error {
	loc, err := time.LoadLocation("Europe/Dublin")
	if err != nil {
		return err
	}
	date, err := time.Parse("2006-01-02", value)
	local := now.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	if err != nil || date.Before(today) || date.After(today.AddDate(1, 0, 0)) {
		return invalid(map[string]string{"date": "Choose today or a date within the next year (Irish time)."})
	}
	return nil
}
