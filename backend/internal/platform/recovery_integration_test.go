package platform

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"net/netip"
	"net/textproto"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/email"
	"github.com/Adeqq1/ticket-online/backend/migrations"
	_ "github.com/go-sql-driver/mysql"
)

type capturedMail struct{ to, subject, plain string }

// fakeSMTP hands back the decoded text/plain part of every message. While reject is set it still
// captures the message but answers the final DATA with 451, like a server that lost the acknowledgement.
func fakeSMTP(t *testing.T, reject *atomic.Bool) (int, <-chan capturedMail) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	mails := make(chan capturedMail, 50)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveSMTP(conn, mails, reject)
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port, mails
}

func serveSMTP(conn net.Conn, mails chan<- capturedMail, reject *atomic.Bool) {
	defer conn.Close()
	text := textproto.NewConn(conn)
	_ = text.PrintfLine("220 test")
	to := ""
	for {
		line, err := text.ReadLine()
		if err != nil {
			return
		}
		switch {
		case strings.HasPrefix(line, "RCPT TO:"):
			to = strings.Trim(strings.TrimPrefix(line, "RCPT TO:"), "<>")
			_ = text.PrintfLine("250 ok")
		case line == "DATA":
			_ = text.PrintfLine("354 go")
			raw, err := io.ReadAll(text.DotReader())
			if err != nil {
				return
			}
			captured := capturedMail{to: to}
			if message, err := mail.ReadMessage(bytes.NewReader(raw)); err == nil {
				captured.subject = message.Header.Get("Subject")
				if _, params, err := mime.ParseMediaType(message.Header.Get("Content-Type")); err == nil {
					if part, err := multipart.NewReader(message.Body, params["boundary"]).NextPart(); err == nil {
						plain, _ := io.ReadAll(part)
						captured.plain = string(plain)
					}
				}
			}
			mails <- captured
			if reject != nil && reject.Load() {
				_ = text.PrintfLine("451 temporary failure")
				continue
			}
			_ = text.PrintfLine("250 queued")
		default:
			_ = text.PrintfLine("250 ok")
		}
	}
}

func randomHex(t *testing.T, size int) string {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(bytes)
}

func TestTicketRecoveryAndResendEmail(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to a disposable MySQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
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
	eventID := "recovery-flow-" + suffix
	const zoneID, gate = "recovery-flow", "Recovery Gate"
	reservationID, orderID, reference := "", "", ""
	var extraReferences []string
	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx, `INSERT INTO events
		(id, artist, city, venue, address, starts_at, genre, status, publication_status, image_url, description, created_at, updated_at)
		VALUES (?, 'Recovery Test', 'Jakarta', 'Test Venue', 'Test Address', ?, 'Test', 'PRESALE', 'PUBLISHED', '', '', ?, ?)`, eventID, now.Add(72*time.Hour), now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO event_zones (event_id, slug, name, description) VALUES (?, ?, 'Test Zone', '')", eventID, zoneID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO ticket_tiers
		(event_id, slug, name, zone_slug, price, capacity, available_quantity, max_per_order, benefit, gate, seating_mode, created_at, updated_at)
		VALUES (?, 'recovery', 'Recovery Ticket', ?, 10000, 2, 2, 2, '', ?, 'FREE_STANDING', ?, ?)`, eventID, zoneID, gate, now, now); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		references := append([]string{reference}, extraReferences...)
		for _, ref := range references {
			_, _ = db.Exec("DELETE FROM recovery_tokens WHERE request_id IN (SELECT id FROM recovery_requests WHERE reference = ?)", ref)
			_, _ = db.Exec("DELETE FROM email_queue WHERE recovery_request_id IN (SELECT id FROM recovery_requests WHERE reference = ?)", ref)
			_, _ = db.Exec("DELETE FROM recovery_requests WHERE reference = ?", ref)
		}
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
	handler := NewHandlerWithOrderAccess(db, logger, 10*time.Minute, "", true, secret)
	ipBytes := make([]byte, 3)
	_, _ = rand.Read(ipBytes)
	clientIP := fmt.Sprintf("10.%d.%d.%d:4000", ipBytes[0], ipBytes[1], ipBytes[2])
	call := func(h http.Handler, ip, method, path string, body any, token string) *httptest.ResponseRecorder {
		encoded, _ := json.Marshal(body)
		request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
		request.RemoteAddr = ip
		request.Header.Set("Content-Type", "application/json")
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		h.ServeHTTP(response, request)
		return response
	}
	expect := func(response *httptest.ResponseRecorder, want int) {
		t.Helper()
		if response.Code != want {
			t.Fatalf("request returned %d, want %d: %s", response.Code, want, response.Body.String())
		}
	}

	key := "recovery-flow-" + suffix
	reserveRequest := httptest.NewRequest(http.MethodPost, "/api/v1/reservations", strings.NewReader(fmt.Sprintf(`{"eventId":%q,"items":[{"tierId":"recovery","quantity":1}]}`, eventID)))
	reserveRequest.Header.Set("Idempotency-Key", key)
	reserved := httptest.NewRecorder()
	handler.ServeHTTP(reserved, reserveRequest)
	expect(reserved, http.StatusCreated)
	var reservation struct{ ID string }
	_ = json.Unmarshal(reserved.Body.Bytes(), &reservation)
	reservationID = reservation.ID
	buyerEmail := "Recovery-" + suffix[:8] + "@Example.com"
	checkoutRequest := httptest.NewRequest(http.MethodPost, "/api/v1/reservations/"+reservationID+"/checkout", strings.NewReader(fmt.Sprintf(
		`{"buyer":{"name":"Recovery Buyer","email":%q,"phone":"081234567890","identity":"123456789012"},"attendees":[{"tierId":"recovery","names":["Recovery Guest"]}]}`, buyerEmail)))
	checkoutRequest.Header.Set("Idempotency-Key", key)
	ordered := httptest.NewRecorder()
	handler.ServeHTTP(ordered, checkoutRequest)
	expect(ordered, http.StatusCreated)
	var order struct{ ID, Reference, AccessToken string }
	_ = json.Unmarshal(ordered.Body.Bytes(), &order)
	orderID, reference = order.ID, order.Reference
	expect(call(handler, clientIP, http.MethodPost, "/api/v1/orders/"+orderID+"/simulate-payment", map[string]string{"method": "QRIS", "result": "SUCCEEDED"}, order.AccessToken), http.StatusCreated)

	port, mails := fakeSMTP(t, nil)
	workerCtx, stopWorker := context.WithCancel(ctx)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		email.NewService(db, email.Config{Host: "127.0.0.1", Port: port, From: "tickets@example.com", TLSMode: "none", FrontendURL: "http://localhost:5173", AccessSecret: secret}, logger).Run(workerCtx, 100*time.Millisecond)
	}()
	t.Cleanup(func() { stopWorker(); <-workerDone })
	waitMail := func(subject string) capturedMail {
		t.Helper()
		deadline := time.After(20 * time.Second)
		for {
			select {
			case got := <-mails:
				if got.to == buyerEmail && strings.Contains(got.subject, reference) && strings.Contains(got.subject, subject) {
					return got
				}
			case <-deadline:
				t.Fatalf("no %q email for %s", subject, reference)
			}
		}
	}
	waitMail("E-ticket")

	// A wrong pair and a matching pair receive the same public response.
	expect(call(handler, clientIP, http.MethodPost, "/api/v1/ticket-recovery", map[string]string{"email": buyerEmail, "reference": "nope"}, ""), http.StatusBadRequest)
	wrong := call(handler, clientIP, http.MethodPost, "/api/v1/ticket-recovery", map[string]string{"email": "other@example.com", "reference": reference}, "")
	right := call(handler, clientIP, http.MethodPost, "/api/v1/ticket-recovery", map[string]string{"email": strings.ToLower(buyerEmail), "reference": strings.ToUpper(reference)}, "")
	expect(wrong, http.StatusAccepted)
	expect(right, http.StatusAccepted)
	if wrong.Body.String() != right.Body.String() {
		t.Fatalf("recovery responses differ: %s vs %s", wrong.Body.String(), right.Body.String())
	}
	recoveryMail := waitMail("Pulihkan")
	match := regexp.MustCompile(`/pulihkan-tiket#token=([A-Za-z0-9_-]{43})`).FindStringSubmatch(recoveryMail.plain)
	if match == nil {
		t.Fatalf("recovery email has no link: %q", recoveryMail.plain)
	}
	token := match[1]
	hash := sha256.Sum256([]byte(token))
	var stored, rawStored int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM recovery_tokens WHERE token_hash = ?", hex.EncodeToString(hash[:])).Scan(&stored)
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM recovery_tokens WHERE token_hash = ?", token).Scan(&rawStored)
	if stored != 1 || rawStored != 0 {
		t.Fatalf("stored token hash rows = %d, raw rows = %d", stored, rawStored)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		var failed int
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM email_queue q JOIN recovery_requests r ON r.id = q.recovery_request_id
			WHERE r.reference = ? AND q.status = 'FAILED' AND q.recipient = ''`, reference).Scan(&failed)
		if failed == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("unmatched recovery request was not closed as failed without a recipient")
		}
		time.Sleep(100 * time.Millisecond)
	}

	// The pair cooldown keeps the generic 202 but queues nothing.
	expect(call(handler, clientIP, http.MethodPost, "/api/v1/ticket-recovery", map[string]string{"email": buyerEmail, "reference": reference}, ""), http.StatusAccepted)
	var requests int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM recovery_requests WHERE reference = ?", reference).Scan(&requests)
	if requests != 2 {
		t.Fatalf("recovery requests = %d, want cooldown to skip the third", requests)
	}

	// Two concurrent verifies of one link: exactly one wins.
	results := make([]*httptest.ResponseRecorder, 2)
	var group sync.WaitGroup
	for index := range results {
		group.Add(1)
		go func() {
			defer group.Done()
			results[index] = call(handler, clientIP, http.MethodPost, "/api/v1/ticket-recovery/verify", map[string]string{"token": token}, "")
		}()
	}
	group.Wait()
	codes := []int{results[0].Code, results[1].Code}
	slices.Sort(codes)
	if codes[0] != http.StatusOK || codes[1] != http.StatusGone {
		t.Fatalf("concurrent verify codes = %v", codes)
	}
	winner := results[0]
	if winner.Code != http.StatusOK {
		winner = results[1]
	}
	var recovered struct {
		OrderID, AccessToken, Reference, ReservationID, ExpiresAt, AccessExpiresAt string
		TicketIDs                                                                  []string
	}
	_ = json.Unmarshal(winner.Body.Bytes(), &recovered)
	if recovered.OrderID != orderID || recovered.AccessToken != order.AccessToken || recovered.Reference != reference || recovered.ReservationID != reservationID || len(recovered.TicketIDs) != 1 || recovered.AccessExpiresAt == "" {
		t.Fatalf("recovered access = %+v", recovered)
	}
	expect(call(handler, clientIP, http.MethodPost, "/api/v1/ticket-recovery/verify", map[string]string{"token": token}, ""), http.StatusGone)
	expect(call(handler, clientIP, http.MethodGet, "/api/v1/orders/"+orderID, nil, recovered.AccessToken), http.StatusOK)

	// An expired link is rejected even when unused.
	expiredRaw := strings.Repeat("A", 43)
	expiredHash := sha256.Sum256([]byte(expiredRaw))
	expiredRequest := randomHex(t, 16)
	extraReferences = append(extraReferences, "TO-expired-"+suffix[:8])
	if _, err := db.ExecContext(ctx, `INSERT INTO recovery_requests (id, reference, pair_key, expires_at, created_at)
		VALUES (?, ?, '', UTC_TIMESTAMP(6) - INTERVAL 1 SECOND, UTC_TIMESTAMP(6))`, expiredRequest, "TO-expired-"+suffix[:8]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO recovery_tokens (token_hash, request_id, order_id, expires_at, created_at)
		VALUES (?, ?, ?, UTC_TIMESTAMP(6) + INTERVAL 1 HOUR, UTC_TIMESTAMP(6))`, hex.EncodeToString(expiredHash[:]), expiredRequest, orderID); err != nil {
		t.Fatal(err)
	}
	expect(call(handler, clientIP, http.MethodPost, "/api/v1/ticket-recovery/verify", map[string]string{"token": expiredRaw}, ""), http.StatusGone)

	// Resend requires the order bearer, then reuses one TICKETS job with a 60 second cooldown.
	expect(call(handler, clientIP, http.MethodPost, "/api/v1/orders/"+orderID+"/resend-email", nil, ""), http.StatusUnauthorized)
	expect(call(handler, clientIP, http.MethodPost, "/api/v1/orders/"+orderID+"/resend-email", nil, strings.Repeat("x", 43)), http.StatusNotFound)
	expect(call(handler, clientIP, http.MethodPost, "/api/v1/orders/"+orderID+"/resend-email", nil, recovered.AccessToken), http.StatusAccepted)
	limited := call(handler, clientIP, http.MethodPost, "/api/v1/orders/"+orderID+"/resend-email", nil, recovered.AccessToken)
	expect(limited, http.StatusTooManyRequests)
	if limited.Header().Get("Retry-After") == "" {
		t.Fatal("resend cooldown has no Retry-After")
	}
	waitMail("E-ticket")
	var ticketJobs int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM email_queue WHERE order_id = ? AND kind = 'TICKETS'", orderID).Scan(&ticketJobs)
	if ticketJobs != 1 {
		t.Fatalf("ticket email jobs = %d, want one reused job", ticketJobs)
	}

	// The IP limit lives in the database, so a new handler (a restart) still enforces it.
	limitIP := fmt.Sprintf("10.%d.%d.%d:4000", ipBytes[2], ipBytes[1], ipBytes[0])
	for index := range 10 {
		ref := fmt.Sprintf("TO-%s%012x", suffix[:8], index)
		extraReferences = append(extraReferences, strings.ToLower(ref))
		expect(call(handler, limitIP, http.MethodPost, "/api/v1/ticket-recovery", map[string]string{"email": "limit@example.com", "reference": ref}, ""), http.StatusAccepted)
	}
	restarted := NewHandlerWithOrderAccess(db, logger, 10*time.Minute, "", true, secret)
	blocked := call(restarted, limitIP, http.MethodPost, "/api/v1/ticket-recovery", map[string]string{"email": "limit@example.com", "reference": reference}, "")
	expect(blocked, http.StatusTooManyRequests)
	if blocked.Header().Get("Retry-After") == "" {
		t.Fatal("IP limit has no Retry-After")
	}
}

func TestRecoveryLimitsBehindTrustedProxy(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to a disposable MySQL database")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrations.Run(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandlerWithPaymentConfig(db, logger, time.Minute, "", false, secret, "", "", netip.MustParsePrefix("172.20.0.3/32"))
	var references []string
	defer func() {
		for _, ref := range references {
			_, _ = db.Exec("DELETE FROM email_queue WHERE recovery_request_id IN (SELECT id FROM recovery_requests WHERE reference = ?)", ref)
			_, _ = db.Exec("DELETE FROM recovery_requests WHERE reference = ?", ref)
		}
	}()
	for _, tc := range []struct {
		path          string
		max, accepted int
	}{
		{"/api/v1/ticket-recovery", 10, http.StatusAccepted},
		{"/api/v1/ticket-recovery/verify", 30, http.StatusBadRequest},
	} {
		t.Run(tc.path, func(t *testing.T) {
			call := func(remote, forwarded string) *httptest.ResponseRecorder {
				ref := "TO-" + randomHex(t, 10)
				references = append(references, ref)
				body := fmt.Sprintf(`{"email":"limit@example.com","reference":%q}`, ref)
				if strings.HasSuffix(tc.path, "/verify") {
					body = `{"token":"invalid"}`
				}
				r := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(body))
				r.RemoteAddr = remote
				r.Header.Set("X-Forwarded-For", forwarded)
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				return w
			}
			for range tc.max {
				if w := call("172.20.0.3:41000", "192.0.2.1"); w.Code != tc.accepted {
					t.Fatalf("client A: %d %s", w.Code, w.Body.String())
				}
			}
			blocked := call("172.20.0.3:42000", "192.0.2.1")
			if blocked.Code != http.StatusTooManyRequests || blocked.Header().Get("Retry-After") == "" {
				t.Fatalf("client A limit: %d, %v", blocked.Code, blocked.Header())
			}
			if w := call("172.20.0.3:41000", "192.0.2.2"); w.Code != tc.accepted {
				t.Fatalf("client B blocked: %d %s", w.Code, w.Body.String())
			}
			if w := call("192.0.2.1:41000", "192.0.2.99"); w.Code != http.StatusTooManyRequests {
				t.Fatalf("direct spoof bypassed limit: %d", w.Code)
			}
		})
	}
}
