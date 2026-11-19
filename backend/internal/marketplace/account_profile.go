package marketplace

import (
	"context"
	"net/http"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
)

type AccountProfile struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

const accountProfileColumns = "COALESCE(first_name,''),COALESCE(last_name,''),COALESCE(email,'')"

func scanAccountProfile(row scanner) (AccountProfile, error) {
	var profile AccountProfile
	err := row.Scan(&profile.FirstName, &profile.LastName, &profile.Email)
	return profile, missing(err)
}

func (s *Store) updateAccountProfile(ctx context.Context, a Actor, input AccountProfile) (AccountProfile, error) {
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.Email = strings.TrimSpace(input.Email)
	fields := map[string]string{}
	if input.FirstName == "" || utf8.RuneCountInString(input.FirstName) > 100 || strings.ContainsFunc(input.FirstName, unicode.IsControl) {
		fields["first_name"] = "Enter a first name of up to 100 characters."
	}
	if utf8.RuneCountInString(input.LastName) > 100 || strings.ContainsFunc(input.LastName, unicode.IsControl) {
		fields["last_name"] = "Enter a last name of up to 100 characters."
	}
	if input.Email != "" {
		address, err := mail.ParseAddress(input.Email)
		if err != nil || len(input.Email) > 254 || address.Address != input.Email {
			fields["email"] = "Enter a valid email address."
		}
	}
	if len(fields) > 0 {
		return AccountProfile{}, invalid(fields)
	}
	return scanAccountProfile(s.Pool.QueryRow(ctx, `UPDATE users
		SET first_name=$2,last_name=NULLIF($3,''),email=NULLIF($4,''),updated_at=now()
		WHERE id=$1 AND status='active' RETURNING `+accountProfileColumns, a.ID, input.FirstName, input.LastName, input.Email))
}

func registerAccountProfile(r chi.Router, s *Store) {
	r.Get("/me/profile", wrap(http.StatusOK, func(_ http.ResponseWriter, r *http.Request) (any, error) {
		return scanAccountProfile(s.Pool.QueryRow(r.Context(), `SELECT `+accountProfileColumns+` FROM users WHERE id=$1 AND status='active'`, actor(r).ID))
	}))
	r.Put("/me/profile", wrap(http.StatusOK, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var input AccountProfile
		if err := decode(w, r, &input); err != nil {
			return nil, err
		}
		return s.updateAccountProfile(r.Context(), actor(r), input)
	}))
}
