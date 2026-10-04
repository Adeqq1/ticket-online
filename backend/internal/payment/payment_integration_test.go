package payment

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/checkout"
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
		(id, reference, reservation_id, status, subtotal, admin_fee, discount, created_at, updated_at, expires_at)
		VALUES (?, ?, ?, 'PENDING', 10000, 7500, 0, ?, ?, ?)`, orderID, ref, reservationID, now, now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var tierID uint64
	if err := db.QueryRowContext(ctx, "SELECT id FROM ticket_tiers WHERE event_id = 'nusa-malam' AND slug = 'festival'").Scan(&tierID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO order_items (order_id, ticket_tier_id, tier_name, quantity, unit_price) SELECT ?, id, name, 2, price FROM ticket_tiers WHERE id = ?", orderID, tierID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO order_buyers (order_id, name, email, phone, identity) VALUES (?, 'Pembeli Tes', 'payment@example.com', '081234567890', '123456789012')", orderID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO order_attendees (order_id, ticket_tier_id, ticket_number, name) VALUES (?, ?, 1, 'Peserta Satu'), (?, ?, 2, 'Peserta Dua')", orderID, tierID, orderID, tierID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM etickets WHERE order_id = ?", orderID)
		_, _ = db.Exec("DELETE FROM order_attendees WHERE order_id = ?", orderID)
		_, _ = db.Exec("DELETE FROM payments WHERE order_id = ?", orderID)
		_, _ = db.Exec("DELETE FROM order_items WHERE order_id = ?", orderID)
		_, _ = db.Exec("DELETE FROM order_buyers WHERE order_id = ?", orderID)
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
		if value.payment.Status != "FAILED" || value.payment.OrderStatus != "PENDING" || value.payment.Amount != 17500 || len(value.payment.Tickets) != 0 {
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
	if err != nil || !reused || succeeded.Status != "SUCCEEDED" || succeeded.OrderStatus != "PAID" || succeeded.PaidAt == "" || len(succeeded.Tickets) != 2 {
		t.Fatalf("successful retry = (%+v, %v, %v)", succeeded, reused, err)
	}
	issuedID := ""
	attendees := map[string]bool{}
	for _, ticket := range succeeded.Tickets {
		attendees[ticket.AttendeeName] = true
		if ticket.AttendeeName == "Peserta Satu" {
			issuedID = ticket.ID
			if ticket.Code != "ET-"+strings.ToUpper(ticket.ID) || ticket.Gate == "" {
				t.Fatalf("unexpected issued ticket snapshot: %+v", ticket)
			}
		}
	}
	if issuedID == "" || !attendees["Peserta Dua"] {
		t.Fatalf("attendee snapshots are incomplete: %+v", succeeded.Tickets)
	}
	listed, err := repository.TicketsForOrder(ctx, orderID)
	found := false
	for _, ticket := range listed {
		found = found || ticket.ID == issuedID
	}
	if err != nil || len(listed) != 2 || !found {
		t.Fatalf("ticket list = (%+v, %v), want persisted tickets", listed, err)
	}
	read, err := repository.Ticket(ctx, issuedID)
	if err != nil || read.ID != issuedID || read.AttendeeName != "Peserta Satu" {
		t.Fatalf("ticket read = (%+v, %v)", read, err)
	}
	replayed, reused, err := repository.Simulate(ctx, orderID, Request{Method: "GOPAY", Result: "SUCCEEDED"})
	found = false
	for _, ticket := range replayed.Tickets {
		found = found || ticket.ID == issuedID
	}
	if err != nil || !reused || len(replayed.Tickets) != 2 || !found {
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

func TestUnpaidOrderExpiryAndPaymentRace(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to a disposable MySQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	for _, scenario := range []string{"abandoned", "late-payment", "paid", "race-expired", "race-paid"} {
		t.Run(scenario, func(t *testing.T) {
			reservationID, err := randomID()
			if err != nil {
				t.Fatal(err)
			}
			// Isolate stock from the other integration test packages sharing this DB.
			result, err := db.ExecContext(ctx, `INSERT INTO ticket_tiers
				(event_id, slug, name, zone_slug, price, capacity, available_quantity, max_per_order, benefit, gate, seating_mode, created_at, updated_at)
				SELECT event_id, ?, name, zone_slug, price, 5, 3, max_per_order, benefit, gate, seating_mode, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)
				FROM ticket_tiers WHERE event_id = 'nusa-malam' AND slug = 'festival'`, reservationID)
			if err != nil {
				t.Fatal(err)
			}
			tierID, err := result.LastInsertId()
			if err != nil {
				t.Fatal(err)
			}
			var orderID string
			t.Cleanup(func() {
				for _, table := range []string{"etickets", "payments", "order_attendees", "order_buyers", "order_items"} {
					if _, err := db.Exec("DELETE FROM "+table+" WHERE order_id = ?", orderID); err != nil {
						t.Error(err)
					}
				}
				for _, query := range []string{"DELETE FROM orders WHERE reservation_id = ?", "DELETE FROM reservation_items WHERE reservation_id = ?", "DELETE FROM reservations WHERE id = ?"} {
					if _, err := db.Exec(query, reservationID); err != nil {
						t.Error(err)
					}
				}
				if _, err := db.Exec("DELETE FROM ticket_tiers WHERE id = ?", tierID); err != nil {
					t.Error(err)
				}
			})
			now := time.Now().UTC()
			key := "expiry-test-" + reservationID
			if _, err := db.ExecContext(ctx, `INSERT INTO reservations
				(id, event_id, status, idempotency_key, request_hash, expires_at, created_at, updated_at)
				VALUES (?, 'nusa-malam', 'ACTIVE', ?, REPEAT('a', 64), ?, ?, ?)`, reservationID, key, now.Add(time.Hour), now, now); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, "INSERT INTO reservation_items (reservation_id, ticket_tier_id, quantity, unit_price) SELECT ?, id, 2, price FROM ticket_tiers WHERE id = ?", reservationID, tierID); err != nil {
				t.Fatal(err)
			}
			order, _, err := checkout.NewRepository(db).Create(ctx, reservationID, key, checkout.Request{
				Buyer:     checkout.Buyer{Name: "Pembeli Tes", Email: "expiry@example.com", Phone: "081234567890", Identity: "123456789012"},
				Attendees: []checkout.Attendees{{TierID: reservationID, Names: []string{"Peserta Satu", "Peserta Dua"}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			orderID = order.ID
			pay := func() error {
				_, _, err := repository.Simulate(ctx, orderID, Request{Method: "QRIS", Result: "SUCCEEDED"})
				return err
			}
			setExpired := func() {
				if _, err := db.ExecContext(ctx, "UPDATE orders SET expires_at = UTC_TIMESTAMP(6) - INTERVAL 1 SECOND WHERE id = ?", orderID); err != nil {
					t.Fatal(err)
				}
			}
			wantStatus, wantStock, wantTickets := "EXPIRED", 5, 0
			switch scenario {
			case "abandoned":
				setExpired()
			case "late-payment":
				setExpired()
				if err := pay(); !errors.Is(err, ErrOrderNotPayable) {
					t.Fatalf("late payment error = %v", err)
				}
			case "paid":
				if err := pay(); err != nil {
					t.Fatal(err)
				}
				setExpired()
				wantStatus, wantStock, wantTickets = "PAID", 3, 2
			case "race-expired", "race-paid":
				if scenario == "race-expired" {
					setExpired()
				} else {
					wantStatus, wantStock, wantTickets = "PAID", 3, 2
				}
				start := make(chan struct{})
				paymentResult, expiryResult := make(chan error, 1), make(chan error, 1)
				go func() { <-start; paymentResult <- pay() }()
				go func() { <-start; expiryResult <- repository.expireOrder(ctx, orderID) }()
				close(start)
				if err := <-paymentResult; scenario == "race-expired" && !errors.Is(err, ErrOrderNotPayable) || scenario == "race-paid" && err != nil {
					t.Fatalf("concurrent payment error = %v", err)
				}
				if err := <-expiryResult; err != nil {
					t.Fatalf("concurrent expiry error = %v", err)
				}
				setExpired()
			}
			// Run the worker batch twice: stock must be restored at most once.
			for range 2 {
				if err := repository.ExpirePendingOrders(ctx); err != nil {
					t.Fatal(err)
				}
			}
			var status string
			var stock, tickets int
			if err := db.QueryRowContext(ctx, "SELECT o.status, tt.available_quantity, (SELECT COUNT(*) FROM etickets WHERE order_id = o.id) FROM orders o JOIN order_items oi ON oi.order_id = o.id JOIN ticket_tiers tt ON tt.id = oi.ticket_tier_id WHERE o.id = ?", orderID).Scan(&status, &stock, &tickets); err != nil {
				t.Fatal(err)
			}
			if status != wantStatus || stock != wantStock || tickets != wantTickets {
				t.Fatalf("status=%s stock=%d tickets=%d; want %s %d %d", status, stock, tickets, wantStatus, wantStock, wantTickets)
			}
		})
	}
}
