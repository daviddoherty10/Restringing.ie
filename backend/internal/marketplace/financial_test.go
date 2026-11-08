package marketplace

import (
	"context"
	"testing"
	"time"
)

func TestIntegrationActualCashFactsAndReportDoNotInferReceipts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	q, e := f.store.CreateQuote(ctx, f.customer, f.shop.ID, f.booking)
	if e != nil {
		t.Fatal(e)
	}
	o, _, e := f.store.CreateOrder(ctx, f.customer, q.ID, "financial-order-test-key")
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	year, month, _ := now.Date()
	quarter := (int(month)-1)/3 + 1
	report, e := f.store.FinancialSummary(ctx, f.admin, f.shop.ID, year, quarter)
	if e != nil || report.CashReceivedCents != 0 || report.BookedOrders != 1 {
		t.Fatal("inferred receipt", report, e)
	}
	in := FinancialInput{OrderID: o.ID, Kind: "cash_received", AmountCents: 2000, OccurredAt: now, Reference: "TEST cash attestation", Reason: "TEST actual amount", Attested: true}
	if _, e = f.store.RecordFinancial(ctx, f.other, f.shop.ID, in, "financial-other-test-key"); e == nil {
		t.Fatal("unrelated business access")
	}
	fact, e := f.store.RecordFinancial(ctx, f.owner, f.shop.ID, in, "financial-cash-test-key")
	if e != nil {
		t.Fatal(e)
	}
	retry, e := f.store.RecordFinancial(ctx, f.owner, f.shop.ID, in, "financial-cash-test-key")
	if e != nil || retry.ID != fact.ID {
		t.Fatal("retry", e)
	}
	in.AmountCents = 1900
	if _, e = f.store.RecordFinancial(ctx, f.owner, f.shop.ID, in, "financial-cash-test-key"); e == nil {
		t.Fatal("altered retry")
	}
	in.Kind = "cash_refund"
	in.RelatedEventID = fact.ID
	in.AmountCents = 2100
	if _, e = f.store.RecordFinancial(ctx, f.owner, f.shop.ID, in, "financial-refund-test-key"); e == nil {
		t.Fatal("excess refund")
	}
	in.AmountCents = 500
	if _, e = f.store.RecordFinancial(ctx, f.owner, f.shop.ID, in, "financial-refund-test-key"); e != nil {
		t.Fatal(e)
	}
	report, e = f.store.FinancialSummary(ctx, f.admin, f.shop.ID, year, quarter)
	if e != nil || report.CashReceivedCents != 2000 || report.CashRefundCents != 500 || report.ReportedReceiptOrders != 1 {
		t.Fatal(report, e)
	}
	if _, e = f.store.FinancialSummary(ctx, f.owner, f.shop.ID, year, quarter); e == nil {
		t.Fatal("nonadmin report")
	}
	if _, e = f.store.Pool.Exec(ctx, `UPDATE seller_financial_events SET amount_cents=1 WHERE id=$1`, fact.ID); e == nil {
		t.Fatal("rewrote immutable fact")
	}
}
