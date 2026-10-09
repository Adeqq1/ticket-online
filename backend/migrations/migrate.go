package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed *.sql
var files embed.FS

const lockName = "ticket-online-migrations"

type migration struct {
	version int
	name    string
	data    []byte
}

func Run(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	locked, err := acquireLock(ctx, conn)
	if !locked {
		return fmt.Errorf("could not acquire migration lock")
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(releaseCtx, "SELECT RELEASE_LOCK(?)", lockName)
	}()

	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT UNSIGNED PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		checksum CHAR(64) NOT NULL,
		applied_at DATETIME(6) NOT NULL
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	items, err := load()
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := apply(ctx, conn, item); err != nil {
			return err
		}
	}
	return nil
}

func acquireLock(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) (bool, error) {
	var locked bool
	if err := db.QueryRowContext(ctx, "SELECT GET_LOCK(?, 30)", lockName).Scan(&locked); err != nil {
		return false, fmt.Errorf("acquire migration lock: %w", err)
	}
	return locked, nil
}

func load() ([]migration, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	items := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".sql" {
			continue
		}
		parts := strings.SplitN(entry.Name(), "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil || version < 1 {
			return nil, fmt.Errorf("invalid migration version in %q", entry.Name())
		}
		data, err := files.ReadFile(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		items = append(items, migration{version: version, name: entry.Name(), data: data})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].version < items[j].version })
	return items, nil
}

func apply(ctx context.Context, db *sql.Conn, item migration) error {
	checksum := sha256.Sum256(item.data)
	checksumText := hex.EncodeToString(checksum[:])
	var appliedChecksum string
	err := db.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE version = ?", item.version).Scan(&appliedChecksum)
	switch err {
	case nil:
		if appliedChecksum != checksumText {
			return fmt.Errorf("migration %s checksum mismatch", item.name)
		}
		return nil
	case sql.ErrNoRows:
	default:
		return fmt.Errorf("check migration %s: %w", item.name, err)
	}

	if item.version == 14 {
		// MySQL DDL commits implicitly. Resume this upgrade before recording its original checksum.
		if err := resumeRecovery(ctx, db, item); err != nil {
			return fmt.Errorf("apply migration %s: %w", item.name, err)
		}
		_, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, UTC_TIMESTAMP(6))", item.version, item.name, checksumText)
		return err
	}
	if item.version == 15 {
		if err := resumeEventPublication(ctx, db); err != nil {
			return fmt.Errorf("apply migration %s: %w", item.name, err)
		}
		_, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, UTC_TIMESTAMP(6))", item.version, item.name, checksumText)
		return err
	}
	if item.version == 16 {
		if err := resumeAdminCatalog(ctx, db); err != nil {
			return fmt.Errorf("apply migration %s: %w", item.name, err)
		}
		_, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, UTC_TIMESTAMP(6))", item.version, item.name, checksumText)
		return err
	}
	if item.version == 19 {
		if err := resumeAdminIssues(ctx, db); err != nil {
			return fmt.Errorf("apply migration %s: %w", item.name, err)
		}
		_, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, UTC_TIMESTAMP(6))", item.version, item.name, checksumText)
		return err
	}
	if item.version == 18 {
		if err := resumeAdminOrders(ctx, db); err != nil {
			return fmt.Errorf("apply migration %s: %w", item.name, err)
		}
		_, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, UTC_TIMESTAMP(6))", item.version, item.name, checksumText)
		return err
	}
	if item.version == 20 {
		if err := resumePaymentEnvironment(ctx, db); err != nil {
			return fmt.Errorf("apply migration %s: %w", item.name, err)
		}
		_, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, UTC_TIMESTAMP(6))", item.version, item.name, checksumText)
		return err
	}
	if item.version == 21 {
		if err := resumeRefunds(ctx, db); err != nil {
			return fmt.Errorf("apply migration %s: %w", item.name, err)
		}
		_, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, UTC_TIMESTAMP(6))", item.version, item.name, checksumText)
		return err
	}
	if item.version == 22 {
		if err := resumeRefundQueue(ctx, db); err != nil {
			return fmt.Errorf("apply migration %s: %w", item.name, err)
		}
		_, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, UTC_TIMESTAMP(6))", item.version, item.name, checksumText)
		return err
	}
	if item.version == 23 {
		if err := resumeAdminSalesReports(ctx, db); err != nil {
			return fmt.Errorf("apply migration %s: %w", item.name, err)
		}
		_, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, UTC_TIMESTAMP(6))", item.version, item.name, checksumText)
		return err
	}
	if item.version == 24 {
		if err := resumeAdminAttendanceReports(ctx, db); err != nil {
			return fmt.Errorf("apply migration %s: %w", item.name, err)
		}
		_, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, UTC_TIMESTAMP(6))", item.version, item.name, checksumText)
		return err
	}
	if item.version == 25 {
		if err := resumeConversion(ctx, db, item); err != nil {
			return fmt.Errorf("apply migration %s: %w", item.name, err)
		}
		_, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, UTC_TIMESTAMP(6))", item.version, item.name, checksumText)
		return err
	}

	if item.version == 26 {
		if err := resumeEventChanges(ctx, db, item); err != nil {
			return fmt.Errorf("apply migration %s: %w", item.name, err)
		}
		_, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (version,name,checksum,applied_at) VALUES (?,?,?,UTC_TIMESTAMP(6))", item.version, item.name, checksumText)
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", item.name, err)
	}
	defer tx.Rollback()
	for _, statement := range statements(string(item.data)) {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply migration %s: %w", item.name, err)
		}
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, UTC_TIMESTAMP(6))", item.version, item.name, checksumText); err != nil {
		return fmt.Errorf("record migration %s: %w", item.name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", item.name, err)
	}
	return nil
}

func resumeConversion(ctx context.Context, db *sql.Conn, item migration) error {
	items := statements(string(item.data))
	if len(items) < 2 {
		return fmt.Errorf("conversion migration is incomplete")
	}
	for _, statement := range items[:len(items)-1] {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='payments' AND COLUMN_NAME='started_at')`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		if _, err := db.ExecContext(ctx, items[len(items)-1]); err != nil {
			return err
		}
	}
	return nil
}

func resumeRefundQueue(ctx context.Context, db *sql.Conn) error {
	columns := []struct{ name, definition string }{
		{"next_attempt_at", "DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)"},
		{"last_checked_at", "DATETIME(6) NULL"},
		{"last_check_error", "VARCHAR(512) NOT NULL DEFAULT ''"},
		{"claim_token", "CHAR(32) NULL"},
		{"lease_until", "DATETIME(6) NULL"},
		{"retry_deadline", "DATETIME(6) NULL"},
	}
	for _, column := range columns {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'order_refunds' AND COLUMN_NAME = ?)`, column.name).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			if _, err := db.ExecContext(ctx, "ALTER TABLE order_refunds ADD COLUMN "+column.name+" "+column.definition); err != nil {
				return err
			}
		}
	}
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'order_refunds' AND INDEX_NAME = 'ix_order_refunds_due')`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		_, err := db.ExecContext(ctx, "ALTER TABLE order_refunds ADD KEY ix_order_refunds_due (status, next_attempt_at, id)")
		return err
	}
	return nil
}

func resumeAdminOrders(ctx context.Context, db *sql.Conn) error {
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND INDEX_NAME = 'idx_orders_admin_created')`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		_, err := db.ExecContext(ctx, "ALTER TABLE orders ADD KEY idx_orders_admin_created (created_at, id)")
		return err
	}
	return nil
}

func resumeAdminSalesReports(ctx context.Context, db *sql.Conn) error {
	for _, index := range []struct{ table, name, definition string }{
		{"payments", "ix_payments_admin_sales", "(status, paid_at, order_id)"},
		{"order_refunds", "ix_order_refunds_admin_sales", "(status, completed_at, order_id)"},
	} {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.STATISTICS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ?)`, index.table, index.name).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			if _, err := db.ExecContext(ctx, "ALTER TABLE "+index.table+" ADD KEY "+index.name+" "+index.definition); err != nil {
				return err
			}
		}
	}
	return nil
}

func resumeAdminAttendanceReports(ctx context.Context, db *sql.Conn) error {
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ticket_checkins' AND INDEX_NAME = 'ix_ticket_checkins_admin_attendance')`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		_, err := db.ExecContext(ctx, "ALTER TABLE ticket_checkins ADD KEY ix_ticket_checkins_admin_attendance (event_id, gate, checked_in_at)")
		return err
	}
	return nil
}

func resumePaymentEnvironment(ctx context.Context, db *sql.Conn) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS payment_environment (
		singleton TINYINT UNSIGNED NOT NULL PRIMARY KEY CHECK (singleton = 1),
		environment ENUM('sandbox', 'production') NOT NULL,
		created_at DATETIME(6) NOT NULL
	)`); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, `INSERT IGNORE INTO payment_environment (singleton, environment, created_at)
		SELECT 1, 'sandbox', UTC_TIMESTAMP(6) WHERE EXISTS (SELECT 1 FROM payments WHERE gateway_order_id IS NOT NULL)`)
	return err
}

func resumeRefunds(ctx context.Context, db *sql.Conn) error {
	if _, err := db.ExecContext(ctx, `ALTER TABLE orders MODIFY status ENUM('PENDING', 'PAID', 'CANCELLED', 'EXPIRED', 'REFUND_PENDING', 'REFUNDED') NOT NULL DEFAULT 'PENDING'`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS order_refunds (
		id CHAR(32) PRIMARY KEY,
		order_id CHAR(32) NOT NULL,
		status ENUM('REQUESTED', 'PROCESSING', 'SUCCEEDED', 'FAILED', 'UNKNOWN') NOT NULL,
		original_order_status ENUM('PAID', 'CANCELLED', 'EXPIRED') NOT NULL,
		amount BIGINT UNSIGNED NOT NULL,
		reason VARCHAR(500) NOT NULL,
		requested_by_staff_id CHAR(32) NOT NULL,
		refund_key VARCHAR(80) NOT NULL,
		gateway_order_id VARCHAR(50) NOT NULL,
		provider_transaction_id VARCHAR(100) NOT NULL DEFAULT '',
		attempts TINYINT UNSIGNED NOT NULL DEFAULT 0,
		last_error VARCHAR(512) NOT NULL DEFAULT '',
		requested_at DATETIME(6) NOT NULL,
		updated_at DATETIME(6) NOT NULL,
		completed_at DATETIME(6) NULL,
		UNIQUE KEY uq_order_refunds_order (order_id),
		UNIQUE KEY uq_order_refunds_key (refund_key),
		KEY ix_order_refunds_reconcile (status, updated_at),
		CONSTRAINT fk_order_refunds_order FOREIGN KEY (order_id) REFERENCES orders (id) ON DELETE RESTRICT ON UPDATE RESTRICT,
		CONSTRAINT fk_order_refunds_staff FOREIGN KEY (requested_by_staff_id) REFERENCES staff_users (id) ON DELETE RESTRICT ON UPDATE RESTRICT,
		CONSTRAINT chk_order_refunds_amount CHECK (amount > 0)
	)`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS order_refund_audit (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
		refund_id CHAR(32) NOT NULL,
		staff_id CHAR(32) NOT NULL,
		action VARCHAR(32) NOT NULL,
		reason VARCHAR(500) NOT NULL,
		created_at DATETIME(6) NOT NULL,
		KEY ix_order_refund_audit_refund (refund_id, id),
		CONSTRAINT fk_order_refund_audit_refund FOREIGN KEY (refund_id) REFERENCES order_refunds (id) ON DELETE RESTRICT ON UPDATE RESTRICT,
		CONSTRAINT fk_order_refund_audit_staff FOREIGN KEY (staff_id) REFERENCES staff_users (id) ON DELETE RESTRICT ON UPDATE RESTRICT
	)`); err != nil {
		return err
	}
	var columnExists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'email_queue' AND COLUMN_NAME = 'refund_snapshot')`).Scan(&columnExists); err != nil {
		return err
	}
	if !columnExists {
		if _, err := db.ExecContext(ctx, "ALTER TABLE email_queue ADD COLUMN refund_snapshot JSON NULL"); err != nil {
			return err
		}
	}
	if _, err := db.ExecContext(ctx, "ALTER TABLE email_queue MODIFY kind ENUM('TICKETS', 'RECOVERY', 'REFUND') NOT NULL"); err != nil {
		return err
	}
	var check string
	err := db.QueryRowContext(ctx, `SELECT cc.CHECK_CLAUSE FROM information_schema.CHECK_CONSTRAINTS cc
		JOIN information_schema.TABLE_CONSTRAINTS tc ON tc.CONSTRAINT_SCHEMA = cc.CONSTRAINT_SCHEMA AND tc.CONSTRAINT_NAME = cc.CONSTRAINT_NAME
		WHERE tc.TABLE_SCHEMA = DATABASE() AND tc.TABLE_NAME = 'email_queue' AND tc.CONSTRAINT_NAME = 'chk_email_queue_target'`).Scan(&check)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil {
		upper := strings.ToUpper(check)
		if strings.Contains(upper, "KIND = 'REFUND'") && strings.Contains(upper, "REFUND_SNAPSHOT IS NOT NULL") {
			return nil
		}
		if _, err := db.ExecContext(ctx, "ALTER TABLE email_queue DROP CHECK chk_email_queue_target"); err != nil {
			return err
		}
	}
	_, err = db.ExecContext(ctx, `ALTER TABLE email_queue ADD CONSTRAINT chk_email_queue_target CHECK (
		(KIND = 'TICKETS' AND order_id IS NOT NULL AND recovery_request_id IS NULL) OR
		(KIND = 'REFUND' AND order_id IS NOT NULL AND recovery_request_id IS NULL AND refund_snapshot IS NOT NULL) OR
		(KIND = 'RECOVERY' AND order_id IS NULL AND recovery_request_id IS NOT NULL))`)
	return err
}

func resumeAdminIssues(ctx context.Context, db *sql.Conn) error {
	columns := []struct{ table, name, definition string }{
		{"payment_reconciliation_cases", "reason", "VARCHAR(80) NOT NULL DEFAULT 'PAYMENT_SUCCEEDED_AFTER_ORDER_CLOSED'"},
		{"payment_reconciliation_cases", "last_checked_at", "DATETIME(6) NULL"},
		{"payment_reconciliation_cases", "last_check_error", "VARCHAR(512) NOT NULL DEFAULT ''"},
		{"payment_reconciliation_cases", "check_token", "CHAR(32) NULL"},
		{"payment_reconciliation_cases", "check_lease_until", "DATETIME(6) NULL"},
		{"email_queue", "superseded_by", "CHAR(32) NULL"},
	}
	for _, column := range columns {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?)`, column.table, column.name).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			if _, err := db.ExecContext(ctx, "ALTER TABLE "+column.table+" ADD COLUMN "+column.name+" "+column.definition); err != nil {
				return err
			}
		}
	}
	var indexExists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'email_queue' AND INDEX_NAME = 'ix_email_queue_admin_failed')`).Scan(&indexExists); err != nil {
		return err
	}
	if !indexExists {
		_, err := db.ExecContext(ctx, "ALTER TABLE email_queue ADD KEY ix_email_queue_admin_failed (status, updated_at, id)")
		return err
	}
	return nil
}

func resumeAdminCatalog(ctx context.Context, db *sql.Conn) error {
	var nullable string
	err := db.QueryRowContext(ctx, `SELECT IS_NULLABLE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='events' AND COLUMN_NAME='starts_at'`).Scan(&nullable)
	if err != nil {
		return err
	}
	if nullable != "YES" {
		if _, err := db.ExecContext(ctx, "ALTER TABLE events MODIFY starts_at DATETIME(6) NULL"); err != nil {
			return err
		}
	}
	var checkExists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='ticket_tiers' AND CONSTRAINT_NAME='chk_ticket_tiers_capacity')`).Scan(&checkExists); err != nil {
		return err
	}
	if checkExists {
		if _, err := db.ExecContext(ctx, "ALTER TABLE ticket_tiers DROP CHECK chk_ticket_tiers_capacity"); err != nil {
			return err
		}
	}
	if _, err := db.ExecContext(ctx, "ALTER TABLE ticket_tiers ADD CONSTRAINT chk_ticket_tiers_capacity CHECK (capacity >= 0)"); err != nil && !strings.Contains(err.Error(), "Duplicate check constraint") {
		return err
	}
	_, err = db.ExecContext(ctx, `UPDATE events e SET publication_status='DRAFT' WHERE publication_status='PUBLISHED' AND (
		e.starts_at IS NULL OR CHAR_LENGTH(TRIM(e.artist)) < 2 OR CHAR_LENGTH(TRIM(e.city)) < 2 OR CHAR_LENGTH(TRIM(e.venue)) < 2 OR CHAR_LENGTH(TRIM(e.address)) < 2 OR
		CHAR_LENGTH(TRIM(e.description))=0 OR e.image_url NOT REGEXP '^https?://[^/ ]+' OR
		NOT EXISTS(SELECT 1 FROM event_lineups l WHERE l.event_id=e.id AND CHAR_LENGTH(TRIM(l.name)) > 0) OR
		NOT EXISTS(SELECT 1 FROM event_zones z WHERE z.event_id=e.id) OR EXISTS(SELECT 1 FROM event_zones z WHERE z.event_id=e.id AND CHAR_LENGTH(TRIM(z.name))=0) OR
		NOT EXISTS(SELECT 1 FROM ticket_tiers t WHERE t.event_id=e.id) OR EXISTS(SELECT 1 FROM ticket_tiers t WHERE t.event_id=e.id AND (CHAR_LENGTH(TRIM(t.name))=0 OR CHAR_LENGTH(TRIM(t.gate))=0)))`)
	return err
}

func resumeEventPublication(ctx context.Context, db *sql.Conn) error {
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'events' AND COLUMN_NAME = 'publication_status')`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		if _, err := db.ExecContext(ctx, "ALTER TABLE events ADD COLUMN publication_status ENUM('DRAFT', 'PUBLISHED', 'ARCHIVED') NOT NULL DEFAULT 'PUBLISHED'"); err != nil {
			return err
		}
	}
	if _, err := db.ExecContext(ctx, "UPDATE events SET publication_status = 'PUBLISHED'"); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, "ALTER TABLE events ALTER COLUMN publication_status SET DEFAULT 'DRAFT'")
	return err
}

// ponytail: only upgrade 014 needs schema-aware recovery; extend per-upgrade when another non-atomic DDL upgrade is introduced.
func resumeRecovery(ctx context.Context, db *sql.Conn, item migration) error {
	exec := func(statement string) error {
		_, err := db.ExecContext(ctx, statement)
		return err
	}
	for _, statement := range statements(string(item.data))[:3] {
		if err := exec(statement); err != nil {
			return err
		}
	}
	exists := func(table, predicate string, args ...any) (bool, error) {
		var found bool
		err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM information_schema."+table+
			" WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'email_queue' AND "+predicate+")", args...).Scan(&found)
		return found, err
	}
	primaryIsID, err := exists("STATISTICS", "INDEX_NAME = 'PRIMARY' AND COLUMN_NAME = 'id'")
	if err != nil {
		return err
	}
	if !primaryIsID {
		found, err := exists("TABLE_CONSTRAINTS", "CONSTRAINT_NAME = 'fk_email_queue_order'")
		if err != nil {
			return err
		}
		if found {
			if err := exec("ALTER TABLE email_queue DROP FOREIGN KEY fk_email_queue_order"); err != nil {
				return err
			}
		}
	}
	for _, column := range []struct{ name, definition string }{
		{"id", "CHAR(32) NULL FIRST"},
		{"kind", "ENUM('TICKETS', 'RECOVERY') NOT NULL DEFAULT 'TICKETS' AFTER id"},
		{"dedupe_key", "VARCHAR(80) NULL AFTER kind"},
		{"recovery_request_id", "CHAR(32) NULL AFTER order_id"},
	} {
		found, err := exists("COLUMNS", "COLUMN_NAME = ?", column.name)
		if err != nil {
			return err
		}
		if !found {
			if err := exec("ALTER TABLE email_queue ADD COLUMN " + column.name + " " + column.definition); err != nil {
				return err
			}
		}
	}
	if err := exec(`UPDATE email_queue SET id = COALESCE(id, order_id), dedupe_key = COALESCE(dedupe_key, CONCAT('tickets:', order_id))
		WHERE kind = 'TICKETS' AND order_id IS NOT NULL AND (id IS NULL OR dedupe_key IS NULL)`); err != nil {
		return err
	}
	var invalid bool
	if err := db.QueryRowContext(ctx, `SELECT
		EXISTS(SELECT 1 FROM email_queue WHERE id IS NULL OR dedupe_key IS NULL) OR
		EXISTS(SELECT 1 FROM email_queue GROUP BY id HAVING COUNT(*) > 1) OR
		EXISTS(SELECT 1 FROM email_queue GROUP BY dedupe_key HAVING COUNT(*) > 1)`).Scan(&invalid); err != nil {
		return err
	}
	if invalid {
		return fmt.Errorf("email_queue contains missing or duplicate recovery keys")
	}
	if !primaryIsID {
		primaryExists, err := exists("STATISTICS", "INDEX_NAME = 'PRIMARY'")
		if err != nil {
			return err
		}
		statement := "ALTER TABLE email_queue MODIFY id CHAR(32) NOT NULL, ADD PRIMARY KEY (id)"
		if primaryExists {
			statement += ", DROP PRIMARY KEY"
		}
		if err := exec(statement); err != nil {
			return err
		}
	}
	for _, column := range []struct{ name, nullable, definition string }{
		{"id", "NO", "CHAR(32) NOT NULL"},
		{"dedupe_key", "NO", "VARCHAR(80) NOT NULL"},
		{"order_id", "YES", "CHAR(32) NULL"},
	} {
		ready, err := exists("COLUMNS", "COLUMN_NAME = ? AND IS_NULLABLE = ?", column.name, column.nullable)
		if err != nil {
			return err
		}
		if !ready {
			if err := exec("ALTER TABLE email_queue MODIFY " + column.name + " " + column.definition); err != nil {
				return err
			}
		}
	}
	for _, index := range []struct{ name, definition string }{
		{"uq_email_queue_dedupe", "UNIQUE KEY uq_email_queue_dedupe (dedupe_key)"},
		{"ix_email_queue_order", "KEY ix_email_queue_order (order_id)"},
		{"ix_email_queue_recovery", "KEY ix_email_queue_recovery (recovery_request_id)"},
	} {
		found, err := exists("STATISTICS", "INDEX_NAME = ?", index.name)
		if err != nil {
			return err
		}
		if !found {
			if err := exec("ALTER TABLE email_queue ADD " + index.definition); err != nil {
				return err
			}
		}
	}
	for _, constraint := range []struct{ name, definition string }{
		{"fk_email_queue_order", "FOREIGN KEY (order_id) REFERENCES orders (id) ON DELETE RESTRICT ON UPDATE RESTRICT"},
		{"fk_email_queue_recovery", "FOREIGN KEY (recovery_request_id) REFERENCES recovery_requests (id) ON DELETE RESTRICT ON UPDATE RESTRICT"},
		{"chk_email_queue_target", "CHECK ((kind = 'TICKETS' AND order_id IS NOT NULL AND recovery_request_id IS NULL) OR (kind = 'RECOVERY' AND order_id IS NULL AND recovery_request_id IS NOT NULL))"},
	} {
		found, err := exists("TABLE_CONSTRAINTS", "CONSTRAINT_NAME = ?", constraint.name)
		if err != nil {
			return err
		}
		if !found {
			if err := exec("ALTER TABLE email_queue ADD CONSTRAINT " + constraint.name + " " + constraint.definition); err != nil {
				return err
			}
		}
	}
	return nil
}

func statements(sqlText string) []string {
	var result []string
	for _, statement := range strings.Split(sqlText, ";") {
		if statement = strings.TrimSpace(statement); statement != "" {
			result = append(result, statement)
		}
	}
	return result
}

func resumeEventChanges(ctx context.Context, db *sql.Conn, item migration) error {
	for _, column := range []struct{ table, name, definition string }{
		{"events", "lifecycle_status", "ENUM('SCHEDULED','POSTPONED','RESCHEDULED','CANCELLED') NOT NULL DEFAULT 'SCHEDULED'"},
		{"events", "change_version", "BIGINT UNSIGNED NOT NULL DEFAULT 0"},
		{"events", "sales_paused", "BOOLEAN NOT NULL DEFAULT FALSE"},
		{"orders", "access_deadline", "DATETIME(6) NULL"},
		{"order_refunds", "manual_reference", "VARCHAR(160) NOT NULL DEFAULT ''"},
		{"order_refunds", "manual_paid_at", "DATETIME(6) NULL"},
		{"email_queue", "event_change_id", "CHAR(32) NULL"},
	} {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND COLUMN_NAME=?)`, column.table, column.name).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			if _, err := db.ExecContext(ctx, "ALTER TABLE "+column.table+" ADD COLUMN "+column.name+" "+column.definition); err != nil {
				return err
			}
		}
	}
	for _, statement := range statements(string(item.data)) {
		if strings.HasPrefix(statement, "ALTER TABLE events") || strings.HasPrefix(statement, "ALTER TABLE orders") {
			continue
		}
		if strings.HasPrefix(statement, "ALTER TABLE order_refunds") {
			statement = "ALTER TABLE order_refunds MODIFY status ENUM('REQUESTED','PROCESSING','SUCCEEDED','FAILED','UNKNOWN','MANUAL_REQUIRED') NOT NULL"
		}
		if strings.HasPrefix(statement, "ALTER TABLE email_queue") {
			if _, err := db.ExecContext(ctx, "ALTER TABLE email_queue MODIFY kind ENUM('TICKETS','RECOVERY','REFUND','EVENT_CHANGE') NOT NULL"); err != nil {
				return err
			}
			var exists bool
			if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='email_queue' AND CONSTRAINT_NAME='chk_email_queue_target')`).Scan(&exists); err != nil {
				return err
			}
			if exists {
				if _, err := db.ExecContext(ctx, "ALTER TABLE email_queue DROP CHECK chk_email_queue_target"); err != nil {
					return err
				}
			}
			statement = "ALTER TABLE email_queue " + statement[strings.Index(statement, "ADD CONSTRAINT"):]
		}
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}
