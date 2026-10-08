package adminreports

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/checkin"
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
	t.Cleanup(func() { _ = db.Close() })
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
		artist := "Report Event"
		if eventID == baseEventID {
			artist = "=HYPERLINK(\"x\",\"x\"),\nReport Event"
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO events (id,artist,city,venue,address,genre,status,image_url,description,created_at,updated_at)
			VALUES (?,?,'Jakarta','Venue','Address','Indie','EARLY_BIRD','','Report test',?,?)`, eventID, artist, now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO event_zones (event_id,slug,name,description) VALUES (?,'zone','Zone','Test')", eventID); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM checkin_attempts WHERE staff_id IN (?,?)", adminID, staffID)
		_, _ = db.Exec("DELETE FROM ticket_checkins WHERE event_id IN (?,?)", baseEventID, otherEventID)
		_, _ = db.Exec("DELETE FROM etickets WHERE order_id IN (SELECT id FROM orders WHERE reservation_id IN (SELECT id FROM reservations WHERE event_id IN (?,?)))", baseEventID, otherEventID)
		_, _ = db.Exec("DELETE FROM order_attendees WHERE order_id IN (SELECT id FROM orders WHERE reservation_id IN (SELECT id FROM reservations WHERE event_id IN (?,?)))", baseEventID, otherEventID)
		_, _ = db.Exec("DELETE FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE reservation_id IN (SELECT id FROM reservations WHERE event_id IN (?,?)))", baseEventID, otherEventID)
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
		paidAt := []time.Time{
			time.Date(2026, 9, 1, 16, 59, 59, 999999000, time.UTC),
			time.Date(2026, 9, 1, 17, 0, 0, 0, time.UTC),
			time.Date(2026, 9, 2, 18, 0, 0, 0, time.UTC),
		}[index]
		if _, err := db.ExecContext(ctx, `INSERT INTO payments (id,order_id,method,amount,status,paid_at,created_at,updated_at)
			VALUES (?,?, 'QRIS', ?, 'SUCCEEDED', ?, ?, ?)`, id(), order.id, order.amount, paidAt, now, now); err != nil {
			t.Fatal(err)
		}
	}
	refundID, pendingID := id(), orders[1].id
	if _, err := db.ExecContext(ctx, `INSERT INTO order_refunds
		(id,order_id,status,original_order_status,amount,reason,requested_by_staff_id,refund_key,gateway_order_id,requested_at,updated_at,completed_at)
		VALUES (?,?,'SUCCEEDED','PAID',1000,'test',?,?,'gateway-report',?,?,?)`, refundID, orders[0].id, adminID, "report-"+refundID, now, now, time.Date(2026, 9, 4, 18, 0, 0, 0, time.UTC)); err != nil {
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
	addTickets := func(status string, slots map[int64]int) (string, []string) {
		t.Helper()
		reservationID, orderID := id(), id()
		quantity, subtotal := 0, int64(0)
		for _, count := range slots {
			quantity += count
			subtotal += int64(count) * 1000
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO reservations (id,event_id,status,idempotency_key,request_hash,expires_at,created_at,updated_at)
			VALUES (?,?,'CONVERTED',?,REPEAT('b',64),?,?,?)`, reservationID, baseEventID, reservationID, now.Add(time.Hour), now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO orders (id,reference,reservation_id,status,subtotal,expires_at,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?,?)`, orderID, orderID, reservationID, status, subtotal, now.Add(time.Hour), now, now); err != nil {
			t.Fatal(err)
		}
		tickets := make([]string, 0, quantity)
		for tierID, count := range slots {
			var tierName string
			if err := db.QueryRowContext(ctx, "SELECT name FROM ticket_tiers WHERE id=?", tierID).Scan(&tierName); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, "INSERT INTO order_items (order_id,ticket_tier_id,tier_name,quantity,unit_price) VALUES (?,?,?,?,1000)", orderID, tierID, tierName, count); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, "INSERT INTO reservation_items (reservation_id,ticket_tier_id,quantity,unit_price) VALUES (?,?,?,1000)", reservationID, tierID, count); err != nil {
				t.Fatal(err)
			}
			for number := 1; number <= count; number++ {
				ticketID := id()
				if _, err := db.ExecContext(ctx, "INSERT INTO order_attendees (order_id,ticket_tier_id,ticket_number,name) VALUES (?,?,?,'Private Attendee')", orderID, tierID, number); err != nil {
					t.Fatal(err)
				}
				snapshot, err := json.Marshal(map[string]string{"id": ticketID, "code": "ET-" + strings.ToUpper(ticketID), "attendeeName": "Private Attendee", "tierName": tierName, "eventId": baseEventID, "gate": "Gate"})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := db.ExecContext(ctx, "INSERT INTO etickets (id,order_id,ticket_tier_id,ticket_number,snapshot,issued_at) VALUES (?,?,?,?,?,?)", ticketID, orderID, tierID, number, snapshot, now); err != nil {
					t.Fatal(err)
				}
				tickets = append(tickets, ticketID)
			}
		}
		return orderID, tickets
	}
	_, paidTicketIDs := addTickets("PAID", map[int64]int{tierIDs[baseEventID+"one"]: 2, tierIDs[baseEventID+"two"]: 1})
	heldOrderID, _ := addTickets("REFUND_PENDING", map[int64]int{tierIDs[baseEventID+"one"]: 1})
	if _, err := db.ExecContext(ctx, "UPDATE ticket_tiers SET available_quantity=17 WHERE id=?", tierIDs[baseEventID+"one"]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE ticket_tiers SET available_quantity=19 WHERE id=?", tierIDs[baseEventID+"two"]); err != nil {
		t.Fatal(err)
	}
	attendanceRefundID := id()
	if _, err := db.ExecContext(ctx, `INSERT INTO order_refunds
		(id,order_id,status,original_order_status,amount,reason,requested_by_staff_id,refund_key,gateway_order_id,requested_at,updated_at)
		VALUES (?,?, 'PROCESSING','PAID',1000,'attendance test',?,?,'gateway-attendance',?,?)`, attendanceRefundID, heldOrderID, adminID, "report-"+attendanceRefundID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO staff_assignments (staff_id,event_id,gate,created_at) VALUES (?,?,'Gate',?)`, staffID, baseEventID, now); err != nil {
		t.Fatal(err)
	}
	checkIn := checkin.NewService(db, staffauth.New(db))
	requestCode := "ET-" + strings.ToUpper(paidTicketIDs[0])
	firstCheckIn, err := checkIn.CheckIn(ctx, staffToken, checkin.Request{EventID: baseEventID, Gate: "Gate", Code: requestCode})
	if err != nil {
		t.Fatalf("first check-in failed: %v", err)
	}
	expectedHour := firstCheckIn.CheckedInAt.In(wib).Truncate(time.Hour).Format("2006-01-02T15:00:00-07:00")
	if _, err := checkIn.CheckIn(ctx, staffToken, checkin.Request{EventID: baseEventID, Gate: "Gate", Code: requestCode}); !errors.Is(err, checkin.ErrTicketUsed) {
		t.Fatalf("repeated check-in error = %v, want ErrTicketUsed", err)
	}
	service := NewService(db, staffauth.New(db))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/reports/sales", nil)
	emptyAttendance, err := service.Attendance(request, adminToken, AttendanceFilter{EventID: otherEventID})
	if err != nil || emptyAttendance.Summary.Issued != 0 || emptyAttendance.Summary.Eligible != 0 || emptyAttendance.Summary.AttendanceRate != nil || len(emptyAttendance.Hourly) != 0 {
		t.Fatalf("empty attendance report = %+v, error = %v", emptyAttendance.Summary, err)
	}
	report, err := service.Sales(request, adminToken, Filter{EventID: baseEventID, DateFrom: "2026-09-02", DateTo: "2026-09-04"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.SuccessfulTransactions != 1 || report.Summary.PaymentAmount != 2000 || report.Summary.RefundAmount != 0 || report.Summary.NetAmount != 2000 || report.Summary.UnfinishedRefunds != 2 || report.Summary.OpenReconciliationCases != 1 {
		t.Fatalf("report summary = %+v", report.Summary)
	}
	if len(report.Daily) != 3 || report.Daily[0].Date != "2026-09-02" || report.Daily[0].PaymentAmount != 2000 || report.Daily[1].Date != "2026-09-03" || report.Daily[1].PaymentAmount != 0 || report.Daily[2].Date != "2026-09-04" || report.Daily[2].RefundAmount != 0 {
		t.Fatalf("daily report = %+v", report.Daily)
	}
	if len(report.ByEvent) != 1 || report.ByEvent[0].ID != baseEventID || report.ByEvent[0].PaymentAmount != 2000 {
		t.Fatalf("event report = %+v", report.ByEvent)
	}
	if len(report.FilterOptions.Events) < 2 {
		t.Fatalf("filtered report event options = %+v, want at least both fixture events", report.FilterOptions.Events)
	}
	optionIDs := map[string]bool{}
	for _, event := range report.FilterOptions.Events {
		optionIDs[event.ID] = true
	}
	if !optionIDs[baseEventID] || !optionIDs[otherEventID] {
		t.Fatalf("filtered report event options = %+v, want %s and %s", report.FilterOptions.Events, baseEventID, otherEventID)
	}
	refundReport, err := service.Sales(request, adminToken, Filter{EventID: baseEventID, DateFrom: "2026-09-05", DateTo: "2026-09-05"})
	if err != nil || refundReport.Summary.PaymentAmount != 0 || refundReport.Summary.RefundAmount != 1000 || refundReport.Summary.NetAmount != -1000 {
		t.Fatalf("refund date report = %+v, error = %v", refundReport.Summary, err)
	}
	emptyReport, err := service.Sales(request, adminToken, Filter{EventID: baseEventID, DateFrom: "2020-01-01", DateTo: "2020-01-01"})
	if err != nil || emptyReport.Summary.PaymentAmount != 0 || emptyReport.Summary.RefundAmount != 0 || len(emptyReport.Daily) != 1 {
		t.Fatalf("empty sales report = %+v, error = %v", emptyReport.Summary, err)
	}
	attendanceReport, err := service.Attendance(request, adminToken, AttendanceFilter{EventID: baseEventID})
	if err != nil {
		t.Fatal(err)
	}
	if attendanceReport.Summary.Capacity != 40 || attendanceReport.Summary.Available != 36 || attendanceReport.Summary.Issued != 4 || attendanceReport.Summary.Eligible != 3 || attendanceReport.Summary.HeldForRefund != 1 || attendanceReport.Summary.CheckedIn != 1 || attendanceReport.Summary.AttendanceRate == nil || *attendanceReport.Summary.AttendanceRate < 33.33 || *attendanceReport.Summary.AttendanceRate > 33.34 {
		t.Fatalf("attendance summary = %+v", attendanceReport.Summary)
	}
	if len(attendanceReport.ByCategory) != 2 || len(attendanceReport.ByGate) != 1 || len(attendanceReport.Hourly) != 1 || attendanceReport.Hourly[0].Hour != expectedHour || attendanceReport.Hourly[0].CheckedIn != 1 {
		t.Fatalf("attendance breakdown = categories %+v, gates %+v, hourly %+v", attendanceReport.ByCategory, attendanceReport.ByGate, attendanceReport.Hourly)
	}
	if attendanceReport.ByCategory[0].Issued+attendanceReport.ByCategory[1].Issued != 4 || attendanceReport.ByGate[0].Capacity != 40 {
		t.Fatalf("attendance category/gate totals = categories %+v gates %+v", attendanceReport.ByCategory, attendanceReport.ByGate)
	}
	filteredAttendance, err := service.Attendance(request, adminToken, AttendanceFilter{EventID: baseEventID, Gate: "Gate"})
	if err != nil || filteredAttendance.Summary.CheckedIn != 1 || filteredAttendance.Summary.Eligible != 3 {
		t.Fatalf("filtered attendance = %+v, error = %v", filteredAttendance.Summary, err)
	}
	if _, err := service.Attendance(request, adminToken, AttendanceFilter{EventID: baseEventID, Gate: "Missing"}); err != ErrInvalidRequest {
		t.Fatalf("invalid gate filter error = %v", err)
	}
	handler := NewHandler(service, slog.Default())
	mux := http.NewServeMux()
	handler.Register(mux)
	call := func(path, token string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		return response
	}
	callRawQuery := func(path, rawQuery, token string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.URL.RawQuery = rawQuery
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		return response
	}
	if got := call("/api/v1/admin/reports/sales", ""); got.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated status = %d", got.Code)
	}
	if got := call("/api/v1/admin/reports/sales", staffToken); got.Code != http.StatusForbidden {
		t.Errorf("STAFF status = %d, body %s", got.Code, got.Body.String())
	}
	if got := call("/api/v1/admin/reports/sales", adminToken); got.Code != http.StatusOK || got.Header().Get("Cache-Control") != "no-store" {
		var decoded Report
		err := json.Unmarshal(got.Body.Bytes(), &decoded)
		t.Errorf("ADMIN response = %d %s; decode error %v", got.Code, got.Body.String(), err)
	}
	for _, path := range []string{"/api/v1/admin/reports/sales", "/api/v1/admin/reports/sales.csv", "/api/v1/admin/reports/attendance", "/api/v1/admin/reports/attendance.csv"} {
		if got := callRawQuery(path, "eventId=%ZZ", adminToken); got.Code != http.StatusBadRequest {
			t.Errorf("malformed query %s status = %d, body %s", path, got.Code, got.Body.String())
		}
	}
	for _, path := range []string{"/api/v1/admin/reports/sales?eventId=missing-event", "/api/v1/admin/reports/sales.csv?eventId=missing-event"} {
		if got := call(path, adminToken); got.Code != http.StatusNotFound {
			t.Errorf("unknown event %s status = %d, body %s", path, got.Code, got.Body.String())
		}
	}
	for _, path := range []string{"/api/v1/admin/reports/sales.csv?eventId=" + baseEventID + "&dateFrom=2026-09-02&dateTo=2026-09-04", "/api/v1/admin/reports/attendance.csv?eventId=" + baseEventID} {
		if got := call(path, ""); got.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated CSV %s status = %d", path, got.Code)
		}
		if got := call(path, staffToken); got.Code != http.StatusForbidden {
			t.Errorf("STAFF CSV %s status = %d, body %s", path, got.Code, got.Body.String())
		}
	}
	salesCSVResponse := call("/api/v1/admin/reports/sales.csv?eventId="+baseEventID+"&dateFrom=2026-09-02&dateTo=2026-09-04", adminToken)
	if salesCSVResponse.Code != http.StatusOK || salesCSVResponse.Header().Get("Content-Type") != "text/csv; charset=utf-8" || !strings.Contains(salesCSVResponse.Header().Get("Content-Disposition"), "admin-sales-2026-09-02-2026-09-04.csv") {
		t.Fatalf("sales CSV response = %d, headers = %v, body = %s", salesCSVResponse.Code, salesCSVResponse.Header(), salesCSVResponse.Body.String())
	}
	salesRows := readReportCSV(t, salesCSVResponse.Body.String())
	if len(salesRows) != 6 || salesRows[1][8] != "ringkasan" || salesRows[1][12] != strconv.FormatInt(report.Summary.SuccessfulTransactions, 10) || salesRows[1][13] != strconv.FormatInt(report.Summary.PaymentAmount, 10) || salesRows[1][14] != strconv.FormatInt(report.Summary.RefundAmount, 10) || salesRows[1][15] != strconv.FormatInt(report.Summary.NetAmount, 10) {
		t.Fatalf("sales CSV rows = %#v", salesRows)
	}
	if salesRows[5][8] != "event" || !strings.HasPrefix(salesRows[5][11], "'") || !strings.Contains(salesRows[5][11], "Report Event") || strings.Contains(salesCSVResponse.Body.String(), "Private Attendee") {
		t.Fatalf("sales CSV event row was not escaped or contains private attendee data: %#v", salesRows[5])
	}
	attendanceCSVResponse := call("/api/v1/admin/reports/attendance.csv?eventId="+baseEventID+"&gate=Gate", adminToken)
	if attendanceCSVResponse.Code != http.StatusOK || attendanceCSVResponse.Header().Get("Content-Type") != "text/csv; charset=utf-8" {
		t.Fatalf("attendance CSV response = %d, headers = %v, body = %s", attendanceCSVResponse.Code, attendanceCSVResponse.Header(), attendanceCSVResponse.Body.String())
	}
	attendanceRows := readReportCSV(t, attendanceCSVResponse.Body.String())
	if len(attendanceRows) != 6 || attendanceRows[1][8] != "ringkasan" || attendanceRows[1][13] != strconv.FormatInt(attendanceReport.Summary.Capacity, 10) || attendanceRows[1][14] != strconv.FormatInt(attendanceReport.Summary.Available, 10) || attendanceRows[1][15] != strconv.FormatInt(attendanceReport.Summary.Issued, 10) || attendanceRows[1][16] != strconv.FormatInt(attendanceReport.Summary.Eligible, 10) || attendanceRows[1][17] != strconv.FormatInt(attendanceReport.Summary.HeldForRefund, 10) || attendanceRows[1][18] != strconv.FormatInt(attendanceReport.Summary.CheckedIn, 10) {
		t.Fatalf("attendance CSV rows = %#v", attendanceRows)
	}
	if !strings.Contains(attendanceCSVResponse.Body.String(), expectedHour) || strings.Contains(attendanceCSVResponse.Body.String(), "Private Attendee") {
		t.Fatalf("attendance CSV omitted hourly data or contains private attendee data: %s", attendanceCSVResponse.Body.String())
	}
}

func readReportCSV(t *testing.T, body string) [][]string {
	t.Helper()
	if !strings.HasPrefix(body, "\xef\xbb\xbf") {
		t.Fatal("report CSV is missing its UTF-8 BOM")
	}
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(body, "\xef\xbb\xbf"))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
