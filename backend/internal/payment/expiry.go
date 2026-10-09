package payment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/operations"
)

func (r *Repository) ExpirePendingOrders(ctx context.Context) error {
	return r.ExpirePendingOrdersWithMidtrans(ctx, "")
}

func (r *Repository) ExpirePendingOrdersWithMidtrans(ctx context.Context, serverKey string) error {
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
	var failures []error
	for _, id := range ids {
		if err := r.expireOrderWithMidtrans(ctx, id, serverKey); err != nil {
			failures = append(failures, fmt.Errorf("expire order %s: %w", id, err))
		}
	}
	return errors.Join(failures...)
}

func (r *Repository) RunExpiryWorker(ctx context.Context, interval time.Duration, serverKey string, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		operations.Process.Begin("payment_expiry", time.Now())
		err := r.ExpirePendingOrdersWithMidtrans(ctx, serverKey)
		operations.Process.Finish("payment_expiry", time.Now(), err)
		if err != nil && ctx.Err() == nil {
			logger.ErrorContext(ctx, "reconcile expired payment orders", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Repository) expireOrder(ctx context.Context, id string) error {
	return r.expireOrderWithMidtrans(ctx, id, "")
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
	if _, err := tx.ExecContext(ctx, "UPDATE orders SET status = IF(EXISTS(SELECT 1 FROM event_change_orders WHERE order_id=? AND stop_pending=TRUE),'CANCELLED','EXPIRED'), updated_at = UTC_TIMESTAMP(6) WHERE id = ?", id, id); err != nil {
		return fmt.Errorf("mark order expired: %w", err)
	}
	return nil
}

// ReconcileClosedOrder uses the same provider confirmation and stock release as expiry.
// Event decisions have already shortened pending order deadlines and blocked new attempts.
func (r *Repository) ReconcileClosedOrder(ctx context.Context, id, key string) error {
	return r.expireOrderWithMidtrans(ctx, id, key)
}
