package adminreports

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
	"github.com/Adeqq1/ticket-online/backend/migrations"
	_ "github.com/go-sql-driver/mysql"
)

func TestSalesReportUsesPaymentAndRefundTimestampsWithoutMultiplyingRows(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to a disposable MySQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	id := func() string {
		t.Helper()
		var value [16]byte
		if _, err := rand.Read(value[:]); err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(value[:])
	}
	adminID, adminToken, staffID, staffToken := id(), id()+id()+id()[:11], id(), id()+id()+id()[:11]
	adminToken, staffToken = adminToken[:43], staffToken[:43]
	addUser := func(userID, token, role, email string) {
		t.Helper()
		hash := sha256.Sum256([]byte(token))
		now := time.Now().UTC()
		if _, err := db.ExecContext(ctx, `INSERT INTO staff_users (id,name,email,password_hash,role,active,created_at,updated_at)
			VALUES (?,'Report Integration',?,'unused',?,TRUE,?,?)`, userID, email, role, now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO staff_sessions (token_hash,staff_id,expires_at,created_at) VALUES (?,?,?,?)", hash[:], userID, now.Add(time.Hour), now); err != nil {
			t.Fatal(err)
		}
	}
	addUser(adminID, adminToken, "ADMIN", "report-admin-"+adminID+"@example.test")
	addUser(staffID, staffToken, "STAFF", "report-staff-"+staffID+"@example.test")
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM staff_sessions WHERE staff_id IN (?,?)", adminID, staffID)
		_, _ = db.Exec("DELETE FROM staff_users WHERE id IN (?,?)", adminID, staffID)
	})
	baseEventID, otherEventID := "report-"+id(), "report-"+id()
	now := time.Now().UTC()
	for _, eventID := range []string{baseEventID, otherEventID} {
		if _, err := db.ExecContext(ctx, `INSERT INTO events (id,artist,city,venue,address,genre,status,image_url,description,created_at,updated_at)
			VALUES (?,'Report Event','Jakarta','Venue','Address','Indie','EARLY_BIRD','','Report test',?,?)`, eventID, now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO event_zones (event_id,slug,name,description) VALUES (?,'zone','Zone','Test')", eventID); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM payment_reconciliation_cases WHERE order_id IN (SELECT id FROM orders WHERE reservation_id IN (SELECT id FROM reservations WHERE event_id IN (?,?)))", baseEventID, otherEventID)
		_, _ = db.Exec("DELETE FROM order_refunds WHERE refund_key LIKE 'report-%'")
		_, _ = db.Exec("DELETE FROM payments WHERE order_id IN (SELECT id FROM orders WHERE reservation_id IN (SELECT id FROM reservations WHERE event_id IN (?,?)))", baseEventID, otherEventID)
		_, _ = db.Exec("DELETE FROM orders WHERE reservation_id IN (SELECT id FROM reservations WHERE event_id IN (?,?))", baseEventID, otherEventID)
		_, _ = db.Exec("DELETE FROM reservation_items WHERE reservation_id IN (SELECT id FROM reservations WHERE event_id IN (?,?))", baseEventID, otherEventID)
		_, _ = db.Exec("DELETE FROM reservations WHERE event_id IN (?,?)", baseEventID, otherEventID)
		_, _ = db.Exec("DELETE FROM ticket_tiers WHERE event_id IN (?,?)", baseEventID, otherEventID)
		_, _ = db.Exec("DELETE FROM event_zones WHERE event_id IN (?,?)", baseEventID, otherEventID)
		_, _ = db.Exec("DELETE FROM events WHERE id IN (?,?)", baseEventID, otherEventID)
	})
	tierIDs := make(map[string]int64)
	for _, eventID := range []string{baseEventID, otherEventID} {
		for _, slug := range []string{"one", "two"} {
			var tierID int64
			result, err := db.ExecContext(ctx, `INSERT INTO ticket_tiers (event_id,slug,name,zone_slug,price,capacity,available_quantity,max_per_order,benefit,gate,seating_mode,created_at,updated_at)
				VALUES (?,?,?, 'zone',1000,20,20,4,'Test','Gate','FREE_STANDING',?,?)`, eventID, slug, slug, now, now)
			if err != nil {
				t.Fatal(err)
			}
			tierID, err = result.LastInsertId()
			if err != nil {
				t.Fatal(err)
			}
			tierIDs[eventID+slug] = tierID
		}
	}
	type orderFixture struct {
		id, event string
		amount    int64
	}
	orders := []orderFixture{{id(), baseEventID, 1000}, {id(), baseEventID, 2000}, {id(), otherEventID, 9000}}
	for index, order := range orders {
		reservationID := order.id
		if _, err := db.ExecContext(ctx, `INSERT INTO reservations (id,event_id,status,idempotency_key,request_hash,expires_at,created_at,updated_at)
			VALUES (?,?,'CONVERTED',?,REPEAT('a',64),?,?,?)`, reservationID, order.event, reservationID, now.Add(time.Hour), now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO orders (id,reference,reservation_id,status,subtotal,expires_at,created_at,updated_at) VALUES (?, ?, ?, 'PAID', ?, ?, ?, ?)`, order.id, order.id, reservationID, order.amount, now.Add(time.Hour), now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO reservation_items (reservation_id,ticket_tier_id,quantity,unit_price) VALUES (?,?,2,1000)", reservationID, tierIDs[order.event+"one"]); err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			if _, err := db.ExecContext(ctx, "INSERT INTO reservation_items (reservation_id,ticket_tier_id,quantity,unit_price) VALUES (?,?,3,1000)", reservationID, tierIDs[order.event+"two"]); err != nil {
				t.Fatal(err)
			}
		}
		paidAt := time.Date(2026, 9, 1+index, 18, 0, 0, 0, time.UTC)
		if _, err := db.ExecContext(ctx, `INSERT INTO payments (id,order_id,method,amount,status,paid_at,created_at,updated_at)
			VALUES (?,?, 'QRIS', ?, 'SUCCEEDED', ?, ?, ?)`, id(), order.id, order.amount, paidAt, now, now); err != nil {
			t.Fatal(err)
		}
	}
	refundID, pendingID := id(), orders[1].id
	if _, err := db.ExecContext(ctx, `INSERT INTO order_refunds
		(id,order_id,status,original_order_status,amount,reason,requested_by_staff_id,refund_key,gateway_order_id,requested_at,updated_at,completed_at)
		VALUES (?,?,'SUCCEEDED','PAID',1000,'test',?,?,'gateway-report',?,?,?)`, refundID, orders[0].id, adminID, "report-"+refundID, now, now, time.Date(2026, 9, 3, 18, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	pendingRefundID := id()
	if _, err := db.ExecContext(ctx, `INSERT INTO order_refunds
		(id,order_id,status,original_order_status,amount,reason,requested_by_staff_id,refund_key,gateway_order_id,requested_at,updated_at)
		VALUES (?,?,'PROCESSING','PAID',2000,'test',?,?,'gateway-pending',?,?)`, pendingRefundID, pendingID, adminID, "report-"+pendingRefundID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO payment_reconciliation_cases (order_id,gateway_order_id,amount,provider_status,created_at,updated_at)
		VALUES (?,? ,2000,'settlement',?,?)`, pendingID, "report-"+pendingID, now, now); err != nil {
		t.Fatal(err)
	}
	service := NewService(db, staffauth.New(db))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/reports/sales", nil)
	report, err := service.Sales(request, adminToken, Filter{EventID: baseEventID, DateFrom: "2026-09-02", DateTo: "2026-09-04"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.SuccessfulTransactions != 2 || report.Summary.PaymentAmount != 3000 || report.Summary.RefundAmount != 1000 || report.Summary.NetAmount != 2000 || report.Summary.UnfinishedRefunds != 1 || report.Summary.OpenReconciliationCases != 1 {
		t.Fatalf("report summary = %+v", report.Summary)
	}
	if len(report.Daily) != 3 || report.Daily[0].Date != "2026-09-02" || report.Daily[0].PaymentAmount != 1000 || report.Daily[1].Date != "2026-09-03" || report.Daily[1].PaymentAmount != 2000 || report.Daily[2].Date != "2026-09-04" || report.Daily[2].RefundAmount != 1000 {
		t.Fatalf("daily report = %+v", report.Daily)
	}
	if len(report.ByEvent) != 1 || report.ByEvent[0].ID != baseEventID || report.ByEvent[0].PaymentAmount != 3000 {
		t.Fatalf("event report = %+v", report.ByEvent)
	}
	handler := NewHandler(service, slog.Default())
	mux := http.NewServeMux()
	handler.Register(mux)
	call := func(token string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/reports/sales", nil)
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		return response
	}
	if got := call(""); got.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated status = %d", got.Code)
	}
	if got := call(staffToken); got.Code != http.StatusForbidden {
		t.Errorf("STAFF status = %d, body %s", got.Code, got.Body.String())
	}
	if got := call(adminToken); got.Code != http.StatusOK || got.Header().Get("Cache-Control") != "no-store" {
		var decoded Report
		err := json.Unmarshal(got.Body.Bytes(), &decoded)
		t.Errorf("ADMIN response = %d %s; decode error %v", got.Code, got.Body.String(), err)
	}
}
