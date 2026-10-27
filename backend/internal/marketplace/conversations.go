package marketplace

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type OrderMessage struct {
	ID         string    `json:"id"`
	Position   string    `json:"position"`
	SenderRole string    `json:"sender_role"`
	Own        bool      `json:"own"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}
type MessageInput struct {
	Body string `json:"body"`
}
type MessageReadInput struct {
	LastMessageID string `json:"last_message_id"`
}
type MessagePage struct {
	Page[OrderMessage]
	UnreadCount int `json:"unread_count"`
}
type ReadResult struct {
	Read bool `json:"read"`
}
type Notification struct {
	ID          string     `json:"id"`
	OrderID     string     `json:"order_id"`
	StringerID  string     `json:"stringer_id"`
	ForStringer bool       `json:"for_stringer"`
	Kind        string     `json:"kind"`
	Title       string     `json:"title"`
	CreatedAt   time.Time  `json:"created_at"`
	ReadAt      *time.Time `json:"read_at"`
}
type NotificationPage struct {
	Page[Notification]
	UnreadCount int `json:"unread_count"`
}

// Both sides are notified except the actor. Recipients are a snapshot of current
// active, verified members/customers; notification reads recheck order access.
func notifyOrder(ctx context.Context, db DB, o Order, actor, key, kind, title string) error {
	_, err := db.Exec(ctx, `INSERT INTO notifications(user_id,order_id,event_key,kind,title)
        SELECT u.id,$1,$2,$3,$4 FROM users u WHERE u.id<>$5 AND u.status='active' AND u.phone_verified_at IS NOT NULL
        AND (u.id=$6 OR EXISTS(SELECT 1 FROM stringer_members m WHERE m.stringer_id=$7 AND m.user_id=u.id))
        ON CONFLICT(user_id,event_key) DO NOTHING`, o.ID, key, kind, title, actor, o.CustomerID, o.StringerID)
	return err
}

func (s *Store) Messages(ctx context.Context, a Actor, id string, f Filters) (MessagePage, error) {
	out := MessagePage{Page: Page[OrderMessage]{Data: []OrderMessage{}, Page: f.Page, PageSize: f.PageSize}}
	o, err := jsonOrder(s.Pool.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id=$1`, id))
	if err != nil {
		return out, err
	}
	if err = canReadOrder(ctx, s.Pool, a, o); err != nil {
		return out, err
	}
	err = s.Pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE sender_id<>$2 AND position>COALESCE((SELECT position FROM order_message_reads WHERE order_id=$1 AND user_id=$2),0)) FROM order_messages WHERE order_id=$1`, id, a.ID).Scan(&out.Total, &out.UnreadCount)
	if err != nil {
		return out, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT id::text,position::text,sender_role,(sender_id=$2),body,created_at FROM order_messages WHERE order_id=$1 ORDER BY position DESC LIMIT $3 OFFSET $4`, id, a.ID, f.PageSize, (f.Page-1)*f.PageSize)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var m OrderMessage
		if err = rows.Scan(&m.ID, &m.Position, &m.SenderRole, &m.Own, &m.Body, &m.CreatedAt); err != nil {
			return out, err
		}
		out.Data = append(out.Data, m)
	}
	return out, rows.Err()
}
func (s *Store) SendMessage(ctx context.Context, a Actor, id, key string, input MessageInput) (OrderMessage, error) {
	var out OrderMessage
	if !keyPattern.MatchString(key) {
		return out, problem(400, "idempotency_key_required", "Supply a valid Idempotency-Key.")
	}
	body := strings.TrimSpace(input.Body)
	if body == "" || !validText(body, 2000) {
		return out, invalid(map[string]string{"body": "Write a message of 1–2000 characters."})
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	// A per-sender lock makes the global message limit hold across businesses.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,7))`, a.ID); err != nil {
		return out, err
	}
	o, err := lockAccessibleOrder(ctx, tx, a, id)
	if err != nil {
		return out, err
	}
	err = tx.QueryRow(ctx, `SELECT id::text,position::text,sender_role,body,created_at FROM order_messages WHERE order_id=$1 AND sender_id=$2 AND idempotency_key=$3`, id, a.ID, key).Scan(&out.ID, &out.Position, &out.SenderRole, &out.Body, &out.CreatedAt)
	if err == nil {
		if out.Body != body {
			return out, problem(409, "idempotency_conflict", "This key was already used for a different message.")
		}
		out.Own = true
		return out, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM order_messages WHERE sender_id=$1 AND created_at>clock_timestamp()-interval '1 minute'`, a.ID).Scan(&count); err != nil {
		return out, err
	}
	if count >= 20 {
		return out, problem(429, "message_rate_limit", "Please wait before sending more messages.")
	}
	role := "stringer"
	if a.ID == o.CustomerID {
		role = "customer"
	} else if a.Admin {
		role = "support"
	}
	err = tx.QueryRow(ctx, `INSERT INTO order_messages(order_id,sender_id,sender_role,body,idempotency_key) VALUES($1,$2,$3,$4,$5) RETURNING id::text,position::text,created_at`, id, a.ID, role, body, key).Scan(&out.ID, &out.Position, &out.CreatedAt)
	if err != nil {
		return out, err
	}
	out.SenderRole = role
	out.Own = true
	out.Body = body
	if err = notifyOrder(ctx, tx, o, a.ID, "message:"+out.ID, "message", "New message about your order"); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
func (s *Store) ReadMessages(ctx context.Context, a Actor, id, messageID string) (ReadResult, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return ReadResult{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = lockAccessibleOrder(ctx, tx, a, id); err != nil {
		return ReadResult{}, err
	}
	var position int64
	if err = tx.QueryRow(ctx, `SELECT position FROM order_messages WHERE order_id=$1 AND id=$2`, id, messageID).Scan(&position); err != nil {
		return ReadResult{}, missing(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO order_message_reads(order_id,user_id,position) VALUES($1,$2,$3) ON CONFLICT(order_id,user_id) DO UPDATE SET position=GREATEST(order_message_reads.position,EXCLUDED.position)`, id, a.ID, position)
	if err != nil {
		return ReadResult{}, err
	}
	return ReadResult{true}, tx.Commit(ctx)
}

const notificationScope = ` FROM notifications n JOIN orders o ON o.id=n.order_id WHERE n.user_id=$1 AND (o.customer_id=$1 OR $2 OR EXISTS(SELECT 1 FROM stringer_members m WHERE m.stringer_id=o.stringer_id AND m.user_id=$1))`

func (s *Store) Notifications(ctx context.Context, a Actor, f Filters) (NotificationPage, error) {
	out := NotificationPage{Page: Page[Notification]{Data: []Notification{}, Page: f.Page, PageSize: f.PageSize}}
	err := s.Pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE n.read_at IS NULL)`+notificationScope, a.ID, a.Admin).Scan(&out.Total, &out.UnreadCount)
	if err != nil {
		return out, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT n.id::text,n.order_id::text,o.stringer_id::text,(o.customer_id<>$1),n.kind,n.title,n.created_at,n.read_at`+notificationScope+` ORDER BY n.created_at DESC,n.id DESC LIMIT $3 OFFSET $4`, a.ID, a.Admin, f.PageSize, (f.Page-1)*f.PageSize)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var n Notification
		if err = rows.Scan(&n.ID, &n.OrderID, &n.StringerID, &n.ForStringer, &n.Kind, &n.Title, &n.CreatedAt, &n.ReadAt); err != nil {
			return out, err
		}
		out.Data = append(out.Data, n)
	}
	return out, rows.Err()
}
func (s *Store) ReadNotification(ctx context.Context, a Actor, id string) (ReadResult, error) {
	var found string
	err := s.Pool.QueryRow(ctx, `UPDATE notifications SET read_at=COALESCE(read_at,clock_timestamp()) WHERE id IN (SELECT n.id`+notificationScope+` AND n.id=$3) RETURNING id::text`, a.ID, a.Admin, id).Scan(&found)
	return ReadResult{err == nil}, missing(err)
}
