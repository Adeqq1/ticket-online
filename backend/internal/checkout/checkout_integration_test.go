package checkout

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

func TestCheckoutPersistsOnceAndReplays(t *testing.T) {
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

	reservationID, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	var tierID, price uint64
	if err := db.QueryRowContext(ctx, "SELECT id, price FROM ticket_tiers WHERE event_id = 'nusa-malam' AND slug = 'festival'").Scan(&tierID, &price); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx, `INSERT INTO reservations
		(id, event_id, status, idempotency_key, request_hash, expires_at, created_at, updated_at)
		VALUES (?, 'nusa-malam', 'ACTIVE', ?, REPEAT('a', 64), ?, ?, ?)`, reservationID, "checkout-test-"+reservationID, now.Add(time.Minute), now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO reservation_items (reservation_id, ticket_tier_id, quantity, unit_price) VALUES (?, ?, 2, ?)", reservationID, tierID, price); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		var orderID string
		if err := db.QueryRowContext(context.Background(), "SELECT id FROM orders WHERE reservation_id = ?", reservationID).Scan(&orderID); err == nil {
			_, _ = db.Exec("DELETE FROM order_buyers WHERE order_id = ?", orderID)
			_, _ = db.Exec("DELETE FROM order_items WHERE order_id = ?", orderID)
			_, _ = db.Exec("DELETE FROM orders WHERE id = ?", orderID)
		}
		_, _ = db.Exec("DELETE FROM reservation_items WHERE reservation_id = ?", reservationID)
		_, _ = db.Exec("DELETE FROM reservations WHERE id = ?", reservationID)
	})

	request := Request{
		Buyer:       Buyer{Name: "Pembeli Tes", Email: "test@example.com", Phone: "081234567890", Identity: "123456789012"},
		VoucherCode: "HEMAT10",
	}
	repository := NewRepository(db)
	first, replay, err := repository.Create(ctx, reservationID, request)
	if err != nil || replay {
		t.Fatalf("first checkout = (%+v, %v, %v)", first, replay, err)
	}
	if first.Subtotal != price*2 || first.AdminFee != adminFee || first.Discount != first.Subtotal/10 || first.Total != first.Subtotal+adminFee-first.Discount {
		t.Fatalf("server totals are incorrect: %+v", first)
	}
	second, replay, err := repository.Create(ctx, reservationID, request)
	if err != nil || !replay || second.ID != first.ID || second.Reference != first.Reference {
		t.Fatalf("retry = (%+v, %v, %v), want the original order", second, replay, err)
	}
	request.Buyer.Name = "Nama Berbeda"
	if _, _, err := repository.Create(ctx, reservationID, request); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed retry error = %v, want idempotency conflict", err)
	}
	var orders, buyers, items int
	if err := db.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM orders WHERE reservation_id = ?), (SELECT COUNT(*) FROM order_buyers WHERE order_id = ?), (SELECT COUNT(*) FROM order_items WHERE order_id = ?)", reservationID, first.ID, first.ID).Scan(&orders, &buyers, &items); err != nil {
		t.Fatal(err)
	}
	if orders != 1 || buyers != 1 || items != 1 {
		t.Fatalf("persisted rows: orders=%d buyers=%d items=%d", orders, buyers, items)
	}
	var status string
	if err := db.QueryRowContext(ctx, "SELECT status FROM reservations WHERE id = ?", reservationID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "CONVERTED" {
		t.Fatalf("reservation status = %q, want CONVERTED", status)
	}
}
