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
	"strconv"
	"strings"
	"time"
)

type FinancialInput struct {
	OrderID        string    `json:"order_id"`
	Kind           string    `json:"kind"`
	AmountCents    int64     `json:"amount_cents"`
	OccurredAt     time.Time `json:"occurred_at"`
	Reference      string    `json:"reference"`
	Reason         string    `json:"reason"`
	RelatedEventID string    `json:"related_event_id"`
	Attested       bool      `json:"attested"`
}
type FinancialEvent struct {
	ID             string    `json:"id"`
	StringerID     string    `json:"stringer_id"`
	OrderID        string    `json:"order_id"`
	ActorID        string    `json:"actor_id"`
	Kind           string    `json:"kind"`
	AmountCents    int64     `json:"amount_cents"`
	Currency       string    `json:"currency"`
	OccurredAt     time.Time `json:"occurred_at"`
	RecordedAt     time.Time `json:"recorded_at"`
	Reference      string    `json:"reference"`
	Reason         string    `json:"reason"`
	RelatedEventID string    `json:"related_event_id"`
}
type FinancialReport struct {
	StringerID               string    `json:"stringer_id"`
	Year                     int       `json:"year"`
	Quarter                  int       `json:"quarter"`
	Currency                 string    `json:"currency"`
	BookedOrders             int       `json:"booked_orders"`
	CompletedOrders          int       `json:"completed_orders"`
	BookedValueCents         int64     `json:"booked_value_cents"`
	LegacyPaidWithoutReceipt int       `json:"legacy_paid_without_receipt"`
	CashReceivedCents        int64     `json:"cash_received_cents"`
	CashRefundCents          int64     `json:"cash_refund_cents"`
	AdjustmentsNetCents      int64     `json:"adjustments_net_cents"`
	PlatformFeesNetCents     int64     `json:"platform_fees_net_cents"`
	ReportedReceiptOrders    int       `json:"reported_receipt_orders"`
	FactCount                int       `json:"fact_count"`
	GeneratedAt              time.Time `json:"generated_at"`
	ReportingStatus          string    `json:"reporting_status"`
}

const financialColumns = `id::text,stringer_id::text,COALESCE(order_id::text,''),actor_id::text,kind,amount_cents,currency,occurred_at,recorded_at,reference,reason,COALESCE(related_event_id::text,'')`

func scanFinancial(row pgx.Row) (out FinancialEvent, e error) {
	e = row.Scan(&out.ID, &out.StringerID, &out.OrderID, &out.ActorID, &out.Kind, &out.AmountCents, &out.Currency, &out.OccurredAt, &out.RecordedAt, &out.Reference, &out.Reason, &out.RelatedEventID)
	return out, missing(e)
}
func (s *Store) RecordFinancial(ctx context.Context, a Actor, business string, in FinancialInput, key string) (FinancialEvent, error) {
	if !keyPattern.MatchString(key) || !in.Attested || !strings.Contains("|cash_received|cash_refund|adjustment_increase|adjustment_decrease|platform_fee|platform_fee_refund|", "|"+in.Kind+"|") || in.AmountCents < 1 || in.AmountCents > 1000000 || strings.TrimSpace(in.Reference) == "" || len(in.Reference) > 200 || strings.TrimSpace(in.Reason) == "" || len(in.Reason) > 1000 || in.OccurredAt.Before(time.Now().AddDate(-10, 0, 0)) || in.OccurredAt.After(time.Now().Add(5*time.Minute)) {
		return FinancialEvent{}, invalid(map[string]string{"fact": "Provide an attested amount, actual past date, source reference, reason and retry key. No tax identifiers."})
	}
	fee := strings.HasPrefix(in.Kind, "platform_fee")
	if fee {
		if e := legalAdmin(a); e != nil {
			return FinancialEvent{}, e
		}
		if in.OrderID != "" {
			return FinancialEvent{}, invalid(map[string]string{"order_id": "Platform subscription fees belong to the business, not a customer order."})
		}
	} else if e := identifier(in.OrderID); e != nil {
		return FinancialEvent{}, e
	}
	if in.RelatedEventID != "" {
		if e := identifier(in.RelatedEventID); e != nil {
			return FinancialEvent{}, e
		}
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return FinancialEvent{}, e
	}
	defer tx.Rollback(ctx)
	if _, e = lockStringer(ctx, tx, business); e != nil {
		return FinancialEvent{}, e
	}
	if e = authorize(ctx, tx, a, business, true); e != nil {
		return FinancialEvent{}, e
	}
	raw, _ := json.Marshal(in)
	digest := sha256.Sum256(raw)
	hash := hex.EncodeToString(digest[:])
	var oldHash string
	e = tx.QueryRow(ctx, `SELECT request_hash FROM seller_financial_events WHERE stringer_id=$1 AND request_key=$2`, business, key).Scan(&oldHash)
	if e == nil {
		if oldHash != hash {
			return FinancialEvent{}, problem(409, "retry_changed", "Retry the original financial fact.")
		}
		out, e := scanFinancial(tx.QueryRow(ctx, `SELECT `+financialColumns+` FROM seller_financial_events WHERE stringer_id=$1 AND request_key=$2`, business, key))
		if e != nil {
			return out, e
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return FinancialEvent{}, e
	}
	if !fee {
		var cash bool
		e = tx.QueryRow(ctx, `SELECT payment_method='cash' FROM orders WHERE id=$1 AND stringer_id=$2 FOR UPDATE`, in.OrderID, business).Scan(&cash)
		if e != nil {
			return FinancialEvent{}, missing(e)
		}
		if !cash {
			return FinancialEvent{}, invalid(map[string]string{"order_id": "Record actual cash facts only for a cash order at this business."})
		}
	}
	needsLink := in.Kind != "cash_received" && in.Kind != "platform_fee"
	if needsLink {
		if in.RelatedEventID == "" {
			return FinancialEvent{}, invalid(map[string]string{"related_event_id": "Refunds and corrections must link to the original receipt/fee."})
		}
		var ok bool
		e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM seller_financial_events WHERE id=$1 AND stringer_id=$2 AND COALESCE(order_id::text,'')=$3 AND kind=$4)`, in.RelatedEventID, business, in.OrderID, map[bool]string{true: "platform_fee", false: "cash_received"}[fee]).Scan(&ok)
		if e != nil {
			return FinancialEvent{}, e
		}
		if !ok {
			return FinancialEvent{}, invalid(map[string]string{"related_event_id": "Choose an original receipt/fee for this business and order."})
		}
	}
	if in.Kind == "cash_refund" || in.Kind == "adjustment_decrease" || in.Kind == "platform_fee_refund" {
		var net int64
		e = tx.QueryRow(ctx, `SELECT COALESCE(sum(CASE WHEN kind IN ('cash_received','adjustment_increase','platform_fee') THEN amount_cents ELSE -amount_cents END),0) FROM seller_financial_events WHERE stringer_id=$1 AND COALESCE(order_id::text,'')=$2`, business, in.OrderID).Scan(&net)
		if e != nil {
			return FinancialEvent{}, e
		}
		if in.AmountCents > net {
			return FinancialEvent{}, invalid(map[string]string{"amount_cents": "Refund/correction exceeds the recorded remaining amount."})
		}
	}
	out, e := scanFinancial(tx.QueryRow(ctx, `INSERT INTO seller_financial_events(stringer_id,order_id,actor_id,kind,amount_cents,occurred_at,reference,reason,related_event_id,request_key,request_hash) VALUES($1,nullif($2,'')::uuid,$3,$4,$5,$6,$7,$8,nullif($9,'')::uuid,$10,$11) RETURNING `+financialColumns, business, in.OrderID, a.ID, in.Kind, in.AmountCents, in.OccurredAt, in.Reference, in.Reason, in.RelatedEventID, key, hash))
	if e != nil {
		return out, e
	}
	if e = auditCompliance(ctx, tx, a, "financial_event", out.ID, in.Kind); e != nil {
		return out, e
	}
	return out, tx.Commit(ctx)
}
func (s *Store) FinancialLedger(ctx context.Context, a Actor, business string, page int) ([]FinancialEvent, error) {
	if e := authorize(ctx, s.Pool, a, business, true); e != nil {
		return nil, e
	}
	rows, e := s.Pool.Query(ctx, `SELECT `+financialColumns+` FROM seller_financial_events WHERE stringer_id=$1 ORDER BY recorded_at DESC,id LIMIT 50 OFFSET $2`, business, (page-1)*50)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []FinancialEvent{}
	for rows.Next() {
		v, e := scanFinancial(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) FinancialSummary(ctx context.Context, a Actor, business string, year, quarter int) (FinancialReport, error) {
	out := FinancialReport{StringerID: business, Year: year, Quarter: quarter, Currency: "EUR", ReportingStatus: "Internal fact summary; accountant applicability, reconciliation and seller-tax-data review required; not a DAC7 filing."}
	if e := legalAdmin(a); e != nil {
		return out, e
	}
	if year < 2000 || year > 2100 || quarter < 1 || quarter > 4 {
		return out, invalid(map[string]string{"period": "Choose year 2000–2100 and quarter 1–4."})
	}
	tx, e := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	var start, end time.Time
	e = tx.QueryRow(ctx, `SELECT make_date($1,($2-1)*3+1,1)::timestamp AT TIME ZONE 'Europe/Dublin',(make_date($1,($2-1)*3+1,1)+interval '3 months') AT TIME ZONE 'Europe/Dublin'`, year, quarter).Scan(&start, &end)
	if e != nil {
		return out, e
	}
	e = tx.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE status='completed'),COALESCE(sum(total_cents),0),count(*) FILTER(WHERE payment_status='paid' AND NOT EXISTS(SELECT 1 FROM seller_financial_events f WHERE f.order_id=orders.id AND f.kind='cash_received')) FROM orders WHERE stringer_id=$1 AND created_at>=$2 AND created_at<$3`, business, start, end).Scan(&out.BookedOrders, &out.CompletedOrders, &out.BookedValueCents, &out.LegacyPaidWithoutReceipt)
	if e != nil {
		return out, e
	}
	e = tx.QueryRow(ctx, `SELECT COALESCE(sum(amount_cents) FILTER(WHERE kind='cash_received'),0),COALESCE(sum(amount_cents) FILTER(WHERE kind='cash_refund'),0),COALESCE(sum(CASE WHEN kind='adjustment_increase' THEN amount_cents WHEN kind='adjustment_decrease' THEN -amount_cents ELSE 0 END),0),COALESCE(sum(CASE WHEN kind='platform_fee' THEN amount_cents WHEN kind='platform_fee_refund' THEN -amount_cents ELSE 0 END),0),count(DISTINCT order_id) FILTER(WHERE kind='cash_received'),count(*),now() FROM seller_financial_events WHERE stringer_id=$1 AND occurred_at>=$2 AND occurred_at<$3`, business, start, end).Scan(&out.CashReceivedCents, &out.CashRefundCents, &out.AdjustmentsNetCents, &out.PlatformFeesNetCents, &out.ReportedReceiptOrders, &out.FactCount, &out.GeneratedAt)
	if e != nil {
		return out, e
	}
	if e = auditCompliance(ctx, tx, a, "financial_report", business, "quarterly_summary_viewed"); e != nil {
		return out, e
	}
	return out, tx.Commit(ctx)
}
func registerFinancial(r chi.Router, s *Store) {
	r.Post("/stringers/{stringerID}/financial-events", wrap(201, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in FinancialInput
		if e := decode(w, r, &in); e != nil {
			return nil, e
		}
		return s.RecordFinancial(r.Context(), actor(r), chi.URLParam(r, "stringerID"), in, r.Header.Get("Idempotency-Key"))
	}))
	r.Get("/stringers/{stringerID}/financial-events", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		f, e := filters(r)
		if e != nil {
			return nil, e
		}
		return s.FinancialLedger(r.Context(), actor(r), chi.URLParam(r, "stringerID"), f.Page)
	}))
	r.Get("/admin/stringers/{stringerID}/financial-report", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		year, _ := strconv.Atoi(r.URL.Query().Get("year"))
		quarter, _ := strconv.Atoi(r.URL.Query().Get("quarter"))
		return s.FinancialSummary(r.Context(), actor(r), chi.URLParam(r, "stringerID"), year, quarter)
	}))
}
