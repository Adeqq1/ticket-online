package platform

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
	"github.com/Adeqq1/ticket-online/backend/migrations"
	_ "github.com/go-sql-driver/mysql"
)

func TestPurchaseIssuesTicketThenCheckInSucceedsOnlyOnce(t *testing.T) {
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
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	var suffix [16]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	eventID := "checkin-flow-" + hex.EncodeToString(suffix[:])
	reservationID, orderID, staffID := "", "", ""
	const zoneID, gate = "checkin-flow", "Workflow Gate"
	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx, `INSERT INTO events
		(id, artist, city, venue, address, starts_at, genre, status, publication_status, image_url, description, created_at, updated_at)
		VALUES (?, 'Workflow Test', 'Jakarta', 'Test Venue', 'Test Address', ?, 'Test', 'PRESALE', 'PUBLISHED', '', '', ?, ?)`, eventID, now.Add(24*time.Hour), now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO event_zones (event_id, slug, name, description) VALUES (?, ?, 'Test Zone', '')", eventID, zoneID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO ticket_tiers
		(event_id, slug, name, zone_slug, price, capacity, available_quantity, max_per_order, benefit, gate, seating_mode, created_at, updated_at)
		VALUES (?, 'workflow', 'Workflow Ticket', ?, 10000, 2, 2, 2, '', ?, 'FREE_STANDING', ?, ?)`, eventID, zoneID, gate, now, now); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM checkin_attempts WHERE event_id = ?", eventID)
		_, _ = db.Exec("DELETE FROM ticket_checkins WHERE event_id = ?", eventID)
		_, _ = db.Exec("DELETE FROM staff_sessions WHERE staff_id = ?", staffID)
		_, _ = db.Exec("DELETE FROM staff_assignments WHERE staff_id = ?", staffID)
		_, _ = db.Exec("DELETE FROM staff_users WHERE id = ?", staffID)
		_, _ = db.Exec("DELETE FROM etickets WHERE order_id = ?", orderID)
		_, _ = db.Exec("DELETE FROM order_attendees WHERE order_id = ?", orderID)
		_, _ = db.Exec("DELETE FROM payments WHERE order_id = ?", orderID)
		_, _ = db.Exec("DELETE FROM order_items WHERE order_id = ?", orderID)
		_, _ = db.Exec("DELETE FROM order_buyers WHERE order_id = ?", orderID)
		_, _ = db.Exec("DELETE FROM orders WHERE id = ?", orderID)
		_, _ = db.Exec("DELETE FROM reservation_items WHERE reservation_id = ?", reservationID)
		_, _ = db.Exec("DELETE FROM reservations WHERE id = ?", reservationID)
		_, _ = db.Exec("DELETE FROM ticket_tiers WHERE event_id = ?", eventID)
		_, _ = db.Exec("DELETE FROM event_zones WHERE event_id = ?", eventID)
		_, _ = db.Exec("DELETE FROM events WHERE id = ?", eventID)
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandlerWithOrderAccess(db, logger, 10*time.Minute, "", true, []byte("0123456789abcdef0123456789abcdef"))
	call := func(method, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, bytes.NewReader(body))
		for name, value := range headers {
			request.Header.Set(name, value)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	checkStatus := func(response *httptest.ResponseRecorder, want int) {
		t.Helper()
		if response.Code != want {
			t.Fatalf("request returned %d, want %d: %s", response.Code, want, response.Body.String())
		}
	}

	reserveBody, _ := json.Marshal(map[string]any{"eventId": eventID, "items": []map[string]any{{"tierId": "workflow", "quantity": 1}}})
	reservationKey := "workflow-reservation-" + eventID
	reserved := call(http.MethodPost, "/api/v1/reservations", reserveBody, map[string]string{"Idempotency-Key": reservationKey})
	checkStatus(reserved, http.StatusCreated)
	var reservation struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(reserved.Body.Bytes(), &reservation); err != nil {
		t.Fatal(err)
	}
	reservationID = reservation.ID
	checkoutBody, _ := json.Marshal(map[string]any{
		"buyer":     map[string]string{"name": "Test Buyer", "email": "workflow@example.com", "phone": "081234567890", "identity": "123456789012"},
		"attendees": []map[string]any{{"tierId": "workflow", "names": []string{"Workflow Guest"}}},
	})
	ordered := call(http.MethodPost, "/api/v1/reservations/"+reservationID+"/checkout", checkoutBody, map[string]string{"Idempotency-Key": reservationKey})
	checkStatus(ordered, http.StatusCreated)
	var order struct {
		ID          string `json:"id"`
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(ordered.Body.Bytes(), &order); err != nil || order.ID == "" || order.AccessToken == "" {
		t.Fatalf("checkout response = %+v; error = %v", order, err)
	}
	orderID = order.ID
	paid := call(http.MethodPost, "/api/v1/orders/"+order.ID+"/simulate-payment", []byte(`{"method":"QRIS","result":"SUCCEEDED"}`), map[string]string{"Authorization": "Bearer " + order.AccessToken, "Content-Type": "application/json"})
	checkStatus(paid, http.StatusCreated)
	var payment struct {
		Tickets []struct {
			ID   string `json:"id"`
			Code string `json:"code"`
			Gate string `json:"gate"`
		} `json:"tickets"`
	}
	if err := json.Unmarshal(paid.Body.Bytes(), &payment); err != nil || len(payment.Tickets) != 1 || payment.Tickets[0].Code != "ET-"+strings.ToUpper(payment.Tickets[0].ID) || payment.Tickets[0].Gate != gate {
		t.Fatalf("payment tickets = %+v; error = %v", payment.Tickets, err)
	}

	staff := staffauth.New(db)
	created, err := staff.CreateStaff(ctx, "Workflow Staff", "workflow-"+eventID+"@example.com", "workflow staff password", []staffauth.Assignment{{EventID: eventID, Gate: gate}})
	if err != nil {
		t.Fatal(err)
	}
	staffID = created.ID
	loginBody, _ := json.Marshal(map[string]string{"email": created.Email, "password": "workflow staff password"})
	loggedIn := call(http.MethodPost, "/api/v1/staff/login", loginBody, map[string]string{"Content-Type": "application/json"})
	checkStatus(loggedIn, http.StatusOK)
	var session struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(loggedIn.Body.Bytes(), &session); err != nil || session.AccessToken == "" {
		t.Fatalf("login response = %+v; error = %v", session, err)
	}
	checkinBody, _ := json.Marshal(map[string]string{"eventId": eventID, "gate": gate, "code": payment.Tickets[0].Code})
	staffHeaders := map[string]string{"Authorization": "Bearer " + session.AccessToken, "Content-Type": "application/json"}
	first := call(http.MethodPost, "/api/v1/staff/check-ins", checkinBody, staffHeaders)
	checkStatus(first, http.StatusCreated)
	var checkinResult struct {
		Status      string `json:"status"`
		CheckedInAt string `json:"checkedInAt"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &checkinResult); err != nil || checkinResult.Status != "CHECKED_IN" || checkinResult.CheckedInAt == "" {
		t.Fatalf("check-in response = %+v; error = %v", checkinResult, err)
	}
	// A gate device can lose the response after the database commits. Recover by
	// reading the server status; the read must preserve the recorded timestamp.
	statusResponse := call(http.MethodGet, "/api/v1/staff/ticket-status?code="+payment.Tickets[0].Code, nil, map[string]string{"Authorization": "Bearer " + session.AccessToken})
	checkStatus(statusResponse, http.StatusOK)
	if statusResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("ticket status cache-control = %q", statusResponse.Header().Get("Cache-Control"))
	}
	var recovered struct {
		Status      string `json:"status"`
		CheckedInAt string `json:"checkedInAt"`
	}
	if err := json.Unmarshal(statusResponse.Body.Bytes(), &recovered); err != nil || recovered.Status != "CHECKED_IN" || recovered.CheckedInAt != checkinResult.CheckedInAt {
		t.Fatalf("recovered status = %+v; error = %v, want original server timestamp", recovered, err)
	}
	second := call(http.MethodPost, "/api/v1/staff/check-ins", checkinBody, staffHeaders)
	checkStatus(second, http.StatusConflict)
	if !strings.Contains(second.Body.String(), "TICKET_ALREADY_USED") {
		t.Fatalf("second check-in response = %s; want TICKET_ALREADY_USED", second.Body.String())
	}
	if _, err := db.ExecContext(ctx, "UPDATE staff_users SET role = 'ADMIN' WHERE id = ?", staffID); err != nil {
		t.Fatal(err)
	}
	adminCheckin := call(http.MethodPost, "/api/v1/staff/check-ins", checkinBody, staffHeaders)
	checkStatus(adminCheckin, http.StatusForbidden)
	var records int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ticket_checkins WHERE event_id = ?", eventID).Scan(&records); err != nil || records != 1 {
		t.Fatalf("check-in records = %d; error = %v", records, err)
	}
	var attempts int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM checkin_attempts WHERE event_id = ?", eventID).Scan(&attempts); err != nil || attempts != 2 {
		t.Fatalf("check-in attempts = %d; error = %v, want success and duplicate only", attempts, err)
	}
}
