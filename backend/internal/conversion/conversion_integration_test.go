package conversion

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"testing"
	"time"

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
	defer db.Close()
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
