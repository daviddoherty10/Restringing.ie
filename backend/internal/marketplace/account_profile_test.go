package marketplace

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestIntegrationAccountProfileAndBooking(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.store.Pool.Exec(ctx, `UPDATE users SET first_name=NULL WHERE id=$1`, f.customer.ID); err != nil {
		t.Fatal(err)
	}
	quote, err := f.store.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if err != nil {
		t.Fatal(err)
	}
	const key = "profile-booking-test-key"
	_, _, err = f.store.CreateOrder(ctx, f.customer, quote.ID, key)
	wantProblem(t, err, "profile_name_required")

	const path = "/me/profile"
	wantStatus(t, f.request(t, http.MethodGet, path, nil, Actor{}, nil), http.StatusUnauthorized)
	invalidEmail := AccountProfile{FirstName: "Aoife", Email: "Aoife <aoife@example.invalid>"}
	wantStatus(t, f.request(t, http.MethodPut, path, invalidEmail, f.customer, nil), http.StatusUnprocessableEntity)
	input := AccountProfile{FirstName: "  Aoife ", Email: "aoife@example.invalid"}
	wantStatus(t, f.request(t, http.MethodPut, path, input, f.customer, nil), http.StatusOK)
	read := func(user Actor) AccountProfile {
		t.Helper()
		w := f.request(t, http.MethodGet, path, nil, user, nil)
		wantStatus(t, w, http.StatusOK)
		var profile AccountProfile
		if e := json.Unmarshal(w.Body.Bytes(), &profile); e != nil {
			t.Fatal(e)
		}
		return profile
	}
	if value := read(f.customer); value.FirstName != "Aoife" || value.Email != input.Email {
		t.Fatalf("unexpected saved profile: %+v", value)
	}
	if value := read(f.other); value.Email != "" {
		t.Fatal("account email leaked to another customer")
	}
	created, replay, err := f.store.CreateOrder(ctx, f.customer, quote.ID, key)
	if err != nil || replay {
		t.Fatalf("expected a new booking after profile setup: %v", err)
	}
	wantStatus(t, f.request(t, http.MethodPut, path, AccountProfile{FirstName: "Aoife"}, f.customer, nil), http.StatusOK)
	if got := read(f.customer); got.Email != "" {
		t.Fatal("optional email was not cleared")
	}
	if _, err := f.store.Pool.Exec(ctx, `UPDATE users SET first_name=NULL WHERE id=$1`, f.customer.ID); err != nil {
		t.Fatal(err)
	}
	existing, replay, err := f.store.CreateOrder(ctx, f.customer, quote.ID, key)
	if err != nil || !replay || existing.ID != created.ID {
		t.Fatalf("existing bookings must remain retryable: %v", err)
	}
}
