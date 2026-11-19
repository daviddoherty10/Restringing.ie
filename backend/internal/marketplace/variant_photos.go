package marketplace

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type VariantPhotoInput struct {
	BusinessImageInput
	Colour string `json:"colour"`
}

func (s *Store) variantPhoto(admin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "variantID")
		if err := identifier(id); err != nil {
			sendError(w, r, err)
			return
		}
		var data []byte
		err := s.Pool.QueryRow(r.Context(), `SELECT v.photo FROM string_variants v
			JOIN string_products p ON p.id=v.product_id
			WHERE v.id=$1 AND v.photo IS NOT NULL AND ($2 OR (v.active AND p.active))`, id, admin).Scan(&data)
		if err != nil {
			sendError(w, r, missing(err))
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(data)
	}
}

func registerVariantPhotoAdmin(r chi.Router, s *Store) {
	const path = "/string-variants/{variantID}/photo"
	r.Get(path, s.variantPhoto(true))
	r.Put(path, wrap(http.StatusOK, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var input VariantPhotoInput
		if err := decodeImage(w, r, &input); err != nil {
			return nil, err
		}
		data, err := cleanBusinessImage(input.Data)
		if err != nil {
			return nil, err
		}
		tag, err := s.Pool.Exec(r.Context(), `UPDATE string_variants SET photo=$2 WHERE id=$1 AND colour=$3`, chi.URLParam(r, "variantID"), data, input.Colour)
		if err == nil && tag.RowsAffected() == 0 {
			return nil, problem(409, "variant_changed", "Reload the variant before uploading a photo. Its colour may have changed.")
		}
		return ReadResult{Read: err == nil}, err
	}))
	r.Delete(path, wrap(http.StatusOK, func(_ http.ResponseWriter, r *http.Request) (any, error) {
		_, err := s.Pool.Exec(r.Context(), `UPDATE string_variants SET photo=NULL WHERE id=$1`, chi.URLParam(r, "variantID"))
		return ReadResult{Read: err == nil}, err
	}))
}
