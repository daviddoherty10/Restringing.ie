package marketplace

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func registerOperationsRoutes(r chi.Router, s *Store) {
	r.Get("/me/notification-preferences", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.NotificationPreferences(r.Context(), actor(r))
	}))
	r.Put("/me/notification-preferences", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var v NotificationPreferences
		if e := decode(w, r, &v); e != nil {
			return nil, e
		}
		return s.SaveNotificationPreferences(r.Context(), actor(r), v)
	}))
	r.Get("/me/staff-invitations", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.MyInvitations(r.Context(), actor(r))
	}))
	r.Post("/me/staff-invitations/{invitationID}/accept", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var v struct{}
		if e := decode(w, r, &v); e != nil {
			return nil, e
		}
		return s.AcceptInvitation(r.Context(), actor(r), chi.URLParam(r, "invitationID"))
	}))
	r.Get("/stringers/{stringerID}/team", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.Team(r.Context(), actor(r), chi.URLParam(r, "stringerID"))
	}))
	r.Post("/stringers/{stringerID}/staff-invitations", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var v StaffInviteInput
		if e := decode(w, r, &v); e != nil {
			return nil, e
		}
		return s.InviteStaff(r.Context(), actor(r), chi.URLParam(r, "stringerID"), v)
	}))
	r.Delete("/stringers/{stringerID}/staff-invitations/{invitationID}", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.RemoveStaff(r.Context(), actor(r), chi.URLParam(r, "stringerID"), chi.URLParam(r, "invitationID"), true)
	}))
	r.Delete("/stringers/{stringerID}/team/{userID}", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.RemoveStaff(r.Context(), actor(r), chi.URLParam(r, "stringerID"), chi.URLParam(r, "userID"), false)
	}))
	r.Put("/stringers/{stringerID}/images/{slot}", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var v BusinessImageInput
		if err := decodeImage(w, r, &v); err != nil {
			return nil, err
		}
		slot := chi.URLParam(r, "slot")
		if slot != "profile" && slot != "banner" {
			return nil, problem(400, "invalid_slot", "Choose profile or banner.")
		}
		return s.SaveBusinessImage(r.Context(), actor(r), chi.URLParam(r, "stringerID"), slot, v)
	}))
	r.Delete("/stringers/{stringerID}/images/{slot}", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.DeleteBusinessImage(r.Context(), actor(r), chi.URLParam(r, "stringerID"), chi.URLParam(r, "slot"))
	}))
}

func decodeImage[T any](w http.ResponseWriter, r *http.Request, out *T) error {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		return problem(415, "unsupported_media_type", "Use application/json.")
	}
	body := http.MaxBytesReader(w, r.Body, 1500000)
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	var value *T
	if decoder.Decode(&value) != nil || value == nil || decoder.Decode(new(any)) != io.EOF {
		return problem(400, "invalid_image", "Supply one image object up to 1 MB decoded.")
	}
	*out = *value
	return nil
}
