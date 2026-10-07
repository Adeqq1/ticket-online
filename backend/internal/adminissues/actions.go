package adminissues

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/Adeqq1/ticket-online/backend/internal/payment"
	"github.com/Adeqq1/ticket-online/backend/internal/recovery"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
)

type rateLimitError struct{ wait int }

func (e rateLimitError) Error() string { return ErrRateLimited.Error() }
func (e rateLimitError) Unwrap() error { return ErrRateLimited }

func (s *Service) Recheck(ctx context.Context, token, id string) (map[string]any, error) {
	if !caseIDPattern.MatchString(id) {
		return nil, ErrInvalidRequest
	}
	if _, err := s.authenticate(ctx, token); err != nil {
		return nil, err
	}
	if s.serverKey == "" {
		return nil, ErrProviderUnavailable
	}
	var caseID uint64
	if _, err := fmt.Sscan(id, &caseID); err != nil {
		return nil, ErrInvalidRequest
	}
	start, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer start.Rollback()
	actor, err := s.staff.AuthenticateTx(ctx, start, token)
	if err != nil {
		return nil, err
	}
	if actor.Role != "ADMIN" {
		return nil, staffauth.ErrForbidden
	}
	var status, gatewayOrderID string
	var leaseUntil sql.NullTime
	err = start.QueryRowContext(ctx, `SELECT status, gateway_order_id, check_lease_until
		FROM payment_reconciliation_cases WHERE id = ? FOR UPDATE`, caseID).
		Scan(&status, &gatewayOrderID, &leaseUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if status != "OPEN" || (leaseUntil.Valid && leaseUntil.Time.After(time.Now().UTC())) {
		return nil, ErrConflict
	}
	blocked, wait, err := recovery.CheckRateLimitsTx(ctx, start,
		recovery.RateLimit{Bucket: s.access.Digest("admin/payment-case-recheck", id), Max: 1, Period: time.Minute})
	if err != nil {
		return nil, err
	}
	if blocked >= 0 {
		if err := start.Commit(); err != nil {
			return nil, err
		}
		return nil, rateLimitError{wait: wait}
	}
	claim, err := newID()
	if err != nil {
		return nil, err
	}
	if _, err := start.ExecContext(ctx, `UPDATE payment_reconciliation_cases SET check_token = ?,
		check_lease_until = DATE_ADD(UTC_TIMESTAMP(6), INTERVAL 30 SECOND), updated_at = UTC_TIMESTAMP(6) WHERE id = ?`, claim, caseID); err != nil {
		return nil, err
	}
	if err := appendAudit(ctx, start, actor, "RECHECK", "PAYMENT_CASE", id, nil, map[string]string{"result": "STARTED"}); err != nil {
		return nil, err
	}
	if err := start.Commit(); err != nil {
		return nil, err
	}
	statusResult, providerErr := s.payments.ReadGatewayStatus(ctx, s.serverKey, gatewayOrderID)
	if providerErr != nil {
		if finishErr := s.finishProviderFailure(ctx, token, actor, caseID, id, claim, providerErr.Error()); finishErr != nil {
			return nil, finishErr
		}
		return nil, ErrProviderFailure
	}
	result, err := s.applyProviderResult(ctx, token, actor, caseID, id, claim, gatewayOrderID, statusResult)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) finishProviderFailure(ctx context.Context, token string, actor staffauth.Principal, caseID uint64, id, claim, reason string) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := s.staff.AuthenticateTx(ctx, tx, token); err != nil {
		return err
	}
	var status string
	var currentClaim sql.NullString
	err = tx.QueryRowContext(ctx, "SELECT status, check_token FROM payment_reconciliation_cases WHERE id = ? FOR UPDATE", caseID).Scan(&status, &currentClaim)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "OPEN" || !currentClaim.Valid || currentClaim.String != claim {
		return ErrConflict
	}
	message := truncate(reason, 512)
	if _, err := tx.ExecContext(ctx, `UPDATE payment_reconciliation_cases SET last_checked_at = UTC_TIMESTAMP(6),
		last_check_error = ?, check_token = NULL, check_lease_until = NULL, updated_at = UTC_TIMESTAMP(6) WHERE id = ?`, message, caseID); err != nil {
		return err
	}
	if err := appendAudit(ctx, tx, actor, "RECHECK", "PAYMENT_CASE", id, nil, map[string]string{"result": "FAILED", "reason": message}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) applyProviderResult(ctx context.Context, token string, actor staffauth.Principal, caseID uint64, id, claim, gatewayOrderID string, provider payment.GatewayStatus) (map[string]any, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := s.staff.AuthenticateTx(ctx, tx, token); err != nil {
		return nil, err
	}
	var orderID, orderStatus, paymentStatus string
	var paymentAmount, caseAmount uint64
	err = tx.QueryRowContext(ctx, `SELECT o.id, o.status, p.status, p.amount, c.amount FROM orders o
		JOIN payments p ON p.order_id = o.id JOIN payment_reconciliation_cases c ON c.order_id = o.id
		WHERE p.gateway_order_id = ? AND c.id = ? FOR UPDATE`, gatewayOrderID, caseID).
		Scan(&orderID, &orderStatus, &paymentStatus, &paymentAmount, &caseAmount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var status, currentGatewayID string
	var currentClaim sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT status, gateway_order_id, check_token FROM payment_reconciliation_cases
		WHERE id = ? FOR UPDATE`, caseID).Scan(&status, &currentGatewayID, &currentClaim)
	if err != nil {
		return nil, err
	}
	if status != "OPEN" || currentGatewayID != gatewayOrderID || !currentClaim.Valid || currentClaim.String != claim {
		return nil, ErrConflict
	}
	if paymentAmount != caseAmount || provider.GrossAmount != fmt.Sprintf("%d.00", paymentAmount) {
		return nil, s.recordProviderMismatch(ctx, tx, actor, caseID, id, "Nominal provider berbeda dari nominal pesanan.")
	}
	if err := s.payments.ApplyGatewayStatusTx(ctx, tx, gatewayOrderID, provider.GrossAmount, provider.TransactionStatus, provider.FraudStatus); err != nil {
		if errors.Is(err, payment.ErrInvalidRequest) {
			return nil, s.recordProviderMismatch(ctx, tx, actor, caseID, id, "Status provider tidak lolos validasi pembayaran.")
		}
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE payment_reconciliation_cases SET provider_status = ?, last_checked_at = UTC_TIMESTAMP(6),
		last_check_error = '', check_token = NULL, check_lease_until = NULL, updated_at = UTC_TIMESTAMP(6) WHERE id = ?`, provider.TransactionStatus, caseID); err != nil {
		return nil, err
	}
	data := map[string]string{"providerStatus": provider.TransactionStatus, "orderStatusBefore": orderStatus, "paymentStatusBefore": paymentStatus}
	if err := appendAudit(ctx, tx, actor, "RECHECK", "PAYMENT_CASE", id, nil, data); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"caseId": id, "providerStatus": provider.TransactionStatus}, nil
}

func (s *Service) recordProviderMismatch(ctx context.Context, tx *sql.Tx, actor staffauth.Principal, caseID uint64, id, reason string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE payment_reconciliation_cases SET last_checked_at = UTC_TIMESTAMP(6),
		last_check_error = ?, check_token = NULL, check_lease_until = NULL, updated_at = UTC_TIMESTAMP(6) WHERE id = ?`, reason, caseID); err != nil {
		return err
	}
	if err := appendAudit(ctx, tx, actor, "RECHECK", "PAYMENT_CASE", id, nil, map[string]string{"result": "FAILED", "reason": reason}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return ErrProviderFailure
}

func (s *Service) AddCaseNote(ctx context.Context, token, id, note string) error {
	note, err := normalizeNote(note)
	if err != nil || !caseIDPattern.MatchString(id) {
		return ErrInvalidRequest
	}
	var numericID uint64
	if _, err := fmt.Sscan(id, &numericID); err != nil {
		return ErrInvalidRequest
	}
	tx, actor, err := s.authorizedTx(ctx, token)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockCase(ctx, tx, numericID, false); err != nil {
		return err
	}
	if err := appendAudit(ctx, tx, actor, "NOTE", "PAYMENT_CASE", id, nil, map[string]string{"note": note}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) ResolveCase(ctx context.Context, token, id, note string) error {
	note, err := normalizeNote(note)
	if err != nil || !caseIDPattern.MatchString(id) {
		return ErrInvalidRequest
	}
	var numericID uint64
	if _, err := fmt.Sscan(id, &numericID); err != nil {
		return ErrInvalidRequest
	}
	tx, actor, err := s.authorizedTx(ctx, token)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockCase(ctx, tx, numericID, true); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE payment_reconciliation_cases SET status = 'RESOLVED', check_token = NULL, check_lease_until = NULL, updated_at = UTC_TIMESTAMP(6) WHERE id = ? AND status = 'OPEN'", numericID); err != nil {
		return err
	}
	if err := appendAudit(ctx, tx, actor, "RESOLVE", "PAYMENT_CASE", id, map[string]string{"status": "OPEN"}, map[string]string{"status": "RESOLVED", "note": note}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) RetryEmailJob(ctx context.Context, token, id string) (map[string]any, error) {
	if !jobIDPattern.MatchString(id) {
		return nil, ErrInvalidRequest
	}
	tx, actor, err := s.authorizedTx(ctx, token)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var kind, status string
	var orderID, requestID sql.NullString
	var refundSnapshot []byte
	if err := tx.QueryRowContext(ctx, `SELECT kind, status, order_id, recovery_request_id, refund_snapshot
		FROM email_queue WHERE id = ? FOR UPDATE`, id).Scan(&kind, &status, &orderID, &requestID, &refundSnapshot); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	var existing sql.NullString
	if err := tx.QueryRowContext(ctx, "SELECT superseded_by FROM email_queue WHERE id = ?", id).Scan(&existing); err != nil {
		return nil, err
	}
	if status != "FAILED" || existing.Valid {
		return nil, ErrConflict
	}
	var retryID string
	if kind == "TICKETS" {
		if !orderID.Valid {
			return nil, ErrConflict
		}
		var orderStatus, buyerEmail string
		var eventStart time.Time
		if err := tx.QueryRowContext(ctx, `SELECT o.status, e.starts_at, b.email FROM orders o
			JOIN reservations r ON r.id = o.reservation_id JOIN events e ON e.id = r.event_id
			JOIN order_buyers b ON b.order_id = o.id WHERE o.id = ? FOR UPDATE`, orderID.String).
			Scan(&orderStatus, &eventStart, &buyerEmail); errors.Is(err, sql.ErrNoRows) {
			return nil, ErrConflict
		} else if err != nil {
			return nil, err
		}
		var tickets, expected uint64
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*), (SELECT COALESCE(SUM(quantity), 0) FROM order_items WHERE order_id = ?)
			FROM etickets WHERE order_id = ?`, orderID.String, orderID.String).Scan(&tickets, &expected); err != nil {
			return nil, err
		}
		if orderStatus != "PAID" || tickets == 0 || tickets != expected || !time.Now().Before(orderaccess.Expiry(eventStart)) {
			return nil, ErrConflict
		}
		blocked, wait, err := recovery.CheckRateLimitsTx(ctx, tx, recovery.RateLimit{Bucket: s.access.Digest("limit/resend-order", orderID.String), Max: 1, Period: time.Minute})
		if err != nil {
			return nil, err
		}
		if blocked >= 0 {
			if err := tx.Commit(); err != nil {
				return nil, err
			}
			return nil, rateLimitError{wait: wait}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE email_queue SET recipient = ?, attempts = 0, next_attempt_at = UTC_TIMESTAMP(6),
			sent_at = NULL, last_error = '', claim_token = NULL, lease_until = NULL, status = 'PENDING', updated_at = UTC_TIMESTAMP(6)
			WHERE id = ? AND status = 'FAILED' AND superseded_by IS NULL`, buyerEmail, id); err != nil {
			return nil, err
		}
		retryID = id
	} else if kind == "RECOVERY" {
		if !requestID.Valid {
			return nil, ErrConflict
		}
		var reference, pairKey string
		var usedAt sql.NullTime
		if err := tx.QueryRowContext(ctx, "SELECT reference, pair_key, used_at FROM recovery_requests WHERE id = ? FOR UPDATE", requestID.String).
			Scan(&reference, &pairKey, &usedAt); errors.Is(err, sql.ErrNoRows) {
			return nil, ErrConflict
		} else if err != nil {
			return nil, err
		}
		if usedAt.Valid {
			return nil, ErrConflict
		}
		order, found, err := recovery.FindOrderByReference(ctx, tx, reference)
		if err != nil {
			return nil, err
		}
		if !found || !order.Eligible(time.Now()) || subtle.ConstantTimeCompare([]byte(pairKey), []byte(recovery.PairKey(s.access, order.BuyerEmail, reference))) != 1 {
			return nil, ErrConflict
		}
		blocked, wait, err := recovery.CheckRateLimitsTx(ctx, tx,
			recovery.RateLimit{Bucket: s.access.Digest("limit/recovery-pair-cooldown", pairKey), Max: 1, Period: time.Minute},
			recovery.RateLimit{Bucket: s.access.Digest("limit/recovery-pair", pairKey), Max: 3, Period: 15 * time.Minute})
		if err != nil {
			return nil, err
		}
		if blocked >= 0 {
			if err := tx.Commit(); err != nil {
				return nil, err
			}
			return nil, rateLimitError{wait: wait}
		}
		retryID, err = recovery.NewID()
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO recovery_requests (id, reference, pair_key, expires_at, created_at)
			VALUES (?, ?, ?, DATE_ADD(UTC_TIMESTAMP(6), INTERVAL ? SECOND), UTC_TIMESTAMP(6))`, retryID, reference, pairKey, int(recovery.RequestTTL.Seconds())); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO email_queue (id, kind, dedupe_key, recovery_request_id, recipient, status, attempts,
			next_attempt_at, last_error, created_at, updated_at) VALUES (?, 'RECOVERY', ?, ?, '', 'PENDING', 0, UTC_TIMESTAMP(6), '', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`, retryID, "recovery:"+retryID, retryID); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE email_queue SET superseded_by = ?, updated_at = UTC_TIMESTAMP(6) WHERE id = ? AND status = 'FAILED' AND superseded_by IS NULL", retryID, id); err != nil {
			return nil, err
		}
	} else if kind == "REFUND" {
		if !orderID.Valid {
			return nil, ErrConflict
		}
		var snapshot refundEmailSnapshot
		if err := decodeRefundEmailSnapshot(refundSnapshot, &snapshot); err != nil {
			return nil, ErrConflict
		}
		var buyerEmail string
		var totalValue string
		var reference string
		if err := tx.QueryRowContext(ctx, `SELECT b.email,CAST(o.total AS CHAR),o.reference FROM orders o
			JOIN order_buyers b ON b.order_id=o.id WHERE o.id=? FOR UPDATE`, orderID.String).Scan(&buyerEmail, &totalValue, &reference); errors.Is(err, sql.ErrNoRows) {
			return nil, ErrConflict
		} else if err != nil {
			return nil, err
		}
		total, parseErr := strconv.ParseUint(totalValue, 10, 64)
		if buyerEmail == "" || parseErr != nil || snapshot.Reference != reference || snapshot.Amount != total {
			return nil, ErrConflict
		}
		blocked, wait, err := recovery.CheckRateLimitsTx(ctx, tx, recovery.RateLimit{Bucket: s.access.Digest("limit/refund-email-retry", orderID.String), Max: 1, Period: time.Minute})
		if err != nil {
			return nil, err
		}
		if blocked >= 0 {
			if err := tx.Commit(); err != nil {
				return nil, err
			}
			return nil, rateLimitError{wait: wait}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE email_queue SET recipient=?, attempts=0, next_attempt_at=UTC_TIMESTAMP(6),
			sent_at=NULL, last_error='', claim_token=NULL, lease_until=NULL, status='PENDING', updated_at=UTC_TIMESTAMP(6)
			WHERE id=? AND status='FAILED' AND superseded_by IS NULL`, buyerEmail, id); err != nil {
			return nil, err
		}
		retryID = id
	} else {
		return nil, ErrConflict
	}
	auditStatus := "PENDING"
	if kind == "RECOVERY" {
		auditStatus = "SUPERSEDED"
	}
	if err := appendAudit(ctx, tx, actor, "RETRY", "EMAIL_JOB", id, map[string]string{"status": "FAILED"}, map[string]string{"status": auditStatus, "retryJobId": retryID, "kind": kind}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"jobId": id, "retryJobId": retryID, "status": "PENDING"}, nil
}

func truncate(value string, limit int) string {
	value = strings.ToValidUTF8(value, "�")
	if len(value) <= limit {
		return value
	}
	for limit > 0 && !utf8.RuneStart(value[limit]) {
		limit--
	}
	return value[:limit]
}

func (s *Service) authorizedTx(ctx context.Context, token string) (*sql.Tx, staffauth.Principal, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, staffauth.Principal{}, err
	}
	actor, err := s.staff.AuthenticateTx(ctx, tx, token)
	if err != nil {
		tx.Rollback()
		return nil, staffauth.Principal{}, err
	}
	if actor.Role != "ADMIN" {
		tx.Rollback()
		return nil, staffauth.Principal{}, staffauth.ErrForbidden
	}
	return tx, actor, nil
}

func lockCase(ctx context.Context, tx *sql.Tx, id uint64, resolving bool) error {
	var status string
	var lease sql.NullTime
	err := tx.QueryRowContext(ctx, "SELECT status, check_lease_until FROM payment_reconciliation_cases WHERE id = ? FOR UPDATE", id).Scan(&status, &lease)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if resolving && (status != "OPEN" || (lease.Valid && lease.Time.After(time.Now().UTC()))) {
		return ErrConflict
	}
	return nil
}

func normalizeNote(note string) (string, error) {
	note = strings.TrimSpace(note)
	if n := utf8.RuneCountInString(note); n < 1 || n > 2000 {
		return "", ErrInvalidRequest
	}
	return note, nil
}

func decodeNote(w http.ResponseWriter, r *http.Request) (string, bool) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	var body struct {
		Note string `json:"note"`
	}
	var extra any
	if decoder.Decode(&body) != nil || !errors.Is(decoder.Decode(&extra), io.EOF) {
		writeError(w, 400, "INVALID_JSON", "JSON request tidak valid")
		return "", false
	}
	return body.Note, true
}

func (h *Handler) recheck(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Recheck(r.Context(), bearer(r), r.PathValue("caseID"))
	h.respond(w, r, result, err)
}

func (h *Handler) addCaseNote(w http.ResponseWriter, r *http.Request) {
	note, ok := decodeNote(w, r)
	if !ok {
		return
	}
	if err := h.service.AddCaseNote(r.Context(), bearer(r), r.PathValue("caseID"), note); err != nil {
		h.respond(w, r, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) resolveCase(w http.ResponseWriter, r *http.Request) {
	note, ok := decodeNote(w, r)
	if !ok {
		return
	}
	if err := h.service.ResolveCase(r.Context(), bearer(r), r.PathValue("caseID"), note); err != nil {
		h.respond(w, r, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
