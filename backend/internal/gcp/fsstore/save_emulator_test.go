package fsstore

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/firestore"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/receipt"
	"github.com/smartlife-lab/mimi/backend/internal/recurring"
)

// Test chạy với Firestore emulator: FIRESTORE_EMULATOR_HOST=127.0.0.1:8085 go test ./internal/gcp/fsstore/
func emulator(t *testing.T) (*Store, *firestore.Client, string) {
	t.Helper()
	if os.Getenv("FIRESTORE_EMULATOR_HOST") == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST chưa đặt")
	}
	c, err := firestore.NewClient(context.Background(), "mimi-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return New(c), c, fmt.Sprintf("t%d", time.Now().UnixNano())
}

func mustSet(t *testing.T, ref *firestore.DocumentRef, v map[string]any) {
	t.Helper()
	if _, err := ref.Set(context.Background(), v); err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, ref *firestore.DocumentRef) map[string]any {
	t.Helper()
	snap, err := ref.Get(context.Background())
	if err != nil {
		t.Fatalf("%s: %v", ref.Path, err)
	}
	return snap.Data()
}

func vnd(v int64) *domain.VND { x := domain.VND(v); return &x }

func milkLine(id string, ml float64) receipt.Line {
	return receipt.Line{
		ID: id, Name: "Vinamilk Sữa tươi", Qty: 2, VND: vnd(70_000), CategoryKey: "thuc-pham", GroupKey: "sua-tuoi", Habit: true,
		Product:   receipt.Product{Brand: "Vinamilk", DetailName: "Vinamilk Sữa tươi 900ml", CanonicalQty: ml, CanonicalUnit: "ml"},
		Structure: "2 x 900ml",
	}
}

func TestSaveReceiptEmulator(t *testing.T) {
	s, c, uid := emulator(t)
	ctx := context.Background()
	u := c.Collection("users").Doc(uid)
	mustSet(t, u, map[string]any{"name": "A"})
	mustSet(t, u.Collection("plan_items").Doc("sua-tuoi"), map[string]any{"status": "open", "name": "Sữa tươi"})
	mustSet(t, u.Collection("recurring").Doc("net"), map[string]any{"name": "Viettel", "amount_vnd": int64(250_000), "cycle": "monthly", "anchor_day": int64(5), "active": true, "next_due_on": "2026-10-05"})
	mustSet(t, u.Collection("recurring_occurrences").Doc("net_2026-10-05"), map[string]any{"recurring_id": "net", "name": "Viettel", "due_on": "2026-10-05", "amount_vnd": int64(250_000), "status": "pending"})

	on, _ := domain.ParseDate("2026-10-06")
	review := milkLine("l3", 0)
	review.GroupKey, review.NeedsReview = "khac", true
	r := receipt.Receipt{
		ID: "r1", Store: "Viettel Store", StoreKey: "viettel-store", PurchasedOn: on, Currency: "VND",
		Lines:    []receipt.Line{milkLine("l1", 1800), {ID: "l2", Name: "Cước Viettel", VND: vnd(250_000), CategoryKey: "hoa-don"}, review},
		TotalVND: 320_000, PayloadHash: "h1",
	}
	out, err := s.SaveReceipt(ctx, uid, r, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.PlanDone) != 1 || out.PlanDone[0] != "sua-tuoi" {
		t.Fatalf("plan done: %+v", out.PlanDone)
	}
	if rd := get(t, u)["plan_reconciled_on"]; rd != "2026-10-06" {
		t.Fatalf("plan_reconciled_on %v", rd)
	}
	if len(out.Recurring) != 1 {
		t.Fatalf("recurring matched: %+v", out.Recurring)
	}
	if st := get(t, u.Collection("recurring_occurrences").Doc("net_2026-10-05"))["status"]; st != "matched" {
		t.Fatalf("occurrence status %v", st)
	}
	if nx := get(t, u.Collection("recurring_occurrences").Doc("net_2026-11-05")); nx["status"] != "pending" {
		t.Fatalf("next occurrence %v", nx)
	}
	it := get(t, u.Collection("items").Doc("sua-tuoi"))
	stock, _ := it["stock"].(map[string]any)
	if asFloat(stock["qty"]) != 1800 || stock["unit"] != "ml" {
		t.Fatalf("stock %v", it["stock"])
	}
	if _, err := u.Collection("items").Doc("khac").Get(ctx); !isNotFound(err) {
		t.Fatal("needs_review line must not create inventory")
	}
	if p := get(t, u.Collection("purchases").Doc("r1_l1")); p["structure"] != "2 x 900ml" || p["payer_uid"] != uid {
		t.Fatalf("purchase %v", p)
	}
	if m := get(t, u.Collection("months").Doc("2026-10")); asInt64(m["spent_vnd"]) != 320_000 {
		t.Fatalf("month %v", m)
	}
	// Gửi lại cùng nội dung: không ghi lại.
	if again, err := s.SaveReceipt(ctx, uid, r, time.UTC); err != nil || !again.Replayed {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if m := get(t, u.Collection("months").Doc("2026-10")); asInt64(m["spent_vnd"]) != 320_000 {
		t.Fatalf("replay double counted %v", m)
	}
}

func TestSaveReceiptAssignmentEmulator(t *testing.T) {
	s, c, id := emulator(t)
	ctx := context.Background()
	owner, buyer, hid := "o"+id, "b"+id, "h"+id
	ou, bu := c.Collection("users").Doc(owner), c.Collection("users").Doc(buyer)
	mustSet(t, ou, map[string]any{"household_id": hid})
	mustSet(t, bu, map[string]any{"household_id": hid})
	for _, g := range []string{"sua-tuoi", "gao"} {
		mustSet(t, ou.Collection("plan_items").Doc(g), map[string]any{"status": "assigned", "assignment_id": "a1", "assigned_to_uid": buyer})
	}
	mustSet(t, c.Collection("households").Doc(hid).Collection("shopping_assignments").Doc("a1"), map[string]any{
		"owner_uid": owner, "buyer_uid": buyer, "plan_id": "default", "status": "assigned",
		"items": map[string]any{"sua-tuoi": map[string]any{"status": "assigned"}, "gao": map[string]any{"status": "assigned"}},
	})
	on, _ := domain.ParseDate("2026-10-06")
	r := receipt.Receipt{ID: "r1", Store: "WinMart", PurchasedOn: on, Currency: "VND", Lines: []receipt.Line{milkLine("l1", 1800)},
		TotalVND: 70_000, PayloadHash: "h", AssignmentID: "a1"}

	// Người không phải Buyer không xác nhận được.
	if _, err := s.SaveReceipt(ctx, owner, r, time.UTC); err == nil {
		t.Fatal("owner must not confirm buyer's assignment")
	}
	out, err := s.SaveReceipt(ctx, buyer, r, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if out.Assignment == nil || out.Assignment.Status != "closed_partial" || len(out.Assignment.Returned) != 1 {
		t.Fatalf("assignment %+v", out.Assignment)
	}
	// Tồn kho vào Owner.
	if _, err := ou.Collection("items").Doc("sua-tuoi").Get(ctx); err != nil {
		t.Fatalf("owner inventory: %v", err)
	}
	if _, err := bu.Collection("items").Doc("sua-tuoi").Get(ctx); !isNotFound(err) {
		t.Fatal("buyer must not get inventory")
	}
	// B4: mỗi người chỉ cộng phần của mình. Cả hóa đơn là món mua hộ nên vào sổ của Owner.
	if m := get(t, ou.Collection("months").Doc("2026-10")); asInt64(m["spent_vnd"]) != 70_000 || asInt64(m["delegated_vnd"]) != 70_000 {
		t.Fatalf("owner month %v", m)
	}
	if m := get(t, bu.Collection("months").Doc("2026-10")); asInt64(m["spent_vnd"]) != 0 || asInt64(m["receipts"]) != 1 {
		t.Fatalf("buyer month %v", m)
	}
	if st := get(t, ou.Collection("plan_items").Doc("sua-tuoi"))["status"]; st != "done" {
		t.Fatalf("bought item %v", st)
	}
	if st := get(t, ou.Collection("plan_items").Doc("gao"))["status"]; st != "open" {
		t.Fatalf("returned item %v", st)
	}
}

func TestRecurringSettleEmulator(t *testing.T) {
	s, c, uid := emulator(t)
	ctx := context.Background()
	u := c.Collection("users").Doc(uid)
	mustSet(t, u, map[string]any{})
	r := recurring.Recurring{ID: "netflix", Name: "Netflix", AmountVND: 104_499, Cycle: recurring.Monthly, NextDueOn: "2026-10-19", Active: true}
	if err := s.CreateRecurring(ctx, uid, r, 19); err != nil {
		t.Fatal(err)
	}
	paid, _ := domain.ParseDate("2026-10-18")
	o, err := s.Settle(ctx, uid, "netflix_2026-10-19", recurring.Paid, paid)
	if err != nil || o.Status != recurring.Paid {
		t.Fatalf("settle %+v %v", o, err)
	}
	if _, err := s.Settle(ctx, uid, "netflix_2026-10-19", recurring.Paid, paid); err == nil {
		t.Fatal("paying twice must fail")
	}
	if m := get(t, u.Collection("months").Doc("2026-10")); asInt64(m["spent_vnd"]) != 104_499 {
		t.Fatalf("month %v", m)
	}
	from, _ := domain.ParseDate("2026-10-01")
	to, _ := domain.ParseDate("2026-12-31")
	occs, err := s.Occurrences(ctx, uid, from, to)
	if err != nil || len(occs) != 2 || occs[1].DueOn != "2026-11-19" || occs[1].Status != recurring.Pending {
		t.Fatalf("occurrences %+v %v", occs, err)
	}
	if got, _ := s.GetRecurring(ctx, uid, "netflix"); got.NextDueOn != "2026-11-19" {
		t.Fatalf("next due %+v", got)
	}
	if err := s.DeleteRecurring(ctx, uid, "netflix"); err != nil {
		t.Fatal(err)
	}
	occs, _ = s.Occurrences(ctx, uid, from, to)
	if len(occs) != 1 || occs[0].Status != recurring.Paid {
		t.Fatalf("after delete %+v", occs)
	}
}
