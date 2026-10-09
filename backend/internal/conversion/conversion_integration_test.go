package conversion

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/recovery"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
	"github.com/Adeqq1/ticket-online/backend/migrations"
	_ "github.com/go-sql-driver/mysql"
)

func TestStageRowsDeduplicatesJourneysAndLimitsStagesTo24Hours(t *testing.T) {
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
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(b[:])
	}
	eventID, journeyID := "conversion-"+id(), id()
	reservationIDs, orderIDs, paymentIDs := []string{id(), id()}, []string{id(), id()}, []string{id(), id()}
	now := time.Now().UTC().Truncate(time.Microsecond)
	journeyStart := now.Add(-30 * time.Hour)
	if _, err := db.ExecContext(ctx, `INSERT INTO events (id,artist,city,venue,address,starts_at,genre,status,image_url,description,created_at,updated_at)
		VALUES (?, 'Conversion test','Jakarta','Venue','Address',?,'Indie','EARLY_BIRD','','Test',?,?)`, eventID, now.Add(24*time.Hour), now, now); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM conversion_events WHERE journey_id=?", journeyID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM conversion_reservations WHERE journey_id=?", journeyID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM payments WHERE order_id IN (?,?)", orderIDs[0], orderIDs[1])
		_, _ = db.ExecContext(context.Background(), "DELETE FROM orders WHERE id IN (?,?)", orderIDs[0], orderIDs[1])
		_, _ = db.ExecContext(context.Background(), "DELETE FROM reservations WHERE id IN (?,?)", reservationIDs[0], reservationIDs[1])
		_, _ = db.ExecContext(context.Background(), "DELETE FROM conversion_journeys WHERE id=?", journeyID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM events WHERE id=?", eventID)
	})
	if _, err := db.ExecContext(ctx, `INSERT INTO conversion_journeys (id,event_id,device,started_at,detail_viewed_at) VALUES (?,?,'mobile',?,?)`, journeyID, eventID, journeyStart, journeyStart); err != nil {
		t.Fatal(err)
	}
	for i := range reservationIDs {
		reservationAt, orderAt, paymentAt := journeyStart.Add(time.Hour), journeyStart.Add(2*time.Hour), journeyStart.Add(3*time.Hour)
		if _, err := db.ExecContext(ctx, `INSERT INTO reservations (id,event_id,status,idempotency_key,request_hash,expires_at,created_at,updated_at) VALUES (?,?, 'CONVERTED', ?, REPEAT('a',64),?,?,?)`, reservationIDs[i], eventID, id(), reservationAt.Add(10*time.Minute), reservationAt, reservationAt); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO conversion_reservations (reservation_id,journey_id,created_at) VALUES (?,?,?)`, reservationIDs[i], journeyID, reservationAt); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO orders (id,reference,reservation_id,status,subtotal,admin_fee,discount,created_at,updated_at,request_hash,expires_at) VALUES (?,?,?,'PAID',100,0,0,?,?,REPEAT('b',64),?)`, orderIDs[i], "CO-"+id()[:24], reservationIDs[i], orderAt, orderAt, orderAt.Add(30*time.Minute)); err != nil {
			t.Fatal(err)
		}
		paymentStatus := "FAILED"
		var paidAt any
		if i == 0 {
			paymentStatus = "SUCCEEDED"
			paidAt = now.Add(-time.Hour)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO payments (id,order_id,method,amount,status,started_at,paid_at,created_at,updated_at) VALUES (?,?,'QRIS',100,?,?,?,?,?)`, paymentIDs[i], orderIDs[i], paymentStatus, paymentAt, paidAt, paymentAt, paymentAt); err != nil {
			t.Fatal(err)
		}
	}
	query := func() Breakdown {
		t.Helper()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		rows, err := stageRows(ctx, tx, journeyStart.Add(-time.Hour), journeyStart.Add(time.Hour), eventID, "mobile", "")
		if err != nil {
			t.Fatal(err)
		}
		return sum(rows)
	}
	first := query()
	if first.Matured.Detail != 1 || first.Matured.Reservation != 1 || first.Matured.Order != 1 || first.Matured.PaymentStarted != 1 || first.Matured.PaymentSucceeded != 0 || first.Lost["paymentToSuccess"] != 1 {
		t.Fatalf("late payment stages = %+v, lost=%v", first.Matured, first.Lost)
	}
	if _, err := db.ExecContext(ctx, "UPDATE payments SET paid_at=? WHERE id=?", paymentAtForSuccess(journeyStart), paymentIDs[0]); err != nil {
		t.Fatal(err)
	}
	second := query()
	if second.Matured.PaymentSucceeded != 1 || second.Lost["paymentToSuccess"] != 0 {
		t.Fatalf("in-window payment stages = %+v, lost=%v", second.Matured, second.Lost)
	}
}

func paymentAtForSuccess(start time.Time) time.Time { return start.Add(23 * time.Hour) }

func TestDelayedTelemetryKeepsReservationLinkAndDirectCheckoutCoverage(t *testing.T) {
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
		value, err := recovery.NewID()
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	eventID, otherEventID, staffID := "conversion-"+id(), "conversion-"+id(), id()
	journeys, orders := []string{id(), id()}, []string{id(), id()}
	otherReservation := id()
	token, _, err := recovery.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	// Staff sessions use an unkeyed token hash, matching AuthenticateTx.
	tokenHash := sha256.Sum256([]byte(token))
	exec(`INSERT INTO staff_users(id,name,email,password_hash,role,active,created_at,updated_at)
 VALUES (?,'Conversion Test',?,'unused','ADMIN',TRUE,UTC_TIMESTAMP(6),UTC_TIMESTAMP(6))`, staffID, staffID+"@example.test")
	exec(`INSERT INTO staff_sessions(token_hash,staff_id,expires_at,created_at) VALUES (?,?,UTC_TIMESTAMP(6)+INTERVAL 1 DAY,UTC_TIMESTAMP(6))`, tokenHash[:], staffID)
	t.Cleanup(func() {
		for _, query := range []string{
			"DELETE FROM conversion_events WHERE journey_id IN (?,?)",
			"DELETE FROM conversion_reservations WHERE journey_id IN (?,?)",
			"DELETE FROM conversion_journeys WHERE id IN (?,?)",
			"DELETE FROM payments WHERE order_id IN (?,?)",
			"DELETE FROM orders WHERE id IN (?,?)",
			"DELETE FROM reservations WHERE id IN (?,?)",
		} {
			ids := orders
			if strings.HasPrefix(query, "DELETE FROM conversion") {
				ids = journeys
			}
			if _, err := db.Exec(query, ids[0], ids[1]); err != nil {
				t.Error(err)
			}
		}
		_, _ = db.Exec("DELETE FROM reservations WHERE id=?", otherReservation)
		_, _ = db.Exec("DELETE FROM events WHERE id IN (?,?)", eventID, otherEventID)
		_, _ = db.Exec("DELETE FROM staff_sessions WHERE staff_id=?", staffID)
		_, _ = db.Exec("DELETE FROM staff_users WHERE id=?", staffID)
	})
	for _, event := range []string{eventID, otherEventID} {
		exec(`INSERT INTO events(id,artist,city,venue,address,starts_at,genre,status,image_url,description,publication_status,created_at,updated_at)
 VALUES (?,'Conversion','Jakarta','Venue','Address',UTC_TIMESTAMP(6)+INTERVAL 2 DAY,'Indie','EARLY_BIRD','','Test','PUBLISHED',UTC_TIMESTAMP(6),UTC_TIMESTAMP(6))`, event)
	}
	for _, order := range orders {
		exec(`INSERT INTO reservations(id,event_id,status,idempotency_key,request_hash,expires_at,created_at,updated_at)
 VALUES (?,?,'CONVERTED',?,REPEAT('a',64),UTC_TIMESTAMP(6)+INTERVAL 1 HOUR,UTC_TIMESTAMP(6),UTC_TIMESTAMP(6))`, order, eventID, order)
		exec(`INSERT INTO orders(id,reference,reservation_id,status,subtotal,expires_at,created_at,updated_at)
 VALUES (?,?,?,'PAID',100,UTC_TIMESTAMP(6)+INTERVAL 1 HOUR,UTC_TIMESTAMP(6),UTC_TIMESTAMP(6))`, order, order, order)
		exec(`INSERT INTO payments(id,order_id,method,amount,status,started_at,paid_at,created_at,updated_at)
 VALUES (?,?,'QRIS',100,'SUCCEEDED',UTC_TIMESTAMP(6),UTC_TIMESTAMP(6),UTC_TIMESTAMP(6),UTC_TIMESTAMP(6))`, id(), order)
	}
	exec(`INSERT INTO reservations(id,event_id,status,idempotency_key,request_hash,expires_at,created_at,updated_at)
 VALUES (?,?,'ACTIVE',?,REPEAT('a',64),UTC_TIMESTAMP(6)+INTERVAL 1 HOUR,UTC_TIMESTAMP(6),UTC_TIMESTAMP(6))`, otherReservation, otherEventID, otherReservation)
	s := New(db, staffauth.New(db))
	record := func(journey, event, kind, device string, want int) {
		t.Helper()
		body, _ := json.Marshal(eventRequest{JourneyID: journey, EventID: event, Kind: kind, Device: device})
		response := httptest.NewRecorder()
		s.record(response, httptest.NewRequest("POST", "/api/v1/conversion/events", bytes.NewReader(body)))
		if response.Code != want {
			t.Fatalf("record telemetry: %d %s", response.Code, response.Body.String())
		}
	}
	LinkReservation(ctx, db, journeys[0], eventID, orders[0])
	var device string
	var detail sql.NullTime
	if err := db.QueryRow("SELECT device,detail_viewed_at FROM conversion_journeys WHERE id=?", journeys[0]).Scan(&device, &detail); err != nil || device != "unknown" || detail.Valid {
		t.Fatalf("placeholder: device=%s, detail=%v, err=%v", device, detail, err)
	}
	record(journeys[0], eventID, "CHECKOUT_STARTED", "mobile", 204)
	record(journeys[0], eventID, "DETAIL_VIEWED", "desktop", 204)
	record(journeys[0], eventID, "DETAIL_VIEWED", "desktop", 204)
	// Normal telemetry-first flow also links without fabricating a detail view.
	record(journeys[1], eventID, "CHECKOUT_STARTED", "unknown", 204)
	record(journeys[1], eventID, "CHECKOUT_STARTED", "mobile", 204)
	LinkReservation(ctx, db, journeys[1], eventID, orders[1])
	LinkReservation(ctx, db, journeys[0], eventID, orders[0])
	LinkReservation(ctx, db, journeys[0], otherEventID, otherReservation)
	record(journeys[0], otherEventID, "DETAIL_VIEWED", "desktop", 400)
	var links int
	if err := db.QueryRow("SELECT COUNT(*) FROM conversion_reservations WHERE journey_id IN (?,?)", journeys[0], journeys[1]).Scan(&links); err != nil || links != 2 {
		t.Fatalf("missing/duplicate/cross-event links: %d %v", links, err)
	}
	if err := db.QueryRow("SELECT device FROM conversion_journeys WHERE id=?", journeys[0]).Scan(&device); err != nil || device != "mobile" {
		t.Fatalf("initial device changed: %s %v", device, err)
	}
	if err := db.QueryRow("SELECT device FROM conversion_journeys WHERE id=?", journeys[1]).Scan(&device); err != nil || device != "unknown" {
		t.Fatalf("initial unknown device changed: %s %v", device, err)
	}
	read := func(device string) Report {
		t.Helper()
		r := httptest.NewRequest("GET", "/api/v1/admin/reports/conversion?eventId="+eventID+"&device="+device, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.report(slog.New(slog.NewTextHandler(io.Discard, nil)))(w, r)
		if w.Code != 200 {
			t.Fatalf("report: %d %s", w.Code, w.Body.String())
		}
		var result Report
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	result := read("")
	if result.Summary.Total != (Stage{1, 1, 1, 1, 1}) || result.UnattributedReservations == nil || *result.UnattributedReservations != 1 || result.UnattributedPayments == nil || *result.UnattributedPayments != 1 {
		t.Fatalf("incorrect funnel/coverage: %+v", result)
	}
	filtered := read("mobile")
	if filtered.Summary.Total.Detail != 1 || filtered.UnattributedReservations != nil || filtered.UnattributedPayments != nil {
		t.Fatalf("device filtering changed coverage: %+v", filtered)
	}
	// Analytics failure must not invalidate the already successful reservations.
	failedDB, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := failedDB.Close(); err != nil {
		t.Fatal(err)
	}
	LinkReservation(ctx, failedDB, journeys[0], eventID, orders[0])
	var state string
	if err := db.QueryRow("SELECT status FROM reservations WHERE id=?", orders[0]).Scan(&state); err != nil || state != "CONVERTED" {
		t.Fatalf("analytics failure changed reservation: %s %v", state, err)
	}
}
