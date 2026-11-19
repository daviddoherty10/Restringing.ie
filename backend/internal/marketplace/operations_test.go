package marketplace

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type operationsTransport func(*http.Request) (*http.Response, error)

func (f operationsTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func testImage(t *testing.T) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 10, 20))
	img.Set(1, 1, color.NRGBA{R: 250, A: 255})
	var b bytes.Buffer
	if e := png.Encode(&b, img); e != nil {
		t.Fatal(e)
	}
	return base64.StdEncoding.EncodeToString(b.Bytes())
}
func TestBusinessImageValidation(t *testing.T) {
	for _, raw := range []string{"", "<svg></svg>", base64.StdEncoding.EncodeToString([]byte("<svg></svg>")), strings.Repeat("A", 1500000)} {
		if _, e := cleanBusinessImage(raw); e == nil {
			t.Fatal("accepted invalid image")
		}
	}
	data, e := cleanBusinessImage(testImage(t))
	if e != nil {
		t.Fatal(e)
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil || format != "png" || cfg.Width != 10 || cfg.Height != 20 {
		t.Fatal(cfg, format, e)
	}
}
func TestIntegrationTeamInvitationsAndRevocation(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	checkMenu := func(a Actor, business, invitation bool) {
		t.Helper()
		response := f.request(t, http.MethodGet, "/me", nil, a, nil)
		wantStatus(t, response, http.StatusOK)
		var account SessionAccount
		if err := json.Unmarshal(response.Body.Bytes(), &account); err != nil || account.HasBusiness != business || account.HasTeamInvitation != invitation || account.Admin != a.Admin {
			t.Fatalf("unexpected session account: %+v (%v)", account, err)
		}
	}
	checkMenu(f.customer, false, false)
	checkMenu(f.owner, true, false)
	checkMenu(f.admin, false, false)
	_, e := s.InviteStaff(ctx, f.customer, f.shop.ID, StaffInviteInput{Phone: "+353800000001"})
	wantProblem(t, e, "forbidden")
	if _, e = s.InviteStaff(ctx, f.owner, f.shop.ID, StaffInviteInput{Phone: "+353800000001"}); e != nil {
		t.Fatal(e)
	}
	invites, e := s.MyInvitations(ctx, f.other)
	if e != nil || len(invites) != 1 {
		t.Fatal(invites, e)
	}
	checkMenu(f.other, false, true)
	id := invites[0].ID
	if _, e = s.AcceptInvitation(ctx, f.customer, id); e == nil {
		t.Fatal("wrong phone accepted")
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := s.AcceptInvitation(ctx, f.other, id); errs <- e }()
	}
	wg.Wait()
	close(errs)
	for e = range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	checkMenu(f.other, true, false)
	profile, e := s.ManagementProfile(ctx, f.other, f.shop.ID)
	if e != nil || profile.MembershipRole != "staff" || profile.Permissions.EditProfile {
		t.Fatal(profile, e)
	}
	_, e = s.Team(ctx, f.other, f.shop.ID)
	wantProblem(t, e, "forbidden")
	_, e = s.RemoveStaff(ctx, f.owner, f.shop.ID, f.owner.ID, false)
	wantProblem(t, e, "owner_protected")
	if _, e = s.RemoveStaff(ctx, f.owner, f.shop.ID, f.other.ID, false); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ManagementProfile(ctx, f.other, f.shop.ID); e == nil {
		t.Fatal("removed staff retained access")
	}
	if _, e = s.AcceptInvitation(ctx, f.other, id); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ManagementProfile(ctx, f.other, f.shop.ID); e == nil {
		t.Fatal("replay restored revoked membership")
	}
	if _, e = s.InviteStaff(ctx, f.owner, f.shop.ID, StaffInviteInput{Phone: "+353800000001"}); e != nil {
		t.Fatal(e)
	}
	if _, e = s.RemoveStaff(ctx, f.owner, f.shop.ID, id, true); e != nil {
		t.Fatal(e)
	}
	_, e = s.AcceptInvitation(ctx, f.other, id)
	wantProblem(t, e, "invitation_expired")
	if _, e = s.InviteStaff(ctx, f.owner, f.shop.ID, StaffInviteInput{Phone: "+353800000001"}); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Pool.Exec(ctx, `UPDATE staff_invitations SET expires_at=now()-interval '1 second' WHERE id=$1`, id); e != nil {
		t.Fatal(e)
	}
	_, e = s.AcceptInvitation(ctx, f.other, id)
	wantProblem(t, e, "invitation_expired")
}
func TestIntegrationBusinessImageOwnershipAndPublicScope(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	input := BusinessImageInput{Data: testImage(t)}
	_, e := s.SaveBusinessImage(ctx, f.customer, f.shop.ID, "profile", input)
	wantProblem(t, e, "forbidden")
	if _, e = s.SaveBusinessImage(ctx, f.owner, f.shop.ID, "profile", input); e != nil {
		t.Fatal(e)
	}
	h := Router(s, "https://app.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/stringers/"+f.shop.ID+"/images/profile", nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, e = s.Review(ctx, f.admin, f.shop.ID, "suspended", "test"); e != nil {
		t.Fatal(e)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/stringers/"+f.shop.ID+"/images/profile", nil))
	if w.Code == 200 {
		t.Fatal("suspended image public")
	}
	if _, e = s.DeleteBusinessImage(ctx, f.owner, f.shop.ID, "profile"); e != nil {
		t.Fatal(e)
	}
}
func TestIntegrationJobSMSQueueRetriesAndCallbacks(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	if _, e := s.SaveNotificationPreferences(ctx, f.owner, NotificationPreferences{SMS: true}); e != nil {
		t.Fatal(e)
	}
	o := trackingOrder(t, f)
	var jobs int
	if e := s.Pool.QueryRow(ctx, `SELECT count(*) FROM notification_deliveries`).Scan(&jobs); e != nil || jobs != 1 {
		t.Fatal(jobs, e)
	}
	// An exact order replay cannot queue a second delivery.
	if _, _, e := s.CreateOrder(ctx, f.customer, o.QuoteID, "tracking-order-key-0001"); e != nil {
		t.Fatal(e)
	}
	var calls atomic.Int32
	sid := "SM" + strings.Repeat("a", 32)
	sms := &JobSMS{AccountSID: "AC" + strings.Repeat("b", 32), ServiceSID: "MG" + strings.Repeat("c", 32), APIKey: "key", APISecret: "secret", AuthToken: "callback-secret", FrontendOrigin: "https://app.example"}
	sms.Client = &http.Client{Transport: operationsTransport(func(r *http.Request) (*http.Response, error) {
		n := calls.Add(1)
		if e := r.ParseForm(); e != nil {
			t.Fatal(e)
		}
		if r.Form.Get("To") != "+353800000002" || strings.Contains(r.Form.Get("Body"), "Fictional") || r.Form.Get("StatusCallback") == "" {
			t.Fatal(r.Form)
		}
		if n == 1 {
			return &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader("{}"))}, nil
		}
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(`{"sid":"` + sid + `"}`))}, nil
	})}
	s.JobSMS = sms
	if e := s.DispatchJobSMS(ctx); e != nil {
		t.Fatal(e)
	}
	var state, id string
	if e := s.Pool.QueryRow(ctx, `SELECT state,id::text FROM notification_deliveries`).Scan(&state, &id); e != nil || state != "pending" {
		t.Fatal(state, e)
	}
	if _, e := s.Pool.Exec(ctx, `UPDATE notification_deliveries SET next_attempt_at=now()`); e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e := s.DispatchJobSMS(ctx); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 2 {
		t.Fatal("duplicate sends", calls.Load())
	}
	callback := func(status string, signed bool) int {
		values := url.Values{"AccountSid": {sms.AccountSID}, "MessageSid": {sid}, "MessageStatus": {status}}
		path := "/api/v1/notifications/sms/status?delivery=" + id
		r := httptest.NewRequest("POST", path, strings.NewReader(values.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if signed {
			r.Header.Set("X-Twilio-Signature", twilioSignature(sms.AuthToken, sms.FrontendOrigin+path, values))
		}
		w := httptest.NewRecorder()
		Router(s, sms.FrontendOrigin).ServeHTTP(w, r)
		return w.Code
	}
	if code := callback("delivered", false); code != 403 {
		t.Fatal(code)
	}
	if code := callback("delivered", true); code != 204 {
		t.Fatal(code)
	}
	if code := callback("sent", true); code != 204 {
		t.Fatal(code)
	}
	if e := s.Pool.QueryRow(ctx, `SELECT state FROM notification_deliveries WHERE id=$1`, id).Scan(&state); e != nil || state != "delivered" {
		t.Fatal(state, e)
	}
	// Opt-out is checked again at dispatch, rather than only when queued.
	if _, e := s.SendMessage(ctx, f.customer, o.ID, "sms-message-key-0001", MessageInput{Body: "private message"}); e != nil {
		t.Fatal(e)
	}
	if _, e := s.SaveNotificationPreferences(ctx, f.owner, NotificationPreferences{SMS: false}); e != nil {
		t.Fatal(e)
	}
	if e := s.DispatchJobSMS(ctx); e != nil {
		t.Fatal(e)
	}
	if calls.Load() != 2 {
		t.Fatal("sent after opt out")
	}
}
func TestIntegrationJobSMSUnknownIsNotRetried(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	if _, e := s.SaveNotificationPreferences(ctx, f.owner, NotificationPreferences{SMS: true}); e != nil {
		t.Fatal(e)
	}
	trackingOrder(t, f)
	var calls int
	s.JobSMS = &JobSMS{Client: &http.Client{Transport: operationsTransport(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	})}}
	for i := 0; i < 2; i++ {
		if e := s.DispatchJobSMS(ctx); e != nil {
			t.Fatal(e)
		}
	}
	if calls != 1 {
		t.Fatal(calls)
	}
	var state string
	if e := s.Pool.QueryRow(ctx, `SELECT state FROM notification_deliveries`).Scan(&state); e != nil || state != "unknown" {
		t.Fatal(state, e)
	}
	// A crashed send is also held for reconciliation, never sent again.
	if _, e := s.Pool.Exec(ctx, `UPDATE notification_deliveries SET state='sending',started_at=$1`, time.Now().Add(-3*time.Minute)); e != nil {
		t.Fatal(e)
	}
	if e := s.DispatchJobSMS(ctx); e != nil {
		t.Fatal(e)
	}
	if calls != 1 {
		t.Fatal("crash retried")
	}
}

func TestJobSMSWireContract(t *testing.T) {
	for _, status := range []int{201, 429, 500, 400} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			p := &JobSMS{AccountSID: "AC" + strings.Repeat("a", 32), APIKey: "SKkey", APISecret: "secret", ServiceSID: "MGservice", FrontendOrigin: "https://preview.example"}
			p.Client = &http.Client{Transport: operationsTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != "https://api.twilio.com/2010-04-01/Accounts/"+p.AccountSID+"/Messages.json" {
					t.Fatal(r.URL)
				}
				user, pass, ok := r.BasicAuth()
				if !ok || user != p.APIKey || pass != p.APISecret {
					t.Fatal("auth")
				}
				if e := r.ParseForm(); e != nil {
					t.Fatal(e)
				}
				if r.PostForm.Get("MessagingServiceSid") != p.ServiceSID || r.PostForm.Get("To") != "+353800000000" || r.PostForm.Get("StatusCallback") != "https://preview.example/api/v1/notifications/sms/status?delivery=test-job" || !strings.Contains(r.PostForm.Get("Body"), "https://preview.example/orders/test-order") {
					t.Fatal(r.PostForm)
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"sid":"SM` + strings.Repeat("a", 32) + `"}`))}, nil
			})}
			_, state := p.Send(context.Background(), "+353800000000", "test-order", "test-job")
			want := map[int]string{201: "accepted", 429: "retry", 500: "unknown", 400: "failed"}[status]
			if state != want {
				t.Fatal(state, want)
			}
		})
	}
}
func TestIntegrationSMSOptOutAndRollback(t *testing.T) {
	f := newFixture(t)
	s := f.store
	ctx := context.Background()
	if _, e := s.SaveNotificationPreferences(ctx, f.owner, NotificationPreferences{SMS: true}); e != nil {
		t.Fatal(e)
	}
	o := trackingOrder(t, f)
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = notifyOrder(ctx, tx, o, f.customer.ID, "rollback-event", "message", "Private"); e != nil {
		t.Fatal(e)
	}
	if e = tx.Rollback(ctx); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = s.Pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE event_key='rollback-event'`).Scan(&count); e != nil || count != 0 {
		t.Fatal(count, e)
	}
	s.JobSMS = &JobSMS{AccountSID: "AC" + strings.Repeat("a", 32), AuthToken: "test-token", FrontendOrigin: "https://preview.example"}
	values := url.Values{"AccountSid": {s.JobSMS.AccountSID}, "From": {"+353800000002"}, "OptOutType": {"STOP"}}
	path := "/api/v1/notifications/sms/inbound"
	r := httptest.NewRequest("POST", path, strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("X-Twilio-Signature", twilioSignature(s.JobSMS.AuthToken, s.JobSMS.FrontendOrigin+path, values))
	w := httptest.NewRecorder()
	Router(s, s.JobSMS.FrontendOrigin).ServeHTTP(w, r)
	if w.Code != 204 {
		t.Fatal(w.Code)
	}
	pref, e := s.NotificationPreferences(ctx, f.owner)
	if e != nil || pref.SMS {
		t.Fatal(pref, e)
	}
}
func TestBusinessImageResizeAndMetadata(t *testing.T) {
	var b bytes.Buffer
	if e := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 2048, 2))); e != nil {
		t.Fatal(e)
	}
	b.WriteString("private trailing metadata")
	data, e := cleanBusinessImage(base64.StdEncoding.EncodeToString(b.Bytes()))
	if e != nil {
		t.Fatal(e)
	}
	cfg, _, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil || cfg.Width != 1024 || cfg.Height != 1 || bytes.Contains(data, []byte("private trailing metadata")) {
		t.Fatal(cfg, e)
	}
}

func TestIntegrationOperationsMigrationRollback(t *testing.T) {
	s := integrationStore(t)
	raw, e := os.ReadFile("../../migrations/00020_notifications_team_and_images.sql")
	if e != nil {
		t.Fatal(e)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatal("missing down migration")
	}
	if _, e = s.Pool.Exec(context.Background(), parts[1]); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Pool.Exec(context.Background(), parts[0]); e != nil {
		t.Fatal(e)
	}
}
