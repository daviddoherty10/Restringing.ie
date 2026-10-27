package marketplace

import (
	"github.com/go-chi/chi/v5"
	"net/http"
)

// Called only within the real session/Origin-protected router group.
func registerCommunicationRoutes(r chi.Router, s *Store) {
	r.Get("/orders/{orderID}/tracking", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.Tracking(r.Context(), actor(r), chi.URLParam(r, "orderID"))
	}))
	r.Get("/orders/{orderID}/tracking-updates", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		f, e := filters(r)
		if e != nil {
			return nil, e
		}
		return s.TrackingUpdates(r.Context(), actor(r), chi.URLParam(r, "orderID"), f)
	}))
	r.Post("/orders/{orderID}/collection-request", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var input CollectionRequest
		if e := decode(w, r, &input); e != nil {
			return nil, e
		}
		return s.ChangeTracking(r.Context(), actor(r), chi.URLParam(r, "orderID"), r.Header.Get("Idempotency-Key"), &input, nil)
	}))
	r.Post("/orders/{orderID}/tracking-updates", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var input TrackingInput
		if e := decode(w, r, &input); e != nil {
			return nil, e
		}
		return s.ChangeTracking(r.Context(), actor(r), chi.URLParam(r, "orderID"), r.Header.Get("Idempotency-Key"), nil, &input)
	}))
	r.Get("/orders/{orderID}/messages", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		f, e := filters(r)
		if e != nil {
			return nil, e
		}
		return s.Messages(r.Context(), actor(r), chi.URLParam(r, "orderID"), f)
	}))
	r.Post("/orders/{orderID}/messages", wrap(201, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var input MessageInput
		if e := decode(w, r, &input); e != nil {
			return nil, e
		}
		return s.SendMessage(r.Context(), actor(r), chi.URLParam(r, "orderID"), r.Header.Get("Idempotency-Key"), input)
	}))
	r.Post("/orders/{orderID}/messages/read", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var input MessageReadInput
		if e := decode(w, r, &input); e != nil {
			return nil, e
		}
		if e := identifier(input.LastMessageID); e != nil {
			return nil, e
		}
		return s.ReadMessages(r.Context(), actor(r), chi.URLParam(r, "orderID"), input.LastMessageID)
	}))
	r.Get("/notifications", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		f, e := filters(r)
		if e != nil {
			return nil, e
		}
		return s.Notifications(r.Context(), actor(r), f)
	}))
	r.Post("/notifications/{notificationID}/read", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var input struct{}
		if e := decode(w, r, &input); e != nil {
			return nil, e
		}
		return s.ReadNotification(r.Context(), actor(r), chi.URLParam(r, "notificationID"))
	}))
}
