package marketplace

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func TestRequestLogUsesRouteTemplateAndOmitsPersonalData(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	router := chi.NewRouter()
	router.Use(middleware.RequestID, requestLog, middleware.Recoverer)
	router.Post("/orders/{orderID}", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) })
	request := httptest.NewRequest(http.MethodPost, "/orders/private-order?phone=private-phone", strings.NewReader("private-body"))
	request.Header.Set("Authorization", "Bearer private-token")
	request.Header.Set("Cookie", "session_token=private-cookie")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["route"] != "/orders/{orderID}" || entry["status"] != float64(503) || entry["level"] != "ERROR" || entry["method"] != "POST" {
		t.Fatal(entry)
	}
	if response.Header().Get("X-Request-ID") == "" || entry["request_id"] != response.Header().Get("X-Request-ID") || entry["duration_ms"].(float64) < 0 {
		t.Fatal("missing correlation or duration", entry)
	}
	if strings.Contains(logs.String(), "private-") {
		t.Fatal("request data leaked into logs", logs.String())
	}
	logs.Reset()
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/private-unmatched", nil))
	if strings.Contains(logs.String(), "private-") || !strings.Contains(logs.String(), `"route":"unmatched"`) {
		t.Fatal("unmatched URL leaked into logs", logs.String())
	}
}

func TestReadinessUnavailableDoesNotChangeLiveness(t *testing.T) {
	router := Router(&Store{}, "https://app.example.invalid")
	for path, want := range map[string]int{"/api/v1/health": 200, "/api/v1/ready": 503} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != want || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(path, response.Code, response.Body.String())
		}
	}
}

func TestIntegrationReadinessChecksPostgres(t *testing.T) {
	s := integrationStore(t)
	router := Router(s, "https://app.example.invalid")
	for _, want := range []int{200, 503} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))
		if response.Code != want {
			t.Fatal(response.Code, response.Body.String())
		}
		s.Pool.Close()
	}
}
