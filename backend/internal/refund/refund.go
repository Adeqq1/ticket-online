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
	err = tx.QueryRowContext(ctx, `SELECT o.status,p.method,p.amount,p.paid_at,COALESCE(p.gateway_order_id,''),p.status,o.total
	FROM orders o JOIN payments p ON p.order_id=o.id WHERE o.id=? FOR UPDATE`, orderID).Scan(&status, &method, &amount, &paidAt, &gatewayID, &paymentStatus, &orderTotal)
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
	if method != "QRIS" && method != "GOPAY" || !s.methods[method] || paymentStatus != "SUCCEEDED" || !paidAt.Valid || !withinRefundWindow(method, paidAt.Time, time.Now()) {
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
	if _, err = tx.ExecContext(ctx, `INSERT INTO order_refunds (id,order_id,status,original_order_status,amount,reason,requested_by_staff_id,refund_key,gateway_order_id,provider_transaction_id,attempts,last_error,requested_at,updated_at,next_attempt_at,retry_deadline)
		VALUES (?,?,'REQUESTED',?,?,?,?,?,?,?,0,'',?,?,?,DATE_ADD(?, INTERVAL 7 DAY))`, key, orderID, status, amount, reason, principal.ID, key, gatewayID, providerStatus.TransactionID, now, now, now, now); err != nil {
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
	result = Result{Status: "REQUESTED", Amount: amount, Reason: reason}
	return result, nil
}

func withinRefundWindow(method string, paidAt, now time.Time) bool {
	window := refundWindow(method)
	return window > 0 && !paidAt.After(now.Add(time.Minute)) && now.Sub(paidAt) <= window
}

type providerStatus struct {
	OrderID           string `json:"order_id"`
	TransactionID     string `json:"transaction_id"`
	GrossAmount       string `json:"gross_amount"`
	TransactionStatus string `json:"transaction_status"`
	PaymentType       string `json:"payment_type"`
	SettlementTime    string `json:"settlement_time"`
	RefundAmount      string `json:"refund_amount"`
}

type refundSendState uint8

const (
	refundSendUnknown refundSendState = iota
	refundSendAccepted
	refundSendRejected
)

type refundSendResult struct {
	state   refundSendState
	message string
}

type providerRefundResponse struct {
	StatusCode        string `json:"status_code"`
	StatusMessage     string `json:"status_message"`
	OrderID           string `json:"order_id"`
	TransactionID     string `json:"transaction_id"`
	RefundKey         string `json:"refund_key"`
	RefundAmount      string `json:"refund_amount"`
	TransactionStatus string `json:"transaction_status"`
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
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	err = decoder.Decode(&value)
	if err == nil {
		var trailing any
		if decodeErr := decoder.Decode(&trailing); !errors.Is(decodeErr, io.EOF) {
			if decodeErr == nil {
				err = errors.New("Midtrans status response contains trailing data")
			} else {
				err = decodeErr
			}
		}
	}
	return value, err
}

func (s *Service) send(ctx context.Context, gatewayID, transactionID, key string, amount uint64) (refundSendResult, error) {
	var result refundSendResult
	body, _ := json.Marshal(map[string]string{"refund_key": key})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.base+"/v2/"+url.PathEscape(gatewayID)+"/refund", strings.NewReader(string(body)))
	if err != nil {
		return result, err
	}
	req.SetBasicAuth(s.key, "")
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	var bodyResult providerRefundResponse
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(&bodyResult); err != nil {
		return result, fmt.Errorf("decode Midtrans refund response (HTTP %d): %w", resp.StatusCode, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return result, errors.New("Midtrans refund response contains trailing data")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || bodyResult.StatusCode != "200" {
		message := bodyResult.StatusMessage
		if message == "" {
			message = fmt.Sprintf("Midtrans refund returned HTTP %d", resp.StatusCode)
		}
		if definiteRefundRejection(bodyResult.StatusCode, message) {
			return refundSendResult{state: refundSendRejected, message: clip(message)}, nil
		}
		return result, fmt.Errorf("Midtrans refund result is uncertain: %s", clip(message))
	}
	if bodyResult.OrderID != gatewayID || bodyResult.TransactionID != transactionID || bodyResult.RefundKey != key || bodyResult.RefundAmount != fmt.Sprintf("%d.00", amount) {
		return result, fmt.Errorf("Midtrans refund response identity or amount did not match the request")
	}
	return refundSendResult{state: refundSendAccepted, message: clip(bodyResult.StatusMessage)}, nil
}

func definiteRefundRejection(statusCode, message string) bool {
	if statusCode != "406" && statusCode != "412" && statusCode != "414" {
		return false
	}
	message = strings.ToLower(message)
	return !strings.Contains(message, "duplicate") && !strings.Contains(message, "already used") && !strings.Contains(message, "already exists")
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
			err := errors.Join(s.deliverRequested(ctx), s.reconcile(ctx))
			operations.Process.Finish("refund", time.Now(), err)
			if err != nil && s.logger != nil {
				s.logger.ErrorContext(ctx, "refund reconciliation failed", "error", err)
			}
		}
	}
}

type queuedRefund struct {
	orderID, gatewayID, transactionID, refundKey, status, claimToken string
	amount                                                           uint64
	retryDeadline                                                    sql.NullTime
}

func (s *Service) claimRefunds(ctx context.Context, status string) ([]queuedRefund, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var query string
	if status == "UNKNOWN" {
		query = `SELECT order_id,gateway_order_id,provider_transaction_id,refund_key,amount,status,retry_deadline
			FROM order_refunds WHERE status='UNKNOWN' AND next_attempt_at<=UTC_TIMESTAMP(6)
			AND (lease_until IS NULL OR lease_until<=UTC_TIMESTAMP(6)) ORDER BY next_attempt_at,id LIMIT 50 FOR UPDATE SKIP LOCKED`
	} else {
		query = `SELECT order_id,gateway_order_id,provider_transaction_id,refund_key,amount,status,retry_deadline
			FROM order_refunds WHERE ((status='REQUESTED' AND next_attempt_at<=UTC_TIMESTAMP(6)) OR
			(status='PROCESSING' AND (lease_until IS NULL OR lease_until<=UTC_TIMESTAMP(6))))
			ORDER BY next_attempt_at,id LIMIT 10 FOR UPDATE SKIP LOCKED`
	}
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	var items []queuedRefund
	for rows.Next() {
		var item queuedRefund
		if err := rows.Scan(&item.orderID, &item.gatewayID, &item.transactionID, &item.refundKey, &item.amount, &item.status, &item.retryDeadline); err != nil {
			rows.Close()
			return nil, err
		}
		token, err := newID()
		if err != nil {
			rows.Close()
			return nil, err
		}
		item.claimToken = token
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range items {
		item := &items[i]
		if item.status != "UNKNOWN" {
			if _, err := tx.ExecContext(ctx, `UPDATE order_refunds SET status='PROCESSING',claim_token=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 3 MINUTE),
				next_attempt_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 3 MINUTE),updated_at=UTC_TIMESTAMP(6) WHERE order_id=?`, item.claimToken, item.orderID); err != nil {
				return nil, err
			}
		} else if _, err := tx.ExecContext(ctx, `UPDATE order_refunds SET claim_token=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 3 MINUTE),
			next_attempt_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 MINUTE),last_checked_at=UTC_TIMESTAMP(6),updated_at=UTC_TIMESTAMP(6) WHERE order_id=?`, item.claimToken, item.orderID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) deliverRequested(ctx context.Context) error {
	items, err := s.claimRefunds(ctx, "REQUESTED")
	if err != nil {
		return err
	}
	var failures []error
	for _, item := range items {
		if item.status == "PROCESSING" {
			provider, checkErr := s.readStatus(ctx, item.gatewayID)
			if checkErr == nil {
				checkErr = validateProviderStatus(provider, item)
			}
			if checkErr != nil {
				failures = append(failures, s.releaseClaim(ctx, item, checkErr.Error(), false))
				continue
			}
			finished, e := s.finishFromProvider(ctx, item, provider)
			if e != nil {
				failures = append(failures, e)
				continue
			}
			if finished {
				continue
			}
			if provider.TransactionStatus != "settlement" || provider.RefundAmount != "" && provider.RefundAmount != "0.00" {
				failures = append(failures, s.releaseClaim(ctx, item, "Refund provider masih diproses; perlu diperiksa ulang.", false))
				continue
			}
		}
		if !item.retryDeadline.Valid || !time.Now().Before(item.retryDeadline.Time) {
			failures = append(failures, s.releaseClaim(ctx, item, "Batas retry refund tujuh hari terlewati; perlu pemeriksaan admin.", false))
			continue
		}
		result, sendErr := s.send(ctx, item.gatewayID, item.transactionID, item.refundKey, item.amount)
		if sendErr != nil {
			if err := s.releaseClaim(ctx, item, sendErr.Error(), true); err != nil {
				failures = append(failures, err)
			}
			continue
		}
		if result.state == refundSendRejected {
			provider, checkErr := s.readStatus(ctx, item.gatewayID)
			if checkErr == nil {
				checkErr = validateProviderStatus(provider, item)
			}
			if checkErr == nil && provider.TransactionStatus == "settlement" && (provider.RefundAmount == "" || provider.RefundAmount == "0.00") {
				if err := s.finish(ctx, item.orderID, false, result.message); err != nil {
					failures = append(failures, err)
				}
				continue
			}
			if checkErr != nil {
				result.message += "; status check: " + checkErr.Error()
			}
		}
		if err := s.releaseClaim(ctx, item, result.message, true); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (s *Service) releaseClaim(ctx context.Context, item queuedRefund, message string, attempted bool) error {
	result, err := s.db.ExecContext(ctx, `UPDATE order_refunds SET status='UNKNOWN',claim_token=NULL,lease_until=NULL,
		next_attempt_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 SECOND),attempts=LEAST(255,attempts+?),last_error=?,updated_at=UTC_TIMESTAMP(6)
		WHERE order_id=? AND claim_token=? AND status IN ('PROCESSING','UNKNOWN')`, boolToInt(attempted), clip(message), item.orderID, item.claimToken)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil {
		return err
	} else if count != 1 {
		return fmt.Errorf("refund claim for order %s was not saved", item.orderID)
	}
	return nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func validateProviderStatus(value providerStatus, item queuedRefund) error {
	if value.OrderID != item.gatewayID || value.TransactionID != item.transactionID || value.GrossAmount != fmt.Sprintf("%d.00", item.amount) {
		return fmt.Errorf("Midtrans refund status did not match order %s", item.orderID)
	}
	return nil
}

func (s *Service) finishFromProvider(ctx context.Context, item queuedRefund, provider providerStatus) (bool, error) {
	if provider.TransactionStatus == "refund" && provider.RefundAmount == fmt.Sprintf("%d.00", item.amount) {
		return true, s.finish(ctx, item.orderID, true, "")
	}
	if provider.TransactionStatus == "refund_failed" && (provider.RefundAmount == "" || provider.RefundAmount == "0.00") {
		return true, s.finish(ctx, item.orderID, false, "Midtrans mengonfirmasi refund gagal")
	}
	return false, nil
}

func (s *Service) reconcile(ctx context.Context) error {
	items, err := s.claimRefunds(ctx, "UNKNOWN")
	if err != nil {
		return err
	}
	var failures []error
	for _, v := range items {
		provider, e := s.readStatus(ctx, v.gatewayID)
		if e == nil {
			e = validateProviderStatus(provider, v)
		}
		finished := false
		if e == nil {
			finished, e = s.finishFromProvider(ctx, v, provider)
		}
		if !finished {
			if releaseErr := s.releaseClaim(ctx, v, errorString(e), false); releaseErr != nil {
				failures = append(failures, releaseErr)
			}
		}
		if e != nil {
			failures = append(failures, e)
		}
	}
	return errors.Join(failures...)
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
