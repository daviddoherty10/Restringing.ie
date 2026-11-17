package marketplace

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type BusinessImageInput struct {
	Data string `json:"data"`
}
type BusinessImage struct {
	Version string `json:"version"`
}

func cleanBusinessImage(raw string) ([]byte, error) {
	data, e := base64.StdEncoding.DecodeString(raw)
	if e != nil || len(data) > 1048576 {
		return nil, invalid(map[string]string{"data": "Choose a PNG or JPEG up to 1 MB."})
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil || (format != "png" && format != "jpeg") || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4096 || cfg.Height > 4096 || cfg.Width*cfg.Height > 4_000_000 {
		return nil, invalid(map[string]string{"data": "Choose a PNG or JPEG up to 4 megapixels and 4096 pixels per side."})
	}
	src, _, e := image.Decode(bytes.NewReader(data))
	if e != nil {
		return nil, invalid(map[string]string{"data": "The image could not be decoded."})
	}
	w, h := cfg.Width, cfg.Height
	if w > 1024 || h > 1024 {
		if w >= h {
			h = h * 1024 / w
			w = 1024
		} else {
			w = w * 1024 / h
			h = 1024
		}
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	if w == cfg.Width && h == cfg.Height {
		draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	} else {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(x, y, src.At(src.Bounds().Min.X+x*cfg.Width/w, src.Bounds().Min.Y+y*cfg.Height/h))
			}
		}
	}
	var out bytes.Buffer
	if e = png.Encode(&out, dst); e != nil {
		return nil, e
	}
	if out.Len() > 1048576 {
		return nil, invalid(map[string]string{"data": "Use a smaller or simpler image; the processed image exceeds 1 MB."})
	}
	return out.Bytes(), nil
}
func (s *Store) SaveBusinessImage(ctx context.Context, a Actor, id, slot string, input BusinessImageInput) (BusinessImage, error) {
	var out BusinessImage
	if slot != "profile" && slot != "banner" {
		return out, problem(404, "not_found", "Unknown image slot.")
	}
	if e := authorize(ctx, s.Pool, a, id, true); e != nil {
		return out, e
	}
	data, e := cleanBusinessImage(input.Data)
	if e != nil {
		return out, e
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	if _, e = lockStringer(ctx, tx, id); e != nil {
		return out, e
	}
	if e = authorize(ctx, tx, a, id, true); e != nil {
		return out, e
	}
	e = tx.QueryRow(ctx, `INSERT INTO business_images(stringer_id,slot,data) VALUES($1,$2,$3) ON CONFLICT(stringer_id,slot) DO UPDATE SET data=EXCLUDED.data,version=gen_random_uuid(),updated_at=now() RETURNING version::text`, id, slot, data).Scan(&out.Version)
	if e != nil {
		return out, e
	}
	return out, tx.Commit(ctx)
}
func (s *Store) DeleteBusinessImage(ctx context.Context, a Actor, id, slot string) (ReadResult, error) {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return ReadResult{}, e
	}
	defer tx.Rollback(ctx)
	if _, e = lockStringer(ctx, tx, id); e != nil {
		return ReadResult{}, e
	}
	if e = authorize(ctx, tx, a, id, true); e != nil {
		return ReadResult{}, e
	}
	if _, e = tx.Exec(ctx, `DELETE FROM business_images WHERE stringer_id=$1 AND slot=$2`, id, slot); e != nil {
		return ReadResult{}, e
	}
	return ReadResult{true}, tx.Commit(ctx)
}
func (s *Store) BusinessImage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "stringerID")
	if e := identifier(id); e != nil {
		sendError(w, r, e)
		return
	}
	if _, e := s.GetStringer(r.Context(), id); e != nil {
		sendError(w, r, e)
		return
	}
	var data []byte
	if e := s.Pool.QueryRow(r.Context(), `SELECT data FROM business_images WHERE stringer_id=$1 AND slot=$2`, id, chi.URLParam(r, "slot")).Scan(&data); e != nil {
		sendError(w, r, missing(e))
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(data)
}
