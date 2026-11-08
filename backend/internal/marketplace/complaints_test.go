package marketplace

import (
	"context"
	"testing"
)

func TestIntegrationCasePrivacyAppealAndPublicNotice(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	in := CaseInput{Kind: "listing", StringerID: f.shop.ID, Location: "TEST listing location", Details: "TEST disputed listing facts", GoodFaith: true}
	r, e := f.store.SubmitCase(ctx, f.customer, in, "complaint-listing-test-key", false)
	if e != nil {
		t.Fatal(e)
	}
	again, e := f.store.SubmitCase(ctx, f.customer, in, "complaint-listing-test-key", false)
	if e != nil || again.ID != r.ID {
		t.Fatal("retry", e)
	}
	other, e := f.store.CaseList(ctx, f.other, false, 1)
	if e != nil || len(other) != 0 {
		t.Fatal("private report leaked", e)
	}
	if _, e = f.store.ActCase(ctx, f.customer, r.ID, CaseAction{Version: 1, Status: "resolved", Reason: "unauthorised"}, false); e == nil {
		t.Fatal("customer decision")
	}
	d, e := f.store.ActCase(ctx, f.admin, r.ID, CaseAction{Version: 1, Status: "resolved", Reason: "TEST reviewed; contact support for next steps"}, false)
	if e != nil {
		t.Fatal(e)
	}
	inbox, e := f.store.CaseInbox(ctx, f.customer)
	if e != nil || len(inbox) != 1 {
		t.Fatal("notification", e)
	}
	d, e = f.store.ActCase(ctx, f.customer, r.ID, CaseAction{Version: d.Version, Reason: "Please independently review TEST evidence"}, true)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.store.ActCase(ctx, f.admin, r.ID, CaseAction{Version: d.Version, Status: "resolved", Reason: "same reviewer"}, false); e == nil {
		t.Fatal("same appeal reviewer")
	}
	notice := CaseInput{Kind: "unlawful_content", Location: "TEST exact content location", Details: "TEST grounds", Contact: "test@example.invalid", GoodFaith: true}
	if _, e = f.store.SubmitCase(ctx, Actor{}, notice, "public-notice-test-key", true); e != nil {
		t.Fatal(e)
	}
	if _, e = f.store.SubmitCase(ctx, Actor{}, in, "public-listing-invalid-key", true); e == nil {
		t.Fatal("public private complaint")
	}
}
