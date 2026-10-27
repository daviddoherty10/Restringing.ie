package marketplace

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func trackingOrder(t *testing.T, f *fixture) Order {
	t.Helper()
	ctx := context.Background()
	q, e := f.store.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if e != nil {
		t.Fatal(e)
	}
	o, _, e := f.store.CreateOrder(ctx, f.customer, q.ID, "tracking-order-key-0001")
	if e != nil {
		t.Fatal(e)
	}
	return o
}
func TestIntegrationTrackingAgreementAndVersions(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	o := trackingOrder(t, f)
	tracking, e := s.Tracking(ctx, f.customer, o.ID)
	if e != nil || tracking.Version != 0 || tracking.CanManage || !tracking.IsCustomer {
		t.Fatalf("tracking: %+v %v", tracking, e)
	}
	_, e = s.Tracking(ctx, f.other, o.ID)
	wantProblem(t, e, "not_found")
	tomorrow := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	req := CollectionRequest{Date: tomorrow, ExpectedVersion: 0}
	u, e := s.ChangeTracking(ctx, f.customer, o.ID, "collection-request-0001", &req, nil)
	if e != nil {
		t.Fatal(e)
	}
	if u.State.Version != 1 || u.State.ConfirmedReadyAt != nil || *u.State.RequestedCollectionDate != tomorrow {
		t.Fatalf("request became confirmation: %+v", u)
	}
	replay, e := s.ChangeTracking(ctx, f.customer, o.ID, "collection-request-0001", &req, nil)
	if e != nil || replay.ID != u.ID {
		t.Fatalf("replay: %+v %v", replay, e)
	}
	changed := req
	changed.ExpectedVersion = 1
	_, e = s.ChangeTracking(ctx, f.customer, o.ID, "collection-request-0001", &changed, nil)
	wantProblem(t, e, "idempotency_conflict")
	_, e = s.ChangeTracking(ctx, f.owner, o.ID, "collection-request-0002", &changed, nil)
	wantProblem(t, e, "forbidden")
	_, e = s.ChangeTracking(ctx, f.customer, o.ID, "collection-past-0001", &CollectionRequest{Date: "2020-01-01", ExpectedVersion: 1}, nil)
	wantProblem(t, e, "invalid_configuration")
	ready := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)
	input := TrackingInput{ConfirmedReadyAt: &ready, CollectionInstructions: "Collect at the workshop after 5pm.", Note: "I can have this ready tomorrow.", ExpectedVersion: 1}
	_, e = s.ChangeTracking(ctx, f.customer, o.ID, "stringer-update-0001", nil, &input)
	wantProblem(t, e, "forbidden")
	_, e = s.ChangeTracking(ctx, f.owner, o.ID, "stringer-update-0001", nil, &input)
	wantProblem(t, e, "order_not_accepted")
	if _, e = s.UpdateStatus(ctx, f.owner, o.ID, "accepted"); e != nil {
		t.Fatal(e)
	}
	u, e = s.ChangeTracking(ctx, f.owner, o.ID, "stringer-update-0001", nil, &input)
	if e != nil {
		t.Fatal(e)
	}
	if u.State.Version != 2 || !u.State.ConfirmedReadyAt.Equal(ready) {
		t.Fatal("confirmation lost")
	}
	_, e = s.ChangeTracking(ctx, f.owner, o.ID, "stringer-update-stale", nil, &input)
	wantProblem(t, e, "tracking_changed")
	input.ExpectedVersion = 2
	input.IssueOpen = true
	input.Note = "The machine needs adjustment; please reply about collection."
	ready = ready.Add(24 * time.Hour)
	if _, e = s.ChangeTracking(ctx, f.owner, o.ID, "stringer-delay-0001", nil, &input); e != nil {
		t.Fatal(e)
	}
	latest, e := s.Tracking(ctx, f.customer, o.ID)
	if e != nil || latest.LatestUpdate == nil || latest.LatestUpdate.Note != input.Note || latest.Version != 3 {
		t.Fatalf("latest update: %+v %v", latest, e)
	}
	got, e := s.GetOrder(ctx, f.customer, o.ID)
	if e != nil || got.Status != "accepted" || got.TotalCents != o.TotalCents || got.PaymentStatus != "unpaid" {
		t.Fatalf("tracking changed fulfilment/money: %+v %v", got, e)
	}
	history, e := s.TrackingUpdates(ctx, f.customer, o.ID, Filters{Page: 1, PageSize: 2})
	if e != nil || history.Total != 3 || len(history.Data) != 2 || history.Data[0].State.Version != 3 {
		t.Fatalf("history: %+v %v", history, e)
	}
	notifications, e := s.Notifications(ctx, f.customer, Filters{Page: 1, PageSize: 20})
	if e != nil || notifications.UnreadCount != 3 {
		t.Fatalf("notifications: %+v %v", notifications, e)
	}
	if _, e = s.UpdateStatus(ctx, f.owner, o.ID, "cancelled"); e != nil {
		t.Fatal(e)
	}
	input.ExpectedVersion = 3
	_, e = s.ChangeTracking(ctx, f.owner, o.ID, "closed-tracking-0001", nil, &input)
	wantProblem(t, e, "order_closed")
	// A previously successful request remains safely replayable after closure.
	replay, e = s.ChangeTracking(ctx, f.customer, o.ID, "collection-request-0001", &req, nil)
	if e != nil || replay.State.Version != 1 {
		t.Fatalf("closed replay: %+v %v", replay, e)
	}
}
func TestIntegrationMessagesPermissionsUnreadAndReplay(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	o := trackingOrder(t, f)
	_, e := s.SendMessage(ctx, f.other, o.ID, "message-key-other-01", MessageInput{Body: "foreign"})
	wantProblem(t, e, "not_found")
	_, e = s.SendMessage(ctx, f.customer, o.ID, "message-key-blank-01", MessageInput{Body: " \n "})
	wantProblem(t, e, "invalid_configuration")
	_, e = s.SendMessage(ctx, f.customer, o.ID, "message-key-null-001", MessageInput{Body: "bad\x00text"})
	wantProblem(t, e, "invalid_configuration")
	m, e := s.SendMessage(ctx, f.customer, o.ID, "message-key-customer-01", MessageInput{Body: "Is tomorrow possible?"})
	if e != nil {
		t.Fatal(e)
	}
	replay, e := s.SendMessage(ctx, f.customer, o.ID, "message-key-customer-01", MessageInput{Body: m.Body})
	if e != nil || replay.ID != m.ID {
		t.Fatalf("message replay: %+v %v", replay, e)
	}
	_, e = s.SendMessage(ctx, f.customer, o.ID, "message-key-customer-01", MessageInput{Body: "Different"})
	wantProblem(t, e, "idempotency_conflict")
	page, e := s.Messages(ctx, f.owner, o.ID, Filters{Page: 1, PageSize: 20})
	if e != nil || page.UnreadCount != 1 || page.Data[0].Own || page.Data[0].SenderRole != "customer" {
		t.Fatalf("unread: %+v %v", page, e)
	}
	m2, e := s.SendMessage(ctx, f.customer, o.ID, "message-key-customer-02", MessageInput{Body: "Could I collect after 6?"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ReadMessages(ctx, f.owner, o.ID, m.ID); e != nil {
		t.Fatal(e)
	}
	page, e = s.Messages(ctx, f.owner, o.ID, Filters{Page: 1, PageSize: 1})
	if e != nil || page.Total != 2 || page.UnreadCount != 1 || page.Data[0].ID != m2.ID {
		t.Fatalf("cursor marked unseen message: %+v %v", page, e)
	}
	if _, e = s.ReadMessages(ctx, f.owner, o.ID, m2.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ReadMessages(ctx, f.owner, o.ID, m.ID); e != nil {
		t.Fatal(e)
	}
	page, _ = s.Messages(ctx, f.owner, o.ID, Filters{Page: 1, PageSize: 20})
	if page.UnreadCount != 0 {
		t.Fatal("read cursor moved backwards")
	}
	_, e = s.ReadMessages(ctx, f.owner, o.ID, f.shop.ID)
	wantProblem(t, e, "not_found")
	_, e = s.SendMessage(ctx, f.admin, o.ID, "message-key-support-01", MessageInput{Body: "Support joined this order."})
	if e != nil {
		t.Fatal(e)
	}
	// Historical notifications do not grant a former team member order access.
	if _, e = s.Pool.Exec(ctx, `DELETE FROM stringer_members WHERE stringer_id=$1 AND user_id=$2`, f.shop.ID, f.owner.ID); e != nil {
		t.Fatal(e)
	}
	notifications, e := s.Notifications(ctx, f.owner, Filters{Page: 1, PageSize: 20})
	if e != nil || notifications.Total != 0 {
		t.Fatalf("former member inbox: %+v %v", notifications, e)
	}
	_, e = s.Messages(ctx, f.owner, o.ID, Filters{Page: 1, PageSize: 20})
	wantProblem(t, e, "not_found")
	_, e = s.SendMessage(ctx, f.owner, o.ID, "message-key-former-01", MessageInput{Body: "No access"})
	wantProblem(t, e, "not_found")
}
func TestIntegrationNotificationIsolationAndAtomicity(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	// A notification failure rolls back the order AND its stock reservation.
	if _, e := s.Pool.Exec(ctx, `ALTER TABLE notifications ADD CONSTRAINT fail_notification CHECK(kind<>'new_order')`); e != nil {
		t.Fatal(e)
	}
	q, e := s.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if e != nil {
		t.Fatal(e)
	}
	_, _, e = s.CreateOrder(ctx, f.customer, q.ID, "atomic-notification-order")
	if e == nil {
		t.Fatal("expected injected notification failure")
	}
	var orders, reserved int
	if e = s.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM orders),(SELECT reserved FROM inventory_lots WHERE id=$1)`, f.lot.ID).Scan(&orders, &reserved); e != nil || orders != 0 || reserved != 0 {
		t.Fatalf("partial order committed: %d/%d %v", orders, reserved, e)
	}
	if _, e = s.Pool.Exec(ctx, `ALTER TABLE notifications DROP CONSTRAINT fail_notification`); e != nil {
		t.Fatal(e)
	}
	o, _, e := s.CreateOrder(ctx, f.customer, q.ID, "atomic-notification-order")
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.CreateOrder(ctx, f.customer, q.ID, "atomic-notification-order"); e != nil {
		t.Fatal(e)
	}
	inbox, e := s.Notifications(ctx, f.owner, Filters{Page: 1, PageSize: 20})
	if e != nil || inbox.Total != 1 || !inbox.Data[0].ForStringer {
		t.Fatalf("new job: %+v %v", inbox, e)
	}
	n := inbox.Data[0]
	_, e = s.ReadNotification(ctx, f.customer, n.ID)
	wantProblem(t, e, "not_found")
	if _, e = s.ReadNotification(ctx, f.owner, n.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ReadNotification(ctx, f.owner, n.ID); e != nil {
		t.Fatal(e)
	}
	inbox, _ = s.Notifications(ctx, f.owner, Filters{Page: 1, PageSize: 20})
	if inbox.UnreadCount != 0 {
		t.Fatal("read failed")
	}
	if _, e = s.UpdateStatus(ctx, f.owner, o.ID, "accepted"); e != nil {
		t.Fatal(e)
	}
	for _, status := range []string{"received", "in_progress", "ready"} {
		if _, e = s.UpdateStatus(ctx, f.owner, o.ID, status); e != nil {
			t.Fatal(e)
		}
	}
	inbox, e = s.Notifications(ctx, f.customer, Filters{Page: 1, PageSize: 20})
	if e != nil || inbox.Total != 4 || inbox.Data[0].Title != "Your racket is ready for collection" {
		t.Fatalf("ready inbox: %+v %v", inbox, e)
	}
}
func TestIntegrationConcurrentTrackingAndMessages(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	s := f.store
	o := trackingOrder(t, f)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, e := s.ChangeTracking(ctx, f.customer, o.ID, fmt.Sprintf("concurrent-collection-%d", i), &CollectionRequest{Date: time.Now().UTC().AddDate(0, 0, i+1).Format("2006-01-02"), ExpectedVersion: 0}, nil)
			results <- e
		}(i)
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		} else {
			wantProblem(t, e, "tracking_changed")
		}
	}
	if success != 1 {
		t.Fatalf("lost update: %d", success)
	}
	messageIDs := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m, e := s.SendMessage(ctx, f.customer, o.ID, "concurrent-message-key", MessageInput{Body: "Only once"})
			if e != nil {
				t.Error(e)
			}
			messageIDs <- m.ID
		}()
	}
	wg.Wait()
	close(messageIDs)
	id := ""
	for v := range messageIDs {
		if id != "" && id != v {
			t.Fatal("duplicate message")
		}
		id = v
	}
	page, e := s.Messages(ctx, f.owner, o.ID, Filters{Page: 1, PageSize: 20})
	if e != nil || page.Total != 1 {
		t.Fatalf("messages: %+v %v", page, e)
	}
}
func TestIntegrationCommunicationHTTPBoundaryAndLimits(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	o := trackingOrder(t, f)
	headers := map[string]string{"Idempotency-Key": "http-message-key-0001"}
	wantStatus(t, f.request(t, "GET", "/orders/"+o.ID+"/tracking", nil, Actor{}, nil), 401)
	wantStatus(t, f.request(t, "GET", "/orders/"+o.ID+"/messages", nil, f.other, nil), 404)
	wantStatus(t, f.request(t, "POST", "/orders/"+o.ID+"/messages", map[string]any{"body": "hi", "sender_role": "stringer"}, f.customer, headers), 400)
	wantStatus(t, f.request(t, "POST", "/orders/"+o.ID+"/messages", MessageInput{Body: "hi"}, f.customer, nil), 400)
	wantStatus(t, f.request(t, "POST", "/orders/"+o.ID+"/messages", MessageInput{Body: "hi"}, f.customer, headers), 201)
	cookieHeaders := map[string]string{"Cookie": "session_token=" + f.tokens[f.customer.ID], "Idempotency-Key": "http-message-key-0001"}
	wantStatus(t, f.request(t, "POST", "/orders/"+o.ID+"/messages", MessageInput{Body: "hi"}, Actor{}, cookieHeaders), 403)
	cookieHeaders["Origin"] = "http://localhost:5173"
	wantStatus(t, f.request(t, "POST", "/orders/"+o.ID+"/messages", MessageInput{Body: "hi"}, Actor{}, cookieHeaders), 201)

	wantStatus(t, f.request(t, "GET", "/orders/"+o.ID+"/messages?page_size=500", nil, f.owner, nil), 400)
	for i := 1; i < 20; i++ {
		if _, e := f.store.SendMessage(ctx, f.customer, o.ID, fmt.Sprintf("rate-message-key-%03d", i), MessageInput{Body: "Test"}); e != nil {
			t.Fatal(e)
		}
	}
	_, e := f.store.SendMessage(ctx, f.customer, o.ID, "rate-message-key-limit", MessageInput{Body: "Too many"})
	wantProblem(t, e, "message_rate_limit")
	// A replay remains safe even when the limit has been reached.
	_, e = f.store.SendMessage(ctx, f.customer, o.ID, "http-message-key-0001", MessageInput{Body: "hi"})
	if e != nil {
		t.Fatal(e)
	}
}

func TestCollectionDateUsesIrishCalendar(t *testing.T) {
	now := time.Date(2026, 7, 1, 23, 30, 0, 0, time.UTC)
	for _, date := range []string{"2026-07-02", "2027-07-02"} {
		if e := validateCollectionDate(date, now); e != nil {
			t.Fatalf("valid Irish date %s: %v", date, e)
		}
	}
	for _, date := range []string{"2026-07-01", "2027-07-03", "2026-02-30", "2026-07-02T00:00:00Z"} {
		wantProblem(t, validateCollectionDate(date, now), "invalid_configuration")
	}
	winter := time.Date(2026, 1, 1, 23, 30, 0, 0, time.UTC)
	if e := validateCollectionDate("2026-01-01", winter); e != nil {
		t.Fatal(e)
	}
}
