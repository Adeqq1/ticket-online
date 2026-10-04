package payment

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/migrations"
	_ "github.com/go-sql-driver/mysql"
)

func TestSimulatedPaymentRetryAndFinalSuccess(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to a disposable MySQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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
	orderID, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	reservationID, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	ref := "TO-" + orderID[:20]
	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx, `INSERT INTO reservations
		(id, event_id, status, idempotency_key, request_hash, order_reference, expires_at, created_at, updated_at)
		VALUES (?, 'nusa-malam', 'CONVERTED', ?, REPEAT('a', 64), ?, ?, ?, ?)`, reservationID, "payment-test-"+reservationID, ref, now.Add(time.Hour), now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO orders
		(id, reference, reservation_id, status, subtotal, admin_fee, discount, created_at, updated_at)
		VALUES (?, ?, ?, 'PENDING', 10000, 7500, 0, ?, ?)`, orderID, ref, reservationID, now, now); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM payments WHERE order_id = ?", orderID)
		_, _ = db.Exec("DELETE FROM orders WHERE id = ?", orderID)
		_, _ = db.Exec("DELETE FROM reservations WHERE id = ?", reservationID)
	})

	repository := NewRepository(db)
	request := Request{Method: "QRIS", Result: "FAILED"}
	type result struct {
		payment Payment
		reused  bool
		err     error
	}
	results := make(chan result, 8)
	for range cap(results) {
		go func() {
			payment, reused, err := repository.Simulate(ctx, orderID, request)
			results <- result{payment, reused, err}
		}()
	}
	var failed Payment
	created := 0
	for range cap(results) {
		value := <-results
		if value.err != nil {
			t.Fatalf("concurrent failed payment: %v", value.err)
		}
		if value.payment.Status != "FAILED" || value.payment.OrderStatus != "PENDING" || value.payment.Amount != 17500 {
			t.Fatalf("concurrent failed payment = %+v", value.payment)
		}
		if failed.ID != "" && failed.ID != value.payment.ID {
			t.Fatalf("concurrent requests returned different payment IDs: %q and %q", failed.ID, value.payment.ID)
		}
		failed = value.payment
		if !value.reused {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("concurrent requests created %d payment rows, want one", created)
	}
	succeeded, reused, err := repository.Simulate(ctx, orderID, Request{Method: "GOPAY", Result: "SUCCEEDED"})
	if err != nil || !reused || succeeded.Status != "SUCCEEDED" || succeeded.OrderStatus != "PAID" || succeeded.PaidAt == "" {
		t.Fatalf("successful retry = (%+v, %v, %v)", succeeded, reused, err)
	}
	if _, reused, err := repository.Simulate(ctx, orderID, Request{Method: "GOPAY", Result: "SUCCEEDED"}); err != nil || !reused {
		t.Fatalf("success retry = reused %v, error %v; want replay", reused, err)
	}
	if _, _, err := repository.Simulate(ctx, orderID, Request{Method: "GOPAY", Result: "FAILED"}); !errors.Is(err, ErrPaymentConflict) {
		t.Fatalf("post-success failure error = %v, want conflict", err)
	}
	var payments int
	var orderStatus string
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM payments WHERE order_id = ?", orderID).Scan(&payments); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT status FROM orders WHERE id = ?", orderID).Scan(&orderStatus); err != nil {
		t.Fatal(err)
	}
	if payments != 1 || orderStatus != "PAID" {
		t.Fatalf("payments=%d order status=%s; want one payment and PAID", payments, orderStatus)
	}
}
