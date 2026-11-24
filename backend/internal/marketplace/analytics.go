package marketplace

import "context"

type StringUsage struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Restrings int    `json:"restrings"`
}

type ActivityMonth struct {
	BookingBusinesses int           `json:"booking_businesses"`
	Month             string        `json:"month"`
	Bookings          int           `json:"bookings"`
	Restrings         int           `json:"restrings"`
	Strings           []StringUsage `json:"strings"`
}

type ActivityReport struct {
	Months []ActivityMonth `json:"months"`
}

func (s *Store) Activity(ctx context.Context, actor Actor, business string) (ActivityReport, error) {
	var report ActivityReport
	if business == "" {
		if err := legalAdmin(actor); err != nil {
			return report, err
		}
	} else if err := authorize(ctx, s.Pool, actor, business, false); err != nil {
		return report, err
	}

	rows, err := s.Pool.Query(ctx, `
	WITH months AS (
		SELECT generate_series(current_month - interval '11 months', current_month, interval '1 month') AS month
		FROM (SELECT date_trunc('month', now() AT TIME ZONE 'Europe/Dublin') AS current_month) clock
	), bookings AS (
		SELECT date_trunc('month',created_at AT TIME ZONE 'Europe/Dublin') AS month, count(*) AS total,count(DISTINCT stringer_id) AS businesses
		FROM orders WHERE ($1='' OR stringer_id=NULLIF($1,'')::uuid)
		  AND created_at >= (SELECT min(month) FROM months) AT TIME ZONE 'Europe/Dublin'
		GROUP BY 1
	), finished AS (
		SELECT o.id,o.configuration,date_trunc('month',min(e.created_at) AT TIME ZONE 'Europe/Dublin') AS month
		FROM order_status_events e JOIN orders o ON o.id=e.order_id
		WHERE e.to_status='completed' AND ($1='' OR o.stringer_id=NULLIF($1,'')::uuid)
		  AND e.created_at >= (SELECT min(month) FROM months) AT TIME ZONE 'Europe/Dublin'
		GROUP BY o.id
	), completions AS (
		SELECT month,count(*) AS total FROM finished GROUP BY month
	), products AS (
		SELECT f.month,p.id,p.brand || ' ' || p.model AS name,count(DISTINCT f.id) AS uses
		FROM finished f
		JOIN string_variants v ON v.id IN (NULLIF(f.configuration->>'variant_id','')::uuid,
		                                    NULLIF(f.configuration->>'crosses_variant_id','')::uuid)
		JOIN string_products p ON p.id=v.product_id
		GROUP BY f.month,p.id
	)
	SELECT to_char(m.month,'YYYY-MM'),coalesce(b.total,0),coalesce(b.businesses,0),coalesce(c.total,0),
	       coalesce(p.id::text,''),coalesce(p.name,''),coalesce(p.uses,0)
	FROM months m LEFT JOIN bookings b USING(month) LEFT JOIN completions c USING(month)
	LEFT JOIN products p USING(month) ORDER BY m.month,p.name,p.id`, business)
	if err != nil {
		return report, err
	}
	defer rows.Close()
	for rows.Next() {
		var month ActivityMonth
		var product StringUsage
		if err := rows.Scan(&month.Month, &month.Bookings, &month.BookingBusinesses, &month.Restrings, &product.ProductID, &product.Name, &product.Restrings); err != nil {
			return report, err
		}
		if len(report.Months) == 0 || report.Months[len(report.Months)-1].Month != month.Month {
			month.Strings = []StringUsage{}
			report.Months = append(report.Months, month)
		}
		if product.ProductID != "" {
			current := &report.Months[len(report.Months)-1]
			current.Strings = append(current.Strings, product)
		}
	}
	return report, rows.Err()
}
