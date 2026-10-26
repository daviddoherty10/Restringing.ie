package config

import (
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
)

type Config struct {
	APIAddr            string
	DatabaseURL        string
	FrontendOrigin     string
	SMSProvider        string
	TwilioServiceSID   string
	TwilioUsername     string
	TwilioPassword     string
	AuthRateKey        []byte
	AuthCallingCodes   []string
	AuthTrustedProxies []*net.IPNet
	AuthCookieSecure   bool
}

func Load() (Config, error) {
	cfg := Config{
		APIAddr:        os.Getenv("API_ADDR"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		FrontendOrigin: os.Getenv("FRONTEND_ORIGIN"),
	}

	if cfg.APIAddr == "" {
		cfg.APIAddr = ":8081"
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.FrontendOrigin == "" {
		cfg.FrontendOrigin = "http://localhost:5173"
	}
	origin, err := url.Parse(cfg.FrontendOrigin)
	if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") || origin.Host == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return Config{}, errors.New("FRONTEND_ORIGIN must be an exact HTTP(S) origin without a path")
	}

	cfg.AuthCookieSecure = origin.Scheme == "https"
	if !cfg.AuthCookieSecure && origin.Hostname() != "localhost" && origin.Hostname() != "127.0.0.1" && origin.Hostname() != "::1" {
		return Config{}, errors.New("FRONTEND_ORIGIN requires HTTPS outside loopback development")
	}
	cfg.SMSProvider = os.Getenv("AUTH_SMS_PROVIDER")
	if cfg.SMSProvider != "" && cfg.SMSProvider != "twilio" {
		return Config{}, errors.New("AUTH_SMS_PROVIDER must be twilio or empty (disabled)")
	}
	cfg.AuthCallingCodes = []string{"+353", "+44"}
	if v := os.Getenv("AUTH_ALLOWED_CALLING_CODES"); v != "" {
		cfg.AuthCallingCodes = strings.Split(v, ",")
	}
	for i, code := range cfg.AuthCallingCodes {
		code = strings.TrimSpace(code)
		if !regexp.MustCompile(`^\+[1-9][0-9]{0,2}$`).MatchString(code) {
			return Config{}, errors.New("AUTH_ALLOWED_CALLING_CODES must contain comma-separated international calling codes")
		}
		cfg.AuthCallingCodes[i] = code
	}
	if cfg.SMSProvider == "twilio" {
		cfg.TwilioServiceSID = os.Getenv("TWILIO_VERIFY_SERVICE_SID")
		cfg.TwilioUsername = os.Getenv("TWILIO_API_KEY_SID")
		cfg.TwilioPassword = os.Getenv("TWILIO_API_KEY_SECRET")
		if !regexp.MustCompile(`^VA[0-9a-fA-F]{32}$`).MatchString(cfg.TwilioServiceSID) || !regexp.MustCompile(`^SK[0-9a-fA-F]{32}$`).MatchString(cfg.TwilioUsername) || cfg.TwilioPassword == "" {
			return Config{}, errors.New("Twilio requires a valid TWILIO_VERIFY_SERVICE_SID, TWILIO_API_KEY_SID and TWILIO_API_KEY_SECRET")
		}
		cfg.AuthRateKey, err = hex.DecodeString(os.Getenv("AUTH_RATE_LIMIT_KEY"))
		if err != nil || len(cfg.AuthRateKey) < 32 {
			return Config{}, errors.New("AUTH_RATE_LIMIT_KEY requires at least 32 random bytes encoded as hex")
		}
	}
	if v := os.Getenv("AUTH_TRUSTED_PROXY_CIDRS"); v != "" {
		for _, raw := range strings.Split(v, ",") {
			_, network, err := net.ParseCIDR(strings.TrimSpace(raw))
			if err != nil || network.String() == "0.0.0.0/0" || network.String() == "::/0" {
				return Config{}, errors.New("AUTH_TRUSTED_PROXY_CIDRS requires explicit proxy networks, never all addresses")
			}
			cfg.AuthTrustedProxies = append(cfg.AuthTrustedProxies, network)
		}
	}
	return cfg, nil
}
