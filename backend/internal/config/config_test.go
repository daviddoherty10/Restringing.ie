package config

import (
	"strings"
	"testing"
)

func configEnv(t *testing.T) {
	t.Helper()
	t.Setenv("BOOKING_LEGAL_ENFORCEMENT", "")
	t.Setenv("TRADER_VERIFICATION_ENFORCEMENT", "")
	for _, k := range []string{"JOB_SMS_PROVIDER", "JOB_SMS_ACCOUNT_SID", "JOB_SMS_MESSAGING_SERVICE_SID", "JOB_SMS_API_KEY_SID", "JOB_SMS_API_KEY_SECRET", "JOB_SMS_AUTH_TOKEN", "BILLING_PROVIDER", "STRIPE_SECRET_KEY", "STRIPE_MONTHLY_PRICE_ID", "STRIPE_WEBHOOK_SECRET", "API_ADDR", "AUTH_SMS_PROVIDER", "TWILIO_VERIFY_SERVICE_SID", "TWILIO_API_KEY_SID", "TWILIO_API_KEY_SECRET", "AUTH_RATE_LIMIT_KEY", "AUTH_ALLOWED_CALLING_CODES", "AUTH_TRUSTED_PROXY_CIDRS", "FRONTEND_ORIGIN"} {
		t.Setenv(k, "")
	}
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
}

func TestComplianceFlagsDefaultDisabledAndRequireExplicitBooleans(t *testing.T) {
	for _, value := range []string{"", "false", "true", "TRUE", "1", "yes"} {
		t.Run("value="+value, func(t *testing.T) {
			for _, flag := range []string{"BOOKING_LEGAL_ENFORCEMENT", "TRADER_VERIFICATION_ENFORCEMENT"} {
				configEnv(t)
				t.Setenv(flag, value)
				c, err := Load()
				if value != "" && value != "false" && value != "true" {
					if err == nil {
						t.Fatalf("%s accepted ambiguous activation value %q", flag, value)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				wantLegal := flag == "BOOKING_LEGAL_ENFORCEMENT" && value == "true"
				wantTrader := flag == "TRADER_VERIFICATION_ENFORCEMENT" && value == "true"
				if c.EnforceLegal != wantLegal || c.EnforceTrader != wantTrader {
					t.Fatalf("%s=%q activated the wrong workflow", flag, value)
				}
			}
		})
	}
}
func TestAuthConfiguration(t *testing.T) {
	configEnv(t)
	c, err := Load()
	if err != nil || c.SMSProvider != "" || c.AuthCookieSecure {
		t.Fatal(c, err)
	}
	t.Setenv("AUTH_SMS_PROVIDER", "twilio")
	if _, err = Load(); err == nil {
		t.Fatal("partial credentials accepted")
	}
	t.Setenv("TWILIO_VERIFY_SERVICE_SID", "VA"+strings.Repeat("a", 32))
	t.Setenv("TWILIO_API_KEY_SID", "SK"+strings.Repeat("b", 32))
	t.Setenv("TWILIO_API_KEY_SECRET", "test-secret")
	t.Setenv("AUTH_RATE_LIMIT_KEY", strings.Repeat("a", 64))
	t.Setenv("FRONTEND_ORIGIN", "https://restringing.example")
	c, err = Load()
	if err != nil || !c.AuthCookieSecure || len(c.AuthRateKey) != 32 {
		t.Fatal("valid production config", err)
	}
	t.Setenv("FRONTEND_ORIGIN", "http://restringing.example")
	if _, err = Load(); err == nil {
		t.Fatal("insecure public cookie config accepted")
	}
	t.Setenv("FRONTEND_ORIGIN", "https://restringing.example")
	t.Setenv("AUTH_TRUSTED_PROXY_CIDRS", "0.0.0.0/0")
	if _, err = Load(); err == nil {
		t.Fatal("all peers trusted")
	}
	t.Setenv("AUTH_TRUSTED_PROXY_CIDRS", "10.20.0.0/24")
	if _, err = Load(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AUTH_RATE_LIMIT_KEY", "short")
	if _, err = Load(); err == nil {
		t.Fatal("weak HMAC key accepted")
	}
}

func TestBillingConfiguration(t *testing.T) {
	configEnv(t)
	t.Setenv("BILLING_PROVIDER", "stripe")
	if _, err := Load(); err == nil {
		t.Fatal("partial billing configuration accepted")
	}
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_example")
	t.Setenv("STRIPE_MONTHLY_PRICE_ID", "price_example")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_example")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BILLING_PROVIDER", "")
	t.Setenv("STRIPE_SECRET_KEY", "partial")
	if _, err := Load(); err != nil {
		t.Fatal("disabled billing required credentials", err)
	}
}

func TestJobSMSConfiguration(t *testing.T) {
	configEnv(t)
	t.Setenv("JOB_SMS_PROVIDER", "")
	c, e := Load()
	if e != nil || c.JobSMSProvider != "" {
		t.Fatal(e)
	}
	t.Setenv("JOB_SMS_PROVIDER", "twilio")
	if _, e = Load(); e == nil {
		t.Fatal("partial SMS credentials accepted")
	}
	t.Setenv("FRONTEND_ORIGIN", "https://preview.example")
	t.Setenv("JOB_SMS_ACCOUNT_SID", "AC"+strings.Repeat("a", 32))
	t.Setenv("JOB_SMS_MESSAGING_SERVICE_SID", "MG"+strings.Repeat("a", 32))
	t.Setenv("JOB_SMS_API_KEY_SID", "SK"+strings.Repeat("a", 32))
	t.Setenv("JOB_SMS_API_KEY_SECRET", "test-secret")
	t.Setenv("JOB_SMS_AUTH_TOKEN", "test-auth")
	if _, e = Load(); e != nil {
		t.Fatal(e)
	}
	t.Setenv("FRONTEND_ORIGIN", "http://localhost:5173")
	if _, e = Load(); e == nil {
		t.Fatal("public callback requires HTTPS")
	}
}
