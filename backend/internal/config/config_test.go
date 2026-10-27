package config

import (
	"strings"
	"testing"
)

func configEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"BILLING_PROVIDER", "STRIPE_SECRET_KEY", "STRIPE_MONTHLY_PRICE_ID", "STRIPE_WEBHOOK_SECRET", "API_ADDR", "AUTH_SMS_PROVIDER", "TWILIO_VERIFY_SERVICE_SID", "TWILIO_API_KEY_SID", "TWILIO_API_KEY_SECRET", "AUTH_RATE_LIMIT_KEY", "AUTH_ALLOWED_CALLING_CODES", "AUTH_TRUSTED_PROXY_CIDRS", "FRONTEND_ORIGIN"} {
		t.Setenv(k, "")
	}
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
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
