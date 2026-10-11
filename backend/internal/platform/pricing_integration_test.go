package platform

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/checkout"
	"github.com/Adeqq1/ticket-online/backend/internal/payment"
	"github.com/Adeqq1/ticket-online/backend/internal/pricing"
	"github.com/Adeqq1/ticket-online/backend/internal/refund"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
	"github.com/Adeqq1/ticket-online/backend/migrations"
)

func TestPricingSnapshotsVoucherQuotaExpiryAndRefund(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to a disposable database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	suffix := randomHex(t, 8)
	code, tier := "DUMMY_"+strings.ToUpper(suffix), "pricing-"+suffix
	auth := staffauth.New(db)
	admin, err := auth.CreateStaff(ctx, "Pricing Admin", "pricing-admin-"+suffix+"@example.test", "pricing dummy password", nil)
	if err != nil {
		t.Fatal(err)
	}
	staff, err := auth.CreateStaff(ctx, "Pricing Staff", "pricing-staff-"+suffix+"@example.test", "pricing dummy password", nil)
	if err != nil {
		t.Fatal(err)
	}
	var original pricing.Settings
	if err = db.QueryRow("SELECT admin_fee,revision FROM checkout_settings WHERE id=1").Scan(&original.AdminFee, &original.Revision); err != nil {
		t.Fatal(err)
	}
	reservations := []string{}
	defer func() {
		remove := func(query string, args ...any) {
			if _, err := db.Exec(query, args...); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
		for _, rid := range reservations {
			var oid string
			err := db.QueryRow("SELECT id FROM orders WHERE reservation_id=?", rid).Scan(&oid)
			if err == nil {
				remove("DELETE FROM order_refund_audit WHERE refund_id IN (SELECT id FROM order_refunds WHERE order_id=?)", oid)
				for _, table := range []string{"email_queue", "order_refunds", "event_refund_rights", "etickets", "order_attendees", "payments", "order_items", "order_buyers"} {
					remove("DELETE FROM "+table+" WHERE order_id=?", oid)
				}
				remove("DELETE FROM orders WHERE id=?", oid)
			} else if !errors.Is(err, sql.ErrNoRows) {
				t.Errorf("cleanup order: %v", err)
			}
			remove("DELETE FROM reservation_items WHERE reservation_id=?", rid)
			remove("DELETE FROM reservations WHERE id=?", rid)
		}
		remove("DELETE FROM event_changes WHERE id=?", code)
		remove("DELETE FROM vouchers WHERE code=?", code)
		remove("DELETE FROM ticket_tiers WHERE event_id='nusa-malam' AND slug=?", tier)
		for _, id := range []string{admin.ID, staff.ID} {
			remove("DELETE FROM admin_audit_log WHERE actor_id=?", id)
			remove("DELETE FROM staff_sessions WHERE staff_id=?", id)
			remove("DELETE FROM staff_users WHERE id=?", id)
		}
		remove("UPDATE checkout_settings SET admin_fee=?,revision=? WHERE id=1", original.AdminFee, original.Revision)
	}()
	if _, err = db.Exec("UPDATE staff_users SET role='ADMIN' WHERE id=?", admin.ID); err != nil {
		t.Fatal(err)
	}
	as, err := auth.Login(ctx, admin.Email, "pricing dummy password", "192.0.2.40")
	if err != nil {
		t.Fatal(err)
	}
	ss, err := auth.Login(ctx, staff.Email, "pricing dummy password", "192.0.2.41")
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHandlerWithOrderAccess(db, logger, 10*time.Minute, "", true, []byte("0123456789abcdef0123456789abcdef"))
	call := func(method, path, token, key string, input any, want int) *httptest.ResponseRecorder {
		t.Helper()
		b, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", key)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: %d want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		return w
	}
	call("GET", "/api/v1/admin/pricing", "", "", nil, 401)
	call("GET", "/api/v1/admin/pricing", ss.AccessToken, "", nil, 403)
	call("PUT", "/api/v1/admin/pricing", ss.AccessToken, "", map[string]any{"adminFee": 0, "revision": original.Revision}, 403)
	call("POST", "/api/v1/admin/vouchers", ss.AccessToken, "", nil, 403)
	call("PUT", "/api/v1/admin/pricing", as.AccessToken, "", map[string]any{"revision": original.Revision}, 422)
	call("PUT", "/api/v1/admin/pricing", as.AccessToken, "", map[string]any{"adminFee": -1, "revision": original.Revision}, 400)
	newSettings := pricing.Settings{AdminFee: 10000, Revision: original.Revision}
	response := call("PUT", "/api/v1/admin/pricing", as.AccessToken, "", newSettings, 200)
	if err = json.Unmarshal(response.Body.Bytes(), &newSettings); err != nil {
		t.Fatal(err)
	}
	call("PUT", "/api/v1/admin/pricing", as.AccessToken, "", original, 409)
	_, err = db.Exec(`INSERT INTO ticket_tiers(event_id,slug,name,zone_slug,price,capacity,available_quantity,max_per_order,benefit,gate,seating_mode,created_at,updated_at)
	 VALUES ('nusa-malam',?,'Pricing Dummy','festival',10001,10,10,1,'Dummy','Gate B','FREE_STANDING',UTC_TIMESTAMP(6),UTC_TIMESTAMP(6))`, tier)
	if err != nil {
		t.Fatal(err)
	}
	v := pricing.Voucher{Code: code, Kind: "PERCENT", Value: 25, StartsAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), EndsAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), Active: true, EventID: "nusa-malam", MinimumSubtotal: 10000, Quota: 1}
	response = call("POST", "/api/v1/admin/vouchers", as.AccessToken, "", v, 201)
	if err = json.Unmarshal(response.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	copy := v
	copy.Revision = 0
	call("POST", "/api/v1/admin/vouchers", as.AccessToken, "", copy, 409)
	reserve := func() (string, string) {
		t.Helper()
		key := "pricing-" + randomHex(t, 16)
		w := call("POST", "/api/v1/reservations", "", key, map[string]any{"eventId": "nusa-malam", "items": []map[string]any{{"tierId": tier, "quantity": 1}}}, 201)
		var r struct{ ID string }
		if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		reservations = append(reservations, r.ID)
		return r.ID, key
	}
	r1, k1 := reserve()
	r2, k2 := reserve()
	call("POST", "/api/v1/reservations/"+r1+"/quote", "", "wrong-reservation-key", map[string]string{"voucherCode": code}, 401)
	for _, rid := range []string{r1, r2} {
		key := k1
		if rid == r2 {
			key = k2
		}
		for i := 0; i < 2; i++ {
			call("POST", "/api/v1/reservations/"+rid+"/quote", "", key, map[string]string{"voucherCode": code}, 200)
		}
	}
	request := checkout.Request{Buyer: checkout.Buyer{Name: "Dummy Buyer", Email: "pricing-buyer@example.test", Phone: "081234567890", Identity: "123456789012"}, VoucherCode: code, Attendees: []checkout.Attendees{{TierID: tier, Names: []string{"Dummy Guest"}}}}
	repo := checkout.NewRepository(db)
	type result struct {
		order    checkout.Order
		err      error
		rid, key string
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, pair := range [][2]string{{r1, k1}, {r2, k2}} {
		wg.Add(1)
		go func(rid, key string) {
			defer wg.Done()
			input := request
			input.Attendees = []checkout.Attendees{{TierID: tier, Names: []string{"Dummy Guest"}}}
			o, _, err := repo.Create(ctx, rid, key, input)
			results <- result{o, err, rid, key}
		}(pair[0], pair[1])
	}
	wg.Wait()
	close(results)
	var winner, loser result
	successes, failures := 0, 0
	for got := range results {
		if got.err == nil {
			winner = got
			successes++
		} else if errors.Is(got.err, pricing.ErrVoucher) {
			loser = got
			failures++
		} else {
			t.Fatalf("concurrent checkout: %v", got.err)
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("quota race: %d successes, %d rejections", successes, failures)
	}
	if winner.order.AdminFee != 10000 || winner.order.Discount != 2500 || winner.order.Total != 17501 {
		t.Fatalf("snapshot: %+v", winner.order)
	}
	newSettings.AdminFee = 20000
	response = call("PUT", "/api/v1/admin/pricing", as.AccessToken, "", newSettings, 200)
	if err = json.Unmarshal(response.Body.Bytes(), &newSettings); err != nil {
		t.Fatal(err)
	}
	v.Active = false
	v.Value = 50
	response = call("PUT", "/api/v1/admin/vouchers/"+code, as.AccessToken, "", v, 200)
	if err = json.Unmarshal(response.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	replayed, replay, err := repo.Create(ctx, winner.rid, winner.key, request)
	if err != nil || !replay || replayed.AdminFee != 10000 || replayed.Discount != 2500 || replayed.Voucher.Value != 25 {
		t.Fatalf("replay repriced old order: %+v %v", replayed, err)
	}
	if _, err = repo.Quote(ctx, loser.rid, loser.key, code); !errors.Is(err, pricing.ErrVoucher) {
		t.Fatal("inactive voucher accepted")
	}
	v.Active = true
	v.Value = 25
	response = call("PUT", "/api/v1/admin/vouchers/"+code, as.AccessToken, "", v, 200)
	if err = json.Unmarshal(response.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("UPDATE orders SET expires_at=UTC_TIMESTAMP(6)-INTERVAL 1 SECOND WHERE id=?", winner.order.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Quote(ctx, loser.rid, loser.key, code); !errors.Is(err, pricing.ErrVoucher) {
		t.Fatalf("unreconciled expiry released quota: %v", err)
	}
	pay := payment.NewRepository(db)
	if err = pay.ExpirePendingOrders(ctx); err != nil {
		t.Fatal(err)
	}
	paidOrder, _, err := repo.Create(ctx, loser.rid, loser.key, request)
	if err != nil {
		t.Fatal(err)
	}
	if paidOrder.AdminFee != 20000 || paidOrder.Discount != 2500 {
		t.Fatalf("new order not using new fee: %+v", paidOrder)
	}
	if _, _, err = pay.Simulate(ctx, paidOrder.ID, payment.Request{Method: "QRIS", Result: "FAILED"}); err != nil {
		t.Fatal(err)
	}
	r3, k3 := reserve()
	if _, err = repo.Quote(ctx, r3, k3, code); !errors.Is(err, pricing.ErrVoucher) {
		t.Fatalf("failed retryable payment released quota: %v", err)
	}
	paid, _, err := pay.Simulate(ctx, paidOrder.ID, payment.Request{Method: "QRIS", Result: "SUCCEEDED"})
	if err != nil || paid.Amount != paidOrder.Total {
		t.Fatalf("payment amount: %+v %v", paid, err)
	}
	// Exercise the existing manual event-refund workflow without a real provider.
	if _, err = db.Exec(`INSERT INTO event_changes(id,event_id,version,action,reason,announcement,staff_id,idempotency_key,request_hash,created_at)
	 SELECT ?,'nusa-malam',COALESCE(MAX(version),0)+1,'CANCELLED','Dummy refund','Dummy refund',?,?,REPEAT('a',64),UTC_TIMESTAMP(6) FROM event_changes WHERE event_id='nusa-malam'`, code, admin.ID, code); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO event_refund_rights(order_id,change_id,requested) VALUES (?,?,TRUE)", paidOrder.ID, code); err != nil {
		t.Fatal(err)
	}
	refunds := refund.New(db, auth, "", "sandbox", nil, logger)
	requested, err := refunds.SubmitEvent(ctx, paidOrder.ID)
	if err != nil || requested.Amount != paidOrder.Total {
		t.Fatalf("refund snapshot: %+v %v", requested, err)
	}
	completed, err := refunds.CompleteManual(ctx, as.AccessToken, paidOrder.ID, refund.ManualInput{Reference: "DUMMY-REFUND", PaidAt: time.Now().UTC().Format(time.RFC3339Nano), Note: "Dummy verified transfer"})
	if err != nil || completed.Amount != paidOrder.Total || completed.Status != "SUCCEEDED" {
		t.Fatalf("refund: %+v %v", completed, err)
	}
	if _, err = repo.Quote(ctx, r3, k3, code); !errors.Is(err, pricing.ErrVoucher) {
		t.Fatalf("refund returned quota: %v", err)
	}
	detail, err := repo.Get(ctx, paidOrder.ID)
	if err != nil || detail.AdminFee != 20000 || detail.Discount != 2500 || detail.Voucher.Value != 25 {
		t.Fatalf("refund repriced snapshot: %+v %v", detail, err)
	}
	listed := call("GET", "/api/v1/admin/pricing", as.AccessToken, "", nil, 200)
	var data struct {
		Vouchers []pricing.ListedVoucher `json:"vouchers"`
		History  []pricing.Audit         `json:"history"`
	}
	if err = json.Unmarshal(listed.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	var used pricing.Usage
	for _, v := range data.Vouchers {
		if v.Code == code {
			used = v.Usage
		}
	}
	if used.Held != 0 || used.Consumed != 1 || used.Released != 1 || len(data.History) < 4 {
		t.Fatalf("usage/audit: %+v history=%d", used, len(data.History))
	}
	adminDetail := call("GET", "/api/v1/admin/orders/"+paidOrder.ID, as.AccessToken, "", nil, 200)
	if !strings.Contains(adminDetail.Body.String(), code) {
		t.Fatal("admin order missing voucher snapshot")
	}
	// Re-use the freed reservation to check eligibility and fixed discount clamping.
	v.Quota = 2
	v.Kind = "FIXED"
	v.Value = 999999
	v.MinimumSubtotal = 20000
	response = call("PUT", "/api/v1/admin/vouchers/"+code, as.AccessToken, "", v, 200)
	json.Unmarshal(response.Body.Bytes(), &v)
	if _, err = repo.Quote(ctx, r3, k3, code); !errors.Is(err, pricing.ErrVoucher) {
		t.Fatal("minimum spend ignored")
	}
	v.MinimumSubtotal = 0
	v.EventID = "ruang-senja"
	response = call("PUT", "/api/v1/admin/vouchers/"+code, as.AccessToken, "", v, 200)
	json.Unmarshal(response.Body.Bytes(), &v)
	if _, err = repo.Quote(ctx, r3, k3, code); !errors.Is(err, pricing.ErrVoucher) {
		t.Fatal("event scope ignored")
	}
	v.EventID = ""
	v.StartsAt = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	v.EndsAt = time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	response = call("PUT", "/api/v1/admin/vouchers/"+code, as.AccessToken, "", v, 200)
	json.Unmarshal(response.Body.Bytes(), &v)
	if _, err = repo.Quote(ctx, r3, k3, code); !errors.Is(err, pricing.ErrVoucher) {
		t.Fatal("future start ignored")
	}
	v.StartsAt = time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	v.EndsAt = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	response = call("PUT", "/api/v1/admin/vouchers/"+code, as.AccessToken, "", v, 200)
	json.Unmarshal(response.Body.Bytes(), &v)
	if _, err = repo.Quote(ctx, r3, k3, code); !errors.Is(err, pricing.ErrVoucher) {
		t.Fatal("expiry ignored")
	}
	v.EndsAt = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	response = call("PUT", "/api/v1/admin/vouchers/"+code, as.AccessToken, "", v, 200)
	json.Unmarshal(response.Body.Bytes(), &v)
	q, err := repo.Quote(ctx, r3, k3, code)
	if err != nil || q.Discount != 10001 || q.Total != 20000 {
		t.Fatalf("fixed discount cap: %+v %v", q, err)
	}
	if _, _, err = repo.Create(ctx, r3, k3, request); err != nil {
		t.Fatal(err)
	}
	tooSmall := v
	tooSmall.Quota = 1
	call("PUT", "/api/v1/admin/vouchers/"+code, as.AccessToken, "", tooSmall, 409)
	old := v
	v.Quota = 3
	response = call("PUT", "/api/v1/admin/vouchers/"+code, as.AccessToken, "", v, 200)
	json.Unmarshal(response.Body.Bytes(), &v)
	call("PUT", "/api/v1/admin/vouchers/"+code, as.AccessToken, "", old, 409)
	newSettings.AdminFee = 0
	response = call("PUT", "/api/v1/admin/pricing", as.AccessToken, "", newSettings, 200)
	if err = json.Unmarshal(response.Body.Bytes(), &newSettings); err != nil {
		t.Fatal(err)
	}
	r4, k4 := reserve()
	q, err = repo.Quote(ctx, r4, k4, "")
	if err != nil || q.AdminFee != 0 || q.Total != 10001 {
		t.Fatalf("zero admin fee: %+v %v", q, err)
	}
	if _, err = repo.Quote(ctx, r4, k4, code); !errors.Is(err, pricing.ErrVoucher) {
		t.Fatal("zero total unsupported payment accepted")
	}
	if _, err = db.Exec("UPDATE staff_sessions SET expires_at=UTC_TIMESTAMP(6)-INTERVAL 1 SECOND WHERE staff_id=?", admin.ID); err != nil {
		t.Fatal(err)
	}
	call("PUT", "/api/v1/admin/pricing", as.AccessToken, "", newSettings, 401)
	t.Log("fee snapshots, concurrent last quota, replay, reconciled expiry, payment retry, permanent use after refund, eligibility, admin audit and role/session checks passed")
}
