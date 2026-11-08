package marketplace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersApplyToPublicErrorsAndPrivateBoundary(t *testing.T) {
	handler := Router(&Store{}, "https://app.example.invalid")
	for _, path := range []string{"/api/v1/health", "/api/v1/me/privacy-requests", "/api/v1/missing"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		for _, name := range []string{"X-Content-Type-Options", "Referrer-Policy", "Content-Security-Policy", "Permissions-Policy", "Cache-Control"} {
			if w.Header().Get(name) == "" {
				t.Fatalf("%s missing %s", path, name)
			}
		}
	}
}
func TestIntegrationPrivacyExportNeverContainsCredentialHashesAndLegalReviewStaysPrivate(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	raw, e := privacyExport(ctx, f.store.Pool, f.customer.ID)
	if e != nil {
		t.Fatal(e)
	}
	for _, secret := range []string{"token_hash", "provider_sid", "request_key", "challenge_id"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("credential field in export", secret)
		}
	}
	f.store.EnforceLegal = true
	if _, e = f.store.PublishLegal(ctx, f.admin, f.shop.ID, testLegalDocument()); e != nil {
		t.Fatal(e)
	}
	d, e := f.store.CheckoutDisclosure(ctx, f.shop.ID)
	if e != nil {
		t.Fatal(e)
	}
	if d.Document.ReviewReference != "" {
		t.Fatal("internal review reference exposed")
	}
}
func TestIntegrationComplianceHistoryIsPrivateAndAuditsCannotBeRewritten(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	r, e := f.store.RequestPrivacy(ctx, f.customer, PrivacyInput{Kind: "access", Details: "TEST history request"}, "history-request-test-key")
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.store.ReviewPrivacy(ctx, f.admin, r.ID, PrivacyReview{Version: 1, Status: "in_review", Outcome: "TEST review started"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.store.ComplianceHistory(ctx, f.customer, "privacy_request", r.ID, 1); e == nil {
		t.Fatal("nonadmin history access")
	}
	history, e := f.store.ComplianceHistory(ctx, f.admin, "privacy_request", r.ID, 1)
	if e != nil || len(history) != 1 || history[0].Reason != "TEST review started" {
		t.Fatal(history, e)
	}
	if _, e = f.store.Pool.Exec(ctx, `UPDATE privacy_decision_events SET reason='changed' WHERE request_id=$1`, r.ID); e == nil {
		t.Fatal("rewrote decision history")
	}
	if _, e = f.store.Pool.Exec(ctx, `DELETE FROM compliance_audit WHERE resource_id=$1`, r.ID); e == nil {
		t.Fatal("deleted audit")
	}
	d := TraderDocument{BusinessType: "sole_trader|registered_business", Declaration: true}
	if _, e = f.store.SubmitTrader(ctx, f.owner, f.shop.ID, TraderSubmission{Document: d}); e == nil {
		t.Fatal("combined category accepted")
	}
}
func TestIntegrationCorrectionPreservesUnselectedAccountFields(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, e := f.store.Pool.Exec(ctx, `UPDATE users SET first_name='TEST first',last_name='TEST last' WHERE id=$1`, f.customer.ID)
	if e != nil {
		t.Fatal(e)
	}
	r, e := f.store.RequestPrivacy(ctx, f.customer, PrivacyInput{Kind: "correction", Details: "Correct only my email"}, "correction-fields-test-key")
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.store.ReviewPrivacy(ctx, f.admin, r.ID, PrivacyReview{Version: 1, Status: "fulfilled", Outcome: "TEST email correction reviewed", IdentityVerified: true, Email: "corrected@example.invalid", CorrectionFields: []string{"email"}})
	if e != nil {
		t.Fatal(e)
	}
	var first, last, email string
	e = f.store.Pool.QueryRow(ctx, `SELECT first_name,last_name,email FROM users WHERE id=$1`, f.customer.ID).Scan(&first, &last, &email)
	if e != nil || first != "TEST first" || last != "TEST last" || email != "corrected@example.invalid" {
		t.Fatal("unselected account fields changed", e)
	}
}
