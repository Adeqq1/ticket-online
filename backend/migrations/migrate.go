package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
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
