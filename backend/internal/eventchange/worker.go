package eventchange

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/eventstate"
	"github.com/Adeqq1/ticket-online/backend/internal/operations"
	"github.com/Adeqq1/ticket-online/backend/internal/recovery"
	"github.com/Adeqq1/ticket-online/backend/internal/reservation"
)

func (s *Service) Run(ctx context.Context, interval time.Duration) {
	operations.Process.Register("event_changes", time.Now())
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		operations.Process.Begin("event_changes", time.Now())
		err := s.Process(ctx)
		operations.Process.Finish("event_changes", time.Now(), err)
		if err != nil && ctx.Err() == nil {
			s.logger.ErrorContext(ctx, "event change work failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) Process(ctx context.Context) error {
	var failures []error
	rows, err := s.db.QueryContext(ctx, `SELECT r.id FROM reservations r JOIN events e ON e.id=r.event_id
 WHERE r.status='ACTIVE' AND e.sales_paused=TRUE ORDER BY r.id LIMIT 100`)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := s.reservations.Cancel(ctx, id); err != nil && !errors.Is(err, reservation.ErrReservationExpired) && !errors.Is(err, reservation.ErrReservationConverted) {
			failures = append(failures, err)
		}
	}
	rows, err = s.db.QueryContext(ctx, `SELECT change_id,order_id,stop_pending FROM event_change_orders
 WHERE processed=FALSE AND next_attempt_at<=UTC_TIMESTAMP(6) ORDER BY next_attempt_at,change_id,order_id LIMIT 100`)
	if err != nil {
		return err
	}
	type work struct {
		change, id string
		stop       bool
	}
	items := []work{}
	for rows.Next() {
		var v work
		if err := rows.Scan(&v.change, &v.id, &v.stop); err != nil {
			rows.Close()
			return err
		}
		items = append(items, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, v := range items {
		if v.stop {
			err = s.payments.ReconcileClosedOrder(ctx, v.id, s.key)
		}
		if err == nil {
			err = s.finishWork(ctx, v.change, v.id)
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("order %s: %w", v.id, err))
			message := err.Error()
			if len(message) > 512 {
				message = message[:512]
			}
			if _, saveErr := s.db.ExecContext(ctx, "UPDATE event_change_orders SET last_error=?,next_attempt_at=UTC_TIMESTAMP(6)+INTERVAL 30 SECOND WHERE change_id=? AND order_id=? AND processed=FALSE", message, v.change, v.id); saveErr != nil {
				failures = append(failures, saveErr)
			}
		}
		err = nil
	}
	rows, err = s.db.QueryContext(ctx, `SELECT rr.order_id FROM event_refund_rights rr LEFT JOIN order_refunds f ON f.order_id=rr.order_id
 WHERE rr.requested=TRUE AND rr.next_attempt_at<=UTC_TIMESTAMP(6) AND (f.status IS NULL OR f.status='FAILED') ORDER BY rr.next_attempt_at,rr.order_id LIMIT 100`)
	if err != nil {
		return err
	}
	ids = nil
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		_, submitErr := s.refunds.SubmitEvent(ctx, id)
		message := ""
		if submitErr != nil {
			failures = append(failures, fmt.Errorf("refund order %s: %w", id, submitErr))
			message = submitErr.Error()
			if len(message) > 512 {
				message = message[:512]
			}
			if _, err := s.db.ExecContext(ctx, "UPDATE event_refund_rights SET next_attempt_at=UTC_TIMESTAMP(6)+INTERVAL 30 SECOND WHERE order_id=? AND requested=TRUE", id); err != nil {
				failures = append(failures, err)
			}
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE event_change_orders w JOIN event_refund_rights rr
 ON rr.change_id=w.change_id AND rr.order_id=w.order_id SET w.last_error=? WHERE w.order_id=? AND w.processed=TRUE`, message, id); err != nil {
			failures = append(failures, err)
		}
	}
	// Event lock also serializes reopening with another admin decision.
	rows, err = s.db.QueryContext(ctx, "SELECT id FROM events WHERE lifecycle_status='RESCHEDULED' AND sales_paused=TRUE")
	if err != nil {
		return err
	}
	ids = nil
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.reopen(ctx, id); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (s *Service) finishWork(ctx context.Context, changeID, id string) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := eventstate.ForOrder(ctx, tx, id, true); err != nil {
		return err
	}
	var status string
	if err := tx.QueryRowContext(ctx, "SELECT status FROM orders WHERE id=? FOR UPDATE", id).Scan(&status); err != nil {
		return err
	}
	var stop, processed bool
	if err := tx.QueryRowContext(ctx, "SELECT stop_pending,processed FROM event_change_orders WHERE change_id=? AND order_id=? FOR UPDATE", changeID, id).Scan(&stop, &processed); err != nil {
		return err
	}
	if processed {
		return nil
	}
	if stop && status == "PENDING" {
		return errors.New("pembayaran lama belum mendapat hasil pasti")
	}
	// Each notification is a durable outbox item tied to the exact decision.
	jobID, err := recovery.NewID()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO email_queue(id,kind,dedupe_key,order_id,event_change_id,recipient,status,attempts,next_attempt_at,last_error,created_at,updated_at)
 SELECT ?,'EVENT_CHANGE',?,?,?,b.email,'PENDING',0,UTC_TIMESTAMP(6),'',UTC_TIMESTAMP(6),UTC_TIMESTAMP(6) FROM order_buyers b WHERE b.order_id=?`, jobID, "change:"+changeID+":"+id, id, changeID, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE event_change_orders SET processed=TRUE,last_error='' WHERE change_id=? AND order_id=?", changeID, id); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) reopen(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status string
	if err := tx.QueryRowContext(ctx, "SELECT lifecycle_status FROM events WHERE id=? FOR UPDATE", id).Scan(&status); err != nil {
		return err
	}
	if status != "RESCHEDULED" {
		return nil
	}
	var pending bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reservations WHERE event_id=? AND status='ACTIVE') OR
 EXISTS(SELECT 1 FROM event_change_orders w JOIN event_changes c ON c.id=w.change_id WHERE c.event_id=? AND w.stop_pending=TRUE AND w.processed=FALSE)`, id, id).Scan(&pending); err != nil {
		return err
	}
	if pending {
		return nil
	}
	if _, err := tx.ExecContext(ctx, "UPDATE events SET sales_paused=FALSE,updated_at=UTC_TIMESTAMP(6) WHERE id=?", id); err != nil {
		return err
	}
	return tx.Commit()
}
