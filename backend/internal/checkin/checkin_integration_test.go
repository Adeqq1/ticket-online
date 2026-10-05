package checkin

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
	"github.com/Adeqq1/ticket-online/backend/migrations"
	_ "github.com/go-sql-driver/mysql"
)

func TestCheckInAuthorizationGateAndSingleUse(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to a disposable MySQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

	eventID, err := checkinID()
	if err != nil {
		t.Fatal(err)
	}
	eventID = "checkin-" + eventID
	zoneID := "checkin"
	gateA, gateB := "Check-in Gate A", "Check-in Gate B"
	if _, err := db.ExecContext(ctx, `INSERT INTO events
		(id, artist, city, venue, address, starts_at, genre, status, image_url, description, created_at, updated_at)
		VALUES (?, 'Check-in Test', 'Jakarta', 'Venue', 'Address', ?, 'Test', 'PRESALE', '', '', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`, eventID, time.Now().Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO event_zones (event_id, slug, name, description) VALUES (?, ?, 'Check-in', 'Test')`, eventID, zoneID); err != nil {
		t.Fatal(err)
	}
	result, err := db.ExecContext(ctx, `INSERT INTO ticket_tiers
		(event_id, slug, name, zone_slug, price, capacity, available_quantity, max_per_order, benefit, gate, seating_mode, created_at, updated_at)
		VALUES (?, 'checkin', 'Check-in Ticket', ?, 10000, 10, 10, 4, '', ?, 'FREE_STANDING', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`, eventID, zoneID, gateA)
	if err != nil {
		t.Fatal(err)
	}
	tierID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	reservationID, err := checkinID()
	if err != nil {
		t.Fatal(err)
	}
	orderID, err := checkinID()
	if err != nil {
		t.Fatal(err)
	}
	orderReference := "TO-" + strings.ToUpper(orderID[:10])
	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx, `INSERT INTO reservations
		(id, event_id, status, idempotency_key, request_hash, order_reference, expires_at, created_at, updated_at)
		VALUES (?, ?, 'CONVERTED', ?, REPEAT('a', 64), ?, ?, ?, ?)`, reservationID, eventID, "checkin-"+reservationID, orderReference, now.Add(time.Hour), now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO orders
		(id, reference, reservation_id, status, subtotal, admin_fee, discount, created_at, updated_at, expires_at)
		VALUES (?, ?, ?, 'PAID', 30000, 7500, 0, ?, ?, ?)`, orderID, orderReference, reservationID, now, now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO order_items (order_id, ticket_tier_id, tier_name, quantity, unit_price)
		VALUES (?, ?, 'Check-in Ticket', 3, 10000)`, orderID, tierID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO order_buyers (order_id, name, email, phone, identity)
		VALUES (?, 'Check-in Buyer', ?, '081234567890', '123456789012')`, orderID, "checkin-"+reservationID+"@example.com"); err != nil {
		t.Fatal(err)
	}
	attendeeNames := []string{"Attendee One", "Attendee Two", "Attendee Three"}
	for i, name := range attendeeNames {
		if _, err := db.ExecContext(ctx, `INSERT INTO order_attendees (order_id, ticket_tier_id, ticket_number, name) VALUES (?, ?, ?, ?)`, orderID, tierID, i+1, name); err != nil {
			t.Fatal(err)
		}
	}
	ticketIDs := make([]string, 3)
	for i, name := range attendeeNames {
		ticketIDs[i], err = checkinID()
		if err != nil {
			t.Fatal(err)
		}
		gate := gateA
		if i == 1 {
			gate = gateB
		}
		snapshot, err := json.Marshal(Ticket{ID: ticketIDs[i], Code: "ET-" + strings.ToUpper(ticketIDs[i]), AttendeeName: name, TierName: "Check-in Ticket", EventID: eventID, Gate: gate})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO etickets (id, order_id, ticket_tier_id, ticket_number, snapshot, issued_at) VALUES (?, ?, ?, ?, ?, ?)`, ticketIDs[i], orderID, tierID, i+1, snapshot, now); err != nil {
			t.Fatal(err)
		}
	}

	var staffIDs [2]string
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM ticket_checkins WHERE event_id = ?", eventID)
		_, _ = db.Exec("DELETE FROM etickets WHERE order_id = ?", orderID)
		_, _ = db.Exec("DELETE FROM order_attendees WHERE order_id = ?", orderID)
		_, _ = db.Exec("DELETE FROM order_buyers WHERE order_id = ?", orderID)
		_, _ = db.Exec("DELETE FROM order_items WHERE order_id = ?", orderID)
		_, _ = db.Exec("DELETE FROM orders WHERE id = ?", orderID)
		_, _ = db.Exec("DELETE FROM reservation_items WHERE reservation_id = ?", reservationID)
		_, _ = db.Exec("DELETE FROM reservations WHERE id = ?", reservationID)
		if staffIDs[0] != "" || staffIDs[1] != "" {
			_, _ = db.Exec("DELETE FROM staff_assignments WHERE staff_id IN (?, ?)", staffIDs[0], staffIDs[1])
			_, _ = db.Exec("DELETE FROM staff_sessions WHERE staff_id IN (?, ?)", staffIDs[0], staffIDs[1])
			_, _ = db.Exec("DELETE FROM staff_users WHERE id IN (?, ?)", staffIDs[0], staffIDs[1])
		}
		_, _ = db.Exec("DELETE FROM ticket_tiers WHERE id = ?", tierID)
		_, _ = db.Exec("DELETE FROM event_zones WHERE event_id = ?", eventID)
		_, _ = db.Exec("DELETE FROM events WHERE id = ?", eventID)
	})

	staff := staffauth.New(db)
	validStaff, err := staff.CreateStaff(ctx, "Check-in Staff", "staff-"+reservationID+"@example.com", "check-in staff password", []staffauth.Assignment{{EventID: eventID, Gate: gateA}})
	if err != nil {
		t.Fatal(err)
	}
	staffIDs[0] = validStaff.ID
	otherStaff, err := staff.CreateStaff(ctx, "Other Gate Staff", "other-"+reservationID+"@example.com", "other staff password", []staffauth.Assignment{{EventID: "nusa-malam", Gate: "Gate C"}})
	if err != nil {
		t.Fatal(err)
	}
	staffIDs[1] = otherStaff.ID
	validSession, err := staff.Login(ctx, validStaff.Email, "check-in staff password", "192.0.2.20:4000")
	if err != nil {
		t.Fatal(err)
	}
	otherSession, err := staff.Login(ctx, otherStaff.Email, "other staff password", "192.0.2.21:4000")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(NewService(db, staff), slog.New(slog.NewTextHandler(io.Discard, nil)))

	call := func(token, code, event, gate string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(Request{EventID: event, Gate: gate, Code: code})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/staff/check-ins", bytes.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		handler.CheckIn(response, req)
		return response
	}
	code := func(id string) string { return "ET-" + strings.ToUpper(id) }

	if response := call("", code(ticketIDs[0]), eventID, gateA); response.Code != http.StatusUnauthorized || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("anonymous scan status=%d cache=%q body=%s", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	if response := call(validSession.AccessToken, code(ticketIDs[1]), eventID, gateA); response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"expectedGate":"`+gateB+`"`) {
		t.Fatalf("wrong gate status=%d body=%s", response.Code, response.Body.String())
	}
	if response := call(otherSession.AccessToken, code(ticketIDs[0]), eventID, gateA); response.Code != http.StatusForbidden {
		t.Fatalf("unassigned gate status=%d body=%s", response.Code, response.Body.String())
	}
	if response := call(validSession.AccessToken, code(strings.Repeat("f", 32)), eventID, gateA); response.Code != http.StatusNotFound {
		t.Fatalf("unknown ticket status=%d body=%s", response.Code, response.Body.String())
	}
	if _, err := db.ExecContext(ctx, "UPDATE orders SET status = 'PENDING' WHERE id = ?", orderID); err != nil {
		t.Fatal(err)
	}
	if response := call(validSession.AccessToken, code(ticketIDs[0]), eventID, gateA); response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "ORDER_NOT_PAID") {
		t.Fatalf("unpaid order status=%d body=%s", response.Code, response.Body.String())
	}
	if _, err := db.ExecContext(ctx, "UPDATE orders SET status = 'PAID' WHERE id = ?", orderID); err != nil {
		t.Fatal(err)
	}
	first := call(validSession.AccessToken, code(ticketIDs[0]), eventID, gateA)
	if first.Code != http.StatusCreated {
		t.Fatalf("valid check-in status=%d body=%s", first.Code, first.Body.String())
	}
	var checkinResult Result
	if err := json.Unmarshal(first.Body.Bytes(), &checkinResult); err != nil || checkinResult.Status != "CHECKED_IN" || checkinResult.Ticket.ID != ticketIDs[0] || checkinResult.CheckedInAt.IsZero() {
		t.Fatalf("valid check-in response = %+v, error=%v", checkinResult, err)
	}
	if response := call(validSession.AccessToken, code(ticketIDs[0]), eventID, gateA); response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "TICKET_ALREADY_USED") || !strings.Contains(response.Body.String(), checkinResult.CheckedInAt.Format(time.RFC3339Nano)) {
		t.Fatalf("repeat check-in status=%d body=%s; want original check-in time", response.Code, response.Body.String())
	}

	start := make(chan struct{})
	statuses := make(chan int, 8)
	var wg sync.WaitGroup
	for range cap(statuses) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			statuses <- call(validSession.AccessToken, code(ticketIDs[2]), eventID, gateA).Code
		}()
	}
	close(start)
	wg.Wait()
	close(statuses)
	created, conflicts := 0, 0
	for status := range statuses {
		switch status {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		default:
			t.Errorf("concurrent check-in status = %d", status)
		}
	}
	if created != 1 || conflicts != 7 {
		t.Fatalf("concurrent check-ins: created=%d conflicts=%d; want 1 and 7", created, conflicts)
	}
	var records int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ticket_checkins WHERE event_id = ?", eventID).Scan(&records); err != nil {
		t.Fatal(err)
	}
	if records != 2 {
		t.Fatalf("check-in records=%d, want two distinct tickets", records)
	}
	expiredSession, err := staff.Login(ctx, validStaff.Email, "check-in staff password", "192.0.2.20:4001")
	if err != nil {
		t.Fatal(err)
	}
	expiredTokenHash := sha256.Sum256([]byte(expiredSession.AccessToken))
	if _, err := db.ExecContext(ctx, "UPDATE staff_sessions SET expires_at = UTC_TIMESTAMP(6) - INTERVAL 1 SECOND WHERE token_hash = ?", expiredTokenHash[:]); err != nil {
		t.Fatal(err)
	}
	if response := call(expiredSession.AccessToken, code(ticketIDs[1]), eventID, gateA); response.Code != http.StatusUnauthorized {
		t.Fatalf("expired staff session status=%d body=%s", response.Code, response.Body.String())
	}
	// The check-in authorization locks staff and session rows before the write.
	// When those shared locks are acquired first, revocation waits for that
	// transaction; requests after the revocation commits must then be rejected.
	withAuthorizationLock := func(token string, revoke func(context.Context) error) {
		t.Helper()
		tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := staff.AuthorizeGateTx(ctx, tx, token, eventID, gateA); err != nil {
			_ = tx.Rollback()
			t.Fatalf("authorize before revocation: %v", err)
		}
		lockCtx, stopLockWait := context.WithTimeout(ctx, 250*time.Millisecond)
		defer stopLockWait()
		started := make(chan struct{})
		finished := make(chan error, 1)
		go func() {
			close(started)
			finished <- revoke(lockCtx)
		}()
		<-started
		if err := <-finished; !errors.Is(err, context.DeadlineExceeded) {
			_ = tx.Rollback()
			t.Fatalf("revocation should wait for the authorized check-in lock, got %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		if err := revoke(ctx); err != nil {
			t.Fatalf("revocation after authorized check-in: %v", err)
		}
	}

	withAuthorizationLock(validSession.AccessToken, func(lockCtx context.Context) error {
		return staff.ReplaceAssignments(lockCtx, validStaff.ID, []staffauth.Assignment{})
	})
	if response := call(validSession.AccessToken, code(ticketIDs[1]), eventID, gateA); response.Code != http.StatusForbidden {
		t.Fatalf("check-in after assignment revocation status=%d body=%s", response.Code, response.Body.String())
	}
	if err := staff.ReplaceAssignments(ctx, validStaff.ID, []staffauth.Assignment{{EventID: eventID, Gate: gateA}}); err != nil {
		t.Fatal(err)
	}
	active := false
	withAuthorizationLock(validSession.AccessToken, func(lockCtx context.Context) error {
		return staff.UpdateStaff(lockCtx, validStaff.ID, nil, &active)
	})
	if response := call(validSession.AccessToken, code(ticketIDs[1]), eventID, gateA); response.Code != http.StatusUnauthorized {
		t.Fatalf("check-in after staff deactivation status=%d body=%s", response.Code, response.Body.String())
	}
	active = true
	if err := staff.UpdateStaff(ctx, validStaff.ID, nil, &active); err != nil {
		t.Fatal(err)
	}
	logoutSession, err := staff.Login(ctx, validStaff.Email, "check-in staff password", "192.0.2.20:4003")
	if err != nil {
		t.Fatal(err)
	}
	withAuthorizationLock(logoutSession.AccessToken, func(lockCtx context.Context) error {
		return staff.Logout(lockCtx, logoutSession.AccessToken)
	})
	if response := call(logoutSession.AccessToken, code(ticketIDs[1]), eventID, gateA); response.Code != http.StatusUnauthorized {
		t.Fatalf("check-in after logout status=%d body=%s", response.Code, response.Body.String())
	}
}

func checkinID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}
