package refund

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/adminissues"
	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
	"github.com/Adeqq1/ticket-online/backend/migrations"
	_ "github.com/go-sql-driver/mysql"
)

func TestRefundQueueResumesAfterRestartAndFinalizesOnlyConfirmedResults(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to a disposable MySQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := cleanupRotationRows(ctx, db); err != nil {
		t.Fatal(err)
	}

	for _, rejected := range []bool{false, true} {
		name := "accepted"
		if rejected {
			name = "definite_rejection"
		}
		t.Run(name, func(t *testing.T) {
			orderID, _ := newID()
			reservationID, _ := newID()
			staffID, _ := newID()
			paymentID, _ := newID()
			tokenBytes := make([]byte, 32)
			if _, err := rand.Read(tokenBytes); err != nil {
				t.Fatal(err)
			}
			token := base64.RawURLEncoding.EncodeToString(tokenBytes)
			tokenHash := sha256.Sum256([]byte(token))
			reference := "TO-" + orderID[:20]
			gatewayID := "gw-" + orderID
			transactionID := "txn-" + orderID
			now := time.Now().UTC()
			tierResult, err := db.ExecContext(ctx, `INSERT INTO ticket_tiers (event_id,slug,name,zone_slug,price,capacity,available_quantity,max_per_order,benefit,gate,seating_mode,created_at,updated_at)
				SELECT event_id,?,name,zone_slug,price,2,1,1,benefit,gate,seating_mode,UTC_TIMESTAMP(6),UTC_TIMESTAMP(6)
				FROM ticket_tiers WHERE event_id='nusa-malam' AND slug='festival'`, "refund-"+orderID[:12])
			if err != nil {
				t.Fatal(err)
			}
			tierIDValue, err := tierResult.LastInsertId()
			if err != nil {
				t.Fatal(err)
			}
			tierID, originalStock := uint64(tierIDValue), uint64(1)
			t.Cleanup(func() {
				_, _ = db.Exec("DELETE FROM admin_audit_log WHERE actor_id=?", staffID)
				_, _ = db.Exec("UPDATE ticket_tiers SET available_quantity=? WHERE id=?", originalStock, tierID)
				_, _ = db.Exec("DELETE FROM email_queue WHERE order_id=?", orderID)
				_, _ = db.Exec("DELETE FROM order_refund_audit WHERE refund_id IN (SELECT id FROM order_refunds WHERE order_id=?)", orderID)
				_, _ = db.Exec("DELETE FROM order_refunds WHERE order_id=?", orderID)
				_, _ = db.Exec("DELETE FROM payments WHERE order_id=?", orderID)
				_, _ = db.Exec("DELETE FROM order_items WHERE order_id=?", orderID)
				_, _ = db.Exec("DELETE FROM order_buyers WHERE order_id=?", orderID)
				_, _ = db.Exec("DELETE FROM orders WHERE id=?", orderID)
				_, _ = db.Exec("DELETE FROM reservations WHERE id=?", reservationID)
				_, _ = db.Exec("DELETE FROM ticket_tiers WHERE id=?", tierID)
				_, _ = db.Exec("DELETE FROM staff_sessions WHERE staff_id=?", staffID)
				_, _ = db.Exec("DELETE FROM staff_users WHERE id=?", staffID)
			})
			if _, err := db.ExecContext(ctx, `INSERT INTO staff_users (id,name,email,password_hash,role,active,created_at,updated_at)
				VALUES (?, 'Refund Admin', ?, 'unused', 'ADMIN', TRUE, ?, ?)`, staffID, "refund-"+orderID[:12]+"@example.test", now, now); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, "INSERT INTO staff_sessions (token_hash,staff_id,expires_at,created_at) VALUES (?,?,?,?)", tokenHash[:], staffID, now.Add(time.Hour), now); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, `INSERT INTO reservations (id,event_id,status,idempotency_key,request_hash,order_reference,expires_at,created_at,updated_at)
				VALUES (?, 'nusa-malam','CONVERTED',?,REPEAT('a',64),?,?,?,?)`, reservationID, "refund-test-"+reservationID, reference, now.Add(time.Hour), now, now); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, `INSERT INTO orders (id,reference,reservation_id,status,subtotal,admin_fee,discount,created_at,updated_at,expires_at)
				VALUES (?,?,?,'PAID',10000,7500,0,?,?,?)`, orderID, reference, reservationID, now, now, now.Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, `INSERT INTO order_items (order_id,ticket_tier_id,tier_name,quantity,unit_price)
				SELECT ?,id,name,1,10000 FROM ticket_tiers WHERE id=?`, orderID, tierID); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, "INSERT INTO order_buyers (order_id,name,email,phone,identity) VALUES (?,'Buyer','refund-buyer@example.test','081234567890','123456789012')", orderID); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, `INSERT INTO payments (id,order_id,method,amount,status,paid_at,gateway_order_id,created_at,updated_at)
				VALUES (?,?, 'QRIS',17500,'SUCCEEDED',?,?,?,?)`, paymentID, orderID, now, gatewayID, now, now); err != nil {
				t.Fatal(err)
			}

			var mu sync.Mutex
			providerStatus := "settlement"
			refundAmount := "0.00"
			postCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v2/"+gatewayID+"/status" && r.URL.Path != "/v2/"+gatewayID+"/refund" {
					t.Errorf("unexpected provider path %s", r.URL.Path)
				}
				mu.Lock()
				defer mu.Unlock()
				if r.Method == http.MethodGet {
					_ = json.NewEncoder(w).Encode(map[string]string{"order_id": gatewayID, "transaction_id": transactionID, "gross_amount": "17500.00", "payment_type": "qris", "settlement_time": now.In(time.FixedZone("WIB", 7*3600)).Format("2006-01-02 15:04:05"), "transaction_status": providerStatus, "refund_amount": refundAmount})
					return
				}
				postCount++
				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["refund_key"] == "" {
					t.Errorf("invalid refund request: %#v, %v", body, err)
				}
				if rejected {
					_ = json.NewEncoder(w).Encode(map[string]string{"status_code": "412", "status_message": "Merchant cannot modify the status of the transaction"})
					return
				}
				providerStatus, refundAmount = "refund", "17500.00"
				_ = json.NewEncoder(w).Encode(map[string]string{"status_code": "200", "status_message": "accepted", "order_id": gatewayID, "transaction_id": transactionID, "refund_key": body["refund_key"], "refund_amount": "17500.00"})
			}))
			defer server.Close()
			service := New(db, staffauth.New(db), "test-key", "sandbox", []string{"QRIS"}, nil)
			service.base = server.URL
			result, err := service.Submit(ctx, token, orderID, "approved buyer request")
			if err != nil || result.Status != "REQUESTED" {
				t.Fatalf("submit = %#v, %v", result, err)
			}
			mu.Lock()
			beforeWorkerPosts := postCount
			mu.Unlock()
			if beforeWorkerPosts != 0 {
				t.Fatal("request handler called provider before the durable queue was processed")
			}
			// A fresh Service instance models restart after commit and before delivery.
			restarted := New(db, staffauth.New(db), "test-key", "sandbox", []string{"QRIS"}, nil)
			restarted.base = server.URL
			if err := restarted.deliverRequested(ctx); err != nil {
				t.Fatal(err)
			}
			var refundState, orderState string
			if err := db.QueryRowContext(ctx, "SELECT status FROM order_refunds WHERE order_id=?", orderID).Scan(&refundState); err != nil {
				t.Fatal(err)
			}
			if rejected {
				if refundState != "FAILED" {
					t.Fatalf("refund status = %s, want FAILED", refundState)
				}
				if err := db.QueryRowContext(ctx, "SELECT status FROM orders WHERE id=?", orderID).Scan(&orderState); err != nil || orderState != "PAID" {
					t.Fatalf("order status = %s, error %v", orderState, err)
				}
			} else {
				if refundState != "UNKNOWN" {
					t.Fatalf("accepted POST status = %s, want UNKNOWN pending provider reconciliation", refundState)
				}
				if _, err := db.ExecContext(ctx, "UPDATE order_refunds SET next_attempt_at=UTC_TIMESTAMP(6)-INTERVAL 1 SECOND WHERE order_id=?", orderID); err != nil {
					t.Fatal(err)
				}
				if err := restarted.reconcile(ctx); err != nil {
					t.Fatal(err)
				}
				if err := db.QueryRowContext(ctx, "SELECT status FROM orders WHERE id=?", orderID).Scan(&orderState); err != nil || orderState != "REFUNDED" {
					t.Fatalf("confirmed order status = %s, error %v", orderState, err)
				}
				var stock uint64
				if err := db.QueryRowContext(ctx, "SELECT available_quantity FROM ticket_tiers WHERE id=?", tierID).Scan(&stock); err != nil || stock != originalStock+1 {
					t.Fatalf("stock after refund = %d, want %d, error %v", stock, originalStock+1, err)
				}
				if err := restarted.finish(ctx, orderID, true, ""); err != nil {
					t.Fatal(err)
				}
				if err := db.QueryRowContext(ctx, "SELECT available_quantity FROM ticket_tiers WHERE id=?", tierID).Scan(&stock); err != nil || stock != originalStock+1 {
					t.Fatalf("repeated finalization changed stock to %d, want %d, error %v", stock, originalStock+1, err)
				}
			}
			var emails int
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM email_queue WHERE order_id=? AND kind='REFUND'", orderID).Scan(&emails); err != nil || emails != 1 {
				t.Fatalf("refund email count = %d, want 1, error %v", emails, err)
			}
			var emailID string
			if err := db.QueryRowContext(ctx, "SELECT id FROM email_queue WHERE order_id=? AND kind='REFUND'", orderID).Scan(&emailID); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, "UPDATE email_queue SET status='FAILED',attempts=3,last_error='smtp unavailable' WHERE id=?", emailID); err != nil {
				t.Fatal(err)
			}
			adminService := adminissues.NewService(db, staffauth.New(db), orderaccess.New(db, []byte("refund-admin-test-order-access")), nil, "")
			detail, err := adminService.EmailDetail(ctx, token, emailID)
			if err != nil || !detail.CanRetry || detail.Kind != "REFUND" {
				t.Fatalf("refund email detail = %#v, %v", detail, err)
			}
			if _, err := adminService.RetryEmailJob(ctx, token, emailID); err != nil {
				t.Fatalf("retry refund email: %v", err)
			}
			var status string
			var attempts int
			if err := db.QueryRowContext(ctx, "SELECT status,attempts FROM email_queue WHERE id=?", emailID).Scan(&status, &attempts); err != nil || status != "PENDING" || attempts != 0 {
				t.Fatalf("retried email status=%s attempts=%d error=%v", status, attempts, err)
			}
			mu.Lock()
			defer mu.Unlock()
			if postCount != 1 {
				t.Fatalf("refund POST count = %d, want 1", postCount)
			}
		})
	}
}

func TestRefundPollingRotatesPastFirstFiftyPendingItems(t *testing.T) {
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
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := cleanupRotationRows(ctx, db); err != nil {
		t.Fatal(err)
	}
	staffID, err := newID()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx, `INSERT INTO staff_users (id,name,email,password_hash,role,active,created_at,updated_at)
		VALUES (?, 'Refund Rotation', ?, 'unused', 'ADMIN', TRUE, ?, ?)`, staffID, "refund-rotation-"+staffID[:10]+"@example.test", now, now); err != nil {
		t.Fatal(err)
	}
	base := uint64(time.Now().UnixNano())
	type fixture struct{ refundID, orderID, reservationID, gatewayID string }
	fixtures := make([]fixture, 51)
	t.Cleanup(func() {
		for _, f := range fixtures {
			for _, step := range []struct{ query, id string }{
				{"DELETE FROM email_queue WHERE order_id=?", f.orderID},
				{"DELETE FROM order_refund_audit WHERE refund_id=?", f.refundID},
				{"DELETE FROM order_refunds WHERE id=?", f.refundID},
				{"DELETE FROM order_buyers WHERE order_id=?", f.orderID},
				{"DELETE FROM orders WHERE id=?", f.orderID},
				{"DELETE FROM reservations WHERE id=?", f.reservationID},
			} {
				if _, err := db.Exec(step.query, step.id); err != nil {
					t.Errorf("cleanup %s: %v", step.query, err)
				}
			}
		}
		if _, err := db.Exec("DELETE FROM staff_users WHERE id=?", staffID); err != nil {
			t.Errorf("cleanup rotation staff: %v", err)
		}
	})
	for i := range fixtures {
		orderID, err := newID()
		if err != nil {
			t.Fatal(err)
		}
		reservationID, err := newID()
		if err != nil {
			t.Fatal(err)
		}
		refundID := fmt.Sprintf("%032x", base+uint64(i+1))
		gatewayID := fmt.Sprintf("rotation-%016x-%02d", base, i+1)
		fixtures[i] = fixture{refundID: refundID, orderID: orderID, reservationID: reservationID, gatewayID: gatewayID}
		reference := "TO-" + orderID[:20]
		if _, err := db.ExecContext(ctx, `INSERT INTO reservations (id,event_id,status,idempotency_key,request_hash,order_reference,expires_at,created_at,updated_at)
			VALUES (?, 'nusa-malam','CONVERTED',?,REPEAT('a',64),?,?,?,?)`, reservationID, "rotate-"+reservationID, reference, now.Add(time.Hour), now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO orders (id,reference,reservation_id,status,subtotal,admin_fee,discount,created_at,updated_at,expires_at)
			VALUES (?,?,?,'REFUND_PENDING',10000,0,0,?,?,?)`, orderID, reference, reservationID, now, now, now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO order_buyers (order_id,name,email,phone,identity) VALUES (?,'Buyer','rotate@example.test','081234567890','123456789012')", orderID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO order_refunds (id,order_id,status,original_order_status,amount,reason,requested_by_staff_id,refund_key,gateway_order_id,provider_transaction_id,requested_at,updated_at,next_attempt_at)
			VALUES (?,?,'UNKNOWN','CANCELLED',10000,'rotation test',?,?,?,'txn',?,?,?)`, refundID, orderID, staffID, "key-"+refundID, gatewayID, now, now, now.Add(-time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	last := fixtures[len(fixtures)-1]
	var mu sync.Mutex
	polled := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gatewayID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v2/"), "/status")
		mu.Lock()
		polled[gatewayID]++
		mu.Unlock()
		status, refunded := "settlement", "0.00"
		if gatewayID == last.gatewayID {
			status, refunded = "refund", "10000.00"
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"order_id": gatewayID, "transaction_id": "txn", "gross_amount": "10000.00", "transaction_status": status, "refund_amount": refunded})
	}))
	defer server.Close()
	service := New(db, staffauth.New(db), "test-key", "sandbox", nil, nil)
	service.base = server.URL
	if err := service.reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	firstBatch := len(polled)
	lastPolled := polled[last.gatewayID] > 0
	mu.Unlock()
	if firstBatch != 50 || lastPolled {
		t.Fatalf("first poll batch visited %d unique items; final item visited=%v", firstBatch, lastPolled)
	}
	if err := service.reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	visited := polled[last.gatewayID]
	mu.Unlock()
	if visited != 1 {
		t.Fatalf("51st refund was polled %d times after first batch was rescheduled", visited)
	}
	var status string
	if err := db.QueryRowContext(ctx, "SELECT status FROM orders WHERE id=?", last.orderID).Scan(&status); err != nil || status != "REFUNDED" {
		t.Fatalf("51st order status=%s error=%v", status, err)
	}
}

func cleanupRotationRows(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `SELECT DISTINCT rf.id,rf.order_id,o.reservation_id,rf.requested_by_staff_id
		FROM order_refunds rf JOIN orders o ON o.id=rf.order_id WHERE rf.gateway_order_id LIKE 'rotation-%'`)
	if err != nil {
		return err
	}
	type staleFixture struct{ refundID, orderID, reservationID, staffID string }
	var stale []staleFixture
	for rows.Next() {
		var f staleFixture
		if err := rows.Scan(&f.refundID, &f.orderID, &f.reservationID, &f.staffID); err != nil {
			rows.Close()
			return err
		}
		stale = append(stale, f)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, f := range stale {
		for _, step := range []struct {
			query string
			id    string
		}{
			{"DELETE FROM email_queue WHERE order_id=?", f.orderID},
			{"DELETE FROM order_refund_audit WHERE refund_id=?", f.refundID},
			{"DELETE FROM order_refunds WHERE id=?", f.refundID},
			{"DELETE FROM order_buyers WHERE order_id=?", f.orderID},
			{"DELETE FROM orders WHERE id=?", f.orderID},
			{"DELETE FROM reservations WHERE id=?", f.reservationID},
		} {
			if _, err := db.ExecContext(ctx, step.query, step.id); err != nil {
				return fmt.Errorf("cleanup stale rotation fixture: %w", err)
			}
		}
	}
	for _, f := range stale {
		if _, err := db.ExecContext(ctx, "DELETE FROM admin_audit_log WHERE actor_id=?", f.staffID); err != nil {
			return fmt.Errorf("cleanup stale rotation admin audit: %w", err)
		}
		if _, err := db.ExecContext(ctx, "DELETE FROM staff_users WHERE id=?", f.staffID); err != nil {
			return fmt.Errorf("cleanup stale rotation staff: %w", err)
		}
	}
	return nil
}
