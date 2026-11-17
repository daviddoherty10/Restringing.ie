package marketplace

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		ww.Header().Set("X-Request-ID", middleware.GetReqID(r.Context()))
		next.ServeHTTP(ww, r)
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}
		// Use route templates, never URLs, query strings, headers or bodies.
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		}
		slog.Log(r.Context(), level, "http_request", "request_id", middleware.GetReqID(r.Context()),
			"method", r.Method, "route", route, "status", status,
			"duration_ms", time.Since(started).Milliseconds())
	})
}

func (s *Store) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if s.Pool == nil || s.Pool.Ping(ctx) != nil {
		sendError(w, r, problem(http.StatusServiceUnavailable, "not_ready", "Service temporarily unavailable."))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
