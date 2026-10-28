package marketplace

import (
	"github.com/go-chi/chi/v5"
	"net/http"
)

func registerGripPublicRoutes(r chi.Router, s *Store) {
	r.Get("/grips", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		f, e := gripFilters(r)
		if e != nil {
			return nil, e
		}
		return s.ListGripProducts(r.Context(), Actor{}, f, false)
	}))
	r.Get("/stringers/{stringerID}/grips", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		f, e := gripFilters(r)
		if e != nil {
			return nil, e
		}
		return s.OfferedGrips(r.Context(), chi.URLParam(r, "stringerID"), f)
	}))
}
func registerGripPrivateRoutes(r chi.Router, s *Store) {
	r.Get("/stringers/{stringerID}/manage/grips", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		f, e := gripFilters(r)
		if e != nil {
			return nil, e
		}
		return s.ManageGrips(r.Context(), actor(r), chi.URLParam(r, "stringerID"), f)
	}))
	r.Put("/stringers/{stringerID}/grips/{gripProductID}", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var p GripOfferingInput
		if e := decode(w, r, &p); e != nil {
			return nil, e
		}
		return s.SaveGripOffering(r.Context(), actor(r), chi.URLParam(r, "stringerID"), chi.URLParam(r, "gripProductID"), p)
	}))
	r.Post("/stringers/{stringerID}/grips/{gripProductID}/stock-events", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var p GripStockInput
		if e := decode(w, r, &p); e != nil {
			return nil, e
		}
		return s.AdjustGripStock(r.Context(), actor(r), chi.URLParam(r, "stringerID"), chi.URLParam(r, "gripProductID"), r.Header.Get("Idempotency-Key"), p)
	}))
	r.Get("/stringers/{stringerID}/grips/{gripProductID}/stock-events", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		f, e := filters(r)
		if e != nil {
			return nil, e
		}
		return s.GripMovements(r.Context(), actor(r), chi.URLParam(r, "stringerID"), chi.URLParam(r, "gripProductID"), f)
	}))
}
func registerGripAdminRoutes(r chi.Router, s *Store) {
	r.Get("/grips", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		f, e := gripFilters(r)
		if e != nil {
			return nil, e
		}
		return s.ListGripProducts(r.Context(), actor(r), f, true)
	}))
	r.Post("/grips", wrap(201, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var p GripInput
		if e := decode(w, r, &p); e != nil {
			return nil, e
		}
		return s.SaveGripProduct(r.Context(), actor(r), "", r.Header.Get("Idempotency-Key"), p)
	}))
	r.Put("/grips/{gripProductID}", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var p GripInput
		if e := decode(w, r, &p); e != nil {
			return nil, e
		}
		return s.SaveGripProduct(r.Context(), actor(r), chi.URLParam(r, "gripProductID"), "", p)
	}))
}
