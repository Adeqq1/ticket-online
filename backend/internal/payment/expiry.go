package payment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (r *Repository) ExpirePendingOrders(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, "SELECT id FROM orders WHERE status = 'PENDING' AND expires_at <= UTC_TIMESTAMP(6) ORDER BY expires_at, id LIMIT 100")
	if err != nil {
		return fmt.Errorf("find expired orders: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan expired order: %w", err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return fmt.Errorf("iterate expired orders: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close expired orders: %w", closeErr)
	}
	for _, id := range ids {
		if err := r.expireOrder(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) expireOrder(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin order expiration: %w", err)
	}
	defer tx.Rollback()
	var status string
	var expiresAt time.Time
	err = tx.QueryRowContext(ctx, "SELECT status, expires_at FROM orders WHERE id = ? FOR UPDATE", id).Scan(&status, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lock order for expiration: %w", err)
	}
	if status != "PENDING" || time.Now().UTC().Before(expiresAt) {
		return nil
	}
	if err := expireLockedOrder(ctx, tx, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit order expiration: %w", err)
	}
	return nil
}

// Both payment and expiry lock the order first, then update tiers in ID order.
func expireLockedOrder(ctx context.Context, tx *sql.Tx, id string) error {
	rows, err := tx.QueryContext(ctx, "SELECT ticket_tier_id, quantity FROM order_items WHERE order_id = ? ORDER BY ticket_tier_id", id)
	if err != nil {
		return fmt.Errorf("read expired order items: %w", err)
	}
	type item struct{ tierID, quantity uint64 }
	var items []item
	for rows.Next() {
		var value item
		if err := rows.Scan(&value.tierID, &value.quantity); err != nil {
			rows.Close()
			return fmt.Errorf("scan expired order item: %w", err)
		}
		items = append(items, value)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return fmt.Errorf("iterate expired order items: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close expired order items: %w", closeErr)
	}
	for _, value := range items {
		if _, err := tx.ExecContext(ctx, "UPDATE ticket_tiers SET available_quantity = available_quantity + ?, updated_at = UTC_TIMESTAMP(6) WHERE id = ?", value.quantity, value.tierID); err != nil {
			return fmt.Errorf("restore unpaid order stock: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE orders SET status = 'EXPIRED', updated_at = UTC_TIMESTAMP(6) WHERE id = ?", id); err != nil {
		return fmt.Errorf("mark order expired: %w", err)
	}
	return nil
}
