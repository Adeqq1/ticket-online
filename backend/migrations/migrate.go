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
	locked, err := acquireLock(ctx, db)
	if err != nil {
		return err
	}
	if !locked {
		return fmt.Errorf("could not acquire migration lock")
	}
	defer func() { _, _ = db.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", lockName) }()

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
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
		if err := apply(ctx, db, item); err != nil {
			return err
		}
	}
	return nil
}

func acquireLock(ctx context.Context, db *sql.DB) (bool, error) {
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

func apply(ctx context.Context, db *sql.DB, item migration) error {
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

	for _, statement := range statements(string(item.data)) {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply migration %s: %w", item.name, err)
		}
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, UTC_TIMESTAMP(6))", item.version, item.name, checksumText); err != nil {
		return fmt.Errorf("record migration %s: %w", item.name, err)
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
