package marketplace

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

type NotificationPreferences struct {
	SMS bool `json:"sms_enabled"`
}
type JobSMS struct {
	AccountSID, APIKey, APISecret, ServiceSID, AuthToken, FrontendOrigin string
	Client                                                               *http.Client
}

func (s *Store) NotificationPreferences(ctx context.Context, a Actor) (NotificationPreferences, error) {
	var out NotificationPreferences
	e := s.Pool.QueryRow(ctx, `SELECT job_sms_enabled FROM users WHERE id=$1 AND status='active'`, a.ID).Scan(&out.SMS)
	return out, missing(e)
}
func (s *Store) SaveNotificationPreferences(ctx context.Context, a Actor, input NotificationPreferences) (NotificationPreferences, error) {
	_, e := s.Pool.Exec(ctx, `UPDATE users SET job_sms_enabled=$2,job_sms_updated_at=clock_timestamp() WHERE id=$1 AND status='active' AND phone_verified_at IS NOT NULL`, a.ID, input.SMS)
	return input, e
}
func (t *JobSMS) Send(ctx context.Context, to, order, job string) (string, string) {
	values := url.Values{"To": {to}, "MessagingServiceSid": {t.ServiceSID}, "Body": {"Restringing.ie: you have an order update. Sign in to view: " + t.FrontendOrigin + "/orders/" + order + ". Manage SMS alerts in Notifications."}, "StatusCallback": {t.FrontendOrigin + "/api/v1/notifications/sms/status?delivery=" + job}, "ValidityPeriod": {"3600"}}
	req, e := http.NewRequestWithContext(ctx, "POST", "https://api.twilio.com/2010-04-01/Accounts/"+t.AccountSID+"/Messages.json", strings.NewReader(values.Encode()))
	if e != nil {
		return "", "unknown"
	}
	req.SetBasicAuth(t.APIKey, t.APISecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := t.Client
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	response, e := client.Do(req)
	if e != nil {
		return "", "unknown"
	}
	defer response.Body.Close()
	if response.StatusCode == 429 {
		return "", "retry"
	}
	if response.StatusCode >= 500 {
		return "", "unknown"
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", "failed"
	}
	var result struct {
		SID string `json:"sid"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&result) != nil || !regexp.MustCompile(`^SM[0-9a-fA-F]{32}$`).MatchString(result.SID) {
		return "", "unknown"
	}
	return result.SID, "accepted"
}

// Sending is committed before the network call. A crash or uncertain response is
// reconciled from callbacks; it is never blindly retried and double-billed.
func (s *Store) DispatchJobSMS(ctx context.Context) error {
	if s.JobSMS == nil {
		return nil
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(20261117,20)`); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `UPDATE notification_deliveries SET state='unknown',updated_at=now() WHERE state='sending' AND started_at<now()-interval '2 minutes'`); e != nil {
		return e
	}
	var id, to, order string
	var attempts int
	e = tx.QueryRow(ctx, `SELECT d.id::text,u.phone_number,n.order_id::text,d.attempts FROM notification_deliveries d JOIN notifications n ON n.id=d.notification_id JOIN users u ON u.id=n.user_id JOIN orders o ON o.id=n.order_id WHERE d.state='pending' AND d.next_attempt_at<=now() ORDER BY d.next_attempt_at,d.id FOR UPDATE OF d SKIP LOCKED LIMIT 1`).Scan(&id, &to, &order, &attempts)
	if errors.Is(e, pgx.ErrNoRows) {
		return tx.Commit(ctx)
	}
	if e != nil {
		return e
	}
	var eligible bool
	e = tx.QueryRow(ctx, `SELECT u.status='active' AND u.phone_verified_at IS NOT NULL AND u.job_sms_enabled AND n.created_at>now()-interval '24 hours' AND (o.customer_id=u.id OR EXISTS(SELECT 1 FROM stringer_members m WHERE m.stringer_id=o.stringer_id AND m.user_id=u.id)) FROM notification_deliveries d JOIN notifications n ON n.id=d.notification_id JOIN users u ON u.id=n.user_id JOIN orders o ON o.id=n.order_id WHERE d.id=$1`, id).Scan(&eligible)
	if e != nil {
		return e
	}
	if !eligible || (!strings.HasPrefix(to, "+353") && !strings.HasPrefix(to, "+44")) {
		_, e = tx.Exec(ctx, `UPDATE notification_deliveries SET state='cancelled',updated_at=now() WHERE id=$1`, id)
		if e != nil {
			return e
		}
		return tx.Commit(ctx)
	}
	var global, personal int
	e = tx.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE n.user_id=(SELECT user_id FROM notifications WHERE id=(SELECT notification_id FROM notification_deliveries WHERE id=$1))) FROM notification_deliveries d JOIN notifications n ON n.id=d.notification_id WHERE d.started_at>now()-interval '24 hours'`, id).Scan(&global, &personal)
	if e != nil {
		return e
	}
	if global >= 1000 || personal >= 50 {
		_, e = tx.Exec(ctx, `UPDATE notification_deliveries SET next_attempt_at=now()+interval '1 hour' WHERE id=$1`, id)
		if e != nil {
			return e
		}
		return tx.Commit(ctx)
	}
	if _, e = tx.Exec(ctx, `UPDATE notification_deliveries SET state='sending',attempts=attempts+1,started_at=now(),updated_at=now() WHERE id=$1`, id); e != nil {
		return e
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	sid, state := s.JobSMS.Send(ctx, to, order, id)
	if state == "retry" && attempts >= 4 {
		state = "failed"
	}
	if state == "retry" {
		_, e = s.Pool.Exec(ctx, `UPDATE notification_deliveries SET state='pending',next_attempt_at=now()+$2*interval '1 second',error_code='rate_limited',updated_at=now() WHERE id=$1 AND state='sending'`, id, (1<<attempts)*30+rand.IntN(10))
		return e
	}
	_, e = s.Pool.Exec(ctx, `UPDATE notification_deliveries SET state=$2,provider_sid=NULLIF($3,''),updated_at=now() WHERE id=$1 AND state='sending'`, id, state, sid)
	return e
}
func (s *Store) RunJobSMS(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if e := s.DispatchJobSMS(ctx); e != nil {
				slog.Error("job_sms_worker_failed", "error_type", "database_or_dispatch")
			}
		}
	}
}
func twilioSignature(token, address string, values url.Values) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(address)
	for _, k := range keys {
		items := append([]string{}, values[k]...)
		sort.Strings(items)
		for _, v := range items {
			b.WriteString(k)
			b.WriteString(v)
		}
	}
	mac := hmac.New(sha1.New, []byte(token))
	mac.Write([]byte(b.String()))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
func (s *Store) JobSMSStatus(w http.ResponseWriter, r *http.Request) {
	if s.JobSMS == nil {
		http.NotFound(w, r)
		return
	}
	id := r.URL.Query().Get("delivery")
	if identifier(id) != nil {
		http.Error(w, "Invalid delivery", 400)
		return
	}
	if r.URL.RawQuery != "delivery="+id {
		http.Error(w, "Invalid query", 400)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/x-www-form-urlencoded" || r.ParseForm() != nil {
		http.Error(w, "Invalid form", 400)
		return
	}
	expected := twilioSignature(s.JobSMS.AuthToken, s.JobSMS.FrontendOrigin+"/api/v1/notifications/sms/status?delivery="+id, r.PostForm)
	if !hmac.Equal([]byte(expected), []byte(r.Header.Get("X-Twilio-Signature"))) {
		http.Error(w, "Invalid signature", 403)
		return
	}
	sid, state := r.PostForm.Get("MessageSid"), r.PostForm.Get("MessageStatus")
	if r.PostForm.Get("AccountSid") != s.JobSMS.AccountSID || !regexp.MustCompile(`^SM[0-9a-fA-F]{32}$`).MatchString(sid) {
		http.Error(w, "Invalid message", 400)
		return
	}
	rank := map[string]int{"accepted": 1, "queued": 1, "sending": 1, "sent": 2, "failed": 3, "undelivered": 3, "delivered": 4}
	if rank[state] == 0 {
		w.WriteHeader(204)
		return
	}
	if state == "queued" || state == "sending" {
		state = "accepted"
	}
	if state == "undelivered" {
		state = "failed"
	}
	code := r.PostForm.Get("ErrorCode")
	if !regexp.MustCompile(`^[0-9]{0,6}$`).MatchString(code) {
		code = ""
	}
	_, e := s.Pool.Exec(r.Context(), `UPDATE notification_deliveries SET state=$2,provider_sid=$3,error_code=$4,updated_at=now() WHERE id=$1 AND (provider_sid IS NULL OR provider_sid=$3) AND state IN ('sending','unknown','accepted','sent','failed') AND CASE state WHEN 'sent' THEN 2 WHEN 'failed' THEN 3 ELSE 0 END <= $5`, id, state, sid, code, rank[r.PostForm.Get("MessageStatus")])
	if e != nil {
		http.Error(w, "Try again", 500)
		return
	}
	if code == "21610" {
		if _, e = s.Pool.Exec(r.Context(), `UPDATE users SET job_sms_enabled=false,job_sms_updated_at=now() WHERE id=(SELECT n.user_id FROM notifications n JOIN notification_deliveries d ON d.notification_id=n.id WHERE d.id=$1 AND d.provider_sid=$2)`, id, sid); e != nil {
			http.Error(w, "Try again", 500)
			return
		}
	}
	w.WriteHeader(204)
}

func (s *Store) JobSMSInbound(w http.ResponseWriter, r *http.Request) {
	if s.JobSMS == nil {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	if r.URL.RawQuery != "" || strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/x-www-form-urlencoded" || r.ParseForm() != nil {
		http.Error(w, "Invalid form", 400)
		return
	}
	expected := twilioSignature(s.JobSMS.AuthToken, s.JobSMS.FrontendOrigin+"/api/v1/notifications/sms/inbound", r.PostForm)
	if !hmac.Equal([]byte(expected), []byte(r.Header.Get("X-Twilio-Signature"))) || r.PostForm.Get("AccountSid") != s.JobSMS.AccountSID {
		http.Error(w, "Invalid signature", 403)
		return
	}
	if r.PostForm.Get("OptOutType") == "STOP" {
		if _, e := s.Pool.Exec(r.Context(), `UPDATE users SET job_sms_enabled=false,job_sms_updated_at=now() WHERE phone_number=$1`, r.PostForm.Get("From")); e != nil {
			http.Error(w, "Try again", 500)
			return
		}
	}
	w.WriteHeader(204)
}
