package marketplace

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func testLegalDocument() LegalDocument {
	return LegalDocument{Version: "TEST-ONLY-v1", Trader: PublicTraderIdentity{Name: "Fictional test trader", Address: "Test address", Email: "trader@example.invalid", Phone: "+447700900111", TraderStatus: "TEST trader"}, Terms: "TEST ONLY terms, not operative legal wording", CancellationNotice: "TEST ONLY cancellation notice", PriceNotice: "TEST ONLY tax disclosure", Turnaround: "TEST ONLY turnaround", AcceptanceLabel: "TEST acceptance", EarlyRequestLabel: "TEST request early work", EarlyAcknowledgementLabel: "TEST acknowledge consequences", SubmitLabel: "TEST booking button", WithdrawalDays: 14, Reviewed: true, ReviewReference: "TEST fixture review"}
}
func TestIntegrationContractualAcknowledgementsAndImmutableConfirmation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.store.EnforceLegal = true
	doc, err := f.store.PublishLegal(ctx, f.admin, f.shop.ID, testLegalDocument())
	if err != nil {
		t.Fatal(err)
	}
	q, err := f.store.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = f.store.CreateOrder(ctx, f.customer, q.ID, "legal-missing-ack-key"); err == nil {
		t.Fatal("API accepted missing acknowledgement")
	}
	a := &BookingAcceptance{VersionID: doc.VersionID, Accepted: true, EarlyRequest: true}
	if _, _, err = f.store.CreateOrder(ctx, f.customer, q.ID, "legal-missing-early-key", a); err == nil {
		t.Fatal("missing separate early acknowledgement accepted")
	}
	a.EarlyAcknowledgement = true
	o, replay, err := f.store.CreateOrder(ctx, f.customer, q.ID, "legal-exact-retry-key", a)
	if err != nil || replay {
		t.Fatal(err)
	}
	c, err := f.store.Confirmation(ctx, f.customer, o.ID)
	if err != nil || !strings.Contains(string(c.Snapshot), "TEST-ONLY-v1") {
		t.Fatal("confirmation missing disclosed version", err)
	}
	if _, err = f.store.Confirmation(ctx, f.other, o.ID); err == nil {
		t.Fatal("another customer read contract")
	}
	if _, err = f.store.Pool.Exec(ctx, `UPDATE booking_confirmations SET snapshot='{}' WHERE order_id=$1`, o.ID); err == nil {
		t.Fatal("contract confirmation mutable")
	}
	second := testLegalDocument()
	second.Version = "TEST-ONLY-v2"
	if _, err = f.store.PublishLegal(ctx, f.admin, f.shop.ID, second); err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.Pool.Exec(ctx, `UPDATE quotes SET created_at=now()-interval '2 days',expires_at=now()-interval '1 day' WHERE id=$1`, q.ID); err != nil {
		t.Fatal(err)
	}
	again, replay, err := f.store.CreateOrder(ctx, f.customer, q.ID, "legal-exact-retry-key", a)
	if err != nil || !replay || again.ID != o.ID {
		t.Fatal("committed retry broken by expiry/version change", err)
	}
	changed := *a
	changed.EarlyRequest = false
	changed.EarlyAcknowledgement = false
	if _, _, err = f.store.CreateOrder(ctx, f.customer, q.ID, "legal-exact-retry-key", &changed); err == nil {
		t.Fatal("retry changed acknowledgements")
	}
	old, err := f.store.Confirmation(ctx, f.customer, o.ID)
	if err != nil || string(old.Snapshot) != string(c.Snapshot) {
		t.Fatal("historical confirmation changed", err)
	}
}
func TestIntegrationChangedTermsAndExpiredLegalQuotesFailWithoutOrders(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.store.EnforceLegal = true
	v, e := f.store.PublishLegal(ctx, f.admin, f.shop.ID, testLegalDocument())
	if e != nil {
		t.Fatal(e)
	}
	q, e := f.store.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if e != nil {
		t.Fatal(e)
	}
	next := testLegalDocument()
	next.Version = "TEST v2"
	if _, e = f.store.PublishLegal(ctx, f.admin, f.shop.ID, next); e != nil {
		t.Fatal(e)
	}
	if _, _, e = f.store.CreateOrder(ctx, f.customer, q.ID, "legal-version-change-key", &BookingAcceptance{VersionID: v.VersionID, Accepted: true}); e == nil {
		t.Fatal("old terms version accepted")
	}
	var n int
	f.store.Pool.QueryRow(ctx, `SELECT count(*) FROM orders`).Scan(&n)
	if n != 0 {
		t.Fatal("failed legal checks left orders or reservations")
	}
	fresh, e := f.store.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if e != nil {
		t.Fatal(e)
	}
	f.store.Pool.Exec(ctx, `UPDATE quotes SET created_at=now()-interval '2 days',expires_at=now()-interval '1 minute' WHERE id=$1`, fresh.ID)
	if _, _, e = f.store.CreateOrder(ctx, f.customer, fresh.ID, "legal-expired-quote-key", &BookingAcceptance{VersionID: v.VersionID, Accepted: true}); e == nil {
		t.Fatal("expired legal quote accepted")
	}
}
func TestIntegrationEarlyPerformanceIsOptionalButEnforcedAtStart(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.store.EnforceLegal = true
	v, e := f.store.PublishLegal(ctx, f.admin, f.shop.ID, testLegalDocument())
	if e != nil {
		t.Fatal(e)
	}
	q, e := f.store.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if e != nil {
		t.Fatal(e)
	}
	o, _, e := f.store.CreateOrder(ctx, f.customer, q.ID, "legal-waiting-period-key", &BookingAcceptance{VersionID: v.VersionID, Accepted: true})
	if e != nil {
		t.Fatal(e)
	}
	for _, status := range []string{"accepted", "received"} {
		if _, e = f.store.UpdateStatus(ctx, f.owner, o.ID, status); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = f.store.UpdateStatus(ctx, f.owner, o.ID, "in_progress"); e == nil {
		t.Fatal("work started without early authorisation")
	}
}
func TestLegalDocumentPublishingRequiresAdminAndReviewedWording(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, e := f.store.PublishLegal(ctx, f.owner, f.shop.ID, testLegalDocument()); e == nil {
		t.Fatal("owner self-published legal approval")
	}
	d := testLegalDocument()
	d.Reviewed = false
	if e := validateLegalDocument(d); e == nil {
		t.Fatal("unreviewed legal text accepted")
	}
	r := f.request(t, "POST", "/admin/stringers/"+f.shop.ID+"/legal-versions", d, f.customer, nil)
	wantStatus(t, r, 403)
	var raw map[string]any
	json.Unmarshal(r.Body.Bytes(), &raw)
}
