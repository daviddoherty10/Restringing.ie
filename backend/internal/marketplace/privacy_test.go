package marketplace

import (
	"context"
	"testing"
)

func TestIntegrationPrivacyIdentityAccessRetriesAndClosure(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	in := PrivacyInput{Kind: "access", Details: "Account access test"}
	p, e := f.store.RequestPrivacy(ctx, f.customer, in, "privacy-access-test-key")
	if e != nil {
		t.Fatal(e)
	}
	again, e := f.store.RequestPrivacy(ctx, f.customer, in, "privacy-access-test-key")
	if e != nil || again.ID != p.ID {
		t.Fatal("retry", e)
	}
	if _, e = f.store.PrivacyExport(ctx, f.other, p.ID, false); e == nil {
		t.Fatal("cross customer export")
	}
	if _, e = f.store.PrivacyList(ctx, f.customer, true, 0); e == nil {
		t.Fatal("nonadmin queue")
	}
	if _, e = f.store.ReviewPrivacy(ctx, f.admin, p.ID, PrivacyReview{Version: 1, Status: "fulfilled", Outcome: "Reviewed"}); e == nil {
		t.Fatal("identity omitted")
	}
	if _, e = f.store.PrivacyExport(ctx, f.admin, p.ID, true); e != nil {
		t.Fatal(e)
	}
	_, e = f.store.ReviewPrivacy(ctx, f.admin, p.ID, PrivacyReview{Version: 1, Status: "fulfilled", Outcome: "Reviewed minimized export; supplementary sources checked", IdentityVerified: true, ExportReviewed: true})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.store.PrivacyExport(ctx, f.customer, p.ID, false); e != nil {
		t.Fatal(e)
	}
	if _, e = f.store.Pool.Exec(ctx, `INSERT INTO sessions(user_id,token_hash,expires_at) VALUES($1,decode(repeat('aa',32),'hex'),now()+interval '7 days')`, f.other.ID); e != nil {
		t.Fatal(e)
	}
	erase, e := f.store.RequestPrivacy(ctx, f.other, PrivacyInput{Kind: "erasure", Details: "Close account"}, "privacy-erasure-test-key")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.store.ReviewPrivacy(ctx, f.admin, erase.ID, PrivacyReview{Version: 1, Status: "fulfilled", Outcome: "Closed; reviewed records retention", IdentityVerified: true, RetentionBasis: "TEST request-accountability record; no orders", RetainUntil: "2030-01-01"}); e != nil {
		t.Fatal(e)
	}
	var activeSessions int
	if e = f.store.Pool.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE user_id=$1 AND revoked_at IS NULL`, f.other.ID).Scan(&activeSessions); e != nil || activeSessions != 0 {
		t.Fatal("closure left active sessions", e)
	}
	var status, phone string
	f.store.Pool.QueryRow(ctx, `SELECT status,phone_number FROM users WHERE id=$1`, f.other.ID).Scan(&status, &phone)
	if status != "closed" || phone != "closed:"+f.other.ID {
		t.Fatal("not anonymised")
	}
}
