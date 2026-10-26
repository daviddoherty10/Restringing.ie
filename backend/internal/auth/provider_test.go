package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestTwilioWireContract(t *testing.T) {
	sid := "VE" + strings.Repeat("a", 32)
	for _, check := range []bool{false, true} {
		t.Run(fmt.Sprint(check), func(t *testing.T) {
			p := &TwilioVerify{ServiceSID: "VA" + strings.Repeat("b", 32), Username: "SK" + strings.Repeat("c", 32), Password: "test-secret"}
			p.Client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.Scheme != "https" || r.URL.Host != "verify.twilio.com" || r.Method != "POST" {
					t.Fatal("provider target")
				}
				u, pw, ok := r.BasicAuth()
				if !ok || u != p.Username || pw != p.Password {
					t.Fatal("provider credentials")
				}
				if err := r.ParseForm(); err != nil {
					t.Fatal(err)
				}
				status := "pending"
				if check {
					status = "approved"
					if !strings.HasSuffix(r.URL.Path, "/VerificationCheck") || r.PostForm.Get("VerificationSid") != sid || r.PostForm.Get("Code") != "731946" || r.PostForm.Get("To") != "" {
						t.Fatal("check not bound to issued provider SID")
					}
				} else {
					if !strings.HasSuffix(r.URL.Path, "/Verifications") || r.PostForm.Get("To") != "+447700900888" || r.PostForm.Get("Channel") != "sms" {
						t.Fatal("send contract")
					}
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"sid":"` + sid + `","status":"` + status + `"}`))}, nil
			})}
			if check {
				if err := p.Check(context.Background(), sid, "731946"); err != nil {
					t.Fatal(err)
				}
			} else {
				got, err := p.Start(context.Background(), "+447700900888")
				if err != nil || got != sid {
					t.Fatal(got, err)
				}
			}
		})
	}
}
func TestTwilioRejectsUnapprovedAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   error
	}{{"wrong code", 200, `{"sid":"VEaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","status":"pending"}`, ErrInvalidCode}, {"different sid", 200, `{"sid":"VEbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","status":"approved"}`, ErrInvalidCode}, {"expired", 404, `{}`, ErrInvalidCode}, {"limited", 429, `{}`, ErrProviderLimited}, {"landline", 400, `{"code":60205}`, ErrUndeliverablePhone}, {"max checks", 400, `{"code":60202}`, ErrProviderLimited}, {"broken", 500, `sensitive details`, ErrProviderUnavailable}, {"bad JSON", 200, `invalid`, ErrProviderUnavailable}} {
		t.Run(tc.name, func(t *testing.T) {
			p := &TwilioVerify{ServiceSID: "VA" + strings.Repeat("b", 32), Username: "test", Password: "secret", Client: &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}}
			if err := p.Check(context.Background(), "VE"+strings.Repeat("a", 32), "731946"); !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
}
func TestPhoneNormalization(t *testing.T) {
	for _, raw := range []string{"+44 (7700) 900-888", " +447700900888 "} {
		got, err := NormalizePhone(raw)
		if err != nil || got != "+447700900888" {
			t.Fatal(got, err)
		}
	}
	for _, raw := range []string{"0871234567", "00353871234567", "+0123456789", "+353871234567 ext 1", "+44\n7700900888", "+1", "+1234567890123456"} {
		if _, err := NormalizePhone(raw); err == nil {
			t.Fatalf("ambiguous phone accepted: %q", raw)
		}
	}
}
func TestProxyIPBoundary(t *testing.T) {
	_, trusted, _ := net.ParseCIDR("10.20.0.0/24")
	s := &PhoneService{TrustedProxies: []*net.IPNet{trusted}}
	for _, tc := range []struct{ peer, header, want string }{{"192.0.2.1:123", "1.2.3.4", "192.0.2.1"}, {"10.20.0.2:123", "1.2.3.4, 192.0.2.1", "192.0.2.1"}, {"10.20.0.2:123", "192.0.2.1,10.20.0.3", "192.0.2.1"}, {"10.20.0.2:123", "invalid", "10.20.0.2"}, {"[2001:db8::1]:123", "1.2.3.4", "2001:db8::1"}} {
		r := httptest.NewRequest("POST", "/", nil)
		r.RemoteAddr = tc.peer
		r.Header.Set("X-Forwarded-For", tc.header)
		if got := s.clientIP(r); got != tc.want {
			t.Fatalf("%+v got %s", tc, got)
		}
	}
}
