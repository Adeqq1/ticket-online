package operations

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
)

type Worker struct {
	Name                string     `json:"name"`
	Running             bool       `json:"running"`
	StartedAt           *time.Time `json:"startedAt"`
	LastFinishedAt      *time.Time `json:"lastFinishedAt"`
	LastSuccessAt       *time.Time `json:"lastSuccessAt"`
	LastFailureAt       *time.Time `json:"lastFailureAt"`
	ConsecutiveFailures int        `json:"consecutiveFailures"`
}

type workerState struct {
	Worker
	launchedAt time.Time
}

type Tracker struct {
	mu             sync.Mutex
	launchedAt     time.Time
	serverFailures []time.Time
	workers        map[string]workerState
}

func NewTracker() *Tracker {
	return &Tracker{launchedAt: time.Now().UTC(), workers: make(map[string]workerState)}
}

// Process is shared by the HTTP server and the three workers in the API process.
var Process = NewTracker()

func (t *Tracker) Request(status int, at time.Time) {
	if status < 500 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.serverFailures = append(t.serverFailures, at.UTC())
	cutoff := at.Add(-5 * time.Minute)
	first := 0
	for first < len(t.serverFailures) && t.serverFailures[first].Before(cutoff) {
		first++
	}
	t.serverFailures = append([]time.Time(nil), t.serverFailures[first:]...)
}

func (t *Tracker) Begin(name string, at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.workers[name]
	state.Name, state.Running = name, true
	started := at.UTC()
	state.StartedAt = &started
	if state.launchedAt.IsZero() {
		state.launchedAt = at.UTC()
	}
	t.workers[name] = state
}

func (t *Tracker) Register(name string, at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.workers[name]
	state.Name = name
	if state.launchedAt.IsZero() {
		state.launchedAt = at.UTC()
	}
	t.workers[name] = state
}

func (t *Tracker) Finish(name string, at time.Time, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.workers[name]
	state.Name, state.Running = name, false
	finished := at.UTC()
	state.LastFinishedAt, state.StartedAt = &finished, nil
	if state.launchedAt.IsZero() {
		state.launchedAt = at.UTC()
	}
	if err != nil {
		state.ConsecutiveFailures++
		state.LastFailureAt = &finished
	} else {
		state.ConsecutiveFailures = 0
		state.LastSuccessAt = &finished
	}
	t.workers[name] = state
}

type workerSnapshot struct {
	Worker
	launchedAt time.Time
}

func (t *Tracker) snapshot() (int, []workerSnapshot, time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	cutoff := now.Add(-5 * time.Minute)
	first := 0
	for first < len(t.serverFailures) && t.serverFailures[first].Before(cutoff) {
		first++
	}
	t.serverFailures = append([]time.Time(nil), t.serverFailures[first:]...)
	workers := make([]workerSnapshot, 0, len(t.workers))
	for _, state := range t.workers {
		workers = append(workers, workerSnapshot{state.Worker, state.launchedAt})
	}
	sort.Slice(workers, func(i, j int) bool { return workers[i].Name < workers[j].Name })
	return len(t.serverFailures), workers, t.launchedAt
}

type Metrics struct {
	CollectedAt               time.Time `json:"collectedAt"`
	API5xxLast5m              int       `json:"api5xxLast5m"`
	FailedEmailJobs           int       `json:"failedEmailJobs"`
	OldestPendingEmailSeconds int64     `json:"oldestPendingEmailSeconds"`
	OpenPaymentCases          int       `json:"openPaymentCases"`
	OpenRefunds               int       `json:"openRefunds"`
	Workers                   []Worker  `json:"workers"`
	Alerts                    []string  `json:"alerts"`
}

type Service struct {
	db         *sql.DB
	staff      *staffauth.Service
	tracker    *Tracker
	mu         sync.RWMutex
	metrics    Metrics
	lastAlerts string
}

func NewService(db *sql.DB, staff *staffauth.Service, tracker *Tracker) *Service {
	if tracker == nil {
		tracker = Process
	}
	return &Service{db: db, staff: staff, tracker: tracker, metrics: Metrics{Workers: []Worker{}, Alerts: []string{}}}
}

func (s *Service) Collect(ctx context.Context) (Metrics, error) {
	var next Metrics
	next.CollectedAt = time.Now().UTC()
	var failed, open, oldest int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM email_queue WHERE status = 'FAILED' AND superseded_by IS NULL
		AND (kind = 'TICKETS' OR (attempts >= 3 AND last_error NOT IN ('tidak ada pesanan yang memenuhi syarat pemulihan', 'permintaan pemulihan kedaluwarsa')))`).Scan(&failed); err != nil {
		return next, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(MAX(GREATEST(TIMESTAMPDIFF(SECOND, created_at, UTC_TIMESTAMP(6)), 0)), 0)
		FROM email_queue WHERE kind = 'TICKETS' AND status = 'PENDING'`).Scan(&open, &oldest); err != nil {
		return next, err
	}
	next.FailedEmailJobs = int(failed)
	next.OldestPendingEmailSeconds = oldest
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM payment_reconciliation_cases WHERE status = 'OPEN'").Scan(&open); err != nil {
		return next, err
	}
	next.OpenPaymentCases = int(open)
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM order_refunds WHERE status IN ('REQUESTED','PROCESSING','UNKNOWN')").Scan(&open); err != nil {
		return next, err
	}
	next.OpenRefunds = int(open)
	apiFailures, workers, _ := s.tracker.snapshot()
	next.API5xxLast5m = apiFailures
	now := next.CollectedAt
	alerts := make([]string, 0)
	if next.API5xxLast5m >= 5 {
		alerts = append(alerts, "API mencatat minimal 5 error 5xx dalam 5 menit terakhir.")
	}
	if next.FailedEmailJobs > 0 {
		alerts = append(alerts, "Ada email gagal yang memerlukan pemeriksaan admin.")
	}
	if next.OldestPendingEmailSeconds >= 300 {
		alerts = append(alerts, "Antrean email tiket tertua menunggu minimal 5 menit.")
	}
	if next.OpenPaymentCases > 0 {
		alerts = append(alerts, "Ada kasus rekonsiliasi pembayaran yang masih terbuka.")
	}
	if next.OpenRefunds > 0 {
		alerts = append(alerts, "Ada refund yang masih menunggu konfirmasi Midtrans.")
	}
	for _, item := range workers {
		next.Workers = append(next.Workers, item.Worker)
		if item.ConsecutiveFailures >= 3 {
			alerts = append(alerts, item.Name+" gagal 3 kali berturut-turut.")
		}
		if item.Running && item.StartedAt != nil && now.Sub(*item.StartedAt) > 5*time.Minute {
			alerts = append(alerts, item.Name+" belum menyelesaikan batch dalam 5 menit.")
		} else if !item.Running && now.Sub(item.launchedAt) > 5*time.Minute && (item.LastSuccessAt == nil || now.Sub(*item.LastSuccessAt) > 5*time.Minute) {
			alerts = append(alerts, item.Name+" belum berhasil dalam 5 menit terakhir.")
		}
	}
	if next.Workers == nil {
		next.Workers = []Worker{}
	}
	next.Alerts = alerts
	s.mu.Lock()
	old := s.lastAlerts
	joined := strings.Join(alerts, "\n")
	s.lastAlerts = joined
	s.metrics = next
	s.mu.Unlock()
	if old != joined {
		slog.Default().InfoContext(ctx, "operational alert state changed", "alerts", alerts)
	}
	return next, nil
}

func (s *Service) Run(ctx context.Context, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	slog.SetDefault(logger)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		if _, err := s.Collect(ctx); err != nil && ctx.Err() == nil {
			logger.ErrorContext(ctx, "collect operational metrics", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) Current() (Metrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.metrics.CollectedAt.IsZero() || time.Since(s.metrics.CollectedAt) > 2*time.Minute {
		return Metrics{}, errors.New("operational snapshot is stale")
	}
	next := s.metrics
	next.API5xxLast5m, _, _ = s.tracker.snapshot()
	return next, nil
}

func (s *Service) Register(mux *http.ServeMux) { mux.HandleFunc("GET /api/v1/admin/operations", s.get) }

func (s *Service) get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	actor, err := s.staff.Authenticate(r.Context(), token)
	if err != nil {
		if errors.Is(err, staffauth.ErrUnauthorized) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "UNAUTHORIZED", "message": "Sesi administrator tidak valid atau sudah berakhir"}})
		} else {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": map[string]string{"code": "SERVICE_UNAVAILABLE", "message": "Sesi administrator tidak dapat diverifikasi"}})
		}
		return
	}
	if actor.Role != "ADMIN" {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "FORBIDDEN", "message": "Akses operasional hanya tersedia untuk admin"}})
		return
	}
	metrics, err := s.Current()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": map[string]string{"code": "SERVICE_UNAVAILABLE", "message": "Data operasional sedang diperbarui"}})
		return
	}
	writeJSON(w, http.StatusOK, metrics)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
