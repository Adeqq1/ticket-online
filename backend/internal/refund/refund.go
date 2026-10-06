package refund

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/operations"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
)

var ErrInvalid = errors.New("refund request is invalid")
var ErrUnavailable = errors.New("refund is not eligible or provider support is not enabled")

type Service struct {
	db        *sql.DB
	staff     *staffauth.Service
	key, base string
	methods   map[string]bool
	client    *http.Client
	logger    *slog.Logger
}
type Result struct {
	Status string `json:"status"`
	Amount uint64 `json:"amount"`
	Reason string `json:"reason"`
}
type Handler struct {
	service *Service
	logger  *slog.Logger
}

func New(db *sql.DB, staff *staffauth.Service, key, environment string, methods []string, logger *slog.Logger) *Service {
	base := "https://api.sandbox.midtrans.com"
	if environment == "production" {
		base = "https://api.midtrans.com"
	}
	allow := make(map[string]bool, len(methods))
	for _, m := range methods {
		allow[m] = true
	}
	return &Service{db: db, staff: staff, key: key, base: base, methods: allow, client: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, logger: logger}
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/admin/orders/{orderID}/refund", h.submit)
}
func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var body struct {
		Reason string `json:"reason"`
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&body) != nil {
		http.Error(w, `{"error":{"code":"INVALID_REQUEST","message":"Permintaan tidak valid"}}`, http.StatusBadRequest)
		return
	}
	var trailing any
	if err := d.Decode(&trailing); !errors.Is(err, io.EOF) {
		http.Error(w, `{"error":{"code":"INVALID_REQUEST","message":"Permintaan tidak valid"}}`, http.StatusBadRequest)
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	result, err := h.service.Submit(r.Context(), token, strings.ToLower(r.PathValue("orderID")), body.Reason)
	switch {
	case err == nil:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(result)
	case errors.Is(err, ErrInvalid):
		http.Error(w, `{"error":{"code":"INVALID_REQUEST","message":"Permintaan refund tidak valid"}}`, 400)
	case errors.Is(err, ErrUnavailable):
		http.Error(w, `{"error":{"code":"REFUND_UNAVAILABLE","message":"Refund belum memenuhi syarat atau belum diaktifkan"}}`, 409)
	case errors.Is(err, staffauth.ErrForbidden):
		http.Error(w, `{"error":{"code":"FORBIDDEN","message":"Akses ditolak"}}`, 403)
	case errors.Is(err, staffauth.ErrUnauthorized):
		http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"Sesi tidak valid"}}`, 401)
	default:
		h.logger.ErrorContext(r.Context(), "refund request failed", "error", err)
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Terjadi kesalahan"}}`, 500)
	}
}

func (s *Service) Submit(ctx context.Context, token, orderID, reason string) (Result, error) {
	var result Result
	reason = strings.TrimSpace(reason)
	if _, decodeErr := hex.DecodeString(orderID); len(orderID) != 32 || decodeErr != nil || len(reason) < 3 || len(reason) > 500 {
		return result, ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	principal, err := s.staff.AuthenticateTx(ctx, tx, token)
	if err != nil {
		return result, err
	}
	if principal.Role != "ADMIN" {
		return result, staffauth.ErrForbidden
	}
	var status, method, gatewayID, paymentStatus string
	var amount, orderTotal uint64
	var paidAt sql.NullTime
	var starts time.Time
	err = tx.QueryRowContext(ctx, `SELECT o.status,p.method,p.amount,p.paid_at,COALESCE(p.gateway_order_id,''),e.starts_at,p.status,o.total
	FROM orders o JOIN payments p ON p.order_id=o.id JOIN reservations r ON r.id=o.reservation_id JOIN events e ON e.id=r.event_id WHERE o.id=? FOR UPDATE`, orderID).Scan(&status, &method, &amount, &paidAt, &gatewayID, &starts, &paymentStatus, &orderTotal)
	if errors.Is(err, sql.ErrNoRows) {
		return result, ErrUnavailable
	}
	if err != nil {
		return result, err
	}
	var existing Result
	err = tx.QueryRowContext(ctx, "SELECT status,amount,reason FROM order_refunds WHERE order_id=?", orderID).Scan(&existing.Status, &existing.Amount, &existing.Reason)
	if err == nil {
		if e := tx.Commit(); e != nil {
			return result, e
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return result, err
	}
	if status != "PAID" && status != "CANCELLED" && status != "EXPIRED" {
		return result, ErrUnavailable
	}
	late := status != "PAID"
	if late {
		var n int
		if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM payment_reconciliation_cases WHERE order_id=? AND status='OPEN'", orderID).Scan(&n); err != nil {
			return result, err
		}
		if n == 0 {
			return result, ErrUnavailable
		}
	}
	if method != "QRIS" && method != "GOPAY" || !s.methods[method] || paymentStatus != "SUCCEEDED" || !paidAt.Valid || time.Since(paidAt.Time) > refundWindow(method) || paidAt.Time.After(time.Now().Add(time.Minute)) || time.Now().Before(starts) && !late {
		return result, ErrUnavailable
	}
	var checkins int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM etickets t JOIN ticket_checkins c ON c.ticket_id=t.id WHERE t.order_id=?", orderID).Scan(&checkins); err != nil {
		return result, err
	}
	if checkins > 0 {
		return result, ErrUnavailable
	}
	if gatewayID == "" || amount == 0 || amount != orderTotal {
		return result, ErrUnavailable
	}
	providerStatus, err := s.readStatus(ctx, gatewayID)
	if err != nil || providerStatus.OrderID != gatewayID || providerStatus.TransactionID == "" || providerStatus.GrossAmount != fmt.Sprintf("%d.00", amount) || providerStatus.TransactionStatus != "settlement" || providerStatus.PaymentType != strings.ToLower(method) {
		return result, ErrUnavailable
	}
	settledAt, err := time.ParseInLocation("2006-01-02 15:04:05", providerStatus.SettlementTime, time.FixedZone("WIB", 7*60*60))
	if err != nil || time.Since(settledAt) > refundWindow(method) || settledAt.After(time.Now().Add(time.Minute)) {
		return result, ErrUnavailable
	}
	key, err := newID()
	if err != nil {
		return result, err
	}
	now := time.Now().UTC()
	if _, err = tx.ExecContext(ctx, `INSERT INTO order_refunds (id,order_id,status,original_order_status,amount,reason,requested_by_staff_id,refund_key,gateway_order_id,provider_transaction_id,requested_at,updated_at)
		VALUES (?,?,'PROCESSING',?,?,?,?,?,?,?,?,?)`, key, orderID, status, amount, reason, principal.ID, key, gatewayID, providerStatus.TransactionID, now, now); err != nil {
		return result, err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO order_refund_audit (refund_id,staff_id,action,reason,created_at) VALUES (?,?,'REQUESTED',?,?)", key, principal.ID, reason, now); err != nil {
		return result, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE orders SET status='REFUND_PENDING',updated_at=? WHERE id=?", now, orderID); err != nil {
		return result, err
	}
	if err = tx.Commit(); err != nil {
		return result, err
	}
	result = Result{Status: "UNKNOWN", Amount: amount, Reason: reason}
	if err = s.send(ctx, gatewayID, key); err != nil {
		_, _ = s.db.ExecContext(ctx, "UPDATE order_refunds SET status='UNKNOWN',attempts=attempts+1,last_error=?,updated_at=? WHERE order_id=? AND status='PROCESSING'", clip(err.Error()), time.Now().UTC(), orderID)
		return result, nil
	}
	_, _ = s.db.ExecContext(ctx, "UPDATE order_refunds SET attempts=attempts+1,updated_at=? WHERE order_id=? AND status='PROCESSING'", time.Now().UTC(), orderID)
	return result, nil
}

type providerStatus struct {
	OrderID           string `json:"order_id"`
	TransactionID     string `json:"transaction_id"`
	GrossAmount       string `json:"gross_amount"`
	TransactionStatus string `json:"transaction_status"`
	PaymentType       string `json:"payment_type"`
	SettlementTime    string `json:"settlement_time"`
}

func (s *Service) readStatus(ctx context.Context, gatewayID string) (providerStatus, error) {
	var value providerStatus
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/v2/"+url.PathEscape(gatewayID)+"/status", nil)
	if err != nil {
		return value, err
	}
	req.SetBasicAuth(s.key, "")
	resp, err := s.client.Do(req)
	if err != nil {
		return value, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return value, fmt.Errorf("Midtrans status returned %d", resp.StatusCode)
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&value)
	return value, err
}

func (s *Service) send(ctx context.Context, gatewayID, key string) error {
	body, _ := json.Marshal(map[string]string{"refund_key": key})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.base+"/v2/"+url.PathEscape(gatewayID)+"/refund", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.SetBasicAuth(s.key, "")
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Midtrans refund returned %d: %s", resp.StatusCode, clip(string(raw)))
	}
	return nil
}

func (s *Service) finish(ctx context.Context, orderID string, success bool, message string) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var original, status, refundReason, staffID string
	var amount uint64
	err = tx.QueryRowContext(ctx, "SELECT original_order_status,status,amount,reason,requested_by_staff_id FROM order_refunds WHERE order_id=? FOR UPDATE", orderID).Scan(&original, &status, &amount, &refundReason, &staffID)
	if err != nil {
		return err
	}
	if status == "SUCCEEDED" || status == "FAILED" {
		return tx.Commit()
	}
	if success {
		if original == "PAID" {
			rows, e := tx.QueryContext(ctx, "SELECT ticket_tier_id,quantity FROM order_items WHERE order_id=? ORDER BY ticket_tier_id", orderID)
			if e != nil {
				return e
			}
			type stock struct{ id, q uint64 }
			var items []stock
			for rows.Next() {
				var v stock
				if e = rows.Scan(&v.id, &v.q); e != nil {
					rows.Close()
					return e
				}
				items = append(items, v)
			}
			if e = rows.Err(); e != nil {
				rows.Close()
				return e
			}
			rows.Close()
			for _, v := range items {
				if _, e = tx.ExecContext(ctx, "UPDATE ticket_tiers SET available_quantity=LEAST(capacity,available_quantity+?),updated_at=UTC_TIMESTAMP(6) WHERE id=?", v.q, v.id); e != nil {
					return e
				}
			}
		}
		_, err = tx.ExecContext(ctx, "UPDATE order_refunds SET status='SUCCEEDED',completed_at=UTC_TIMESTAMP(6),updated_at=UTC_TIMESTAMP(6),last_error='' WHERE order_id=?", orderID)
		if err == nil {
			_, err = tx.ExecContext(ctx, "UPDATE orders SET status='REFUNDED',updated_at=UTC_TIMESTAMP(6) WHERE id=?", orderID)
		}
		if err == nil {
			_, err = tx.ExecContext(ctx, "UPDATE payment_reconciliation_cases SET status='RESOLVED',updated_at=UTC_TIMESTAMP(6) WHERE order_id=? AND status='OPEN'", orderID)
		}
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE order_refunds SET status='FAILED',last_error=?,completed_at=UTC_TIMESTAMP(6),updated_at=UTC_TIMESTAMP(6) WHERE order_id=?", clip(message), orderID)
		if err == nil {
			_, err = tx.ExecContext(ctx, "UPDATE orders SET status=?,updated_at=UTC_TIMESTAMP(6) WHERE id=?", original, orderID)
		}
	}
	if err != nil {
		return err
	}
	var reference, recipient string
	if err = tx.QueryRowContext(ctx, `SELECT o.reference,b.email FROM orders o JOIN order_buyers b ON b.order_id=o.id WHERE o.id=?`, orderID).Scan(&reference, &recipient); err != nil {
		return err
	}
	var finalStatus string
	if success {
		finalStatus = "SUCCEEDED"
	} else {
		finalStatus = "FAILED"
	}
	snapshot, _ := json.Marshal(map[string]any{"status": finalStatus, "amount": amount, "reason": refundReason, "reference": reference})
	jobID, e := newID()
	if e != nil {
		return e
	}
	if _, err = tx.ExecContext(ctx, `INSERT IGNORE INTO email_queue (id,kind,dedupe_key,order_id,recipient,status,attempts,next_attempt_at,refund_snapshot,last_error,created_at,updated_at)
	VALUES (?,'REFUND',?,?,?,'PENDING',0,UTC_TIMESTAMP(6),?,'',UTC_TIMESTAMP(6),UTC_TIMESTAMP(6))`, jobID, "refund:"+orderID, orderID, recipient, snapshot); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO order_refund_audit (refund_id,staff_id,action,reason,created_at) SELECT id,?,?,?,UTC_TIMESTAMP(6) FROM order_refunds WHERE order_id=?", staffID, finalStatus, refundReason, orderID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) Run(ctx context.Context, interval time.Duration) {
	operations.Process.Register("refund", time.Now())
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			operations.Process.Begin("refund", time.Now())
			err := s.reconcile(ctx)
			operations.Process.Finish("refund", time.Now(), err)
			if err != nil && s.logger != nil {
				s.logger.ErrorContext(ctx, "refund reconciliation failed", "error", err)
			}
		}
	}
}
func (s *Service) reconcile(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, "SELECT order_id,gateway_order_id,provider_transaction_id,amount FROM order_refunds WHERE status IN ('UNKNOWN','PROCESSING') ORDER BY updated_at LIMIT 50")
	if err != nil {
		return err
	}
	type item struct {
		id, gateway, transaction string
		amount                   uint64
	}
	var items []item
	for rows.Next() {
		var v item
		if rows.Scan(&v.id, &v.gateway, &v.transaction, &v.amount) != nil {
			continue
		}
		items = append(items, v)
	}
	rows.Close()
	var failures []error
	for _, v := range items {
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/v2/"+url.PathEscape(v.gateway)+"/status", nil)
		if e != nil {
			failures = append(failures, e)
			continue
		}
		req.SetBasicAuth(s.key, "")
		resp, e := s.client.Do(req)
		if e != nil {
			failures = append(failures, e)
			continue
		}
		var data struct {
			OrderID           string `json:"order_id"`
			TransactionID     string `json:"transaction_id"`
			TransactionStatus string `json:"transaction_status"`
			RefundAmount      string `json:"refund_amount"`
			GrossAmount       string `json:"gross_amount"`
		}
		e = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&data)
		resp.Body.Close()
		if e != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 || data.OrderID != v.gateway || data.TransactionID != v.transaction || data.GrossAmount != fmt.Sprintf("%d.00", v.amount) {
			if e == nil {
				e = fmt.Errorf("Midtrans refund status did not match order %s", v.id)
			}
			failures = append(failures, e)
			continue
		}
		if data.TransactionStatus == "refund" && data.RefundAmount == fmt.Sprintf("%d.00", v.amount) {
			if e = s.finish(ctx, v.id, true, ""); e != nil {
				failures = append(failures, e)
			}
		} else if data.TransactionStatus == "refund_failed" && (data.RefundAmount == "" || data.RefundAmount == "0.00") {
			if e = s.finish(ctx, v.id, false, "Midtrans mengonfirmasi refund gagal"); e != nil {
				failures = append(failures, e)
			}
		}
	}
	return errors.Join(failures...)
}

func refundWindow(method string) time.Duration {
	if method == "QRIS" {
		return 7 * 24 * time.Hour
	}
	if method == "GOPAY" {
		return 45 * 24 * time.Hour
	}
	return 0
}
func clip(v string) string {
	if len(v) > 512 {
		return v[:512]
	}
	return v
}
func newID() (string, error) {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	return hex.EncodeToString(b[:]), nil
}
