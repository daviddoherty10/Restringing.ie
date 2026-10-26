// Generate the API contract: go run ./cmd/openapi > ../api/openapi.json
package main

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	a "github.com/daviddoherty10/Restringing.ie/backend/internal/auth"
	m "github.com/daviddoherty10/Restringing.ie/backend/internal/marketplace"
)

type schema = map[string]any

var definitions = map[string]any{}
var names = map[reflect.Type]string{}
var uuidFields = regexp.MustCompile(`(^id$|_id$)`)

func ref(name string) schema { return schema{"$ref": "#/components/schemas/" + name} }
func describe(t reflect.Type) schema {
	if t == reflect.TypeOf(time.Time{}) {
		return schema{"type": "string", "format": "date-time"}
	}
	if t.Kind() == reflect.Pointer {
		return schema{"anyOf": []any{describe(t.Elem()), schema{"type": "null"}}}
	}
	if name, ok := names[t]; ok {
		return ref(name)
	}
	switch t.Kind() {
	case reflect.String:
		return schema{"type": "string"}
	case reflect.Bool:
		return schema{"type": "boolean"}
	case reflect.Int, reflect.Int32, reflect.Int64:
		return schema{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return schema{"type": "number"}
	case reflect.Slice:
		return schema{"type": "array", "items": describe(t.Elem())}
	case reflect.Map:
		return schema{"type": "object", "additionalProperties": describe(t.Elem())}
	case reflect.Struct:
		return object(t)
	}
	return schema{}
}
func object(t reflect.Type) schema {
	properties := schema{}
	required := []string{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous {
			embedded := object(f.Type)
			for k, v := range embedded["properties"].(schema) {
				properties[k] = v
			}
			required = append(required, embedded["required"].([]string)...)
			continue
		}
		tag := strings.Split(f.Tag.Get("json"), ",")
		if tag[0] == "-" || tag[0] == "" {
			continue
		}
		p := describe(f.Type)
		if f.Type.Kind() == reflect.String && uuidFields.MatchString(tag[0]) && tag[0] != "challenge_id" {
			p["format"] = "uuid"
		}
		if strings.HasSuffix(tag[0], "_cents") {
			p["minimum"] = 0
			p["maximum"] = 1000000
			if tag[0] == "total_cents" {
				p["maximum"] = 10000000
			}
		}
		switch tag[0] {
		case "comfort", "control", "power", "spin", "durability":
			p["minimum"] = 1
			p["maximum"] = 5
		case "gauge_mm":
			p["minimum"] = 0.4
			p["maximum"] = 2
			p["multipleOf"] = 0.01
		case "instructions":
			p["maxLength"] = 500
		case "prestretch_percent", "max_prestretch_percent":
			p["minimum"] = 0
			p["maximum"] = 30
		case "sport":
			p["enum"] = []string{"tennis", "badminton", "squash"}
		case "material":
			p["enum"] = []string{"multifilament", "synthetic_gut", "polyester", "natural_gut", "nylon", "hybrid"}
		case "string_source":
			p["enum"] = []string{"stock", "customer"}
		case "experience":
			p["enum"] = []string{"new", "club", "advanced"}
		case "sessions_per_week":
			p["minimum"] = 0
			p["maximum"] = 21
		case "knots":
			p["enum"] = []int{2, 4}
		case "kind":
			p["enum"] = []string{"set", "reel"}
		case "preferences":
			p["propertyNames"] = schema{"enum": []string{"comfort", "control", "power", "spin", "durability"}}
			p["additionalProperties"] = schema{"type": "integer", "minimum": 0, "maximum": 5}
		}
		properties[tag[0]] = p
		if len(tag) == 1 || tag[1] != "omitempty" {
			required = append(required, tag[0])
		}
	}
	return schema{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}
func register(name string, value any) {
	t := reflect.TypeOf(value)
	names[t] = name
	definitions[name] = object(t)
}
func inputFrom(name, original string, omit ...string) {
	raw, _ := json.Marshal(definitions[original])
	var v schema
	json.Unmarshal(raw, &v)
	properties := v["properties"].(map[string]any)
	for _, key := range omit {
		delete(properties, key)
	}
	required := []string{}
	for _, key := range v["required"].([]any) {
		if _, ok := properties[key.(string)]; ok {
			required = append(required, key.(string))
		}
	}
	v["required"] = required
	definitions[name] = v
}
func query(name string, required bool, s schema) schema {
	return schema{"name": name, "in": "query", "required": required, "schema": s}
}
func content(name string) schema { return schema{"application/json": schema{"schema": ref(name)}} }
func main() {
	register("AuthRequest", a.RequestCodeInput{})
	register("AuthVerify", a.VerifyCodeInput{})
	register("AuthChallenge", a.Challenge{})
	register("AuthConfig", a.AuthConfig{})
	register("LogoutResponse", a.LogoutResponse{})
	register("Product", m.Product{})
	register("Variant", m.Variant{})
	register("Racket", m.Racket{})
	register("Service", m.Service{})
	register("Stringer", m.Stringer{})
	register("BusinessPermissions", m.BusinessPermissions{})
	register("ManagementProfile", m.ManagementProfile{})
	register("OrderSummary", m.OrderSummary{})
	register("ProfileInput", m.ProfileInput{})
	register("Booking", m.Booking{})
	register("Configuration", m.Configuration{})
	register("PriceLine", m.PriceLine{})
	register("Quote", m.Quote{})
	register("OrderEvent", m.OrderEvent{})
	register("Order", m.Order{})
	register("OrderCustomer", m.OrderCustomer{})
	register("IncomingOrder", m.IncomingOrder{})
	register("BookingOptions", m.BookingOptions{})
	register("Lot", m.Lot{})
	register("StockLot", m.StockLot{})
	register("VariantInput", m.VariantInput{})
	register("RecommendationRequest", m.RecommendationRequest{})
	register("Recommendation", m.Recommendation{})
	register("Recommendations", m.Recommendations{})
	register("VariantPage", m.Page[m.Variant]{})
	register("StringerPage", m.Page[m.Stringer]{})
	register("RacketPage", m.Page[m.Racket]{})
	register("OrderPage", m.Page[m.Order]{})
	register("LotPage", m.Page[m.StockLot]{})
	register("Problem", m.Problem{})
	register("IDResponse", struct {
		ID string `json:"id"`
	}{})
	register("OrderCreate", struct {
		QuoteID string `json:"quote_id"`
	}{})
	register("StatusInput", struct {
		Status string `json:"status"`
	}{})
	register("ApprovalInput", struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}{})
	register("InventoryInput", struct {
		Price     int  `json:"material_price_cents"`
		Available bool `json:"available"`
	}{})
	register("LotInput", struct {
		Kind      string `json:"kind"`
		Quantity  int    `json:"quantity"`
		SetLength int    `json:"set_length_mm"`
	}{})
	register("AdjustmentInput", struct {
		Delta  int    `json:"quantity_delta"`
		Reason string `json:"reason"`
	}{})
	register("Me", struct {
		ID    string `json:"id"`
		Admin bool   `json:"admin"`
	}{})
	register("Health", struct {
		Status string `json:"status"`
	}{})
	definitions["ErrorEnvelope"] = schema{"type": "object", "properties": schema{"error": ref("Problem")}, "required": []string{"error"}}
	definitions["ServiceList"] = schema{"type": "array", "items": ref("Service")}
	definitions["StringerList"] = schema{"type": "array", "items": ref("Stringer")}
	definitions["ManagementProfileList"] = schema{"type": "array", "items": ref("ManagementProfile")}
	inputFrom("ProductInput", "Product", "id")
	inputFrom("RacketInput", "Racket", "id")
	inputFrom("ServiceInput", "Service", "id")
	for _, name := range []string{"Booking", "Configuration"} {
		v := definitions[name].(schema)
		required := []string{}
		for _, key := range v["required"].([]string) {
			if key != "stencil" && key != "prestretch_percent" && key != "instructions" {
				required = append(required, key)
			}
		}
		v["required"] = required
	}
	definitions["AuthRequest"].(schema)["properties"].(schema)["phone_number"].(schema)["maxLength"] = 40
	for _, name := range []string{"AuthVerify", "AuthChallenge"} {
		p := definitions[name].(schema)["properties"].(schema)["challenge_id"].(schema)
		p["pattern"] = "^[A-Za-z0-9_-]{43}$"
	}
	definitions["AuthVerify"].(schema)["properties"].(schema)["code"].(schema)["pattern"] = "^[0-9]{4,10}$"
	definitions["ManagementProfile"].(schema)["properties"].(schema)["membership_role"].(schema)["enum"] = []string{"owner", "staff", "admin"}
	definitions["OrderSummary"].(schema)["properties"].(schema)["booked_value_cents"].(schema)["maximum"] = 9007199254740991
	// Optional transient answers must never become required health disclosures.
	definitions["RecommendationRequest"].(schema)["required"] = []string{"racket_model_id", "experience"}
	definitions["LotInput"].(schema)["required"] = []string{"kind", "quantity"}
	paths := schema{}
	add := func(method, path, id, summary, request, response string, private bool, params ...schema) {
		responses := schema{"200": schema{"description": "Success", "content": content(response)}}
		if method == "post" && path != "/stringers/{stringerID}/recommendations" && path != "/auth/phone/verify" && path != "/auth/logout" {
			delete(responses, "200")
			responses["201"] = schema{"description": "Created", "content": content(response)}
		}
		if path == "/orders" && method == "post" {
			responses["200"] = schema{"description": "Idempotent replay", "content": content(response)}
			params = append(params, schema{"name": "Idempotency-Key", "in": "header", "required": true, "schema": schema{"type": "string", "pattern": `^[A-Za-z0-9._:-]{16,100}$`}})
		}
		if strings.HasPrefix(path, "/auth/") && method == "post" {
			params = append(params, schema{"name": "Origin", "in": "header", "required": true, "schema": schema{"type": "string"}, "description": "Must exactly match configured FRONTEND_ORIGIN."})
			if path == "/auth/phone/verify" || path == "/auth/logout" {
				responses["200"].(schema)["headers"] = schema{"Set-Cookie": schema{"description": "Host-only HttpOnly SameSite=Lax session cookie; Secure under HTTPS. Logout expires it.", "schema": schema{"type": "string"}}}
			}
		}
		responses["default"] = schema{"description": "Error: 400 invalid request; 401 session; 403 permission/Origin; 404 missing or inaccessible resource; 409 stale quote, stock, idempotency or transition conflict; 415 content type; 422 invalid fields; 429 rate limit; 500 internal; 503 authentication temporarily unavailable. See error.code and optional fields.", "content": content("ErrorEnvelope")}
		for _, part := range strings.Split(path, "/") {
			if strings.HasPrefix(part, "{") {
				name := strings.Trim(part, "{}")
				p := schema{"type": "string", "format": "uuid"}
				if name == "sport" {
					p = schema{"type": "string", "enum": []string{"tennis", "badminton", "squash"}}
				}
				params = append(params, schema{"name": name, "in": "path", "required": true, "schema": p})
			}
		}
		op := schema{"operationId": id, "summary": summary, "responses": responses, "security": []any{}}
		if len(params) > 0 {
			op["parameters"] = params
		}
		if private {
			op["security"] = []any{schema{"sessionCookie": []string{}}, schema{"bearerSession": []string{}}}
		}
		if request != "" {
			op["requestBody"] = schema{"required": true, "content": content(request)}
		}
		if _, ok := paths[path]; !ok {
			paths[path] = schema{}
		}
		paths[path].(schema)[method] = op
	}
	pagination := []schema{query("page", false, schema{"type": "integer", "minimum": 1, "maximum": 10000, "default": 1}), query("page_size", false, schema{"type": "integer", "minimum": 1, "maximum": 50, "default": 20})}
	search := append(append([]schema{}, pagination...), query("q", false, schema{"type": "string", "maxLength": 100}))
	catalogue := append(append([]schema{}, search...), query("material", false, schema{"type": "string", "enum": []string{"multifilament", "synthetic_gut", "polyester", "natural_gut", "nylon", "hybrid"}}), query("brand", false, schema{"type": "string"}), query("sport", false, schema{"type": "string", "enum": []string{"tennis", "badminton", "squash"}}), query("gauge_mm", false, schema{"type": "number", "minimum": 0.4, "maximum": 2}), query("variant_id", false, schema{"type": "string", "format": "uuid"}))
	add("get", "/auth/config", "authConfig", "SMS availability and allowed calling codes", "", "AuthConfig", false)
	add("post", "/auth/phone/request", "requestPhoneCode", "Request a rate-limited SMS code; requires trusted Origin", "AuthRequest", "AuthChallenge", false)
	add("post", "/auth/phone/verify", "verifyPhoneCode", "Verify phone and issue HttpOnly session cookie; requires trusted Origin", "AuthVerify", "Me", false)
	add("post", "/auth/logout", "logout", "Revoke current session and clear cookie; requires trusted Origin", "", "LogoutResponse", false)
	add("get", "/health", "health", "Liveness", "", "Health", false)
	add("get", "/strings", "listStrings", "Search the shared string-variant catalogue", "", "VariantPage", false, catalogue...)
	add("get", "/rackets", "listRackets", "Search verified model-specific technical constraints", "", "RacketPage", false, append(append([]schema{}, search...), query("sport", false, schema{"type": "string"}))...)
	add("get", "/stringers", "listStringers", "Search approved stringers", "", "StringerPage", false, append(append([]schema{}, search...), query("sport", false, schema{"type": "string"}), query("county", false, schema{"type": "string"}))...)
	add("get", "/stringers/{stringerID}", "getStringer", "Read an approved public profile", "", "Stringer", false)
	add("get", "/stringers/{stringerID}/inventory", "listInventory", "Offered, enabled variants with unreserved stock; quote checks model-specific length", "", "VariantPage", false, catalogue...)
	add("get", "/stringers/{stringerID}/services", "listServices", "Read active service capabilities and prices", "", "ServiceList", false)
	add("get", "/stringers/{stringerID}/booking-configuration", "bookingConfiguration", "Intersect racket and service tension/knot constraints", "", "BookingOptions", false, query("racket_model_id", true, schema{"type": "string", "format": "uuid"}))
	add("post", "/stringers/{stringerID}/recommendations", "recommendStrings", "Transient, explained preference matching against available stock", "RecommendationRequest", "Recommendations", false)
	add("get", "/me", "getMe", "Read the verified session identity", "", "Me", true)
	add("get", "/me/stringers", "myStringers", "Read own businesses, applications, membership and permissions", "", "ManagementProfileList", true)
	add("post", "/stringers", "applyStringer", "Submit an application and create owner membership", "ProfileInput", "Stringer", true)
	add("put", "/stringers/{stringerID}", "updateProfile", "Owner or admin: replace editable profile fields", "ProfileInput", "Stringer", true)
	add("get", "/stringers/{stringerID}/manage/profile", "managementProfile", "Private business profile, membership and effective permissions", "", "ManagementProfile", true)
	add("get", "/stringers/{stringerID}/manage/summary", "orderSummary", "Members or admin: all-time booking counts and non-cancelled booking value, not revenue", "", "OrderSummary", true)
	add("get", "/stringers/{stringerID}/manage/services", "manageServices", "Members or admin: include inactive services", "", "ServiceList", true)
	add("put", "/stringers/{stringerID}/services/{sport}", "saveService", "Owner or admin: replace service policy and pre-stretch options", "ServiceInput", "Service", true)
	add("get", "/stringers/{stringerID}/manage/inventory", "manageInventory", "Members or admin: include disabled offerings", "", "VariantPage", true, catalogue...)
	add("put", "/stringers/{stringerID}/inventory/{variantID}", "saveInventory", "Members or admin: set material price and availability", "InventoryInput", "IDResponse", true)
	add("get", "/stringers/{stringerID}/inventory/lots", "listLots", "Members or admin: physical and reserved quantities", "", "LotPage", true, append(append([]schema{}, search...), query("sport", false, schema{"type": "string", "enum": []string{"tennis", "badminton", "squash"}}), query("variant_id", false, schema{"type": "string", "format": "uuid"}))...)
	add("post", "/stringers/{stringerID}/inventory/{variantID}/lots", "addLot", "Receive sets or a reel; quantities use sets or millimetres", "LotInput", "Lot", true)
	add("post", "/stringers/{stringerID}/inventory/lots/{lotID}/adjustments", "adjustLot", "Audited stock adjustment; cannot remove reservations", "AdjustmentInput", "Lot", true)
	add("post", "/stringers/{stringerID}/quotes", "createQuote", "Validate and persist a 10-minute server quotation; no stock hold yet", "Booking", "Quote", true)
	add("post", "/orders", "createOrder", "Reprice quote and atomically reserve stock; payment remains unpaid", "OrderCreate", "Order", true)
	orderFilters := append(append([]schema{}, pagination...), query("status", false, schema{"type": "string", "enum": []string{"placed", "accepted", "received", "in_progress", "ready", "completed", "cancelled"}}))
	add("get", "/orders", "myOrders", "Customer's order history", "", "OrderPage", true, orderFilters...)
	add("get", "/orders/{orderID}", "getOrder", "Customer, business members or admin; includes status history", "", "Order", true)
	add("get", "/stringers/{stringerID}/orders/{orderID}", "incomingOrder", "Business members or admin: order details and fulfilment contact; scoped to this business", "", "IncomingOrder", true)
	add("get", "/stringers/{stringerID}/orders", "incomingOrders", "Business members or admin: incoming orders", "", "OrderPage", true, orderFilters...)
	add("patch", "/orders/{orderID}/status", "updateStatus", "Validate workflow; customers may cancel their placed orders", "StatusInput", "Order", true)
	add("get", "/admin/stringer-applications", "applications", "Admin: pending application queue", "", "StringerPage", true, pagination...)
	add("patch", "/admin/stringers/{stringerID}/approval", "reviewStringer", "Admin: approve, reject or suspend with audit history", "ApprovalInput", "Stringer", true)
	for _, v := range []struct{ path, name, input, output, param string }{{"string-products", "Product", "ProductInput", "Product", "productID"}, {"string-variants", "Variant", "VariantInput", "IDResponse", "variantID"}, {"rackets", "Racket", "RacketInput", "Racket", "racketID"}} {
		add("post", "/admin/"+v.path, "create"+v.name, "Admin: create curated "+v.name, v.input, v.output, true)
		add("put", "/admin/"+v.path+"/{"+v.param+"}", "update"+v.name, "Admin: replace curated "+v.name, v.input, v.output, true)
	}
	doc := schema{"openapi": "3.1.0", "info": schema{"title": "Restringing.ie Marketplace API", "version": "0.1.0", "description": "Money: integer EUR cents. Tension: integer centi-kg. Reel quantities: integer millimetres. Session tokens are issued only after phone OTP verification, via the configured SMS provider. Cookie writes require the configured frontend Origin. Catalogue ratings are editorial, not medical claims. See docs/backend-marketplace.md for validation, limits, examples and deployment prerequisites."}, "servers": []schema{{"url": "http://localhost:8081/api/v1"}}, "paths": paths, "components": schema{"schemas": definitions, "securitySchemes": schema{"sessionCookie": schema{"type": "apiKey", "in": "cookie", "name": "session_token"}, "bearerSession": schema{"type": "http", "scheme": "bearer", "bearerFormat": "32 random bytes encoded as unpadded base64url"}}}}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(doc); err != nil {
		panic(err)
	}
}
