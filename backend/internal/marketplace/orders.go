package marketplace

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateOrder(ctx context.Context, a Actor, quoteID, key string) (Order, bool, error) {
	quoteID = strings.ToLower(quoteID)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Order{}, false, err
	}
	defer tx.Rollback(ctx)
	// Serialize retries even before an order row exists. Unique constraints are
	// the final defence; the payload must also match the original request.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.ID+":"+key); err != nil {
		return Order{}, false, err
	}
	existing, err := jsonOrder(tx.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE customer_id=$1 AND idempotency_key=$2`, a.ID, key))
	if err == nil {
		if existing.QuoteID != quoteID {
			return Order{}, false, problem(409, "idempotency_conflict", "This key was already used with a different quote.")
		}
		return existing, true, tx.Commit(ctx)
	}
	var p *Problem
	if !errors.As(err, &p) || p.Status != 404 {
		return Order{}, false, err
	}
	var q Quote
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT id::text,stringer_id::text,configuration,fingerprint,expires_at FROM quotes WHERE id=$1 AND customer_id=$2 FOR UPDATE`, quoteID, a.ID).Scan(&q.ID, &q.StringerID, &raw, &q.Fingerprint, &q.ExpiresAt)
	if err != nil {
		return Order{}, false, missing(err)
	}
	if expired(q.ExpiresAt) {
		return Order{}, false, problem(409, "quote_expired", "Request a fresh quote before ordering.")
	}
	if err = json.Unmarshal(raw, &q.Configuration); err != nil {
		return Order{}, false, err
	}
	shop, err := lockStringer(ctx, tx, q.StringerID)
	if err != nil {
		return Order{}, false, err
	}
	if err = requireOpen(shop); err != nil {
		return Order{}, false, err
	}
	if err = requireBookingCapacity(ctx, tx, q.StringerID); err != nil {
		return Order{}, false, err
	}
	fresh, r, v, err := calculate(ctx, tx, q.StringerID, q.Configuration.Booking)
	if err != nil {
		return Order{}, false, err
	}
	if fresh.Fingerprint != q.Fingerprint {
		return Order{}, false, problem(409, "quote_changed", "Prices, catalogue details or service rules changed. Request a new quote.")
	}
	cfg, _ := json.Marshal(fresh.Configuration)
	lines, _ := json.Marshal(fresh.Breakdown)
	o, err := jsonOrder(tx.QueryRow(ctx, `INSERT INTO orders(customer_id,stringer_id,quote_id,idempotency_key,configuration,breakdown,total_cents,currency) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING `+orderCols, a.ID, q.StringerID, q.ID, key, cfg, lines, fresh.TotalCents, fresh.Currency))
	if err != nil {
		return Order{}, false, err
	}
	if q.Configuration.StringSource == "stock" {
		lot, n, e := chooseLot(ctx, tx, v.InventoryID, r.StringLengthMM, true)
		if e != nil {
			return Order{}, false, e
		}
		if _, err = tx.Exec(ctx, `UPDATE inventory_lots SET reserved=reserved+$2 WHERE id=$1`, lot, n); err != nil {
			return Order{}, false, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO inventory_reservations(order_id,lot_id,quantity) VALUES($1,$2,$3)`, o.ID, lot, n); err != nil {
			return Order{}, false, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO inventory_movements(lot_id,actor_id,quantity_delta,reserved_delta,reason,order_id) VALUES($1,$2,0,$3,'order reservation',$4)`, lot, a.ID, n, o.ID); err != nil {
			return Order{}, false, err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO order_status_events(order_id,actor_id,to_status) VALUES($1,$2,'placed')`, o.ID, a.ID); err != nil {
		return Order{}, false, err
	}
	return o, false, tx.Commit(ctx)
}

func canReadOrder(ctx context.Context, db DB, a Actor, o Order) error {
	if a.ID == o.CustomerID || a.Admin {
		return nil
	}
	if err := authorize(ctx, db, a, o.StringerID, false); err != nil {
		var p *Problem
		if errors.As(err, &p) && p.Status == 403 {
			return problem(404, "not_found", "The requested order was not found.")
		}
		return err
	}
	return nil
}
func (s *Store) GetOrder(ctx context.Context, a Actor, id string) (Order, error) {
	o, err := jsonOrder(s.Pool.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id=$1`, id))
	if err != nil {
		return o, err
	}
	if err = canReadOrder(ctx, s.Pool, a, o); err != nil {
		return Order{}, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT from_status,to_status,created_at FROM order_status_events WHERE order_id=$1 ORDER BY created_at,id`, id)
	if err != nil {
		return o, err
	}
	defer rows.Close()
	o.Events = []OrderEvent{}
	for rows.Next() {
		var e OrderEvent
		if err = rows.Scan(&e.From, &e.To, &e.CreatedAt); err != nil {
			return o, err
		}
		o.Events = append(o.Events, e)
	}
	if err = rows.Err(); err != nil {
		return o, err
	}
	rows.Close()
	payments, err := s.Pool.Query(ctx, `SELECT from_status,to_status,reason,created_at FROM order_payment_events WHERE order_id=$1 ORDER BY created_at,id`, id)
	if err != nil {
		return o, err
	}
	defer payments.Close()
	o.PaymentEvents = []PaymentEvent{}
	for payments.Next() {
		var e PaymentEvent
		if err = payments.Scan(&e.From, &e.To, &e.Reason, &e.CreatedAt); err != nil {
			return o, err
		}
		o.PaymentEvents = append(o.PaymentEvents, e)
	}
	return o, payments.Err()
}
func (s *Store) ListOrders(ctx context.Context, a Actor, stringerID, status string, f Filters) (Page[Order], error) {
	out := Page[Order]{Data: []Order{}, Page: f.Page, PageSize: f.PageSize}
	if stringerID != "" {
		if err := authorize(ctx, s.Pool, a, stringerID, false); err != nil {
			return out, err
		}
	}
	base := ` FROM orders WHERE (($1='' AND customer_id=$2) OR stringer_id=NULLIF($1,'')::uuid) AND ($3='' OR status=$3)`
	if err := s.Pool.QueryRow(ctx, `SELECT count(*)`+base, stringerID, a.ID, status).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+orderCols+base+` ORDER BY created_at DESC,id LIMIT $4 OFFSET $5`, stringerID, a.ID, status, f.PageSize, (f.Page-1)*f.PageSize)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		o, e := jsonOrder(rows)
		if e != nil {
			return out, e
		}
		out.Data = append(out.Data, o)
	}
	return out, rows.Err()
}

func (s *Store) UpdateStatus(ctx context.Context, a Actor, id, status string) (Order, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback(ctx)
	var stringerID string
	if err = tx.QueryRow(ctx, `SELECT stringer_id::text FROM orders WHERE id=$1`, id).Scan(&stringerID); err != nil {
		return Order{}, missing(err)
	}
	// Every business mutation takes this lock first, including stock adjustments.
	if _, err = lockStringer(ctx, tx, stringerID); err != nil {
		return Order{}, err
	}
	o, err := jsonOrder(tx.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return o, err
	}
	manager := a.Admin
	if !manager {
		err = authorize(ctx, tx, a, stringerID, false)
		if err == nil {
			manager = true
		} else {
			var p *Problem
			if !errors.As(err, &p) || p.Status != 403 {
				return Order{}, err
			}
		}
	}
	if !manager && a.ID != o.CustomerID {
		return Order{}, problem(404, "not_found", "The requested order was not found.")
	}
	if !CanTransition(o.Status, status, manager) {
		return Order{}, problem(409, "invalid_transition", "This status transition is not permitted.")
	}
	if status == "accepted" {
		now, err := databaseTime(ctx, tx)
		if err != nil {
			return Order{}, err
		}
		usage, err := businessUsage(ctx, tx, stringerID, now)
		if err != nil {
			return Order{}, err
		}
		if !usage.CanAccept {
			return Order{}, problem(409, "booking_limit_reached", "The free plan includes five accepted bookings per Irish calendar month. Upgrade or wait until the next month to accept another booking.")
		}
		if _, err = tx.Exec(ctx, `INSERT INTO booking_usage(order_id,stringer_id,accepted_at) VALUES($1,$2,$3)`, id, stringerID, now); err != nil {
			return Order{}, err
		}
	}
	if status == "in_progress" || status == "cancelled" {
		var lot, state string
		var n int
		err = tx.QueryRow(ctx, `SELECT lot_id::text,quantity,state FROM inventory_reservations WHERE order_id=$1 FOR UPDATE`, id).Scan(&lot, &n, &state)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Order{}, err
		}
		if err == nil && state == "reserved" {
			quantityDelta := 0
			newState := "released"
			reason := "order cancellation"
			if status == "in_progress" {
				quantityDelta = -n
				newState = "consumed"
				reason = "string used for order"
			}
			if _, err = tx.Exec(ctx, `UPDATE inventory_lots SET quantity=quantity+$2,reserved=reserved-$3 WHERE id=$1`, lot, quantityDelta, n); err != nil {
				return Order{}, err
			}
			if _, err = tx.Exec(ctx, `UPDATE inventory_reservations SET state=$2 WHERE order_id=$1`, id, newState); err != nil {
				return Order{}, err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO inventory_movements(lot_id,actor_id,quantity_delta,reserved_delta,reason,order_id) VALUES($1,$2,$3,$4,$5,$6)`, lot, a.ID, quantityDelta, -n, reason, id); err != nil {
				return Order{}, err
			}
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO order_status_events(order_id,actor_id,from_status,to_status) VALUES($1,$2,$3,$4)`, id, a.ID, o.Status, status); err != nil {
		return Order{}, err
	}
	o, err = jsonOrder(tx.QueryRow(ctx, `UPDATE orders SET status=$2,updated_at=now() WHERE id=$1 RETURNING `+orderCols, id, status))
	if err != nil {
		return o, err
	}
	return o, tx.Commit(ctx)
}

func (s *Store) IncomingOrder(ctx context.Context, a Actor, stringerID, orderID string) (IncomingOrder, error) {
	var out IncomingOrder
	if err := authorize(ctx, s.Pool, a, stringerID, false); err != nil {
		return out, err
	}
	order, err := s.GetOrder(ctx, a, orderID)
	if err != nil {
		return out, err
	}
	if order.StringerID != stringerID {
		return out, problem(404, "not_found", "The requested order was not found.")
	}
	out.Order = order
	err = s.Pool.QueryRow(ctx, `SELECT phone_number,COALESCE(first_name,''),COALESCE(last_name,'') FROM users WHERE id=$1`, order.CustomerID).Scan(&out.Customer.Phone, &out.Customer.FirstName, &out.Customer.LastName)
	return out, missing(err)
}
