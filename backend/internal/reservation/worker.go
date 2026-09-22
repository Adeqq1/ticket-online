package reservation

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

type Worker struct {
	repository *Repository
	interval   time.Duration
	logger     *slog.Logger
}

func NewWorker(repository *Repository, interval time.Duration, logger *slog.Logger) *Worker {
	return &Worker{repository: repository, interval: interval, logger: logger}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runBatch(ctx)
		}
	}
}

func (w *Worker) runBatch(ctx context.Context) {
	rows, err := w.repository.db.QueryContext(ctx, "SELECT id FROM reservations WHERE status = 'ACTIVE' AND expires_at <= UTC_TIMESTAMP(6) ORDER BY expires_at, id LIMIT 100")
	if err != nil {
		w.logger.ErrorContext(ctx, "find expired reservations", "error", err)
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			w.logger.ErrorContext(ctx, "scan expired reservation", "error", err)
			rows.Close()
			return
		}
		ids = append(ids, id)
	}
	rows.Close()
	processed := 0
	for _, id := range ids {
		if err := w.repository.expireOne(ctx, id); err == nil || err == ErrReservationExpired {
			processed++
		} else if err != ErrReservationNotExpired && err != sql.ErrNoRows {
			w.logger.ErrorContext(ctx, "expire reservation", "reservation_id", id, "error", err)
		}
	}
	if processed > 0 {
		w.logger.InfoContext(ctx, "expired reservations processed", "count", processed)
	}
}
