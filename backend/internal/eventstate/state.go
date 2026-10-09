// Package eventstate shares authoritative event decisions across checkout, tickets and admission.
package eventstate

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrClosed = errors.New("acara berubah; transaksi atau check-in tidak tersedia")

type State struct {
	ID             string  `json:"id"`
	Status         string  `json:"status"`
	Version        uint64  `json:"version"`
	SalesPaused    bool    `json:"salesPaused"`
	StartsAt       *string `json:"startsAt"`
	Announcement   string  `json:"announcement"`
	RefundDeadline *string `json:"refundDeadline"`
}

type Query interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func Read(ctx context.Context, q Query, id string, lock bool) (State, error) {
	var s State
	var starts, deadline sql.NullTime
	suffix := ""
	if lock {
		suffix = " FOR SHARE"
	}
	err := q.QueryRowContext(ctx, `SELECT e.id,e.lifecycle_status,e.change_version,e.sales_paused,e.starts_at,
 COALESCE(c.announcement,''),c.refund_deadline FROM events e
 LEFT JOIN event_changes c ON c.event_id=e.id AND c.version=e.change_version WHERE e.id=?`+suffix, id).
		Scan(&s.ID, &s.Status, &s.Version, &s.SalesPaused, &starts, &s.Announcement, &deadline)
	if starts.Valid && s.Status != "POSTPONED" && s.Status != "CANCELLED" {
		v := starts.Time.UTC().Format(time.RFC3339Nano)
		s.StartsAt = &v
	}
	if deadline.Valid {
		v := deadline.Time.UTC().Format(time.RFC3339Nano)
		s.RefundDeadline = &v
	}
	return s, err
}

func ForOrder(ctx context.Context, q Query, id string, lock bool) (State, error) {
	var eventID string
	if err := q.QueryRowContext(ctx, `SELECT r.event_id FROM orders o JOIN reservations r ON r.id=o.reservation_id WHERE o.id=?`, id).Scan(&eventID); err != nil {
		return State{}, err
	}
	return Read(ctx, q, eventID, lock)
}

func ForReservation(ctx context.Context, q Query, id string, lock bool) (State, error) {
	var eventID string
	if err := q.QueryRowContext(ctx, "SELECT event_id FROM reservations WHERE id=?", id).Scan(&eventID); err != nil {
		return State{}, err
	}
	return Read(ctx, q, eventID, lock)
}

func (s State) CanSell() bool {
	return (s.Status == "SCHEDULED" || s.Status == "RESCHEDULED") && !s.SalesPaused
}
func (s State) CanCheckIn(now time.Time) bool {
	if s.Status != "SCHEDULED" && s.Status != "RESCHEDULED" {
		return false
	}
	// Existing events keep their admission policy; changed events use the new concert day in WIB.
	if s.Version == 0 {
		return true
	}
	if s.StartsAt == nil {
		return false
	}
	start, err := time.Parse(time.RFC3339Nano, *s.StartsAt)
	wib := time.FixedZone("WIB", 7*60*60)
	return err == nil && now.In(wib).Format("2006-01-02") == start.In(wib).Format("2006-01-02")
}

// PaymentBlocked also rejects old pending orders after sales reopen on a replacement schedule.
func PaymentBlocked(ctx context.Context, q Query, id string, s State) (bool, error) {
	if !s.CanSell() {
		return true, nil
	}
	var old bool
	err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM event_change_orders WHERE order_id=? AND stop_pending=TRUE)", id).Scan(&old)
	return old, err
}

// Deadline is zero while a changed order still requires access without a final expiry.
func Deadline(ctx context.Context, q Query, id string) (time.Time, error) {
	var deadline sql.NullTime
	var held bool
	err := q.QueryRowContext(ctx, `SELECT COALESCE(o.access_deadline,
 DATE(e.starts_at + INTERVAL 7 HOUR) + INTERVAL 1 DAY - INTERVAL 7 HOUR),
 (e.lifecycle_status='POSTPONED' AND o.status<>'REFUNDED') OR
 EXISTS(SELECT 1 FROM event_change_orders w WHERE w.order_id=o.id AND w.stop_pending=TRUE AND w.processed=FALSE) OR
 EXISTS(SELECT 1 FROM event_refund_rights rr LEFT JOIN order_refunds f ON f.order_id=rr.order_id
 WHERE rr.order_id=o.id AND rr.requested=TRUE AND (f.status IS NULL OR f.status<>'SUCCEEDED'))
 FROM orders o JOIN reservations r ON r.id=o.reservation_id JOIN events e ON e.id=r.event_id WHERE o.id=?`, id).Scan(&deadline, &held)
	if held {
		return time.Time{}, err
	}
	if err == nil && !deadline.Valid {
		return time.Time{}, ErrClosed
	}
	return deadline.Time, err
}

func TimeJSON(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	v := t.UTC().Format(time.RFC3339Nano)
	return &v
}
