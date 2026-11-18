package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestIntegrationBookingReviewPublication(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	orderFor := func(customer Actor, status string) string {
		t.Helper()
		var id string
		err := f.store.Pool.QueryRow(ctx, `WITH q AS (
			INSERT INTO quotes(customer_id,stringer_id,configuration,breakdown,fingerprint,total_cents,expires_at)
			VALUES($1,$2,'{}','[]',repeat('a',64),1500,now()+interval '1 hour') RETURNING id
		)
		INSERT INTO orders(customer_id,stringer_id,quote_id,idempotency_key,configuration,breakdown,total_cents,currency,status)
		SELECT $1,$2,id,id::text,'{}','[]',1500,'EUR',$3 FROM q RETURNING id::text`, customer.ID, f.shop.ID, status).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	path := "/stringers/" + f.shop.ID + "/reviews"
	public := func(want int) PublicReviews {
		t.Helper()
		r := f.request(t, http.MethodGet, path, nil, Actor{}, nil)
		wantStatus(t, r, http.StatusOK)
		var data PublicReviews
		if err := json.Unmarshal(r.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		if data.Count != want || (want == 0 && (data.Average != 0 || len(data.Reviews) != 0)) {
			t.Fatalf("unexpected public review data: %+v, want count %d", data, want)
		}
		return data
	}

	input := ReviewInput{Rating: 5, Comment: "Quick turnaround"}
	reviewPath := "/orders/" + orderFor(f.customer, "placed") + "/review"
	wantStatus(t, f.request(t, http.MethodPut, reviewPath, input, f.customer, nil), http.StatusConflict)
	first := orderFor(f.customer, "completed")
	reviewPath = "/orders/" + first + "/review"
	wantStatus(t, f.request(t, http.MethodPut, reviewPath, input, f.other, nil), http.StatusNotFound)
	wantStatus(t, f.request(t, http.MethodPut, reviewPath, ReviewInput{Rating: 6}, f.customer, nil), http.StatusUnprocessableEntity)
	wantStatus(t, f.request(t, http.MethodPut, reviewPath, input, f.customer, nil), http.StatusOK)
	wantStatus(t, f.request(t, http.MethodPut, reviewPath, input, f.customer, nil), http.StatusOK)
	if _, err := f.store.saveReview(ctx, f.owner, orderFor(f.owner, "completed"), input); err == nil {
		t.Fatal("business member reviewed own business")
	}

	customers := []Actor{f.customer, f.other}
	for i := 0; i < 3; i++ {
		var customer Actor
		if err := f.store.Pool.QueryRow(ctx, `INSERT INTO users(phone_number,phone_verified_at)
			VALUES($1,now()) RETURNING id::text`, fmt.Sprintf("+35380001200%d", i)).Scan(&customer.ID); err != nil {
			t.Fatal(err)
		}
		customers = append(customers, customer)
	}
	var fifth BookingReview
	var finalOrder string
	for i, customer := range customers {
		finalOrder = orderFor(customer, "completed")
		saved, err := f.store.saveReview(ctx, customer, finalOrder, ReviewInput{Rating: 4})
		if err != nil {
			t.Fatal(err)
		}
		if i == 4 {
			fifth = *saved.Review
		} else {
			public(0)
		}
	}
	if result := public(6); len(result.Reviews) != 6 || result.Average < 4.16 || result.Average > 4.17 {
		t.Fatalf("unexpected published reviews: %+v", result)
	}

	reportPath := path + "/" + fifth.ID + "/report"
	report := ReviewReportInput{Reason: "Contains material unrelated to the booking"}
	wantStatus(t, f.request(t, http.MethodPut, reportPath, report, f.other, nil), http.StatusForbidden)
	wantStatus(t, f.request(t, http.MethodPut, reportPath, report, f.owner, nil), http.StatusOK)
	public(6)
	queue, err := f.store.reportedReviews(ctx, f.admin, 1)
	if err != nil || queue.Total != 1 || len(queue.Data) != 1 {
		t.Fatalf("unexpected moderation queue: %+v, %v", queue, err)
	}
	moderationPath := "/admin/reviews/" + fifth.ID
	decision := ReviewDecision{Version: queue.Data[0].Version, Hide: true}
	wantStatus(t, f.request(t, http.MethodPatch, moderationPath, decision, f.owner, nil), http.StatusForbidden)
	wantStatus(t, f.request(t, http.MethodPatch, moderationPath, decision, f.admin, nil), http.StatusOK)
	public(0)
	if _, err = f.store.withdrawReview(ctx, customers[4], finalOrder); err != nil {
		t.Fatal(err)
	}
	resaved, err := f.store.saveReview(ctx, customers[4], finalOrder, input)
	if err != nil || !resaved.Hidden {
		t.Fatalf("resubmission bypassed moderation: %+v, %v", resaved, err)
	}
	public(0)

	replacement := orderFor(customers[4], "completed")
	if _, err = f.store.saveReview(ctx, customers[4], replacement, input); err != nil {
		t.Fatal(err)
	}
	public(6)
	if _, err = f.store.withdrawReview(ctx, customers[4], replacement); err != nil {
		t.Fatal(err)
	}
	public(0)
}
