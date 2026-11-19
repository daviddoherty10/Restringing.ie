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
	if t == reflect.TypeOf(json.RawMessage{}) {
		return schema{"type": "object", "additionalProperties": true}
	}
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
			if tag[0] == "total_cents" || tag[0] == "estimated_total_cents" {
				p["maximum"] = 10000000
			}
		}
		switch tag[0] {
		case "comfort", "control", "power", "spin", "durability", "rating":
			p["minimum"] = 1
			p["maximum"] = 5
		case "gauge_mm":
			p["minimum"] = 0.4
			p["maximum"] = 2
			p["multipleOf"] = 0.01
		case "mains_length_mm", "crosses_length_mm":
			p["minimum"] = 0
			p["maximum"] = 30000
		case "hybrid_technical_source":
			p["maxLength"] = 1000
		case "express_turnaround", "express_turnaround_snapshot":
			p["maxLength"] = 160
		case "customer_mains_description", "customer_crosses_description", "racket_details":
			p["maxLength"] = 160
		case "instructions":
			p["maxLength"] = 500
		case "prestretch_percent", "max_prestretch_percent":
			p["minimum"] = 0
			p["maximum"] = 30
		case "sport":
			p["enum"] = []string{"tennis", "badminton", "squash"}
		case "material":
			p["enum"] = []string{"multifilament", "synthetic_gut", "polyester", "natural_gut", "nylon", "hybrid"}
		case "payment_method":
			p["enum"] = []string{"cash", "onsite", "online"}
		case "string_source":
			p["enum"] = []string{"stock", "customer"}
		case "experience":
			p["enum"] = []string{"new", "club", "advanced"}
		case "sessions_per_week":
			p["minimum"] = 0
			p["maximum"] = 21
		case "knots":
			p["enum"] = []int{2, 4}
		case "grip_snapshot":
			p["maxLength"] = 400
		case "kind":
			if t == reflect.TypeOf(m.GripInput{}) {
				p["enum"] = []string{"overgrip", "replacement_grip"}
			}
			if t == reflect.TypeOf(m.Lot{}) || t == reflect.TypeOf(m.StockLot{}) {
				p["enum"] = []string{"set", "reel", "half_racket"}
			}
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
	register("ReviewInput", m.ReviewInput{})
	definitions["ReviewInput"].(schema)["required"] = []string{"rating"}
	register("OwnReview", m.OwnReview{})
	register("PublicReviews", m.PublicReviews{})
	register("ReviewReportInput", m.ReviewReportInput{})
	register("ReviewDecision", m.ReviewDecision{})
	register("ReportedReviewPage", m.Page[m.ReportedReview]{})
	register("AuthRequest", a.RequestCodeInput{})
	register("AuthVerify", a.VerifyCodeInput{})
	register("AuthChallenge", a.Challenge{})
	register("AuthConfig", a.AuthConfig{})
	register("LogoutResponse", a.LogoutResponse{})
	register("GripInput", m.GripInput{})
	register("GripProduct", m.GripProduct{})
	register("GripOffering", m.GripOffering{})
	register("GripStock", m.GripStock{})
	register("GripOfferingInput", m.GripOfferingInput{})
	register("GripStockInput", m.GripStockInput{})
	register("GripMovement", m.GripMovement{})
	register("GripProductPage", m.Page[m.GripProduct]{})
	register("GripOfferingPage", m.Page[m.GripOffering]{})
	register("GripStockPage", m.Page[m.GripStock]{})
	register("GripMovementPage", m.Page[m.GripMovement]{})
	register("Product", m.Product{})
	register("CatalogueVariant", m.CatalogueVariant{})
	register("ProductPage", m.Page[m.Product]{})
	register("CatalogueVariantPage", m.Page[m.CatalogueVariant]{})
	register("Variant", m.Variant{})
	register("Racket", m.Racket{})
	register("Service", m.Service{})
	register("Stringer", m.Stringer{})
	register("BusinessPermissions", m.BusinessPermissions{})
	register("ManagementProfile", m.ManagementProfile{})
	register("OrderSummary", m.OrderSummary{})
	register("BusinessUsage", m.BusinessUsage{})
	register("RedirectURL", m.RedirectURL{})
	register("PaymentEvent", m.PaymentEvent{})
	register("EmptyInput", struct{}{})
	register("WebhookResponse", struct {
		Received bool `json:"received"`
	}{})
	definitions["StripeWebhook"] = schema{"type": "object", "additionalProperties": true}
	register("ProfileInput", m.ProfileInput{})
	register("Booking", m.Booking{})
	register("Configuration", m.Configuration{})
	register("PriceLine", m.PriceLine{})
	register("Quote", m.Quote{})
	register("OrderEvent", m.OrderEvent{})
	register("Order", m.Order{})
	register("TrackingState", m.TrackingState{})
	register("TrackingUpdate", m.TrackingUpdate{})
	register("OrderTracking", m.OrderTracking{})
	register("CollectionRequest", m.CollectionRequest{})
	register("TrackingInput", m.TrackingInput{})
	register("OrderMessage", m.OrderMessage{})
	register("MessageInput", m.MessageInput{})
	register("MessageReadInput", m.MessageReadInput{})
	register("MessagePage", m.MessagePage{})
	register("ReadResult", m.ReadResult{})
	register("Notification", m.Notification{})
	register("NotificationPage", m.NotificationPage{})
	register("NotificationPreferences", m.NotificationPreferences{})
	register("Team", m.Team{})
	register("TeamMember", m.TeamMember{})
	register("StaffInvitation", m.StaffInvitation{})
	register("StaffInviteInput", m.StaffInviteInput{})
	register("BusinessImageInput", m.BusinessImageInput{})
	register("VariantPhotoInput", m.VariantPhotoInput{})
	register("BusinessImage", m.BusinessImage{})
	definitions["StaffInvitationList"] = schema{"type": "array", "items": ref("StaffInvitation")}
	definitions["TwilioStatus"] = schema{"type": "object", "additionalProperties": true}
	register("TrackingUpdatePage", m.Page[m.TrackingUpdate]{})
	register("OrderCustomer", m.OrderCustomer{})
	register("IncomingOrder", m.IncomingOrder{})
	register("PublicTraderIdentity", m.PublicTraderIdentity{})
	register("ComplianceHistoryEvent", m.ComplianceHistoryEvent{})
	definitions["ComplianceHistory"] = schema{"type": "array", "items": ref("ComplianceHistoryEvent")}
	register("FinancialInput", m.FinancialInput{})
	register("FinancialEvent", m.FinancialEvent{})
	register("FinancialReport", m.FinancialReport{})
	definitions["FinancialLedger"] = schema{"type": "array", "items": ref("FinancialEvent")}
	register("CaseInput", m.CaseInput{})
	register("ComplianceCase", m.ComplianceCase{})
	register("CaseAction", m.CaseAction{})
	register("CaseDecision", m.CaseDecision{})
	register("CaseReceipt", m.CaseReceipt{})
	register("ComplianceNotification", m.ComplianceNotification{})
	definitions["CaseList"] = schema{"type": "array", "items": ref("ComplianceCase")}
	definitions["DecisionList"] = schema{"type": "array", "items": ref("CaseDecision")}
	definitions["CaseInbox"] = schema{"type": "array", "items": ref("ComplianceNotification")}
	register("TraderDocument", m.TraderDocument{})
	register("TraderRecord", m.TraderRecord{})
	register("TraderSubmission", m.TraderSubmission{})
	register("TraderReview", m.TraderReview{})
	definitions["TraderList"] = schema{"type": "array", "items": ref("TraderRecord")}
	register("PrivacyRequest", m.PrivacyRequest{})
	register("PrivacyInput", m.PrivacyInput{})
	register("PrivacyReview", m.PrivacyReview{})
	definitions["PrivacyList"] = schema{"type": "array", "items": ref("PrivacyRequest")}
	definitions["PrivacyExport"] = schema{"type": "object", "additionalProperties": true}
	register("LegalDocument", m.LegalDocument{})
	register("CheckoutDisclosure", m.CheckoutDisclosure{})
	register("BookingAcceptance", m.BookingAcceptance{})
	register("BookingConfirmation", m.BookingConfirmation{})
	register("BookingOptions", m.BookingOptions{})
	register("Lot", m.Lot{})
	register("StockLot", m.StockLot{})
	register("VariantInput", m.VariantInput{})
	register("RecommendationRequest", m.RecommendationRequest{})
	register("SuggestedSetup", m.SuggestedSetup{})
	register("Recommendation", m.Recommendation{})
	register("Recommendations", m.Recommendations{})
	register("VariantPage", m.Page[m.Variant]{})
	register("StringerPage", m.Page[m.Stringer]{})
	register("RacketPage", m.Page[m.Racket]{})
	register("OrderPage", m.Page[m.Order]{})
	register("SavedRacket", m.SavedRacket{})
	definitions["SavedRacketList"] = schema{"type": "array", "items": ref("SavedRacket")}
	register("LotPage", m.Page[m.StockLot]{})
	register("Problem", m.Problem{})
	register("IDResponse", struct {
		ID string `json:"id"`
	}{})
	register("PaymentOptions", m.PaymentOptions{})
	register("PaymentSettingsInput", m.PaymentSettingsInput{})
	register("OnsiteReceiptInput", struct {
		Method string `json:"method"`
	}{})
	register("OrderCreate", struct {
		Acceptance    *m.BookingAcceptance `json:"acceptance,omitempty"`
		QuoteID       string               `json:"quote_id"`
		PaymentMethod string               `json:"payment_method,omitempty"`
	}{})
	register("StatusInput", struct {
		Status string `json:"status"`
	}{})
	register("ApprovalInput", struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}{})
	register("InventoryInput", m.InventoryInput{})
	register("LotInput", struct {
		Kind      string `json:"kind"`
		Quantity  int    `json:"quantity"`
		SetLength int    `json:"set_length_mm"`
	}{})
	register("AdjustmentInput", struct {
		Delta  int    `json:"quantity_delta"`
		Reason string `json:"reason"`
	}{})
	register("Me", a.Identity{})
	register("SessionAccount", m.SessionAccount{})
	register("AccountProfile", m.AccountProfile{})
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
	for _, name := range []string{"MessageInput", "OrderMessage"} {
		definitions[name].(schema)["properties"].(schema)["body"].(schema)["maxLength"] = 2000
	}
	for _, name := range []string{"TrackingInput", "TrackingUpdate"} {
		definitions[name].(schema)["properties"].(schema)["note"].(schema)["maxLength"] = 1000
	}
	definitions["CollectionRequest"].(schema)["properties"].(schema)["date"].(schema)["format"] = "date"
	definitions["LotInput"].(schema)["properties"].(schema)["kind"].(schema)["enum"] = []string{"set", "reel", "half_racket"}
	for field, limit := range map[string]int{"brand": 100, "model": 120, "colour": 80, "description": 1000} {
		definitions["GripInput"].(schema)["properties"].(schema)[field].(schema)["maxLength"] = limit
	}
	definitions["BusinessImageInput"].(schema)["properties"].(schema)["data"].(schema)["maxLength"] = 1398104
	definitions["StaffInviteInput"].(schema)["properties"].(schema)["phone_number"].(schema)["maxLength"] = 40
	definitions["GripStockInput"].(schema)["properties"].(schema)["reason"].(schema)["maxLength"] = 500
	delta := definitions["GripStockInput"].(schema)["properties"].(schema)["quantity_delta"].(schema)
	delta["minimum"] = -1000000
	delta["maximum"] = 1000000
	delta["not"] = schema{"const": 0}
	definitions["OrderCreate"].(schema)["properties"].(schema)["payment_method"].(schema)["enum"] = []string{"cash", "onsite", "online"}
	definitions["AuthRequest"].(schema)["properties"].(schema)["phone_number"].(schema)["maxLength"] = 40
	for _, name := range []string{"AuthVerify", "AuthChallenge"} {
		p := definitions[name].(schema)["properties"].(schema)["challenge_id"].(schema)
		p["pattern"] = "^[A-Za-z0-9_-]{43}$"
	}
	definitions["AuthVerify"].(schema)["properties"].(schema)["code"].(schema)["pattern"] = "^[0-9]{4,10}$"
	definitions["ManagementProfile"].(schema)["properties"].(schema)["membership_role"].(schema)["enum"] = []string{"owner", "staff", "admin"}
	definitions["OrderSummary"].(schema)["properties"].(schema)["booked_value_cents"].(schema)["maximum"] = 9007199254740991
	// Optional transient answers must never become required health disclosures.
	definitions["RecommendationRequest"].(schema)["required"] = []string{"experience"}
	for _, name := range []string{"Booking", "RecommendationRequest"} {
		definitions[name].(schema)["description"] = "Supply sport for a model-free booking, or a legacy racket_model_id. If both are supplied, they must match."
	}
	definitions["LotInput"].(schema)["required"] = []string{"kind", "quantity"}
	paths := schema{}
	add := func(method, path, id, summary, request, response string, private bool, params ...schema) {
		responses := schema{"200": schema{"description": "Success", "content": content(response)}}
		if method == "post" && path != "/stringers/{stringerID}/recommendations" && path != "/auth/phone/verify" && path != "/auth/logout" && path != "/orders/{orderID}/cash-receipt" && path != "/billing/stripe/webhook" && path != "/payments/stripe/webhook" && !strings.HasSuffix(path, "/read") && !strings.HasSuffix(path, "/tracking-updates") && !strings.HasSuffix(path, "/collection-request") && !strings.HasSuffix(path, "/stock-events") {
			delete(responses, "200")
			responses["201"] = schema{"description": "Created", "content": content(response)}
		}
		if method == "post" && (strings.HasSuffix(path, "/onsite-receipt") || path == "/orders" || strings.HasSuffix(path, "/messages") || strings.HasSuffix(path, "/collection-request") || strings.HasSuffix(path, "/tracking-updates") || path == "/admin/grips" || strings.HasSuffix(path, "/stock-events")) {
			if path == "/orders" {
				responses["200"] = schema{"description": "Idempotent replay", "content": content(response)}
			}
			params = append(params, schema{"name": "Idempotency-Key", "in": "header", "required": true, "schema": schema{"type": "string", "pattern": `^[A-Za-z0-9._:-]{16,100}$`}})
		}
		if strings.HasPrefix(path, "/auth/") && method == "post" {
			params = append(params, schema{"name": "Origin", "in": "header", "required": true, "schema": schema{"type": "string"}, "description": "Must exactly match configured FRONTEND_ORIGIN."})
			if path == "/auth/phone/verify" || path == "/auth/logout" {
				responses["200"].(schema)["headers"] = schema{"Set-Cookie": schema{"description": "Host-only HttpOnly SameSite=Lax session cookie; Secure under HTTPS. Logout expires it.", "schema": schema{"type": "string"}}}
			}
		}
		responses["default"] = schema{"description": "Error: 400 invalid request; 401 session; 403 permission/Origin; 404 missing or inaccessible resource; 409 stale quote, stock, idempotency or transition conflict; 415 content type; 422 invalid fields; 429 rate limit; 500 internal; 503 authentication temporarily unavailable. See error.code and optional fields.", "content": content("ErrorEnvelope")}
		if path == "/ready" {
			responses["503"] = schema{"description": "PostgreSQL unavailable or readiness deadline exceeded", "content": content("ErrorEnvelope")}
		}
		for _, part := range strings.Split(path, "/") {
			if strings.HasPrefix(part, "{") {
				name := strings.Trim(part, "{}")
				p := schema{"type": "string", "format": "uuid"}
				if name == "slot" {
					p = schema{"type": "string", "enum": []string{"profile", "banner"}}
				}
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
			if path == "/notifications/sms/status" || path == "/notifications/sms/inbound" {
				op["requestBody"] = schema{"required": true, "content": schema{"application/x-www-form-urlencoded": schema{"schema": ref("TwilioStatus")}}}
				params = append(params, schema{"name": "X-Twilio-Signature", "in": "header", "required": true, "schema": schema{"type": "string"}})
				op["parameters"] = params
				responses["204"] = schema{"description": "Callback acknowledged"}
				delete(responses, "200")
			}

		}
		if method == "get" && (strings.HasSuffix(path, "/images/{slot}") || strings.HasSuffix(path, "/{variantID}/photo")) {
			responses["200"] = schema{"description": "Sanitized PNG", "content": schema{"image/png": schema{"schema": schema{"type": "string", "format": "binary"}}}}
		}
		if _, ok := paths[path]; !ok {
			paths[path] = schema{}
		}
		paths[path].(schema)[method] = op
	}
	pagination := []schema{query("page", false, schema{"type": "integer", "minimum": 1, "maximum": 10000, "default": 1}), query("page_size", false, schema{"type": "integer", "minimum": 1, "maximum": 50, "default": 20})}
	gripSearch := append(append([]schema{}, pagination...), query("q", false, schema{"type": "string", "maxLength": 100}), query("brand", false, schema{"type": "string", "maxLength": 100}), query("kind", false, schema{"type": "string", "enum": []string{"overgrip", "replacement_grip"}}), query("grip_product_id", false, schema{"type": "string", "format": "uuid"}))
	add("get", "/grips", "gripCatalogue", "Active shared grip catalogue", "", "GripProductPage", false, gripSearch...)
	add("get", "/stringers/{stringerID}/grips", "offeredGrips", "Active offered grips with an unreserved unit; price includes fitting", "", "GripOfferingPage", false, gripSearch...)
	add("get", "/stringers/{stringerID}/manage/grips", "manageGrips", "Business members: grip prices, availability and unit balances", "", "GripStockPage", true, gripSearch...)
	add("put", "/stringers/{stringerID}/grips/{gripProductID}", "saveGripOffering", "Business members: replace fitted price and availability; stock unchanged", "GripOfferingInput", "GripStock", true)
	add("post", "/stringers/{stringerID}/grips/{gripProductID}/stock-events", "adjustGripStock", "Business members: idempotent audited whole-unit receipt/correction", "GripStockInput", "GripMovement", true)
	add("get", "/stringers/{stringerID}/grips/{gripProductID}/stock-events", "gripStockEvents", "Business members: stock ledger, newest first", "", "GripMovementPage", true, pagination...)
	add("get", "/admin/grips", "adminGripCatalogue", "Admins: include inactive catalogue grips", "", "GripProductPage", true, append(append([]schema{}, gripSearch...), query("active", false, schema{"type": "boolean"}))...)
	add("post", "/admin/grips", "createGripProduct", "Admins: idempotently create a shared grip product", "GripInput", "GripProduct", true)
	add("put", "/admin/grips/{gripProductID}", "updateGripProduct", "Admins: replace shared grip metadata; historical orders unchanged", "GripInput", "GripProduct", true)
	search := append(append([]schema{}, pagination...), query("q", false, schema{"type": "string", "maxLength": 100}))
	catalogue := append(append([]schema{}, search...), query("material", false, schema{"type": "string", "enum": []string{"multifilament", "synthetic_gut", "polyester", "natural_gut", "nylon", "hybrid"}}), query("brand", false, schema{"type": "string"}), query("sport", false, schema{"type": "string", "enum": []string{"tennis", "badminton", "squash"}}), query("gauge_mm", false, schema{"type": "number", "minimum": 0.4, "maximum": 2}), query("variant_id", false, schema{"type": "string", "format": "uuid"}))
	add("post", "/billing/stripe/webhook", "stripeBillingWebhook", "Verified Stripe snapshot webhook; raw-body signature required", "StripeWebhook", "WebhookResponse", false, schema{"name": "Stripe-Signature", "in": "header", "required": true, "schema": schema{"type": "string"}})
	add("get", "/stringers/{stringerID}/billing", "businessBilling", "Business members: Irish-month usage and subscription summary", "", "BusinessUsage", true)
	add("post", "/stringers/{stringerID}/billing/checkout", "subscriptionCheckout", "Owner/admin: start or resume subscription checkout", "EmptyInput", "RedirectURL", true)
	add("post", "/stringers/{stringerID}/billing/portal", "subscriptionPortal", "Owner/admin: manage existing billing", "EmptyInput", "RedirectURL", true)
	add("post", "/orders/{orderID}/cash-receipt", "confirmCashReceipt", "Business member/admin: idempotently confirm cash received", "EmptyInput", "Order", true)
	add("get", "/auth/config", "authConfig", "SMS availability and allowed calling codes", "", "AuthConfig", false)
	add("post", "/auth/phone/request", "requestPhoneCode", "Request a rate-limited SMS code; requires trusted Origin", "AuthRequest", "AuthChallenge", false)
	add("post", "/auth/phone/verify", "verifyPhoneCode", "Verify phone and issue HttpOnly session cookie; requires trusted Origin", "AuthVerify", "Me", false)
	add("post", "/auth/logout", "logout", "Revoke current session and clear cookie; requires trusted Origin", "", "LogoutResponse", false)
	add("get", "/health", "health", "Liveness", "", "Health", false)
	add("get", "/ready", "readiness", "Readiness: PostgreSQL reachable within two seconds; 503 otherwise", "", "Health", false)
	add("get", "/strings", "listStrings", "Search the shared string-variant catalogue", "", "VariantPage", false, append(append([]schema{}, catalogue...), query("individual_only", false, schema{"type": "boolean"}))...)

	add("get", "/stringers/{stringerID}/payment-options", "paymentOptions", "Payment choices offered by this business", "", "PaymentOptions", false)
	add("get", "/stringers/{stringerID}/manage/payment-options", "paymentSettings", "Members: payment setup status", "", "PaymentOptions", true)
	add("put", "/stringers/{stringerID}/manage/payment-options", "savePaymentSettings", "Owners: configure onsite and online payment options", "PaymentSettingsInput", "PaymentOptions", true)
	add("post", "/stringers/{stringerID}/payments/onboarding", "paymentOnboarding", "Owners: start hosted Stripe setup", "EmptyInput", "RedirectURL", true)
	add("post", "/orders/{orderID}/payment-checkout", "orderPaymentCheckout", "Customer: pay the saved order directly to the stringer", "EmptyInput", "RedirectURL", true)
	add("post", "/orders/{orderID}/onsite-receipt", "onsiteReceipt", "Members: record an onsite cash or card receipt", "OnsiteReceiptInput", "Order", true)
	add("post", "/payments/stripe/webhook", "racketPaymentWebhook", "Signed connected-account payment and refund events", "StripeWebhook", "WebhookResponse", false)
	add("get", "/rackets", "listRackets", "Search verified model-specific technical constraints", "", "RacketPage", false, append(append([]schema{}, search...), query("sport", false, schema{"type": "string"}))...)
	add("get", "/stringers", "listStringers", "Search approved stringers", "", "StringerPage", false, append(append([]schema{}, search...), query("sport", false, schema{"type": "string"}), query("county", false, schema{"type": "string"}))...)
	add("get", "/stringers/{stringerID}", "getStringer", "Read an approved public profile", "", "Stringer", false)
	add("get", "/stringers/{stringerID}/inventory", "listInventory", "Offered, enabled variants with unreserved stock; quote checks model-specific length", "", "VariantPage", false, catalogue...)
	add("get", "/stringers/{stringerID}/services", "listServices", "Read active service capabilities and prices", "", "ServiceList", false)
	add("get", "/stringers/{stringerID}/hybrid-inventory", "hybridInventory", "Bookable half-racket portions for one component; combinations are validated again at quote/order", "", "VariantPage", false, append(append([]schema{}, catalogue...), query("racket_model_id", false, schema{"type": "string", "format": "uuid"}), query("component", true, schema{"type": "string", "enum": []string{"mains", "crosses"}}))...)
	add("get", "/stringers/{stringerID}/booking-configuration", "bookingConfiguration", "Sport service limits; optional legacy catalogue model constraints", "", "BookingOptions", false, query("racket_model_id", false, schema{"type": "string", "format": "uuid"}), query("sport", false, schema{"type": "string", "enum": []string{"tennis", "badminton", "squash"}}))
	add("post", "/stringers/{stringerID}/recommendations", "recommendStrings", "Complete starting setups with explained rules, estimated prices and current offered stock", "RecommendationRequest", "Recommendations", false)
	add("get", "/me/profile", "getAccountProfile", "Read your own account profile", "", "AccountProfile", true)
	add("put", "/me/profile", "updateAccountProfile", "Update your own name and optional email", "AccountProfile", "AccountProfile", true)
	add("get", "/me", "getMe", "Read the verified session identity and account menu access", "", "SessionAccount", true)
	add("get", "/me/stringers", "myStringers", "Read own businesses, applications, membership and permissions", "", "ManagementProfileList", true)
	add("post", "/stringers", "applyStringer", "Submit an application and create owner membership", "ProfileInput", "Stringer", true)
	add("put", "/stringers/{stringerID}", "updateProfile", "Owner or admin: replace editable profile fields", "ProfileInput", "Stringer", true)
	add("get", "/stringers/{stringerID}/manage/profile", "managementProfile", "Private business profile, membership and effective permissions", "", "ManagementProfile", true)
	add("get", "/stringers/{stringerID}/manage/summary", "orderSummary", "Members or admin: all-time booking counts and non-cancelled booking value, not revenue", "", "OrderSummary", true)
	add("get", "/stringers/{stringerID}/manage/services", "manageServices", "Members or admin: include inactive services", "", "ServiceList", true)
	add("put", "/stringers/{stringerID}/services/{sport}", "saveService", "Owner or admin: replace service policy and pre-stretch options", "ServiceInput", "Service", true)
	add("get", "/stringers/{stringerID}/manage/inventory", "manageInventory", "Members or admin: include disabled offerings", "", "VariantPage", true, catalogue...)
	add("put", "/stringers/{stringerID}/inventory/{variantID}", "saveInventory", "Members or admin: set full-bed material price, availability and optional stock tracking", "InventoryInput", "IDResponse", true)
	add("get", "/stringers/{stringerID}/inventory/lots", "listLots", "Members or admin: physical and reserved quantities", "", "LotPage", true, append(append([]schema{}, search...), query("sport", false, schema{"type": "string", "enum": []string{"tennis", "badminton", "squash"}}), query("variant_id", false, schema{"type": "string", "format": "uuid"}))...)
	add("post", "/stringers/{stringerID}/inventory/{variantID}/lots", "addLot", "Receive sets, reel millimetres or whole half-racket portions", "LotInput", "Lot", true)
	add("post", "/stringers/{stringerID}/inventory/lots/{lotID}/adjustments", "adjustLot", "Audited stock adjustment; cannot remove reservations", "AdjustmentInput", "Lot", true)
	add("post", "/stringers/{stringerID}/quotes", "createQuote", "Validate and persist a 10-minute server quotation; no stock hold yet", "Booking", "Quote", true)
	add("get", "/stringers/{stringerID}/checkout-disclosure", "checkoutDisclosure", "Reviewed checkout disclosure; inactive until explicitly enabled", "", "CheckoutDisclosure", false)
	add("post", "/admin/stringers/{stringerID}/legal-versions", "publishLegalVersion", "Publish supplied, reviewed contractual wording", "LegalDocument", "CheckoutDisclosure", true)
	add("get", "/orders/{orderID}/confirmation", "bookingConfirmation", "Immutable, access-controlled booking and contractual snapshot", "", "BookingConfirmation", true)
	add("post", "/orders", "createOrder", "Reprice quote and atomically reserve stock; payment remains unpaid", "OrderCreate", "Order", true)
	orderFilters := append(append([]schema{}, pagination...), query("status", false, schema{"type": "string", "enum": []string{"placed", "accepted", "received", "in_progress", "ready", "completed", "cancelled"}}))
	add("get", "/admin/compliance-history/{resourceID}", "complianceHistory", "Administrator append-only decision history; fixed pages of 50", "", "ComplianceHistory", true, query("kind", true, schema{"type": "string", "enum": []string{"privacy_request", "trader", "case"}}), query("page", false, schema{"type": "integer", "minimum": 1, "maximum": 10000}))
	add("post", "/stringers/{stringerID}/financial-events", "recordFinancial", "Owner/admin: append an attested actual amount; idempotent and audited", "FinancialInput", "FinancialEvent", true)
	add("get", "/stringers/{stringerID}/financial-events", "financialLedger", "Owner/admin: immutable financial facts, newest first", "", "FinancialLedger", true, pagination...)
	add("get", "/admin/stringers/{stringerID}/financial-report", "financialReport", "Admin: internal quarter facts, no inferred cash receipts or DAC7 filing", "", "FinancialReport", true, query("year", true, schema{"type": "integer", "minimum": 2000, "maximum": 2100}), query("quarter", true, schema{"type": "integer", "minimum": 1, "maximum": 4}))
	add("post", "/content-notices", "contentNotice", "Public unlawful-content notice; exact location, grounds, contact, good faith and Idempotency-Key", "CaseInput", "CaseReceipt", false)
	add("get", "/stringers/{stringerID}/reviews", "publicReviews", "Rating and recent reviews once five distinct eligible customers have reviewed", "", "PublicReviews", false)
	add("get", "/orders/{orderID}/review", "ownBookingReview", "Completed booking customer: view own review", "", "OwnReview", true)
	add("put", "/orders/{orderID}/review", "saveBookingReview", "Completed booking customer: create or edit one review", "ReviewInput", "OwnReview", true)
	add("put", "/orders/{orderID}/review/withdrawal", "withdrawBookingReview", "Completed booking customer: remove own review", "", "ReadResult", true)
	add("put", "/stringers/{stringerID}/reviews/{reviewID}/report", "reportBookingReview", "Business members: report a review for moderation", "ReviewReportInput", "ReadResult", true)
	add("get", "/admin/reviews", "reportedBookingReviews", "Administrators: reported reviews", "", "ReportedReviewPage", true, pagination...)
	add("patch", "/admin/reviews/{reviewID}", "moderateBookingReview", "Administrators: keep or hide a reported review", "ReviewDecision", "ReadResult", true)
	add("get", "/me/cases", "myCases", "Private own reports and outcomes", "", "CaseList", true, pagination...)
	add("get", "/admin/cases", "caseQueue", "Administrator complaint queue", "", "CaseList", true, pagination...)
	add("post", "/me/cases", "submitCase", "Authenticated listing/order complaint", "CaseInput", "CaseReceipt", true)
	add("patch", "/admin/cases/{caseID}", "reviewCase", "Reasoned administrator decision; independent appeal reviewer", "CaseAction", "CaseDecision", true)
	add("post", "/cases/{caseID}/review", "appealCase", "Reporter or affected business owner: request review once", "CaseAction", "CaseDecision", true)
	add("get", "/stringers/{stringerID}/decisions", "businessDecisions", "Owner/admin decisions without reporter private data", "", "DecisionList", true)
	add("get", "/me/case-notifications", "caseInbox", "Own in-app case updates", "", "CaseInbox", true)
	add("get", "/admin/traders", "traderQueue", "Administrator private verification queue", "", "TraderList", true, pagination...)
	add("get", "/stringers/{stringerID}/trader", "traderRecord", "Owner/admin private verification record", "", "TraderRecord", true)
	add("put", "/stringers/{stringerID}/trader", "submitTrader", "Owner/admin version-checked identity submission", "TraderSubmission", "TraderRecord", true)
	add("patch", "/admin/stringers/{stringerID}/trader", "reviewTrader", "Administrator identity/contact/registration review", "TraderReview", "TraderRecord", true)
	add("get", "/stringers/{stringerID}/identity", "publicTrader", "Verified public identity only; null if not yet reviewed", "", "PublicTraderIdentity", false)
	add("get", "/me/privacy-requests", "privacyRequests", "Own privacy requests and response deadlines", "", "PrivacyList", true, pagination...)
	add("post", "/me/privacy-requests", "requestPrivacy", "Submit an authenticated rights request with an Idempotency-Key", "PrivacyInput", "PrivacyRequest", true)
	add("get", "/admin/privacy-requests", "privacyQueue", "Administrator privacy request queue", "", "PrivacyList", true, pagination...)
	add("patch", "/admin/privacy-requests/{requestID}", "reviewPrivacy", "Version-checked identity and retention review", "PrivacyReview", "PrivacyRequest", true)
	add("get", "/admin/privacy-requests/{requestID}/export-preview", "previewPrivacyExport", "Administrator: review minimized export and supplementary records", "", "PrivacyExport", true)
	add("get", "/me/privacy-requests/{requestID}/export", "privacyExport", "Own reviewed, released export", "", "PrivacyExport", true)
	add("get", "/orders/saved-rackets", "savedRackets", "Latest booking setup per named customer racket", "", "SavedRacketList", true)
	add("get", "/orders", "myOrders", "Customer's order history", "", "OrderPage", true, orderFilters...)
	add("get", "/orders/{orderID}/tracking", "orderTracking", "Order participants: current collection agreement and effective permissions", "", "OrderTracking", true)
	add("get", "/orders/{orderID}/tracking-updates", "trackingUpdates", "Order participants: collection history, newest version first", "", "TrackingUpdatePage", true, pagination...)
	add("post", "/orders/{orderID}/collection-request", "requestCollection", "Order customer: request Irish collection date; version checked, idempotent", "CollectionRequest", "TrackingUpdate", true)
	add("post", "/orders/{orderID}/tracking-updates", "updateTracking", "Business member/admin: estimate, instructions and issue flag; version checked, idempotent", "TrackingInput", "TrackingUpdate", true)
	add("get", "/orders/{orderID}/messages", "orderMessages", "Order participants: newest messages and own unread count", "", "MessagePage", true, pagination...)
	add("post", "/orders/{orderID}/messages", "sendOrderMessage", "Order participants: plain text, idempotent, per-user rate limit", "MessageInput", "OrderMessage", true)
	add("post", "/orders/{orderID}/messages/read", "readOrderMessages", "Mark through a returned message ID monotonically", "MessageReadInput", "ReadResult", true)
	add("get", "/me/notification-preferences", "notificationPreferences", "Own SMS opt-in preference", "", "NotificationPreferences", true)
	add("put", "/me/notification-preferences", "saveNotificationPreferences", "Set own SMS opt-in preference", "NotificationPreferences", "NotificationPreferences", true)
	add("post", "/notifications/sms/inbound", "jobSMSInbound", "Twilio signed Advanced Opt-Out callback", "TwilioStatus", "", false)
	add("post", "/notifications/sms/status", "jobSMSStatus", "Twilio signed delivery callback", "TwilioStatus", "", false, query("delivery", true, schema{"type": "string", "format": "uuid"}))
	add("get", "/me/staff-invitations", "staffInvitations", "Invitations matching current verified phone", "", "StaffInvitationList", true)
	add("post", "/me/staff-invitations/{invitationID}/accept", "acceptStaffInvitation", "Accept own pending staff invitation", "EmptyInput", "ReadResult", true)
	add("get", "/stringers/{stringerID}/team", "team", "Owner/admin team and pending invitations", "", "Team", true)
	add("post", "/stringers/{stringerID}/staff-invitations", "inviteStaff", "Owner/admin invites a verified phone to staff", "StaffInviteInput", "ReadResult", true)
	add("delete", "/stringers/{stringerID}/staff-invitations/{invitationID}", "revokeStaffInvitation", "Owner/admin revokes invitation", "", "ReadResult", true)
	add("delete", "/stringers/{stringerID}/team/{userID}", "removeStaff", "Owner/admin removes staff; owners protected", "", "ReadResult", true)
	add("get", "/strings/{variantID}/photo", "stringVariantPhoto", "Exact colour variant photo for active strings", "", "", false)
	add("get", "/admin/string-variants/{variantID}/photo", "adminVariantPhoto", "Preview a variant photo including inactive variants", "", "", true)
	add("put", "/admin/string-variants/{variantID}/photo", "saveVariantPhoto", "Upload a PNG or JPEG for the current variant colour", "VariantPhotoInput", "ReadResult", true)
	add("delete", "/admin/string-variants/{variantID}/photo", "deleteVariantPhoto", "Remove the variant photo", "", "ReadResult", true)
	add("get", "/stringers/{stringerID}/images/{slot}", "businessImage", "Approved public profile/banner PNG", "", "", false)
	add("put", "/stringers/{stringerID}/images/{slot}", "saveBusinessImage", "Owner/admin uploads base64 PNG/JPEG, 1 MB decoded, 4 megapixels", "BusinessImageInput", "BusinessImage", true)
	add("delete", "/stringers/{stringerID}/images/{slot}", "deleteBusinessImage", "Owner/admin deletes profile/banner", "", "ReadResult", true)
	add("get", "/notifications", "notifications", "Own in-app inbox; current order access required", "", "NotificationPage", true, pagination...)
	add("post", "/notifications/{notificationID}/read", "readNotification", "Mark own accessible notification read, idempotently", "EmptyInput", "ReadResult", true)
	add("get", "/orders/{orderID}", "getOrder", "Customer, business members or admin; includes status history", "", "Order", true)
	add("get", "/stringers/{stringerID}/orders/{orderID}", "incomingOrder", "Business members or admin: order details and fulfilment contact; scoped to this business", "", "IncomingOrder", true)
	add("get", "/stringers/{stringerID}/orders", "incomingOrders", "Business members or admin: incoming orders", "", "OrderPage", true, orderFilters...)
	add("patch", "/orders/{orderID}/status", "updateStatus", "Validate workflow; customers may cancel their placed orders", "StatusInput", "Order", true)
	add("get", "/admin/stringer-applications", "applications", "Admin: pending application queue", "", "StringerPage", true, pagination...)
	add("patch", "/admin/stringers/{stringerID}/approval", "reviewStringer", "Admin: approve, reject or suspend with audit history", "ApprovalInput", "Stringer", true)
	adminState := query("active", false, schema{"type": "boolean"})
	curation := append(append([]schema{}, search...), query("brand", false, schema{"type": "string"}), query("sport", false, schema{"type": "string", "enum": []string{"tennis", "badminton", "squash"}}), query("material", false, schema{"type": "string", "enum": []string{"multifilament", "synthetic_gut", "polyester", "natural_gut", "nylon", "hybrid"}}))
	add("get", "/admin/string-products", "catalogueProducts", "Admin: search all products including inactive", "", "ProductPage", true, append(append([]schema{}, curation...), adminState)...)
	add("get", "/admin/string-products/{productID}", "catalogueProduct", "Admin: read a product including inactive", "", "Product", true)
	add("get", "/admin/string-variants", "catalogueVariants", "Admin: search variants including inactive products and variants", "", "CatalogueVariantPage", true, append(append([]schema{}, curation...), adminState, query("product_id", false, schema{"type": "string", "format": "uuid"}), query("gauge_mm", false, schema{"type": "number", "minimum": 0.4, "maximum": 2}))...)
	add("get", "/admin/string-variants/{variantID}", "catalogueVariant", "Admin: read a variant including inactive", "", "CatalogueVariant", true)
	add("get", "/admin/rackets", "catalogueRackets", "Admin: search all racket models including unverified", "", "RacketPage", true, append(append([]schema{}, search...), query("sport", false, schema{"type": "string", "enum": []string{"tennis", "badminton", "squash"}}), query("brand", false, schema{"type": "string"}), query("verified", false, schema{"type": "boolean"}))...)
	add("get", "/admin/rackets/{racketID}", "catalogueRacket", "Admin: read a racket including unverified", "", "Racket", true)

	for _, v := range []struct{ path, name, input, output, param string }{{"string-products", "Product", "ProductInput", "Product", "productID"}, {"string-variants", "Variant", "VariantInput", "IDResponse", "variantID"}, {"rackets", "Racket", "RacketInput", "Racket", "racketID"}} {
		add("post", "/admin/"+v.path, "create"+v.name, "Admin: create curated "+v.name, v.input, v.output, true)
		add("put", "/admin/"+v.path+"/{"+v.param+"}", "update"+v.name, "Admin: replace curated "+v.name, v.input, v.output, true)
	}
	doc := schema{"openapi": "3.1.0", "info": schema{"title": "Restringing.ie Marketplace API", "version": "0.1.0", "description": "Money: integer EUR cents. Tension: integer hundredths of kilograms or pounds. Pound bookings set the legacy kilogram fields to zero. Reel quantities: integer millimetres. Session tokens are issued only after phone OTP verification, via the configured SMS provider. Cookie writes require the configured frontend Origin. Catalogue ratings are editorial, not medical claims. See docs/engineering/backend-marketplace.md for validation, limits, examples and deployment prerequisites."}, "servers": []schema{{"url": "http://localhost:8081/api/v1"}}, "paths": paths, "components": schema{"schemas": definitions, "securitySchemes": schema{"sessionCookie": schema{"type": "apiKey", "in": "cookie", "name": "session_token"}, "bearerSession": schema{"type": "http", "scheme": "bearer", "bearerFormat": "32 random bytes encoded as unpadded base64url"}}}}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(doc); err != nil {
		panic(err)
	}
}
