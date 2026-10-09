package eventchange

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Adeqq1/ticket-online/backend/internal/eventstate"
	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/Adeqq1/ticket-online/backend/internal/payment"
	"github.com/Adeqq1/ticket-online/backend/internal/recovery"
	"github.com/Adeqq1/ticket-online/backend/internal/refund"
	"github.com/Adeqq1/ticket-online/backend/internal/reservation"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
)

var ErrInvalid = errors.New("data perubahan acara tidak valid")
var ErrConflict = errors.New("dampak acara berubah; muat pratinjau baru")

type Input struct {
	Action          string  `json:"action"`
	Reason          string  `json:"reason"`
	Announcement    string  `json:"announcement"`
	StartsAt        *string `json:"startsAt"`
	RefundDeadline  *string `json:"refundDeadline"`
	ExpectedVersion uint64  `json:"expectedVersion"`
	Snapshot        string  `json:"snapshot"`
}
type Impact struct {
	ActiveReservations uint64 `json:"activeReservations"`
	PendingOrders      uint64 `json:"pendingOrders"`
	PendingPayments    uint64 `json:"pendingPayments"`
	PaidOrders         uint64 `json:"paidOrders"`
	IssuedTickets      uint64 `json:"issuedTickets"`
	CheckIns           uint64 `json:"checkIns"`
	ExistingRefunds    uint64 `json:"existingRefunds"`
	AutomaticAmount    uint64 `json:"automaticAmount"`
	ManualAmount       uint64 `json:"manualAmount"`
}
type Preview struct {
	Event    eventstate.State `json:"event"`
	Snapshot string           `json:"snapshot"`
	Impact   Impact           `json:"impact"`
}
type Change struct {
	ID               string  `json:"id"`
	Version          uint64  `json:"version"`
	Action           string  `json:"action"`
	Reason           string  `json:"reason"`
	Announcement     string  `json:"announcement"`
	PreviousStartsAt *string `json:"previousStartsAt"`
	StartsAt         *string `json:"startsAt"`
	RefundDeadline   *string `json:"refundDeadline"`
	StaffID          string  `json:"staffId"`
	CreatedAt        string  `json:"createdAt"`
	RemainingOrders  uint64  `json:"remainingOrders"`
	Errors           uint64  `json:"errors"`
}
type Service struct {
	db           *sql.DB
	staff        *staffauth.Service
	access       *orderaccess.Access
	refunds      *refund.Service
	payments     *payment.Repository
	reservations *reservation.Repository
	key          string
	logger       *slog.Logger
}

func New(db *sql.DB, staff *staffauth.Service, access *orderaccess.Access, refunds *refund.Service, payments *payment.Repository, key string, logger *slog.Logger) *Service {
	return &Service{db: db, staff: staff, access: access, refunds: refunds, payments: payments, reservations: reservation.NewRepository(db, 15*time.Minute), key: key, logger: logger}
}

func validate(in Input, now time.Time) error {
	if strings.TrimSpace(in.Reason) != in.Reason || len(in.Reason) < 3 || len(in.Reason) > 500 || utf8.RuneCountInString(in.Announcement) < 3 || utf8.RuneCountInString(in.Announcement) > 2000 {
		return ErrInvalid
	}
	switch in.Action {
	case "CANCELLED", "POSTPONED":
		if in.StartsAt != nil || in.RefundDeadline != nil {
			return ErrInvalid
		}
	case "RESCHEDULED":
		if in.StartsAt == nil || in.RefundDeadline == nil {
			return ErrInvalid
		}
		start, e := time.Parse(time.RFC3339Nano, *in.StartsAt)
		deadline, d := time.Parse(time.RFC3339Nano, *in.RefundDeadline)
		if e != nil || d != nil || !start.After(now) || !deadline.After(now) || !deadline.Before(start) {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

func (s *Service) authorize(ctx context.Context, tx *sql.Tx, r *http.Request) (staffauth.Principal, error) {
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		return staffauth.Principal{}, staffauth.ErrUnauthorized
	}
	p, err := s.staff.AuthenticateTx(ctx, tx, token)
	if err == nil && p.Role != "ADMIN" {
		err = staffauth.ErrForbidden
	}
	return p, err
}

func (s *Service) preview(ctx context.Context, tx *sql.Tx, eventID string, in Input) (Preview, error) {
	var result Preview
	if err := validate(in, time.Now()); err != nil {
		return result, err
	}
	// Exclusive event lock serializes the snapshot with every affected buyer/staff write.
	var locked string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM events WHERE id=? FOR UPDATE", eventID).Scan(&locked); err != nil {
		return result, err
	}
	state, err := eventstate.Read(ctx, tx, eventID, false)
	if err != nil {
		return result, err
	}
	if state.Version != in.ExpectedVersion || state.Status == "CANCELLED" {
		return result, ErrConflict
	}
	if state.SalesPaused && in.Action != "CANCELLED" {
		var pending bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reservations WHERE event_id=? AND status='ACTIVE') OR
        EXISTS(SELECT 1 FROM event_change_orders w JOIN event_changes c ON c.id=w.change_id
        WHERE c.event_id=? AND w.stop_pending=TRUE AND w.processed=FALSE)`, eventID, eventID).Scan(&pending); err != nil {
			return result, err
		}
		if pending {
			return result, ErrConflict
		}
	}
	if in.Action == "RESCHEDULED" {
		var deadline sql.NullTime
		if err := tx.QueryRowContext(ctx, `SELECT MAX(refund_deadline) FROM event_changes WHERE event_id=?`, eventID).Scan(&deadline); err != nil {
			return result, err
		}
		next, _ := time.Parse(time.RFC3339Nano, *in.RefundDeadline)
		if deadline.Valid && next.Before(deadline.Time) {
			return result, ErrInvalid
		}
	}
	result.Event = state
	hasher := sha256.New()
	var updated time.Time
	if err := tx.QueryRowContext(ctx, "SELECT updated_at FROM events WHERE id=?", eventID).Scan(&updated); err != nil {
		return result, err
	}
	fmt.Fprintf(hasher, "%s:%d:%s", eventID, state.Version, updated.UTC().Format(time.RFC3339Nano))
	rows, err := tx.QueryContext(ctx, `SELECT r.id,r.status,r.updated_at,COALESCE(o.id,''),COALESCE(o.status,''),COALESCE(o.total,0),
 COALESCE(p.status,''),COALESCE(p.method,''),p.paid_at,COALESCE(f.status,''),
 (SELECT COUNT(*) FROM etickets t WHERE t.order_id=o.id),
 (SELECT COUNT(*) FROM etickets t JOIN ticket_checkins c ON c.ticket_id=t.id WHERE t.order_id=o.id),
 o.updated_at,p.updated_at,f.updated_at
 FROM reservations r LEFT JOIN orders o ON o.reservation_id=r.id LEFT JOIN payments p ON p.order_id=o.id
 LEFT JOIN order_refunds f ON f.order_id=o.id WHERE r.event_id=? ORDER BY r.id`, eventID)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var rid, rs, oid, os, ps, method, fs string
		var rt time.Time
		var amount, tickets, checkins uint64
		var paid, ot, pt, ft sql.NullTime
		if err := rows.Scan(&rid, &rs, &rt, &oid, &os, &amount, &ps, &method, &paid, &fs, &tickets, &checkins, &ot, &pt, &ft); err != nil {
			return result, err
		}
		fmt.Fprintf(hasher, "|%s:%s:%v:%s:%s:%d:%s:%s:%v:%s:%d:%d:%v:%v:%v", rid, rs, rt, oid, os, amount, ps, method, paid, fs, tickets, checkins, ot, pt, ft)
		if rs == "ACTIVE" {
			result.Impact.ActiveReservations++
		}
		if os == "PENDING" {
			result.Impact.PendingOrders++
		}
		if os == "PENDING" && ps == "PENDING" {
			result.Impact.PendingPayments++
		}
		if ps == "SUCCEEDED" || os == "PAID" {
			result.Impact.PaidOrders++
		}
		result.Impact.IssuedTickets += tickets
		result.Impact.CheckIns += checkins
		if fs != "" && fs != "FAILED" {
			result.Impact.ExistingRefunds++
		}
		if (ps == "SUCCEEDED" || os == "PAID") && (fs == "" || fs == "FAILED") {
			if paid.Valid && s.refunds.Supports(method, paid.Time, time.Now()) {
				result.Impact.AutomaticAmount += amount
			} else {
				result.Impact.ManualAmount += amount
			}
		}
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	result.Snapshot = hex.EncodeToString(hasher.Sum(nil))
	return result, nil
}

func (s *Service) commit(ctx context.Context, tx *sql.Tx, eventID, key string, in Input, p staffauth.Principal) (map[string]any, error) {
	if len(key) < 16 || len(key) > 100 {
		return nil, ErrInvalid
	}
	var lockID string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM events WHERE id=? FOR UPDATE", eventID).Scan(&lockID); err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(in)
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	var existing, stored string
	var version uint64
	err := tx.QueryRowContext(ctx, "SELECT id,request_hash,version FROM event_changes WHERE event_id=? AND idempotency_key=?", eventID, key).Scan(&existing, &stored, &version)
	if err == nil {
		if stored != hash {
			return nil, ErrConflict
		}
		return map[string]any{"id": existing, "version": version}, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	preview, err := s.preview(ctx, tx, eventID, in)
	if err != nil {
		return nil, err
	}
	// Recheck under the lock to make concurrent identical submissions safe.
	err = tx.QueryRowContext(ctx, "SELECT id,request_hash,version FROM event_changes WHERE event_id=? AND idempotency_key=?", eventID, key).Scan(&existing, &stored, &version)
	if err == nil {
		if stored != hash {
			return nil, ErrConflict
		}
		return map[string]any{"id": existing, "version": version}, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if preview.Snapshot != in.Snapshot {
		return nil, ErrConflict
	}
	id, err := recovery.NewID()
	if err != nil {
		return nil, err
	}
	version = preview.Event.Version + 1
	var previous sql.NullTime
	if err := tx.QueryRowContext(ctx, "SELECT starts_at FROM events WHERE id=?", eventID).Scan(&previous); err != nil {
		return nil, err
	}
	var start, deadline any
	if in.StartsAt != nil {
		v, _ := time.Parse(time.RFC3339Nano, *in.StartsAt)
		start = v.UTC()
	}
	if in.RefundDeadline != nil {
		v, _ := time.Parse(time.RFC3339Nano, *in.RefundDeadline)
		deadline = v.UTC()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO event_changes (id,event_id,version,action,reason,announcement,previous_starts_at,starts_at,refund_deadline,staff_id,idempotency_key,request_hash,created_at)
 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,UTC_TIMESTAMP(6))`, id, eventID, version, in.Action, in.Reason, in.Announcement, previous, start, deadline, p.ID, key, hash); err != nil {
		return nil, err
	}
	// Keep the old timestamp internally for historical joins; effective startsAt is absent while postponed/cancelled.
	if _, err := tx.ExecContext(ctx, `UPDATE orders o JOIN reservations r ON r.id=o.reservation_id JOIN events e ON e.id=r.event_id SET o.access_deadline=GREATEST(
 COALESCE(o.access_deadline,DATE(e.starts_at + INTERVAL 7 HOUR) + INTERVAL 1 DAY - INTERVAL 7 HOUR),
 COALESCE(DATE(? + INTERVAL 7 HOUR) + INTERVAL 1 DAY - INTERVAL 7 HOUR,o.access_deadline,e.starts_at),
 IF(?='CANCELLED',UTC_TIMESTAMP(6)+INTERVAL 90 DAY,COALESCE(o.access_deadline,e.starts_at))) WHERE r.event_id=?`, start, in.Action, eventID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE events SET lifecycle_status=?,change_version=?,sales_paused=TRUE,starts_at=COALESCE(?,starts_at),updated_at=UTC_TIMESTAMP(6) WHERE id=?", in.Action, version, start, eventID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO event_change_orders (change_id,order_id,stop_pending)
 SELECT ?,o.id,o.status='PENDING' FROM orders o JOIN reservations r ON r.id=o.reservation_id WHERE r.event_id=?`, id, eventID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE orders o JOIN reservations r ON r.id=o.reservation_id SET o.expires_at=LEAST(o.expires_at,UTC_TIMESTAMP(6)) WHERE r.event_id=? AND o.status='PENDING'`, eventID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO event_refund_rights (order_id,change_id,requested,deadline)
 SELECT o.id,?,?='CANCELLED',? FROM orders o JOIN reservations r ON r.id=o.reservation_id LEFT JOIN payments p ON p.order_id=o.id
 WHERE r.event_id=? AND (o.status IN ('PAID','REFUND_PENDING') OR p.status='SUCCEEDED') AND o.status<>'REFUNDED'
 ON DUPLICATE KEY UPDATE change_id=VALUES(change_id),requested=requested OR VALUES(requested),deadline=VALUES(deadline)`, id, in.Action, deadline, eventID); err != nil {
		return nil, err
	}
	before, _ := json.Marshal(preview)
	after, _ := json.Marshal(in)
	if _, err := tx.ExecContext(ctx, `INSERT INTO admin_audit_log (actor_id,actor_name,action,object_type,object_id,before_json,after_json,created_at)
 VALUES (?,?,'EVENT_CHANGE','EVENT',?,?,?,UTC_TIMESTAMP(6))`, p.ID, p.Name, eventID, before, after); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "version": version}, tx.Commit()
}

func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/admin/events/{eventID}/changes/preview", s.handle)
	mux.HandleFunc("POST /api/v1/admin/events/{eventID}/changes", s.handle)
	mux.HandleFunc("GET /api/v1/admin/events/{eventID}/changes", s.handle)
	mux.HandleFunc("POST /api/v1/orders/{orderID}/refund-request", s.requestRefund)
}
func (s *Service) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	defer tx.Rollback()
	p, err := s.authorize(r.Context(), tx, r)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	eventID := r.PathValue("eventID")
	if r.Method == "GET" {
		v, err := s.history(r.Context(), tx, eventID)
		s.respond(w, r, v, err)
		return
	}
	var in Input
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil {
		s.respond(w, r, nil, ErrInvalid)
		return
	}
	var extra any
	if !errors.Is(d.Decode(&extra), io.EOF) {
		s.respond(w, r, nil, ErrInvalid)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/preview") {
		v, err := s.preview(r.Context(), tx, eventID, in)
		s.respond(w, r, v, err)
		return
	}
	v, err := s.commit(r.Context(), tx, eventID, r.Header.Get("Idempotency-Key"), in, p)
	s.respond(w, r, v, err)
}
func (s *Service) history(ctx context.Context, tx *sql.Tx, eventID string) ([]Change, error) {
	rows, err := tx.QueryContext(ctx, `SELECT c.id,c.version,c.action,c.reason,c.announcement,c.previous_starts_at,c.starts_at,c.refund_deadline,c.staff_id,c.created_at,
 (SELECT COUNT(*) FROM event_change_orders w WHERE w.change_id=c.id AND w.processed=FALSE),
 (SELECT COUNT(*) FROM event_change_orders w WHERE w.change_id=c.id AND w.last_error<>'')
 FROM event_changes c WHERE c.event_id=? ORDER BY c.version DESC`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Change{}
	for rows.Next() {
		var c Change
		var old, start, deadline sql.NullTime
		var created time.Time
		if err := rows.Scan(&c.ID, &c.Version, &c.Action, &c.Reason, &c.Announcement, &old, &start, &deadline, &c.StaffID, &created, &c.RemainingOrders, &c.Errors); err != nil {
			return nil, err
		}
		if old.Valid {
			c.PreviousStartsAt = eventstate.TimeJSON(old.Time)
		}
		if start.Valid {
			c.StartsAt = eventstate.TimeJSON(start.Time)
		}
		if deadline.Valid {
			c.RefundDeadline = eventstate.TimeJSON(deadline.Time)
		}
		c.CreatedAt = created.UTC().Format(time.RFC3339Nano)
		result = append(result, c)
	}
	return result, rows.Err()
}
func (s *Service) requestRefund(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		s.respond(w, r, nil, orderaccess.ErrUnauthorized)
		return
	}
	id := r.PathValue("orderID")
	if _, err := s.access.AuthorizeOrder(r.Context(), id, token); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	defer tx.Rollback()
	if _, err := eventstate.ForOrder(r.Context(), tx, id, true); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	var lockedOrder string
	if err := tx.QueryRowContext(r.Context(), "SELECT id FROM orders WHERE id=? FOR UPDATE", id).Scan(&lockedOrder); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	var requested bool
	var deadline sql.NullTime
	err = tx.QueryRowContext(r.Context(), "SELECT requested,deadline FROM event_refund_rights WHERE order_id=? FOR UPDATE", id).Scan(&requested, &deadline)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	if !requested && deadline.Valid && !time.Now().Before(deadline.Time) {
		s.respond(w, r, nil, ErrConflict)
		return
	}
	if _, err := tx.ExecContext(r.Context(), "UPDATE event_refund_rights SET requested=TRUE WHERE order_id=?", id); err != nil {
		s.respond(w, r, nil, err)
		return
	}
	s.respond(w, r, map[string]string{"status": "REQUESTED"}, tx.Commit())
}
func (s *Service) respond(w http.ResponseWriter, r *http.Request, value any, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if err == nil {
		_ = json.NewEncoder(w).Encode(value)
		return
	}
	status, code, message := 500, "INTERNAL_ERROR", "Perubahan acara belum dapat diproses"
	switch {
	case errors.Is(err, ErrInvalid):
		status, code, message = 422, "INVALID_REQUEST", ErrInvalid.Error()
	case errors.Is(err, ErrConflict):
		status, code, message = 409, "EVENT_CHANGE_CONFLICT", ErrConflict.Error()
	case errors.Is(err, staffauth.ErrUnauthorized), errors.Is(err, orderaccess.ErrUnauthorized):
		status, code, message = 401, "UNAUTHORIZED", "Akses tidak valid"
	case errors.Is(err, staffauth.ErrForbidden):
		status, code, message = 403, "FORBIDDEN", "Akses ADMIN diperlukan"
	case errors.Is(err, orderaccess.ErrExpired):
		status, code, message = 410, "ACCESS_TOKEN_EXPIRED", "Akses telah berakhir"
	case errors.Is(err, sql.ErrNoRows), errors.Is(err, orderaccess.ErrNotFound):
		status, code, message = 404, "NOT_FOUND", "Data tidak ditemukan"
	default:
		s.logger.ErrorContext(r.Context(), "event change failed", "error", err)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}
