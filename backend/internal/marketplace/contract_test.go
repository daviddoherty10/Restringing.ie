package marketplace

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestOpenAPIListsEveryRoute(t *testing.T) {
	raw, err := os.ReadFile("../../../api/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	err = chi.Walk(Router(&Store{}, "http://localhost:5173").(chi.Routes), func(method, path string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		path = strings.TrimPrefix(path, "/api/v1")
		key := strings.ToLower(method) + " " + path
		seen[key] = true
		if _, ok := doc.Paths[path][strings.ToLower(method)]; !ok {
			t.Errorf("route missing from OpenAPI: %s", key)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for path, ops := range doc.Paths {
		for method := range ops {
			if !seen[method+" "+path] {
				t.Errorf("OpenAPI declares unimplemented route: %s %s", method, path)
			}
		}
	}
}
