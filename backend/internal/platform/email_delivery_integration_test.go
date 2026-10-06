package platform

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha512"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/email"
	"github.com/Adeqq1/ticket-online/backend/migrations"
	_ "github.com/go-sql-driver/mysql"
)

// TestEmailFailureWebhookReplayAndCrossDeviceRecovery pays through the Midtrans webhook, lets SMTP fail,
// replays the webhook, and recovers the order from a device that has no stored access.
func TestEmailFailureWebhookReplayAndCrossDeviceRecovery(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to a disposable MySQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
	suffix := randomHex(t, 16)
	eventID := "email-delivery-" + suffix
	const zoneID, gate, serverKey = "email-delivery", "Email Gate", "sandbox-test-key"
	reservationID, orderID, reference := "", "", ""
	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx, `INSERT INTO events
		(id, artist, city, venue, address, starts_at, genre, status, image_url, description, created_at, updated_at)
		VALUES (?, 'Email Test', 'Jakarta', 'Test Venue', 'Test Address', ?, 'Test', 'PRESALE', '', '', ?, ?)`, eventID, now.Add(72*time.Hour), now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO event_zones (event_id, slug, name, description) VALUES (?, ?, 'Test Zone', '')", eventID, zoneID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO ticket_tiers
		(event_id, slug, name, zone_slug, price, capacity, available_quantity, max_per_order, benefit, gate, seating_mode, created_at, updated_at)
		VALUES (?, 'email', 'Email Ticket', ?, 10000, 2, 2, 2, '', ?, 'FREE_STANDING', ?, ?)`, eventID, zoneID, gate, now, now); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM recovery_tokens WHERE order_id = ?", orderID)
		_, _ = db.Exec("DELETE FROM email_queue WHERE recovery_request_id IN (SELECT id FROM recovery_requests WHERE reference = ?)", reference)
		_, _ = db.Exec("DELETE FROM recovery_requests WHERE reference = ?", reference)
		_, _ = db.Exec("DELETE FROM email_queue WHERE order_id = ?", orderID)
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

	secret := []byte("0123456789abcdef0123456789abcdef")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandlerWithPaymentConfig(db, logger, 10*time.Minute, "", false, secret, serverKey, "http://localhost:5173")
	ipBytes := make([]byte, 3)
	_, _ = rand.Read(ipBytes)
	clientIP := fmt.Sprintf("10.%d.%d.%d:5000", ipBytes[0], ipBytes[1], ipBytes[2])
	call := func(method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
		var encoded []byte
		if body != nil {
			encoded, _ = json.Marshal(body)
		}
		request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
		request.RemoteAddr = clientIP
		request.Header.Set("Content-Type", "application/json")
		for name, value := range headers {
			request.Header.Set(name, value)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	bearer := func(token string) map[string]string { return map[string]string{"Authorization": "Bearer " + token} }
	expect := func(response *httptest.ResponseRecorder, want int) {
		t.Helper()
		if response.Code != want {
			t.Fatalf("request returned %d, want %d: %s", response.Code, want, response.Body.String())
		}
	}
	count := func(query string, args ...any) int {
		t.Helper()
		var value int
		if err := db.QueryRowContext(ctx, query, args...).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	waitFor := func(what, query string, args ...any) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for count(query, args...) == 0 {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	key := "email-delivery-" + suffix
	expect(call(http.MethodPost, "/api/v1/reservations", map[string]any{"eventId": eventID, "items": []map[string]any{{"tierId": "email", "quantity": 1}}}, map[string]string{"Idempotency-Key": key}), http.StatusCreated)
	if err := db.QueryRowContext(ctx, "SELECT id FROM reservations WHERE idempotency_key = ?", key).Scan(&reservationID); err != nil {
		t.Fatal(err)
	}
	buyerEmail := "delivery-" + suffix[:8] + "@example.com"
	ordered := call(http.MethodPost, "/api/v1/reservations/"+reservationID+"/checkout", map[string]any{
		"buyer":     map[string]string{"name": "Delivery Buyer", "email": buyerEmail, "phone": "081234567890", "identity": "123456789012"},
		"attendees": []map[string]any{{"tierId": "email", "names": []string{"Delivery Guest"}}},
	}, map[string]string{"Idempotency-Key": key})
	expect(ordered, http.StatusCreated)
	var order struct {
		ID, Reference, AccessToken string
		Total                      uint64
	}
	_ = json.Unmarshal(ordered.Body.Bytes(), &order)
	orderID, reference = order.ID, order.Reference
	gatewayOrderID := "TO-" + suffix[:20]
	if _, err := db.ExecContext(ctx, `INSERT INTO payments (id, order_id, method, amount, status, gateway_order_id, created_at, updated_at)
		VALUES (?, ?, 'QRIS', ?, 'PENDING', ?, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`, randomHex(t, 16), orderID, order.Total, gatewayOrderID); err != nil {
		t.Fatal(err)
	}
	grossAmount := fmt.Sprintf("%d.00", order.Total)
	signature := sha512.Sum512([]byte(gatewayOrderID + "200" + grossAmount + serverKey))
	settlement := map[string]string{"order_id": gatewayOrderID, "status_code": "200", "gross_amount": grossAmount, "signature_key": hex.EncodeToString(signature[:]), "transaction_status": "settlement"}

	// A forged webhook is rejected before it can touch the order.
	forged := map[string]string{}
	for name, value := range settlement {
		forged[name] = value
	}
	forged["signature_key"] = strings.Repeat("0", 128)
	expect(call(http.MethodPost, "/api/v1/payments/midtrans/notification", forged, nil), http.StatusUnauthorized)
	if count("SELECT COUNT(*) FROM orders WHERE id = ? AND status = 'PENDING'", orderID) != 1 {
		t.Fatal("forged webhook changed the order")
	}

	// The provider may deliver the same settlement concurrently; tickets are issued once.
	var group sync.WaitGroup
	codes := make([]int, 3)
	for index := range codes {
		group.Add(1)
		go func() {
			defer group.Done()
			codes[index] = call(http.MethodPost, "/api/v1/payments/midtrans/notification", settlement, nil).Code
		}()
	}
	group.Wait()
	for _, code := range codes {
		if code != http.StatusOK {
			t.Fatalf("concurrent webhook codes = %v", codes)
		}
	}
	paidState := func() {
		t.Helper()
		if count("SELECT COUNT(*) FROM orders WHERE id = ? AND status = 'PAID'", orderID) != 1 ||
			count("SELECT COUNT(*) FROM payments WHERE order_id = ? AND status = 'SUCCEEDED'", orderID) != 1 ||
			count("SELECT COUNT(*) FROM etickets WHERE order_id = ?", orderID) != 1 {
			t.Fatal("order is not exactly PAID with one payment and one e-ticket")
		}
	}
	paidState()

	// SMTP rejects every message: the job retries up to three attempts, then stops as FAILED.
	var reject atomic.Bool
	reject.Store(true)
	port, mails := fakeSMTP(t, &reject)
	workerCtx, stopWorker := context.WithCancel(ctx)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		email.NewService(db, email.Config{Host: "127.0.0.1", Port: port, From: "tickets@example.com", TLSMode: "none", FrontendURL: "http://localhost:5173", AccessSecret: secret}, logger).Run(workerCtx, 50*time.Millisecond)
	}()
	t.Cleanup(func() { stopWorker(); <-workerDone })
	nextMail := func(subject string) capturedMail {
		t.Helper()
		deadline := time.After(20 * time.Second)
		for {
			select {
			case got := <-mails:
				if got.to == buyerEmail && strings.Contains(got.subject, subject) {
					return got
				}
			case <-deadline:
				t.Fatalf("no %q email", subject)
			}
		}
	}
	const job = "SELECT COUNT(*) FROM email_queue WHERE order_id = ? AND kind = 'TICKETS' AND attempts = ? AND status = ? AND last_error LIKE 'SMTP%'"
	waitFor("first failed attempt", job, orderID, 1, "PENDING")

	// A webhook replayed after the email failure changes nothing and does not queue a second email.
	expect(call(http.MethodPost, "/api/v1/payments/midtrans/notification", settlement, nil), http.StatusOK)
	paidState()
	if count("SELECT COUNT(*) FROM email_queue WHERE order_id = ?", orderID) != 1 || count(job, orderID, 1, "PENDING") != 1 {
		t.Fatal("webhook replay duplicated or reset the ticket email job")
	}
	// Skip the 1 and 5 minute backoff instead of waiting for it.
	for _, step := range []struct {
		attempts int
		status   string
	}{{2, "PENDING"}, {3, "FAILED"}} {
		if _, err := db.ExecContext(ctx, "UPDATE email_queue SET next_attempt_at = UTC_TIMESTAMP(6) WHERE order_id = ? AND status = 'PENDING'", orderID); err != nil {
			t.Fatal(err)
		}
		waitFor(fmt.Sprintf("attempt %d", step.attempts), job, orderID, step.attempts, step.status)
	}
	time.Sleep(300 * time.Millisecond)
	if count(job, orderID, 3, "FAILED") != 1 {
		t.Fatal("exhausted email job was retried beyond three attempts")
	}
	paidState()

	// SMTP recovers; resend restarts the same job with a fresh attempt budget.
	reject.Store(false)
	for len(mails) > 0 {
		<-mails
	}
	expect(call(http.MethodPost, "/api/v1/orders/"+orderID+"/resend-email", nil, bearer(order.AccessToken)), http.StatusAccepted)
	ticketMail := nextMail("E-ticket")
	waitFor("resent email", "SELECT COUNT(*) FROM email_queue WHERE order_id = ? AND status = 'SENT' AND attempts = 1 AND last_error = ''", orderID)

	// Device B has no stored access. The emailed ticket link alone opens the ticket.
	expect(call(http.MethodGet, "/api/v1/orders/"+orderID, nil, nil), http.StatusUnauthorized)
	link := regexp.MustCompile(`/tiket/([0-9a-f]{32})#access_token=([A-Za-z0-9_-]{43})`).FindStringSubmatch(ticketMail.plain)
	if link == nil {
		t.Fatalf("ticket email has no ticket link: %q", ticketMail.plain)
	}
	expect(call(http.MethodGet, "/api/v1/tickets/"+link[1], nil, bearer(link[2])), http.StatusOK)

	// Device B recovers the whole order with email and reference.
	expect(call(http.MethodPost, "/api/v1/ticket-recovery", map[string]string{"email": buyerEmail, "reference": reference}, nil), http.StatusAccepted)
	recoveryMail := nextMail("Pulihkan")
	match := regexp.MustCompile(`/pulihkan-tiket#token=([A-Za-z0-9_-]{43})`).FindStringSubmatch(recoveryMail.plain)
	if match == nil {
		t.Fatalf("recovery email has no link: %q", recoveryMail.plain)
	}
	verify := func() *httptest.ResponseRecorder {
		return call(http.MethodPost, "/api/v1/ticket-recovery/verify", map[string]string{"token": match[1]}, nil)
	}
	// An expired token is refused without being consumed.
	if _, err := db.ExecContext(ctx, "UPDATE recovery_tokens SET expires_at = UTC_TIMESTAMP(6) - INTERVAL 1 SECOND WHERE order_id = ?", orderID); err != nil {
		t.Fatal(err)
	}
	expect(verify(), http.StatusGone)
	if count("SELECT COUNT(*) FROM recovery_tokens WHERE order_id = ? AND used_at IS NULL", orderID) != 1 {
		t.Fatal("expired token rejection consumed the token")
	}
	if _, err := db.ExecContext(ctx, "UPDATE recovery_tokens SET expires_at = UTC_TIMESTAMP(6) + INTERVAL 10 MINUTE WHERE order_id = ?", orderID); err != nil {
		t.Fatal(err)
	}
	verified := verify()
	expect(verified, http.StatusOK)
	var recovered struct {
		OrderID, AccessToken string
		TicketIDs            []string
	}
	_ = json.Unmarshal(verified.Body.Bytes(), &recovered)
	if recovered.OrderID != orderID || len(recovered.TicketIDs) != 1 || recovered.TicketIDs[0] != link[1] {
		t.Fatalf("recovered access = %+v", recovered)
	}
	expect(call(http.MethodGet, "/api/v1/orders/"+orderID, nil, bearer(recovered.AccessToken)), http.StatusOK)
	expect(call(http.MethodGet, "/api/v1/orders/"+orderID+"/tickets", nil, bearer(recovered.AccessToken)), http.StatusOK)
	// Reusing the link fails.
	expect(verify(), http.StatusGone)
}
