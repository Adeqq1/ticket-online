package platform

import (
	"context"
	"database/sql"
	"fmt"
)

func BindPaymentEnvironment(ctx context.Context, db *sql.DB, environment string) error {
	if environment != "sandbox" && environment != "production" {
		return fmt.Errorf("invalid Midtrans environment")
	}
	_, err := db.ExecContext(ctx, `INSERT IGNORE INTO payment_environment (singleton, environment, created_at)
		VALUES (1, ?, UTC_TIMESTAMP(6))`, environment)
	if err != nil {
		return fmt.Errorf("bind payment environment: %w", err)
	}
	var stored string
	if err := db.QueryRowContext(ctx, "SELECT environment FROM payment_environment WHERE singleton = 1").Scan(&stored); err != nil {
		return fmt.Errorf("read payment environment binding: %w", err)
	}
	if stored != environment {
		return fmt.Errorf("database is bound to Midtrans %s; refusing to start as %s", stored, environment)
	}
	return nil
}
