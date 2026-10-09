package adminreports

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/migrations"
	"github.com/go-sql-driver/mysql"
)

func TestPhase12EvaluationSQL(t *testing.T) {
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
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	script, err := os.ReadFile("../../../scripts/phase12-evaluation.sql")
	if err != nil {
		t.Fatal(err)
	}
	// Execute the shipped script on one connection, just like the mysql client.
	evaluate := func(from, to any) (map[string]sql.NullString, error) {
		defer conn.ExecContext(ctx, "ROLLBACK")
		if _, err := conn.ExecContext(ctx, "SET @phase12_date_from = ?, @phase12_date_to_exclusive = ?", from, to); err != nil {
			return nil, err
		}
		var result map[string]sql.NullString
		for _, statement := range strings.Split(string(script), ";") {
			var lines []string
			for _, line := range strings.Split(statement, "\n") {
				if !strings.HasPrefix(strings.TrimSpace(line), "--") {
					lines = append(lines, line)
				}
			}
			statement = strings.TrimSpace(strings.Join(lines, "\n"))
			if statement == "" {
				continue
			}
			if !strings.HasPrefix(statement, "WITH dates AS") {
				if _, err := conn.ExecContext(ctx, statement); err != nil {
					return nil, err
				}
				continue
			}
			// A write to a real table must fail inside the script's read-only snapshot.
			_, writeErr := conn.ExecContext(ctx, "UPDATE events SET artist=artist WHERE id='phase12-no-event'")
			var mysqlErr *mysql.MySQLError
			if !errors.As(writeErr, &mysqlErr) || mysqlErr.Number != 1792 {
				return nil, fmt.Errorf("expected read-only transaction rejection, got %v", writeErr)
			}
			rows, err := conn.QueryContext(ctx, statement)
			if err != nil {
				return nil, err
			}
			columns, err := rows.Columns()
			if err != nil {
				rows.Close()
				return nil, err
			}
			allowed := []string{"date_from", "date_to_exclusive", "time_zone", "snapshot_at_utc", "payment_environment", "period_status", "paid_orders", "orders_without_buyer", "unique_buyers", "repeat_buyers", "repeat_buyer_percent"}
			if !slices.Equal(columns, allowed) {
				rows.Close()
				return nil, fmt.Errorf("unexpected output columns: %v", columns)
			}
			values, targets := make([]sql.NullString, len(columns)), make([]any, len(columns))
			for i := range values {
				targets[i] = &values[i]
			}
			if !rows.Next() {
				rows.Close()
				return nil, fmt.Errorf("missing aggregate row")
			}
			if err := rows.Scan(targets...); err != nil {
				rows.Close()
				return nil, err
			}
			result = make(map[string]sql.NullString, len(columns))
			for i, column := range columns {
				result[column] = values[i]
			}
			extra := rows.Next()
			err = rows.Err()
			rows.Close()
			if err != nil {
				return nil, err
			}
			if extra {
				return nil, fmt.Errorf("expected only one aggregate row")
			}
		}
		return result, nil
	}
	check := func(from, to any, expected map[string]string, nulls ...string) {
		t.Helper()
		result, err := evaluate(from, to)
		if err != nil {
			t.Fatal(err)
		}
		for key, want := range expected {
			if got := result[key]; !got.Valid || got.String != want {
				t.Errorf("%s: got %v, want %q", key, got, want)
			}
		}
		for _, key := range nulls {
			if result[key].Valid {
				t.Errorf("%s: expected NULL, got %v", key, result[key])
			}
		}
		if !result["snapshot_at_utc"].Valid {
			t.Error("missing snapshot timestamp")
		}
	}
	check("2001-02-01", "2001-03-01", map[string]string{"period_status": "OK", "paid_orders": "0", "unique_buyers": "0", "repeat_buyers": "0", "orders_without_buyer": "0", "time_zone": "Asia/Jakarta"}, "repeat_buyer_percent")

	id := func() string {
		var bytes [16]byte
		if _, err := rand.Read(bytes[:]); err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(bytes[:])
	}
	eventID := "phase12-" + id()
	now := time.Now().UTC()
	if _, err := conn.ExecContext(ctx, `INSERT INTO events (id,artist,city,venue,address,starts_at,genre,status,image_url,description,created_at,updated_at)
		VALUES (?,'Phase 12','Jakarta','Venue','Address',?,'Indie','EARLY_BIRD','','Test',?,?)`, eventID, now, now, now); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, statement := range []string{
			"DELETE FROM payments WHERE order_id IN (SELECT id FROM orders WHERE reservation_id IN (SELECT id FROM reservations WHERE event_id=?))",
			"DELETE FROM order_buyers WHERE order_id IN (SELECT id FROM orders WHERE reservation_id IN (SELECT id FROM reservations WHERE event_id=?))",
			"DELETE FROM orders WHERE reservation_id IN (SELECT id FROM reservations WHERE event_id=?)",
			"DELETE FROM reservations WHERE event_id=?",
			"DELETE FROM events WHERE id=?",
		} {
			if _, err := conn.ExecContext(context.Background(), statement, eventID); err != nil {
				t.Errorf("clean phase12 fixture: %v", err)
			}
		}
	})
	start := time.Date(2001, 1, 31, 17, 0, 0, 0, time.UTC)
	end := time.Date(2001, 2, 28, 17, 0, 0, 0, time.UTC)
	before, middle, last := start.Add(-time.Microsecond), start.Add(time.Hour), end.Add(-time.Microsecond)
	domain := "@" + eventID + ".example.test"
	fixtures := []struct {
		email, paymentStatus, orderStatus string
		paidAt                            *time.Time
	}{
		{"alice" + domain, "SUCCEEDED", "PAID", &before},
		{" ALICE" + domain + " ", "SUCCEEDED", "PAID", &start},
		{"bob" + domain, "SUCCEEDED", "PAID", &middle},
		{"BOB" + domain, "SUCCEEDED", "REFUNDED", &last},
		{"carol" + domain, "FAILED", "PENDING", &before},
		{"carol" + domain, "SUCCEEDED", "PAID", &middle},
		{"carol" + domain, "SUCCEEDED", "PAID", &end},
		{"dave" + domain, "FAILED", "PENDING", &middle},
		{"eve" + domain, "PENDING", "PENDING", nil},
		{"café" + domain, "SUCCEEDED", "PAID", &middle},
		{"cafe" + domain, "SUCCEEDED", "PAID", &middle},
		{"", "SUCCEEDED", "PAID", &middle},
		{"no-time" + domain, "SUCCEEDED", "PAID", nil},
	}
	for _, fixture := range fixtures {
		orderID := id()
		if _, err := conn.ExecContext(ctx, `INSERT INTO reservations (id,event_id,status,idempotency_key,request_hash,expires_at,created_at,updated_at)
			VALUES (?,?,'CONVERTED',?,REPEAT('a',64),?,?,?)`, orderID, eventID, orderID, now.Add(time.Hour), now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO orders (id,reference,reservation_id,status,subtotal,expires_at,created_at,updated_at)
			VALUES (?,?,?,?,1000,?,?,?)`, orderID, orderID, orderID, fixture.orderStatus, now.Add(time.Hour), now, now); err != nil {
			t.Fatal(err)
		}
		if fixture.email != "" {
			if _, err := conn.ExecContext(ctx, `INSERT INTO order_buyers (order_id,name,email,phone,identity) VALUES (?,'Test Buyer',?,'081234567890','123456789012')`, orderID, fixture.email); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO payments (id,order_id,method,amount,status,paid_at,created_at,updated_at)
			VALUES (?,?,'QRIS',1000,?,?,?,?)`, id(), orderID, fixture.paymentStatus, fixture.paidAt, now, now); err != nil {
			t.Fatal(err)
		}
	}
	check("2001-02-01", "2001-03-01", map[string]string{"period_status": "OK", "paid_orders": "7", "orders_without_buyer": "1", "unique_buyers": "5", "repeat_buyers": "2", "repeat_buyer_percent": "40.00"})
	// A later payment makes Carol repeat only when the reporting window includes it.
	check("2001-03-01", "2001-03-02", map[string]string{"period_status": "OK", "paid_orders": "1", "unique_buyers": "1", "repeat_buyers": "1", "repeat_buyer_percent": "100.00"})
	todayWIB := now.In(wib).Format("2006-01-02")
	fromWIB := now.In(wib).AddDate(0, 0, -30).Format("2006-01-02")
	check(nil, nil, map[string]string{"period_status": "OK", "date_from": fromWIB, "date_to_exclusive": todayWIB, "paid_orders": "0"}, "repeat_buyer_percent")
	for _, period := range [][2]string{{"2001-03-01", "2001-02-01"}, {"2001-02-01", "2001-02-01"}, {"2001-2-01", "2001-03-01"}, {"2000-01-01", "2001-03-01"}, {todayWIB, now.In(wib).AddDate(0, 0, 1).Format("2006-01-02")}} {
		check(period[0], period[1], map[string]string{"period_status": "INVALID_PERIOD"}, "paid_orders", "orders_without_buyer", "unique_buyers", "repeat_buyers", "repeat_buyer_percent")
	}
	if result, err := evaluate("2001-02-30", "2001-03-01"); err == nil && result["period_status"].String != "INVALID_PERIOD" {
		t.Error("invalid calendar date was accepted")
	}
	var remaining int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM payments WHERE order_id IN (SELECT id FROM orders WHERE reservation_id IN (SELECT id FROM reservations WHERE event_id=?))", eventID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != len(fixtures) {
		t.Errorf("fixture changed after evaluation: got %d payments, want %d", remaining, len(fixtures))
	}
}
