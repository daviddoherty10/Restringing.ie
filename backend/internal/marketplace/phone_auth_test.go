package marketplace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/daviddoherty10/Restringing.ie/backend/internal/auth"
)

const authOrigin = "https://restringing.example"
const testPhone = "+447700900888"

type fakeVerify struct {
	starts, checks     atomic.Int32
	startErr, checkErr error
}

func (p *fakeVerify) Start(context.Context, string) (string, error) {
	p.starts.Add(1)
	return "VE" + strings.Repeat("a", 32), p.startErr
}
func (p *fakeVerify) Check(_ context.Context, _ string, code string) error {
	p.checks.Add(1)
	if p.checkErr != nil {
		return p.checkErr
	}
	if code != "731946" {
		return auth.ErrInvalidCode
	}
	return nil
}
func phoneRouter(s *Store, p *fakeVerify) http.Handler {
	return Router(s, authOrigin, &auth.PhoneService{Pool: s.Pool, Provider: p, RateKey: bytes.Repeat([]byte{11}, 32), FrontendOrigin: authOrigin, CookieSecure: true, AllowedCallingCodes: []string{"+353", "+44"}})
}
func authCall(h http.Handler, method, path string, body any, cookie *http.Cookie, origin string) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, "/api/v1"+path, bytes.NewReader(raw))
	r.RemoteAddr = "192.0.2.9:1234"
	r.Header.Set("Content-Type", "application/json")
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func requestChallenge(t *testing.T, h http.Handler, phone string) auth.Challenge {
	t.Helper()
	w := authCall(h, "POST", "/auth/phone/request", auth.RequestCodeInput{Phone: phone}, nil, authOrigin)
	if w.Code != 201 {
		t.Fatalf("request code %d: %s", w.Code, w.Body)
	}
	var c auth.Challenge
	if err := json.Unmarshal(w.Body.Bytes(), &c); err != nil || c.ID == "" || c.ResendAfter != 60 {
		t.Fatalf("challenge %s %v", w.Body, err)
	}
	return c
}
func verifyChallenge(h http.Handler, c auth.Challenge, code string, cookie *http.Cookie) *httptest.ResponseRecorder {
	return authCall(h, "POST", "/auth/phone/verify", auth.VerifyCodeInput{ChallengeID: c.ID, Code: code}, cookie, authOrigin)
}
func sessionCookie(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("verify %d: %s", w.Code, w.Body)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies %v", cookies)
	}
	c := cookies[0]
	if c.Name != "session_token" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge != 604800 || len(c.Value) != 43 {
		t.Fatalf("unsafe cookie %+v", c)
	}
	return c
}
func TestPhoneSignupBookingLogout(t *testing.T) {
	f := newFixture(t)
	p := &fakeVerify{}
	h := phoneRouter(f.store, p)
	c := requestChallenge(t, h, "+44 (7700) 900-888")
	if p.starts.Load() != 1 {
		t.Fatal("provider not called")
	}
	var count int
	f.store.Pool.QueryRow(context.Background(), `SELECT count(*) FROM users WHERE phone_number=$1`, testPhone).Scan(&count)
	if count != 0 {
		t.Fatal("account created before verification")
	}
	if w := verifyChallenge(h, c, "000000", nil); w.Code != 400 {
		t.Fatalf("wrong code: %d", w.Code)
	}
	w := verifyChallenge(h, c, "731946", nil)
	cookie := sessionCookie(t, w)
	if strings.Contains(w.Body.String(), cookie.Value) {
		t.Fatal("token exposed in JSON")
	}
	var me auth.Identity
	json.Unmarshal(w.Body.Bytes(), &me)
	if me.Admin || me.ID == "" {
		t.Fatal("new user privileges")
	}
	var role, status string
	f.store.Pool.QueryRow(context.Background(), `SELECT platform_role,status FROM users WHERE id=$1`, me.ID).Scan(&role, &status)
	if role != "user" || status != "active" {
		t.Fatal("wrong account defaults")
	}
	if w := verifyChallenge(h, c, "731946", nil); w.Code != 400 {
		t.Fatal("code replay issued session")
	}
	if w := authCall(h, "GET", "/me", nil, cookie, ""); w.Code != 200 {
		t.Fatalf("session unusable %s", w.Body)
	}
	// New OTP-created identity can use the existing secure price/order workflow.
	q := authCall(h, "POST", "/stringers/"+f.shop.ID+"/quotes", f.booking, cookie, authOrigin)
	if q.Code != 201 {
		t.Fatalf("quote %s", q.Body)
	}
	var quote Quote
	json.Unmarshal(q.Body.Bytes(), &quote)
	r := httptest.NewRequest("POST", "/api/v1/orders", strings.NewReader(`{"quote_id":"`+quote.ID+`"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", authOrigin)
	r.Header.Set("Idempotency-Key", "otp-flow-booking-12345")
	r.AddCookie(cookie)
	o := httptest.NewRecorder()
	h.ServeHTTP(o, r)
	if o.Code != 201 {
		t.Fatalf("booking %d %s", o.Code, o.Body)
	}
	history := authCall(h, "GET", "/orders", nil, cookie, "")
	var orders Page[Order]
	json.Unmarshal(history.Body.Bytes(), &orders)
	if history.Code != 200 || len(orders.Data) != 1 || orders.Data[0].CustomerID != me.ID {
		t.Fatalf("history %s", history.Body)
	}
	if w := authCall(h, "GET", "/orders/"+orders.Data[0].ID, nil, &http.Cookie{Name: "session_token", Value: f.tokens[f.other.ID]}, ""); w.Code != 404 {
		t.Fatal("other customer can read OTP user's order")
	}
	if w := authCall(h, "POST", "/auth/logout", nil, cookie, "https://attacker.example"); w.Code != 403 {
		t.Fatal("logout CSRF")
	}
	out := authCall(h, "POST", "/auth/logout", nil, cookie, authOrigin)
	if out.Code != 200 || out.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("logout failed")
	}
	if w := authCall(h, "GET", "/me", nil, cookie, ""); w.Code != 401 {
		t.Fatal("logout did not revoke token")
	}
	if w := authCall(h, "POST", "/auth/logout", nil, cookie, authOrigin); w.Code != 200 {
		t.Fatal("logout not idempotent")
	}
}
func TestPhoneAuthExpiryAttemptsAndConcurrency(t *testing.T) {
	t.Run("expiry", func(t *testing.T) {
		s := integrationStore(t)
		p := &fakeVerify{}
		h := phoneRouter(s, p)
		c := requestChallenge(t, h, testPhone)
		s.Pool.Exec(context.Background(), `UPDATE auth_challenges SET expires_at=now()-interval '1 second'`)
		if w := verifyChallenge(h, c, "731946", nil); w.Code != 400 || p.checks.Load() != 0 {
			t.Fatal("expired challenge reached provider")
		}
	})
	t.Run("attempt budget", func(t *testing.T) {
		s := integrationStore(t)
		p := &fakeVerify{}
		h := phoneRouter(s, p)
		c := requestChallenge(t, h, testPhone)
		for i := 0; i < 5; i++ {
			if w := verifyChallenge(h, c, "000000", nil); w.Code != 400 {
				t.Fatalf("wrong code %d", w.Code)
			}
		}
		if w := verifyChallenge(h, c, "731946", nil); w.Code != 400 || p.checks.Load() != 5 {
			t.Fatal("attempt budget bypassed")
		}
	})
	t.Run("concurrent approval", func(t *testing.T) {
		s := integrationStore(t)
		p := &fakeVerify{}
		h := phoneRouter(s, p)
		c := requestChallenge(t, h, testPhone)
		var wg sync.WaitGroup
		var successes atomic.Int32
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if verifyChallenge(h, c, "731946", nil).Code == 200 {
					successes.Add(1)
				}
			}()
		}
		wg.Wait()
		var count int
		s.Pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions`).Scan(&count)
		if successes.Load() != 1 || count != 1 || p.checks.Load() != 1 {
			t.Fatalf("approval race: %d sessions %d checks %d", successes.Load(), count, p.checks.Load())
		}
	})
	t.Run("concurrent send", func(t *testing.T) {
		s := integrationStore(t)
		p := &fakeVerify{}
		h := phoneRouter(s, p)
		var wg sync.WaitGroup
		var sent atomic.Int32
		for i := 0; i < 6; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if authCall(h, "POST", "/auth/phone/request", auth.RequestCodeInput{Phone: testPhone}, nil, authOrigin).Code == 201 {
					sent.Add(1)
				}
			}()
		}
		wg.Wait()
		if sent.Load() != 1 || p.starts.Load() != 1 {
			t.Fatal("resend cooldown race")
		}
	})
}
func TestPhoneAuthExistingIdentityAndRotation(t *testing.T) {
	for _, state := range []string{"active", "suspended", "closed"} {
		t.Run(state, func(t *testing.T) {
			f := newFixture(t)
			h := phoneRouter(f.store, &fakeVerify{})
			f.store.Pool.Exec(context.Background(), `UPDATE users SET phone_number=$1,status=$2 WHERE id=$3`, testPhone, state, f.admin.ID)
			c := requestChallenge(t, h, testPhone)
			old := &http.Cookie{Name: "session_token", Value: f.tokens[f.customer.ID]}
			w := verifyChallenge(h, c, "731946", old)
			if state != "active" {
				if w.Code != 403 || len(w.Result().Cookies()) != 0 {
					t.Fatal("inactive account signed in")
				}
				return
			}
			sessionCookie(t, w)
			var me auth.Identity
			json.Unmarshal(w.Body.Bytes(), &me)
			if me.ID != f.admin.ID || !me.Admin {
				t.Fatal("existing identity or role changed")
			}
			if w := authCall(h, "GET", "/me", nil, old, ""); w.Code != 401 {
				t.Fatal("old browser session not rotated")
			}
		})
	}
}
func TestPhoneAuthBoundariesAndRateLimits(t *testing.T) {
	t.Run("origin and country", func(t *testing.T) {
		s := integrationStore(t)
		p := &fakeVerify{}
		h := phoneRouter(s, p)
		for _, origin := range []string{"", "https://attacker.example"} {
			if w := authCall(h, "POST", "/auth/phone/request", auth.RequestCodeInput{Phone: testPhone}, nil, origin); w.Code != 403 {
				t.Fatal("untrusted send")
			}
			if w := authCall(h, "POST", "/auth/phone/verify", auth.VerifyCodeInput{ChallengeID: strings.Repeat("a", 43), Code: "731946"}, nil, origin); w.Code != 403 {
				t.Fatal("login CSRF")
			}
		}
		if w := authCall(h, "POST", "/auth/phone/request", auth.RequestCodeInput{Phone: "+15005550006"}, nil, authOrigin); w.Code != 422 {
			t.Fatal("country boundary bypassed")
		}
		if p.starts.Load() != 0 {
			t.Fatal("invalid request sent SMS")
		}
	})
	t.Run("phone hour budget", func(t *testing.T) {
		s := integrationStore(t)
		p := &fakeVerify{}
		h := phoneRouter(s, p)
		for i := 0; i < 5; i++ {
			requestChallenge(t, h, testPhone)
			s.Pool.Exec(context.Background(), `UPDATE auth_challenges SET created_at=now()-interval '61 seconds'`)
		}
		w := authCall(h, "POST", "/auth/phone/request", auth.RequestCodeInput{Phone: testPhone}, nil, authOrigin)
		if w.Code != 429 || w.Header().Get("Retry-After") == "" || p.starts.Load() != 5 {
			t.Fatal("phone budget bypassed")
		}
	})
	t.Run("IP and global budgets", func(t *testing.T) {
		for _, global := range []bool{false, true} {
			t.Run(map[bool]string{false: "ip", true: "global"}[global], func(t *testing.T) {
				s := integrationStore(t)
				p := &fakeVerify{}
				h := phoneRouter(s, p)
				requestChallenge(t, h, testPhone)
				if global {
					s.Pool.Exec(context.Background(), `UPDATE auth_rate_limits SET requests=200`)
				} else {
					s.Pool.Exec(context.Background(), `UPDATE auth_rate_limits SET requests=20`)
				}
				w := authCall(h, "POST", "/auth/phone/request", auth.RequestCodeInput{Phone: "+447700900889"}, nil, authOrigin)
				if w.Code != 429 || p.starts.Load() != 1 {
					t.Fatal("IP/global budget bypassed")
				}
			})
		}
	})
	t.Run("provider failure", func(t *testing.T) {
		s := integrationStore(t)
		p := &fakeVerify{startErr: errors.New("secret provider response")}
		h := phoneRouter(s, p)
		w := authCall(h, "POST", "/auth/phone/request", auth.RequestCodeInput{Phone: testPhone}, nil, authOrigin)
		if w.Code != 503 || strings.Contains(w.Body.String(), "secret") {
			t.Fatal("provider failure leaked")
		}
		p.startErr = nil
		s.Pool.Exec(context.Background(), `UPDATE auth_challenges SET created_at=now()-interval '61 seconds'`)
		c := requestChallenge(t, h, testPhone)
		p.checkErr = auth.ErrProviderUnavailable
		w = verifyChallenge(h, c, "731946", nil)
		if w.Code != 503 {
			t.Fatal("provider check failure")
		}
		p.checkErr = nil
		if w := verifyChallenge(h, c, "731946", nil); w.Code != 400 {
			t.Fatal("ambiguous provider response replayed")
		}
	})
	t.Run("resend invalidates older challenge", func(t *testing.T) {
		s := integrationStore(t)
		h := phoneRouter(s, &fakeVerify{})
		first := requestChallenge(t, h, testPhone)
		s.Pool.Exec(context.Background(), `UPDATE auth_challenges SET created_at=now()-interval '61 seconds'`)
		second := requestChallenge(t, h, testPhone)
		if verifyChallenge(h, first, "731946", nil).Code != 400 {
			t.Fatal("old challenge still works")
		}
		sessionCookie(t, verifyChallenge(h, second, "731946", nil))
	})
	t.Run("disabled provider", func(t *testing.T) {
		s := integrationStore(t)
		h := Router(s, authOrigin)
		if w := authCall(h, "POST", "/auth/phone/request", auth.RequestCodeInput{Phone: testPhone}, nil, authOrigin); w.Code != 503 {
			t.Fatal("unconfigured auth silently enabled")
		}
	})
}
