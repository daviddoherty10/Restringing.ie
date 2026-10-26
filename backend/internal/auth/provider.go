package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var ErrInvalidCode = errors.New("invalid or expired verification")
var ErrProviderLimited = errors.New("verification provider rate limit")
var ErrProviderUnavailable = errors.New("verification provider unavailable")
var ErrUndeliverablePhone = errors.New("phone does not support SMS")

// VerificationProvider must only approve a code after the phone owner verifies it.
// Implementations never return or log codes, credentials or provider response bodies.
type VerificationProvider interface {
	Start(context.Context, string) (string, error)
	Check(context.Context, string, string) error
}

type TwilioVerify struct {
	ServiceSID, Username, Password string
	Client                         *http.Client
}

var serviceSID = regexp.MustCompile(`^VA[0-9a-fA-F]{32}$`)
var verificationSID = regexp.MustCompile(`^VE[0-9a-fA-F]{32}$`)

func (p *TwilioVerify) call(ctx context.Context, path string, values url.Values) (string, string, error) {
	if !serviceSID.MatchString(p.ServiceSID) || p.Username == "" || p.Password == "" {
		return "", "", ErrProviderUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://verify.twilio.com/v2/Services/"+p.ServiceSID+path, strings.NewReader(values.Encode()))
	if err != nil {
		return "", "", ErrProviderUnavailable
	}
	req.SetBasicAuth(p.Username, p.Password)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	res, err := client.Do(req)
	if err != nil {
		return "", "", ErrProviderUnavailable
	}
	defer res.Body.Close()
	var body struct {
		SID    string `json:"sid"`
		Status string `json:"status"`
		Code   int    `json:"code"`
	}
	err = json.NewDecoder(io.LimitReader(res.Body, 16384)).Decode(&body)
	if body.Code == 60205 {
		return "", "", ErrUndeliverablePhone
	}
	if res.StatusCode == 429 || body.Code == 60202 || body.Code == 60203 || body.Code == 60207 || body.Code == 60212 {
		return "", "", ErrProviderLimited
	}
	if res.StatusCode == 404 {
		return "", "", ErrInvalidCode
	}
	if err != nil || res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", "", ErrProviderUnavailable
	}
	if !verificationSID.MatchString(body.SID) {
		return "", "", ErrProviderUnavailable
	}
	return body.SID, body.Status, nil
}

func (p *TwilioVerify) Start(ctx context.Context, phone string) (string, error) {
	sid, status, err := p.call(ctx, "/Verifications", url.Values{"To": {phone}, "Channel": {"sms"}})
	if err != nil {
		return "", err
	}
	if status != "pending" {
		return "", ErrProviderUnavailable
	}
	return sid, nil
}
func (p *TwilioVerify) Check(ctx context.Context, sid, code string) error {
	if !verificationSID.MatchString(sid) {
		return ErrInvalidCode
	}
	got, status, err := p.call(ctx, "/VerificationCheck", url.Values{"VerificationSid": {sid}, "Code": {code}})
	if err != nil {
		return err
	}
	if got != sid || status != "approved" {
		return ErrInvalidCode
	}
	return nil
}
