package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const SessionLifetime = 7 * 24 * time.Hour
const ResendSeconds = 60

type RequestCodeInput struct {
	Phone string `json:"phone_number"`
}
type VerifyCodeInput struct {
	ChallengeID string `json:"challenge_id"`
	Code        string `json:"code"`
}
type Challenge struct {
	ID          string    `json:"challenge_id"`
	ExpiresAt   time.Time `json:"expires_at"`
	ResendAfter int       `json:"resend_after_seconds"`
}
type Identity struct {
	ID    string `json:"id"`
	Admin bool   `json:"admin"`
}
type AuthConfig struct {
	Available   bool     `json:"available"`
	Countries   []string `json:"allowed_calling_codes"`
	ResendAfter int      `json:"resend_after_seconds"`
}
type LogoutResponse struct {
	SignedOut bool `json:"signed_out"`
}

type PhoneService struct {
	Pool           *pgxpool.Pool
	Provider       VerificationProvider
	RateKey        []byte
	FrontendOrigin string
	CookieSecure   bool
	// E.164 calling codes, e.g. +353 and +44. This is also a spending boundary.
	AllowedCallingCodes []string
	// Only explicit reverse-proxy peer networks may supply X-Forwarded-For.
	TrustedProxies []*net.IPNet
}

func (s *PhoneService) Register(r chi.Router) {
	r.Get("/auth/config", func(w http.ResponseWriter, r *http.Request) {
		s.respond(w, 200, AuthConfig{s.enabled(), s.AllowedCallingCodes, ResendSeconds})
	})
	r.Post("/auth/phone/request", s.requestCode)
	r.Post("/auth/phone/verify", s.verifyCode)
	r.Post("/auth/logout", s.logout)
}
func (s *PhoneService) enabled() bool { return s.Provider != nil && len(s.RateKey) >= 32 }
func (s *PhoneService) respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (s *PhoneService) fail(w http.ResponseWriter, status int, code, message string) {
	s.respond(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func (s *PhoneService) trusted(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Origin") != s.FrontendOrigin {
		s.fail(w, 403, "untrusted_origin", "Sign in from the configured website.")
		return false
	}
	return true
}
func (s *PhoneService) decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || kind != "application/json" {
		s.fail(w, 415, "content_type", "Use application/json.")
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	d.DisallowUnknownFields()
	if err = d.Decode(dst); err != nil {
		s.fail(w, 400, "invalid_request", "Check the sign-in details and try again.")
		return false
	}
	if err = d.Decode(new(any)); err != io.EOF {
		s.fail(w, 400, "invalid_request", "Send one JSON object.")
		return false
	}
	return true
}

var e164 = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
var codePattern = regexp.MustCompile(`^[0-9]{4,10}$`)

func NormalizePhone(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) > 40 {
		return "", errors.New("invalid phone")
	}
	// Only harmless formatting is removed. International calling code is explicit;
	// no guessed country, trunk prefix, extension or ambiguous numeric conversion.
	raw = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(raw)
	if !e164.MatchString(raw) {
		return "", errors.New("invalid phone")
	}
	return raw, nil
}
func randomToken() (string, []byte, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	hash := sha256.Sum256([]byte(token))
	return token, hash[:], nil
}
func tokenHash(token string) []byte {
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(b) != 32 {
		return nil
	}
	h := sha256.Sum256([]byte(token))
	return h[:]
}
func (s *PhoneService) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer == nil {
		return "unknown"
	}
	trusted := func(ip net.IP) bool {
		for _, n := range s.TrustedProxies {
			if n.Contains(ip) {
				return true
			}
		}
		return false
	}
	if !trusted(peer) {
		return peer.String()
	}
	// Walk right-to-left, skipping only proxies explicitly trusted by the operator.
	// Forged leftmost addresses are never accepted ahead of an untrusted hop.
	chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(chain) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(chain[i]))
		if ip == nil {
			return peer.String()
		}
		if !trusted(ip) {
			return ip.String()
		}
	}
	return peer.String()
}
func (s *PhoneService) limit(ctx context.Context, tx pgx.Tx, key string, maximum int) (bool, error) {
	mac := hmac.New(sha256.New, s.RateKey)
	mac.Write([]byte(key))
	digest := mac.Sum(nil)
	var count int
	err := tx.QueryRow(ctx, `INSERT INTO auth_rate_limits(key_hash,window_start,requests,expires_at) VALUES($1,floor(extract(epoch from now())/3600)::bigint,1,date_trunc('hour',now())+interval '2 hours') ON CONFLICT(key_hash,window_start) DO UPDATE SET requests=auth_rate_limits.requests+1 RETURNING requests`, digest).Scan(&count)
	return count <= maximum, err
}
func (s *PhoneService) limited(w http.ResponseWriter, seconds int) {
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	s.fail(w, 429, "rate_limited", "Too many attempts. Please wait before trying again.")
}
func (s *PhoneService) unavailable(w http.ResponseWriter) {
	s.fail(w, 503, "authentication_unavailable", "SMS sign-in is temporarily unavailable. Please try again later.")
}
func (s *PhoneService) requestCode(w http.ResponseWriter, r *http.Request) {
	if !s.trusted(w, r) {
		return
	}
	if !s.enabled() {
		s.unavailable(w)
		return
	}
	var input RequestCodeInput
	if !s.decode(w, r, &input) {
		return
	}
	phone, err := NormalizePhone(input.Phone)
	allowed := false
	for _, prefix := range s.AllowedCallingCodes {
		if strings.HasPrefix(phone, prefix) {
			allowed = true
		}
	}
	if err != nil || !allowed {
		s.fail(w, 422, "invalid_phone", "Enter a supported phone number including its international calling code.")
		return
	}
	token, hash, err := randomToken()
	if err != nil {
		s.unavailable(w)
		return
	}
	ctx := r.Context()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		s.unavailable(w)
		return
	}
	defer tx.Rollback(ctx)
	// Global/IP/phone buckets are shared by all API instances. Fixed lock order.
	for _, v := range []struct {
		key string
		max int
	}{{"send:global", 200}, {"send:ip:" + s.clientIP(r), 20}, {"send:phone:" + phone, 5}} {
		ok, e := s.limit(ctx, tx, v.key, v.max)
		if e != nil {
			s.unavailable(w)
			return
		}
		if !ok {
			s.limited(w, 3600)
			return
		}
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,1))`, phone); err != nil {
		s.unavailable(w)
		return
	}
	var cooldown bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth_challenges WHERE phone_number=$1 AND created_at>now()-interval '60 seconds')`, phone).Scan(&cooldown)
	if err != nil {
		s.unavailable(w)
		return
	}
	if cooldown {
		s.limited(w, ResendSeconds)
		return
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_challenges SET consumed_at=now() WHERE phone_number=$1 AND consumed_at IS NULL`, phone); err != nil {
		s.unavailable(w)
		return
	}
	var expires time.Time
	if err = tx.QueryRow(ctx, `INSERT INTO auth_challenges(token_hash,phone_number) VALUES($1,$2) RETURNING expires_at`, hash, phone).Scan(&expires); err != nil {
		s.unavailable(w)
		return
	}
	// Retention cleanup is bounded per send; it never touches business/customer data.
	if _, err = tx.Exec(ctx, `DELETE FROM auth_challenges WHERE token_hash IN (SELECT token_hash FROM auth_challenges WHERE expires_at<now()-interval '24 hours' LIMIT 200)`); err != nil {
		s.unavailable(w)
		return
	}
	if _, err = tx.Exec(ctx, `DELETE FROM auth_rate_limits WHERE (key_hash,window_start) IN (SELECT key_hash,window_start FROM auth_rate_limits WHERE expires_at<now() LIMIT 200)`); err != nil {
		s.unavailable(w)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		s.unavailable(w)
		return
	}
	sid, err := s.Provider.Start(ctx, phone)
	if err != nil {
		if errors.Is(err, ErrUndeliverablePhone) {
			s.fail(w, 422, "undeliverable_phone", "SMS could not be sent to this number. Check it or use a mobile number.")
		} else if errors.Is(err, ErrProviderLimited) {
			s.limited(w, ResendSeconds)
		} else {
			s.unavailable(w)
		}
		return
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE auth_challenges SET provider_sid=$2 WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at>now()`, hash, sid)
	if err != nil || tag.RowsAffected() != 1 {
		s.unavailable(w)
		return
	}
	s.respond(w, 201, Challenge{token, expires, ResendSeconds})
}
func (s *PhoneService) verifyCode(w http.ResponseWriter, r *http.Request) {
	if !s.trusted(w, r) {
		return
	}
	if !s.enabled() {
		s.unavailable(w)
		return
	}
	var input VerifyCodeInput
	if !s.decode(w, r, &input) {
		return
	}
	hash := tokenHash(input.ChallengeID)
	if hash == nil || !codePattern.MatchString(input.Code) {
		s.fail(w, 422, "invalid_code", "Enter the numeric code from your message.")
		return
	}
	ctx := r.Context()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		s.unavailable(w)
		return
	}
	defer tx.Rollback(ctx)
	ok, err := s.limit(ctx, tx, "check:ip:"+s.clientIP(r), 60)
	if err != nil {
		s.unavailable(w)
		return
	}
	if !ok {
		s.limited(w, 3600)
		return
	}
	var phone, sid string
	// Claim exactly one attempt durably BEFORE the network call. Crashes and network
	// failures cannot roll back attempt counts; concurrent checks cannot both issue.
	err = tx.QueryRow(ctx, `UPDATE auth_challenges SET attempts=attempts+1,processing=true WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at>now() AND attempts<5 AND NOT processing AND provider_sid<>'' RETURNING phone_number,provider_sid`, hash).Scan(&phone, &sid)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = tx.Commit(ctx)
		s.fail(w, 400, "invalid_verification", "Code invalid, expired or already being checked. Request a new code if needed.")
		return
	}
	if err != nil {
		s.unavailable(w)
		return
	}
	ok, err = s.limit(ctx, tx, "check:phone:"+phone, 10)
	if err != nil {
		s.unavailable(w)
		return
	}
	if !ok {
		s.limited(w, 3600)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		s.unavailable(w)
		return
	}
	err = s.Provider.Check(ctx, sid, input.Code)
	if err != nil {
		// An uncertain provider result cannot be retried as an approved verification.
		// Invalid codes may retry within the remaining attempt budget.
		_, updateErr := s.Pool.Exec(ctx, `UPDATE auth_challenges SET processing=false,consumed_at=CASE WHEN $2 THEN now() ELSE consumed_at END WHERE token_hash=$1`, hash, !errors.Is(err, ErrInvalidCode))
		if updateErr != nil {
			s.unavailable(w)
			return
		}
		if errors.Is(err, ErrProviderLimited) {
			s.limited(w, ResendSeconds)
		} else if errors.Is(err, ErrInvalidCode) {
			s.fail(w, 400, "invalid_verification", "Code invalid or expired. Check the message or request a new code.")
		} else {
			s.unavailable(w)
		}
		return
	}
	// Provider approval is necessary, never sufficient: consume the challenge and
	// check live account status atomically with session insertion.
	tx, err = s.Pool.Begin(ctx)
	if err != nil {
		s.unavailable(w)
		return
	}
	defer tx.Rollback(ctx)
	var claimed string
	err = tx.QueryRow(ctx, `UPDATE auth_challenges SET consumed_at=now(),processing=false WHERE token_hash=$1 AND consumed_at IS NULL AND processing AND expires_at>now() RETURNING phone_number`, hash).Scan(&claimed)
	if err != nil || claimed != phone {
		s.fail(w, 400, "invalid_verification", "Verification expired. Request a new code.")
		return
	}
	var me Identity
	var role, status string
	err = tx.QueryRow(ctx, `INSERT INTO users(phone_number,phone_verified_at) VALUES($1,now()) ON CONFLICT(phone_number) DO UPDATE SET phone_verified_at=CASE WHEN users.status='active' THEN now() ELSE users.phone_verified_at END,updated_at=CASE WHEN users.status='active' THEN now() ELSE users.updated_at END RETURNING id::text,platform_role,status`, phone).Scan(&me.ID, &role, &status)
	if err != nil {
		s.unavailable(w)
		return
	}
	if status != "active" {
		if err = tx.Commit(ctx); err != nil {
			s.unavailable(w)
			return
		}
		s.fail(w, 403, "account_unavailable", "This account cannot sign in. Contact support.")
		return
	}
	token, digest, err := randomToken()
	if err != nil {
		s.unavailable(w)
		return
	}
	if _, err = tx.Exec(ctx, `INSERT INTO sessions(user_id,token_hash,expires_at) VALUES($1,$2,now()+interval '7 days')`, me.ID, digest); err != nil {
		s.unavailable(w)
		return
	}
	if cookie, e := r.Cookie("session_token"); e == nil {
		if old := tokenHash(cookie.Value); old != nil {
			if _, err = tx.Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL`, old); err != nil {
				s.unavailable(w)
				return
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		s.unavailable(w)
		return
	}
	me.Admin = role == "admin"
	http.SetCookie(w, &http.Cookie{Name: "session_token", Value: token, Path: "/", HttpOnly: true, Secure: s.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: int(SessionLifetime.Seconds()), Expires: time.Now().Add(SessionLifetime)})
	s.respond(w, 200, me)
}
func (s *PhoneService) logout(w http.ResponseWriter, r *http.Request) {
	if !s.trusted(w, r) {
		return
	}
	var token string
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		token = strings.TrimPrefix(h, "Bearer ")
	} else if c, e := r.Cookie("session_token"); e == nil {
		token = c.Value
	}
	if hash := tokenHash(token); hash != nil {
		if _, err := s.Pool.Exec(r.Context(), `UPDATE sessions SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL`, hash); err != nil {
			s.unavailable(w)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: "session_token", Value: "", Path: "/", HttpOnly: true, Secure: s.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
	s.respond(w, 200, LogoutResponse{true})
}
