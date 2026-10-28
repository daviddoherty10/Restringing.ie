package marketplace

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/daviddoherty10/Restringing.ie/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgconn"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{16,100}$`)

func actor(r *http.Request) Actor { p := auth.Current(r); return Actor{ID: p.UserID, Admin: p.Admin} }
func identifier(id string) error {
	if !uuidPattern.MatchString(id) {
		return problem(400, "invalid_id", "Expected a UUID identifier.")
	}
	return nil
}

type reply struct {
	Status int
	Body   any
}
type endpoint func(http.ResponseWriter, *http.Request) (any, error)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func sendError(w http.ResponseWriter, r *http.Request, err error) {
	var p *Problem
	if !errors.As(err, &p) {
		var pg *pgconn.PgError
		if errors.As(err, &pg) {
			switch pg.Code {
			case "23505":
				p = &Problem{Status: 409, Code: "conflict", Message: "This resource or order already exists."}
			case "23503", "23514", "22001":
				p = &Problem{Status: 422, Code: "invalid_reference", Message: "A referenced record or field violates the data rules."}
			case "40001", "40P01":
				p = &Problem{Status: 409, Code: "retry_transaction", Message: "A concurrent change occurred. Retry the request with the same idempotency key."}
			}
		}
		if p == nil {
			log.Printf("request %s failed: %T", middleware.GetReqID(r.Context()), err)
			p = &Problem{Status: 500, Code: "internal_error", Message: "The request could not be completed."}
		}
	}
	writeJSON(w, p.Status, map[string]any{"error": p})
}
func wrap(status int, f endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for _, param := range []string{"stringerID", "variantID", "orderID", "productID", "racketID", "lotID", "notificationID", "gripProductID"} {
			if value := chi.URLParam(r, param); value != "" {
				if err := identifier(value); err != nil {
					sendError(w, r, err)
					return
				}
			}
		}
		out, err := f(w, r)
		if err != nil {
			sendError(w, r, err)
			return
		}
		if v, ok := out.(reply); ok {
			writeJSON(w, v.Status, v.Body)
			return
		}
		writeJSON(w, status, out)
	}
}
func decode(w http.ResponseWriter, r *http.Request, out any) error {
	if media := strings.Split(r.Header.Get("Content-Type"), ";")[0]; media != "application/json" {
		return problem(415, "unsupported_media_type", "Use Content-Type: application/json.")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	decoder := json.NewDecoder(r.Body)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return problem(400, "invalid_json", "Supply a JSON object with only documented fields (maximum 16 KiB).")
	}
	if len(raw) == 0 || raw[0] != '{' {
		return problem(400, "invalid_json", "Supply a JSON object, not null or an array.")
	}
	strict := json.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	if err := strict.Decode(out); err != nil {
		return problem(400, "invalid_json", "Supply a JSON object with only documented fields (maximum 16 KiB).")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return problem(400, "invalid_json", "Supply exactly one JSON object.")
	}
	return nil
}
func filters(r *http.Request) (Filters, error) {
	q := r.URL.Query()
	f := Filters{Query: strings.TrimSpace(q.Get("q")), VariantID: q.Get("variant_id"), Material: q.Get("material"), Brand: q.Get("brand"), Sport: q.Get("sport"), County: q.Get("county"), Page: 1, PageSize: 20}
	if f.VariantID != "" {
		if err := identifier(f.VariantID); err != nil {
			return f, err
		}
	}
	if raw := q.Get("gauge_mm"); raw != "" {
		v, e := strconv.ParseFloat(raw, 64)
		if e != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0.4 || v > 2 {
			return f, problem(400, "invalid_gauge", "Gauge must be 0.40–2.00 mm.")
		}
		f.GaugeMM = v
	}
	for k, target := range map[string]*int{"page": &f.Page, "page_size": &f.PageSize} {
		if value := q.Get(k); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil {
				return f, problem(400, "invalid_pagination", "Page and page_size must be integers.")
			}
			*target = n
		}
	}
	if f.Page < 1 || f.Page > 10000 || f.PageSize < 1 || f.PageSize > 50 {
		return f, problem(400, "invalid_pagination", "Use page 1–10000 and page_size 1–50.")
	}
	if len(f.Query) > 100 || len(f.Brand) > 100 || len(f.County) > 120 || (f.Sport != "" && !validSport(f.Sport)) || (f.Material != "" && !validMaterial(f.Material)) {
		return f, problem(400, "invalid_filters", "Invalid search filters.")
	}
	return f, nil
}

// Router is also the integration-test entry point. All private routes use the
// real hashed-session boundary; there are no trusted user-ID headers or demo roles.
func Router(s *Store, frontendOrigin string, phoneAuth ...*auth.PhoneService) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			w.Header().Add("Vary", "Origin")
			if origin == frontendOrigin {
				w.Header().Set("Access-Control-Allow-Origin", frontendOrigin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Expose-Headers", "Retry-After")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")
			}
			if r.Method == http.MethodOptions {
				if origin != frontendOrigin {
					writeJSON(w, 403, map[string]string{"error": "untrusted_origin"})
					return
				}
				w.WriteHeader(204)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	r.Get("/api/v1/health", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	r.Route("/api/v1", func(r chi.Router) {
		phone := &auth.PhoneService{Pool: s.Pool, FrontendOrigin: frontendOrigin, AllowedCallingCodes: []string{"+353", "+44"}, CookieSecure: strings.HasPrefix(frontendOrigin, "https://")}
		if len(phoneAuth) > 0 && phoneAuth[0] != nil {
			phone = phoneAuth[0]
		}
		phone.Register(r)
		r.Post("/billing/stripe/webhook", s.BillingWebhook)
		registerGripPublicRoutes(r, s)
		r.Get("/strings", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
			f, e := filters(r)
			if e != nil {
				return nil, e
			}
			return s.ListVariants(r.Context(), "", f, false)
		}))
		r.Get("/rackets", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
			f, e := filters(r)
			if e != nil {
				return nil, e
			}
			return s.ListRackets(r.Context(), f)
		}))
		r.Get("/stringers", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
			f, e := filters(r)
			if e != nil {
				return nil, e
			}
			return s.ListStringers(r.Context(), f)
		}))
		r.Get("/stringers/{stringerID}", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
			return s.GetStringer(r.Context(), chi.URLParam(r, "stringerID"))
		}))
		r.Get("/stringers/{stringerID}/inventory", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
			id := chi.URLParam(r, "stringerID")
			if _, e := s.GetStringer(r.Context(), id); e != nil {
				return nil, e
			}
			f, e := filters(r)
			if e != nil {
				return nil, e
			}
			return s.ListVariants(r.Context(), id, f, true)
		}))
		r.Get("/stringers/{stringerID}/services", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
			id := chi.URLParam(r, "stringerID")
			if _, e := s.GetStringer(r.Context(), id); e != nil {
				return nil, e
			}
			return s.Services(r.Context(), id, false)
		}))
		r.Get("/stringers/{stringerID}/booking-configuration", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
			id := r.URL.Query().Get("racket_model_id")
			if e := identifier(id); e != nil {
				return nil, e
			}
			return s.BookingOptions(r.Context(), chi.URLParam(r, "stringerID"), id)
		}))
		r.Post("/stringers/{stringerID}/recommendations", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
			var req RecommendationRequest
			if e := decode(w, r, &req); e != nil {
				return nil, e
			}
			if e := identifier(req.RacketModelID); e != nil {
				return nil, e
			}
			if req.CurrentVariantID != "" {
				if e := identifier(req.CurrentVariantID); e != nil {
					return nil, e
				}
			}
			return s.Recommend(r.Context(), chi.URLParam(r, "stringerID"), req)
		}))
		r.Group(func(r chi.Router) {
			r.Use(auth.Require(s.Pool, frontendOrigin))
			r.Get("/me", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
				a := actor(r)
				return map[string]any{"id": a.ID, "admin": a.Admin}, nil
			}))
			r.Get("/stringers/{stringerID}/billing", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
				return s.Usage(r.Context(), actor(r), chi.URLParam(r, "stringerID"))
			}))
			r.Post("/stringers/{stringerID}/billing/checkout", wrap(201, func(w http.ResponseWriter, r *http.Request) (any, error) {
				var input struct{}
				if err := decode(w, r, &input); err != nil {
					return nil, err
				}
				return s.StartSubscription(r.Context(), actor(r), chi.URLParam(r, "stringerID"))
			}))
			r.Post("/stringers/{stringerID}/billing/portal", wrap(201, func(w http.ResponseWriter, r *http.Request) (any, error) {
				var input struct{}
				if err := decode(w, r, &input); err != nil {
					return nil, err
				}
				return s.BillingPortal(r.Context(), actor(r), chi.URLParam(r, "stringerID"))
			}))
			r.Post("/orders/{orderID}/cash-receipt", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
				var input struct{}
				if err := decode(w, r, &input); err != nil {
					return nil, err
				}
				return s.ConfirmCash(r.Context(), actor(r), chi.URLParam(r, "orderID"))
			}))
			registerGripPrivateRoutes(r, s)
			registerCommunicationRoutes(r, s)
			r.Get("/me/stringers", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) { return s.MyStringers(r.Context(), actor(r)) }))
			r.Post("/stringers", wrap(201, func(w http.ResponseWriter, r *http.Request) (any, error) {
				var p ProfileInput
				if e := decode(w, r, &p); e != nil {
					return nil, e
				}
				return s.Apply(r.Context(), actor(r), p)
			}))
			r.Put("/stringers/{stringerID}", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
				var p ProfileInput
				if e := decode(w, r, &p); e != nil {
					return nil, e
				}
				return s.UpdateProfile(r.Context(), actor(r), chi.URLParam(r, "stringerID"), p)
			}))
			r.Get("/stringers/{stringerID}/manage/profile", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
				return s.ManagementProfile(r.Context(), actor(r), chi.URLParam(r, "stringerID"))
			}))
			r.Get("/stringers/{stringerID}/manage/summary", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
				return s.OrderSummary(r.Context(), actor(r), chi.URLParam(r, "stringerID"))
			}))
			r.Get("/stringers/{stringerID}/manage/services", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
				id := chi.URLParam(r, "stringerID")
				if e := authorize(r.Context(), s.Pool, actor(r), id, false); e != nil {
					return nil, e
				}
				return s.Services(r.Context(), id, true)
			}))
			r.Put("/stringers/{stringerID}/services/{sport}", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
				var p Service
				if e := decode(w, r, &p); e != nil {
					return nil, e
				}
				if p.Sport != chi.URLParam(r, "sport") {
					return nil, problem(422, "sport_mismatch", "Body sport must match the URL.")
				}
				return s.SaveService(r.Context(), actor(r), chi.URLParam(r, "stringerID"), p)
			}))
			r.Get("/stringers/{stringerID}/manage/inventory", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
				id := chi.URLParam(r, "stringerID")
				if e := authorize(r.Context(), s.Pool, actor(r), id, false); e != nil {
					return nil, e
				}
				f, e := filters(r)
				if e != nil {
					return nil, e
				}
				return s.ListVariants(r.Context(), id, f, false)
			}))
			r.Put("/stringers/{stringerID}/inventory/{variantID}", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
				var p struct {
					Price     int  `json:"material_price_cents"`
					Available bool `json:"available"`
				}
				if e := decode(w, r, &p); e != nil {
					return nil, e
				}
				id, e := s.SaveInventory(r.Context(), actor(r), chi.URLParam(r, "stringerID"), chi.URLParam(r, "variantID"), p.Price, p.Available)
				return map[string]string{"id": id}, e
			}))
			r.Get("/stringers/{stringerID}/inventory/lots", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
				f, e := filters(r)
				if e != nil {
					return nil, e
				}
				f.VariantID = r.URL.Query().Get("variant_id")
				if f.VariantID != "" {
					if e := identifier(f.VariantID); e != nil {
						return nil, e
					}
				}
				return s.Lots(r.Context(), actor(r), chi.URLParam(r, "stringerID"), f)
			}))
			r.Post("/stringers/{stringerID}/inventory/{variantID}/lots", wrap(201, func(w http.ResponseWriter, r *http.Request) (any, error) {
				var p struct {
					Kind        string `json:"kind"`
					Quantity    int    `json:"quantity"`
					SetLengthMM int    `json:"set_length_mm"`
				}
				if e := decode(w, r, &p); e != nil {
					return nil, e
				}
				return s.AddLot(r.Context(), actor(r), chi.URLParam(r, "stringerID"), chi.URLParam(r, "variantID"), p.Kind, p.Quantity, p.SetLengthMM)
			}))
			r.Post("/stringers/{stringerID}/inventory/lots/{lotID}/adjustments", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
				var p struct {
					Delta  int    `json:"quantity_delta"`
					Reason string `json:"reason"`
				}
				if e := decode(w, r, &p); e != nil {
					return nil, e
				}
				return s.AdjustLot(r.Context(), actor(r), chi.URLParam(r, "stringerID"), chi.URLParam(r, "lotID"), p.Delta, p.Reason)
			}))
			r.Post("/stringers/{stringerID}/quotes", wrap(201, func(w http.ResponseWriter, r *http.Request) (any, error) {
				var b Booking
				if e := decode(w, r, &b); e != nil {
					return nil, e
				}
				if e := identifier(b.RacketModelID); e != nil {
					return nil, e
				}
				if b.GripProductID != "" {
					if e := identifier(b.GripProductID); e != nil {
						return nil, e
					}
				}
				if b.VariantID != "" {
					if e := identifier(b.VariantID); e != nil {
						return nil, e
					}
				}
				return s.CreateQuote(r.Context(), actor(r), chi.URLParam(r, "stringerID"), b)
			}))
			r.Post("/orders", wrap(201, func(w http.ResponseWriter, r *http.Request) (any, error) {
				var p struct {
					QuoteID       string `json:"quote_id"`
					PaymentMethod string `json:"payment_method"`
				}
				if e := decode(w, r, &p); e != nil {
					return nil, e
				}
				if p.PaymentMethod != "" && p.PaymentMethod != "cash" {
					return nil, problem(422, "payment_method_unavailable", "Choose cash payment directly to the stringer. Online customer checkout is not enabled.")
				}
				if e := identifier(p.QuoteID); e != nil {
					return nil, e
				}
				key := r.Header.Get("Idempotency-Key")
				if !keyPattern.MatchString(key) {
					return nil, problem(400, "idempotency_key_required", "Use an Idempotency-Key of 16–100 letters, digits, dots, underscores, colons or hyphens.")
				}
				o, replay, e := s.CreateOrder(r.Context(), actor(r), p.QuoteID, key)
				status := 201
				if replay {
					status = 200
				}
				return reply{status, o}, e
			}))
			r.Get("/orders", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
				f, e := filters(r)
				if e != nil {
					return nil, e
				}
				return s.ListOrders(r.Context(), actor(r), "", r.URL.Query().Get("status"), f)
			}))
			r.Get("/stringers/{stringerID}/orders/{orderID}", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
				return s.IncomingOrder(r.Context(), actor(r), chi.URLParam(r, "stringerID"), chi.URLParam(r, "orderID"))
			}))
			r.Get("/stringers/{stringerID}/orders", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
				f, e := filters(r)
				if e != nil {
					return nil, e
				}
				return s.ListOrders(r.Context(), actor(r), chi.URLParam(r, "stringerID"), r.URL.Query().Get("status"), f)
			}))
			r.Get("/orders/{orderID}", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
				return s.GetOrder(r.Context(), actor(r), chi.URLParam(r, "orderID"))
			}))
			r.Patch("/orders/{orderID}/status", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
				var p struct {
					Status string `json:"status"`
				}
				if e := decode(w, r, &p); e != nil {
					return nil, e
				}
				return s.UpdateStatus(r.Context(), actor(r), chi.URLParam(r, "orderID"), p.Status)
			}))
			r.Route("/admin", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if !actor(r).Admin {
							sendError(w, r, problem(403, "forbidden", "Administrator access is required."))
							return
						}
						next.ServeHTTP(w, r)
					})
				})
				registerGripAdminRoutes(r, s)
				r.Get("/string-products", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
					f, e := catalogueFilters(r)
					if e != nil {
						return nil, e
					}
					return s.CatalogueProducts(r.Context(), actor(r), f)
				}))
				r.Get("/string-products/{productID}", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
					return s.CatalogueProduct(r.Context(), actor(r), chi.URLParam(r, "productID"))
				}))
				r.Get("/string-variants", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
					f, e := catalogueFilters(r)
					if e != nil {
						return nil, e
					}
					return s.CatalogueVariants(r.Context(), actor(r), f)
				}))
				r.Get("/string-variants/{variantID}", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
					return s.CatalogueVariant(r.Context(), actor(r), chi.URLParam(r, "variantID"))
				}))
				r.Get("/rackets", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
					f, e := catalogueFilters(r)
					if e != nil {
						return nil, e
					}
					return s.CatalogueRackets(r.Context(), actor(r), f)
				}))
				r.Get("/rackets/{racketID}", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
					return s.CatalogueRacket(r.Context(), actor(r), chi.URLParam(r, "racketID"))
				}))
				r.Get("/stringer-applications", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
					f, e := filters(r)
					if e != nil {
						return nil, e
					}
					return s.Applications(r.Context(), actor(r), f)
				}))
				r.Patch("/stringers/{stringerID}/approval", wrap(200, func(w http.ResponseWriter, r *http.Request) (any, error) {
					var p struct {
						Status string `json:"status"`
						Reason string `json:"reason"`
					}
					if e := decode(w, r, &p); e != nil {
						return nil, e
					}
					return s.Review(r.Context(), actor(r), chi.URLParam(r, "stringerID"), p.Status, p.Reason)
				}))
				for _, method := range []string{http.MethodPost, http.MethodPut} {
					productPath, variantPath, racketPath := "/string-products", "/string-variants", "/rackets"
					if method == http.MethodPut {
						productPath += "/{productID}"
						variantPath += "/{variantID}"
						racketPath += "/{racketID}"
					}
					status := 201
					if method == http.MethodPut {
						status = 200
					}
					r.Method(method, productPath, wrap(status, func(w http.ResponseWriter, r *http.Request) (any, error) {
						var p Product
						if e := decode(w, r, &p); e != nil {
							return nil, e
						}
						return s.SaveProduct(r.Context(), chi.URLParam(r, "productID"), p)
					}))
					r.Method(method, variantPath, wrap(status, func(w http.ResponseWriter, r *http.Request) (any, error) {
						var p VariantInput
						if e := decode(w, r, &p); e != nil {
							return nil, e
						}
						if e := identifier(p.ProductID); e != nil {
							return nil, e
						}
						id, e := s.SaveVariant(r.Context(), chi.URLParam(r, "variantID"), p)
						return map[string]string{"id": id}, e
					}))
					r.Method(method, racketPath, wrap(status, func(w http.ResponseWriter, r *http.Request) (any, error) {
						var p Racket
						if e := decode(w, r, &p); e != nil {
							return nil, e
						}
						return s.SaveRacket(r.Context(), chi.URLParam(r, "racketID"), p)
					}))
				}
			})
		})
	})
	return r
}
