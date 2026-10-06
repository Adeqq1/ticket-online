package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"testing"
)

func TestLoadMigrationsInVersionOrder(t *testing.T) {
	items, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 20 {
		t.Fatalf("migration count = %d, want 20", len(items))
	}
	for index, item := range items {
		if item.version != index+1 {
			t.Fatalf("migration[%d] version = %d, want %d", index, item.version, index+1)
		}
	}
	if len(statements(string(items[0].data))) < 6 {
		t.Fatal("initial migration was not split into executable statements")
	}
	if !strings.Contains(string(items[12].data), "CREATE TABLE IF NOT EXISTS email_queue") {
		t.Fatal("email queue migration was not loaded")
	}
	if !strings.Contains(string(items[13].data), "CREATE TABLE IF NOT EXISTS recovery_tokens") {
		t.Fatal("ticket recovery migration was not loaded")
	}
	if !strings.Contains(string(items[16].data), "CREATE TABLE IF NOT EXISTS admin_audit_log") {
		t.Fatal("admin audit migration was not loaded")
	}
	if !strings.Contains(string(items[17].data), "idx_orders_admin_created") {
		t.Fatal("admin order pagination index was not loaded")
	}
	if !strings.Contains(string(items[18].data), "check_lease_until") || !strings.Contains(string(items[18].data), "superseded_by") {
		t.Fatal("admin issue migration was not loaded")
	}
	if !strings.Contains(string(items[19].data), "payment_environment") || !strings.Contains(string(items[19].data), "'sandbox'") {
		t.Fatal("Midtrans environment migration was not loaded")
	}
}

func TestStatementsIgnoresEmptyStatements(t *testing.T) {
	got := statements(" CREATE TABLE example (id INT); ; ")
	if len(got) != 1 || got[0] != "CREATE TABLE example (id INT)" {
		t.Fatalf("unexpected statements: %#v", got)
	}
}

// Each case gets its own database: no application schema is modified or dropped.
func TestRecoveryMigrationResumesAfterDDL(t *testing.T) {
	dsn := os.Getenv("MYSQL_MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_MIGRATION_TEST_DSN to a MySQL account that can create disposable databases")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DBName = ""
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	items, err := load()
	if err != nil {
		t.Fatal(err)
	}
	original := statements(string(items[13].data))
	// Split the new resumable path into every DDL boundary, including individual columns and constraints.
	steps := append([]string{}, original[:4]...)
	steps = append(steps,
		"ALTER TABLE email_queue ADD COLUMN id CHAR(32) NULL FIRST",
		"ALTER TABLE email_queue ADD COLUMN kind ENUM('TICKETS', 'RECOVERY') NOT NULL DEFAULT 'TICKETS' AFTER id",
		"ALTER TABLE email_queue ADD COLUMN dedupe_key VARCHAR(80) NULL AFTER kind",
		"ALTER TABLE email_queue ADD COLUMN recovery_request_id CHAR(32) NULL AFTER order_id",
		"UPDATE email_queue SET id = order_id, dedupe_key = CONCAT('tickets:', order_id)",
		"ALTER TABLE email_queue DROP PRIMARY KEY, MODIFY id CHAR(32) NOT NULL, ADD PRIMARY KEY (id)",
		"ALTER TABLE email_queue MODIFY dedupe_key VARCHAR(80) NOT NULL",
		"ALTER TABLE email_queue MODIFY order_id CHAR(32) NULL",
		"ALTER TABLE email_queue ADD UNIQUE KEY uq_email_queue_dedupe (dedupe_key)",
		"ALTER TABLE email_queue ADD KEY ix_email_queue_order (order_id)",
		"ALTER TABLE email_queue ADD KEY ix_email_queue_recovery (recovery_request_id)",
		"ALTER TABLE email_queue ADD CONSTRAINT fk_email_queue_order FOREIGN KEY (order_id) REFERENCES orders (id) ON DELETE RESTRICT ON UPDATE RESTRICT",
		"ALTER TABLE email_queue ADD CONSTRAINT fk_email_queue_recovery FOREIGN KEY (recovery_request_id) REFERENCES recovery_requests (id) ON DELETE RESTRICT ON UPDATE RESTRICT",
		"ALTER TABLE email_queue ADD CONSTRAINT chk_email_queue_target CHECK ((kind = 'TICKETS' AND order_id IS NOT NULL AND recovery_request_id IS NULL) OR (kind = 'RECOVERY' AND order_id IS NULL AND recovery_request_id IS NOT NULL))",
	)
	cases := []struct {
		name     string
		partial  []string
		recorded bool
	}{}
	for n := 0; n <= len(steps); n++ {
		cases = append(cases, struct {
			name     string
			partial  []string
			recorded bool
		}{fmt.Sprintf("resumable_step_%d", n), steps[:n], false})
	}
	for n := 5; n <= len(original); n++ {
		cases = append(cases, struct {
			name     string
			partial  []string
			recorded bool
		}{fmt.Sprintf("original_step_%d", n), original[:n], false})
	}
	cases = append(cases, struct {
		name     string
		partial  []string
		recorded bool
	}{"already_applied_original", original, true})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			name := fmt.Sprintf("ticket_migration_14_%d", time.Now().UnixNano())
			if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if _, err := admin.Exec("DROP DATABASE " + name); err != nil {
					t.Error(err)
				}
			})
			testCfg := *cfg
			testCfg.DBName = name
			db, err := sql.Open("mysql", testCfg.FormatDSN())
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			conn, err := db.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			exec := func(query string, args ...any) {
				t.Helper()
				if _, err := conn.ExecContext(ctx, query, args...); err != nil {
					t.Fatal(err)
				}
			}
			exec("CREATE TABLE schema_migrations (version BIGINT UNSIGNED PRIMARY KEY, name VARCHAR(255) NOT NULL, checksum CHAR(64) NOT NULL, applied_at DATETIME(6) NOT NULL)")
			for _, item := range items[:13] {
				if err := apply(ctx, conn, item); err != nil {
					t.Fatal(err)
				}
			}
			for i, status := range []string{"PENDING", "SENT", "FAILED"} {
				id := fmt.Sprintf("%032d", i+1)
				exec(`INSERT INTO reservations (id, event_id, status, idempotency_key, request_hash, expires_at, created_at, updated_at)
					SELECT ?, id, 'CONVERTED', ?, ?, '2027-01-01', '2026-01-01', '2026-01-01' FROM events LIMIT 1`, id, id, strings.Repeat("a", 64))
				exec(`INSERT INTO orders (id, reference, reservation_id, status, subtotal, expires_at, created_at, updated_at)
					VALUES (?, ?, ?, 'PAID', 10000, '2027-01-01', '2026-01-01', '2026-01-01')`, id, "TO-"+fmt.Sprintf("%020d", i), id)
				exec(`INSERT INTO email_queue (order_id, recipient, status, attempts, next_attempt_at, lease_until, claim_token, sent_at, last_error, created_at, updated_at)
					VALUES (?, 'buyer@example.com', ?, ?, '2026-01-02', NULL, NULL, IF(? = 'SENT', '2026-01-03', NULL), ?, '2026-01-01', '2026-01-04')`, id, status, i+1, status, "previous error "+status)
			}
			snapshot := func() []string {
				t.Helper()
				rows, err := conn.QueryContext(ctx, `SELECT CONCAT_WS('|', order_id, recipient, status, attempts, next_attempt_at,
					COALESCE(lease_until, ''), COALESCE(claim_token, ''), COALESCE(sent_at, ''), last_error, created_at, updated_at) FROM email_queue ORDER BY order_id`)
				if err != nil {
					t.Fatal(err)
				}
				defer rows.Close()
				var values []string
				for rows.Next() {
					var value string
					if err := rows.Scan(&value); err != nil {
						t.Fatal(err)
					}
					values = append(values, value)
				}
				if err := rows.Err(); err != nil {
					t.Fatal(err)
				}
				return values
			}
			before := snapshot()
			if len(before) != 3 {
				t.Fatalf("fixture has %d jobs", len(before))
			}
			for _, step := range tc.partial {
				exec(step)
			}
			checksum := fmt.Sprintf("%x", sha256.Sum256(items[13].data))
			if tc.recorded {
				exec("INSERT INTO schema_migrations VALUES (14, ?, ?, UTC_TIMESTAMP(6))", items[13].name, checksum)
			}
			if err := Run(ctx, db); err != nil {
				t.Fatal(err)
			}
			if err := Run(ctx, db); err != nil {
				t.Fatalf("rerun: %v", err)
			}
			if after := snapshot(); !slices.Equal(before, after) {
				t.Fatalf("jobs changed: before %v, after %v", before, after)
			}
			var actualChecksum, primary string
			if err := conn.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE version = 14").Scan(&actualChecksum); err != nil || actualChecksum != checksum {
				t.Fatalf("checksum %q, error %v", actualChecksum, err)
			}
			if err := conn.QueryRowContext(ctx, "SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'email_queue' AND INDEX_NAME = 'PRIMARY'").Scan(&primary); err != nil || primary != "id" {
				t.Fatalf("primary key %q, error %v", primary, err)
			}
			var constraints, indexes, keys, columns int
			checks := []struct {
				query string
				dest  *int
				want  int
			}{
				{"SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'email_queue' AND CONSTRAINT_NAME IN ('fk_email_queue_order', 'fk_email_queue_recovery', 'chk_email_queue_target', 'chk_email_queue_attempts')", &constraints, 4},
				{"SELECT COUNT(DISTINCT INDEX_NAME) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'email_queue' AND INDEX_NAME IN ('uq_email_queue_dedupe', 'ix_email_queue_order', 'ix_email_queue_recovery')", &indexes, 3},
				{"SELECT COUNT(*) FROM email_queue WHERE id = order_id AND dedupe_key = CONCAT('tickets:', order_id) AND kind = 'TICKETS' AND recovery_request_id IS NULL", &keys, 3},
				{"SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'email_queue' AND ((COLUMN_NAME IN ('id', 'dedupe_key') AND IS_NULLABLE = 'NO') OR (COLUMN_NAME IN ('order_id', 'recovery_request_id') AND IS_NULLABLE = 'YES'))", &columns, 4},
			}
			for _, check := range checks {
				if err := conn.QueryRowContext(ctx, check.query).Scan(check.dest); err != nil || *check.dest != check.want {
					t.Fatalf("schema check: got %d, want %d, error %v", *check.dest, check.want, err)
				}
			}
			if tc.recorded {
				exec("UPDATE schema_migrations SET checksum = ? WHERE version = 14", strings.Repeat("0", 64))
				if err := Run(ctx, db); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
					t.Fatalf("checksum enforcement: %v", err)
				}
			}
		})
	}
}
