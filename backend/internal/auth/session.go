// Package auth supplies the session boundary for the phone OTP sign-in flow.
// Only a successful OTP verification may issue an opaque session token.
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Principal struct {
	UserID string
	Admin  bool
}
type contextKey struct{}

func Current(r *http.Request) Principal {
	p, _ := r.Context().Value(contextKey{}).(Principal)
	return p
}

func Require(pool *pgxpool.Pool, frontendOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fail := func(status int, code, message string) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Cache-Control", "no-store")
				w.WriteHeader(status)
				json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
			}
			var token string
			header := r.Header.Get("Authorization")
			if header != "" {
				if !strings.HasPrefix(header, "Bearer ") {
					fail(401, "unauthorized", "A valid session is required.")
					return
				}
				token = strings.TrimPrefix(header, "Bearer ")
			} else {
				cookie, err := r.Cookie("session_token")
				if err != nil {
					fail(401, "unauthorized", "A valid session is required.")
					return
				}
				token = cookie.Value
				// Cookie-bearing writes need an explicit trusted Origin. Bearer clients
				// do not depend on ambient browser credentials and use no cookie fallback.
				if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Header.Get("Origin") != frontendOrigin {
					fail(403, "untrusted_origin", "A trusted Origin is required for cookie-authenticated writes.")
					return
				}
			}
			decoded, err := base64.RawURLEncoding.DecodeString(token)
			if err != nil || len(decoded) != 32 {
				fail(401, "unauthorized", "A valid session is required.")
				return
			}
			digest := sha256.Sum256([]byte(token))
			var p Principal
			var role string
			err = pool.QueryRow(r.Context(), `SELECT u.id::text,u.platform_role FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>now() AND s.revoked_at IS NULL AND u.status='active' AND u.phone_verified_at IS NOT NULL`, digest[:]).Scan(&p.UserID, &role)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					fail(401, "unauthorized", "A valid session is required.")
				} else {
					fail(503, "authentication_unavailable", "Session verification is temporarily unavailable.")
				}
				return
			}
			p.Admin = role == "admin"
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, p)))
		})
	}
}
