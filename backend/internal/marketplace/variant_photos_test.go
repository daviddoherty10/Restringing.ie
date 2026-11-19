package marketplace

import (
	"context"
	"testing"
)

func TestIntegrationStringVariantPhotoScope(t *testing.T) {
	f := newFixture(t)
	id := f.variant.ID
	public := "/strings/" + id + "/photo"
	admin := "/admin/string-variants/" + id + "/photo"
	input := VariantPhotoInput{BusinessImageInput: BusinessImageInput{Data: testImage(t)}, Colour: f.variant.Colour}
	wantStatus(t, f.request(t, "GET", public, nil, Actor{}, nil), 404)
	wantStatus(t, f.request(t, "PUT", admin, input, f.customer, nil), 403)
	wantStatus(t, f.request(t, "PUT", admin, VariantPhotoInput{Colour: input.Colour}, f.admin, nil), 422)
	wantStatus(t, f.request(t, "PUT", admin, input, f.admin, nil), 200)
	w := f.request(t, "GET", public, nil, Actor{}, nil)
	wantStatus(t, w, 200)
	if w.Header().Get("Content-Type") != "image/png" {
		t.Fatal("expected sanitized PNG")
	}
	ctx := context.Background()
	v, err := loadVariant(ctx, f.store.Pool, f.shop.ID, id)
	if err != nil || !v.HasPhoto {
		t.Fatalf("booking variant is missing its photo flag: %+v, %v", v, err)
	}
	edit := VariantInput{ProductID: v.ProductID, GaugeMM: v.GaugeMM, Colour: v.Colour, Active: false}
	if _, err := f.store.SaveVariant(ctx, id, edit); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, f.request(t, "GET", public, nil, Actor{}, nil), 404)
	wantStatus(t, f.request(t, "GET", admin, nil, f.admin, nil), 200)
	edit.Colour, edit.Active = "Blue", true
	if _, err := f.store.SaveVariant(ctx, id, edit); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, f.request(t, "GET", public, nil, Actor{}, nil), 404)
	wantStatus(t, f.request(t, "PUT", admin, input, f.admin, nil), 409)
	input.Colour = edit.Colour
	wantStatus(t, f.request(t, "PUT", admin, input, f.admin, nil), 200)
	wantStatus(t, f.request(t, "DELETE", admin, nil, f.admin, nil), 200)
	wantStatus(t, f.request(t, "GET", public, nil, Actor{}, nil), 404)
}
