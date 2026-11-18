package marketplace

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

const publicReviewMinimum = 5

type ReviewInput struct {
	Rating  int    `json:"rating"`
	Comment string `json:"comment"`
}

type BookingReview struct {
	ID string `json:"id"`
	ReviewInput
}

type OwnReview struct {
	Review *BookingReview `json:"review"`
	Hidden bool           `json:"hidden"`
}

type PublicReviews struct {
	Count   int             `json:"count"`
	Average float64         `json:"average"`
	Reviews []BookingReview `json:"reviews"`
}

type ReviewReportInput struct {
	Reason string `json:"reason"`
}

type ReportedReview struct {
	BookingReview
	Business string `json:"business"`
	Reason   string `json:"reason"`
	Version  int    `json:"version"`
}

type ReviewDecision struct {
	Version int  `json:"version"`
	Hide    bool `json:"hide"`
}

func reviewableOrder(ctx context.Context, db DB, a Actor, order string) error {
	var status string
	var member bool
	err := db.QueryRow(ctx, `SELECT o.status, EXISTS(
		SELECT 1 FROM stringer_members m WHERE m.stringer_id=o.stringer_id AND m.user_id=o.customer_id
	) FROM orders o JOIN users u ON u.id=o.customer_id
		WHERE o.id=$1 AND o.customer_id=$2 AND u.status='active' FOR SHARE OF u`, order, a.ID).Scan(&status, &member)
	if err != nil {
		return missing(err)
	}
	if status != "completed" || member {
		return problem(409, "review_unavailable", "Customers can review a stringer after collecting their racket.")
	}
	return nil
}

func (s *Store) ownReview(ctx context.Context, a Actor, order string) (OwnReview, error) {
	if err := reviewableOrder(ctx, s.Pool, a, order); err != nil {
		return OwnReview{}, err
	}
	var out OwnReview
	var review BookingReview
	err := s.Pool.QueryRow(ctx, `SELECT id::text,rating,comment,hidden FROM booking_reviews WHERE order_id=$1 AND NOT withdrawn`, order).
		Scan(&review.ID, &review.Rating, &review.Comment, &out.Hidden)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return out, nil
		}
		return out, err
	}
	out.Review = &review
	return out, nil
}

func (s *Store) saveReview(ctx context.Context, a Actor, order string, in ReviewInput) (OwnReview, error) {
	in.Comment = strings.TrimSpace(in.Comment)
	if in.Rating < 1 || in.Rating > 5 || utf8.RuneCountInString(in.Comment) > 500 || strings.ContainsRune(in.Comment, 0) {
		return OwnReview{}, invalid(map[string]string{"review": "Choose 1 to 5 stars and use up to 500 characters."})
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return OwnReview{}, err
	}
	defer tx.Rollback(ctx)
	if err := reviewableOrder(ctx, tx, a, order); err != nil {
		return OwnReview{}, err
	}
	review := BookingReview{ReviewInput: in}
	out := OwnReview{Review: &review}
	err = tx.QueryRow(ctx, `INSERT INTO booking_reviews(order_id,rating,comment) VALUES($1,$2,$3)
		ON CONFLICT(order_id) DO UPDATE SET rating=EXCLUDED.rating, comment=EXCLUDED.comment,
		withdrawn=false, version=booking_reviews.version+1
		RETURNING id::text,hidden`, order, in.Rating, in.Comment).Scan(&review.ID, &out.Hidden)
	if err != nil {
		return OwnReview{}, err
	}
	return out, tx.Commit(ctx)
}

func (s *Store) withdrawReview(ctx context.Context, a Actor, order string) (ReadResult, error) {
	if err := reviewableOrder(ctx, s.Pool, a, order); err != nil {
		return ReadResult{}, err
	}
	// Retain moderation status so re-submission cannot republish a hidden review.
	_, err := s.Pool.Exec(ctx, `UPDATE booking_reviews SET withdrawn=true,comment='',version=version+1
		WHERE order_id=$1 AND NOT withdrawn`, order)
	return ReadResult{Read: err == nil}, err
}

func (s *Store) publicReviews(ctx context.Context, business string) (PublicReviews, error) {
	out := PublicReviews{Reviews: []BookingReview{}}
	if _, err := s.GetStringer(ctx, business); err != nil {
		return out, err
	}
	// Threshold and visible rows must use the same snapshot, including concurrent withdrawals.
	err := s.Pool.QueryRow(ctx, `WITH visible AS (
		SELECT r.id,r.rating,r.comment,r.created_at,o.customer_id
		FROM booking_reviews r JOIN orders o ON o.id=r.order_id JOIN users u ON u.id=o.customer_id
		WHERE o.stringer_id=$1 AND o.status='completed' AND u.status='active' AND NOT r.hidden AND NOT r.withdrawn
		  AND NOT EXISTS (SELECT 1 FROM stringer_members m WHERE m.stringer_id=o.stringer_id AND m.user_id=o.customer_id)
	), totals AS (
		SELECT count(*) AS count,avg(rating) AS average,count(DISTINCT customer_id) AS customers FROM visible
	)
	SELECT CASE WHEN customers >= $2 THEN count ELSE 0 END,
	       CASE WHEN customers >= $2 THEN average::float8 ELSE 0 END,
	       CASE WHEN customers >= $2 THEN COALESCE((
	          SELECT jsonb_agg(jsonb_build_object('id',id,'rating',rating,'comment',comment) ORDER BY created_at DESC,id)
	          FROM (SELECT * FROM visible ORDER BY created_at DESC,id LIMIT 10) recent
	       ),'[]'::jsonb) ELSE '[]'::jsonb END
	FROM totals`, business, publicReviewMinimum).Scan(&out.Count, &out.Average, &out.Reviews)
	return out, err
}

func (s *Store) reportReview(ctx context.Context, a Actor, business, review string, in ReviewReportInput) (ReadResult, error) {
	reason := strings.TrimSpace(in.Reason)
	if reason == "" || utf8.RuneCountInString(reason) > 500 || strings.ContainsRune(reason, 0) {
		return ReadResult{}, invalid(map[string]string{"reason": "Enter a reason up to 500 characters."})
	}
	if err := authorize(ctx, s.Pool, a, business, false); err != nil {
		return ReadResult{}, err
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE booking_reviews r SET report_reason=COALESCE(r.report_reason,$3),
		version=r.version+CASE WHEN r.report_reason IS NULL THEN 1 ELSE 0 END
		FROM orders o WHERE r.order_id=o.id AND r.id=$1 AND o.stringer_id=$2 AND NOT r.hidden AND NOT r.withdrawn`, review, business, reason)
	if err == nil && tag.RowsAffected() == 0 {
		err = problem(404, "not_found", "Review is unavailable.")
	}
	return ReadResult{Read: err == nil}, err
}

func (s *Store) reportedReviews(ctx context.Context, a Actor, page int) (Page[ReportedReview], error) {
	const size = 20
	out := Page[ReportedReview]{Data: []ReportedReview{}, Page: page, PageSize: size}
	if !a.Admin {
		return out, problem(403, "forbidden", "Administrator access required.")
	}
	const from = ` FROM booking_reviews r JOIN orders o ON o.id=r.order_id JOIN stringers s ON s.id=o.stringer_id
		WHERE report_reason IS NOT NULL AND NOT withdrawn`
	if err := s.Pool.QueryRow(ctx, `SELECT count(*)`+from).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT r.id::text,r.rating,r.comment,s.name,r.report_reason,r.version`+from+
		` ORDER BY r.created_at,r.id LIMIT $1 OFFSET $2`, size, (page-1)*size)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var v ReportedReview
		if err := rows.Scan(&v.ID, &v.Rating, &v.Comment, &v.Business, &v.Reason, &v.Version); err != nil {
			return out, err
		}
		out.Data = append(out.Data, v)
	}
	return out, rows.Err()
}

func (s *Store) moderateReview(ctx context.Context, a Actor, review string, in ReviewDecision) (ReadResult, error) {
	if !a.Admin {
		return ReadResult{}, problem(403, "forbidden", "Administrator access required.")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return ReadResult{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE booking_reviews SET hidden=$3,report_reason=NULL,version=version+1
		WHERE id=$1 AND version=$2 AND report_reason IS NOT NULL AND NOT withdrawn`, review, in.Version, in.Hide)
	if err != nil {
		return ReadResult{}, err
	}
	if tag.RowsAffected() == 0 {
		return ReadResult{}, problem(409, "review_changed", "The review changed. Refresh the queue before deciding.")
	}
	action := "keep"
	if in.Hide {
		action = "hide"
	}
	if err = auditCompliance(ctx, tx, a, "booking_review", review, action); err != nil {
		return ReadResult{}, err
	}
	return ReadResult{Read: true}, tx.Commit(ctx)
}

func registerReviewsPrivate(r chi.Router, s *Store) {
	r.Get("/orders/{orderID}/review", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.ownReview(r.Context(), actor(r), chi.URLParam(r, "orderID"))
	}))
	r.Put("/orders/{orderID}/review", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in ReviewInput
		if err := decode(w, r, &in); err != nil {
			return nil, err
		}
		return s.saveReview(r.Context(), actor(r), chi.URLParam(r, "orderID"), in)
	}))
	r.Put("/orders/{orderID}/review/withdrawal", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.withdrawReview(r.Context(), actor(r), chi.URLParam(r, "orderID"))
	}))
	r.Put("/stringers/{stringerID}/reviews/{reviewID}/report", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in ReviewReportInput
		if err := decode(w, r, &in); err != nil {
			return nil, err
		}
		return s.reportReview(r.Context(), actor(r), chi.URLParam(r, "stringerID"), chi.URLParam(r, "reviewID"), in)
	}))
	r.Get("/admin/reviews", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		f, err := filters(r)
		if err != nil {
			return nil, err
		}
		return s.reportedReviews(r.Context(), actor(r), f.Page)
	}))
	r.Patch("/admin/reviews/{reviewID}", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in ReviewDecision
		if err := decode(w, r, &in); err != nil {
			return nil, err
		}
		return s.moderateReview(r.Context(), actor(r), chi.URLParam(r, "reviewID"), in)
	}))
}
