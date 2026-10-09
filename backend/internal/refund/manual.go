package refund

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/eventstate"
	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
)

type ManualInput struct {
	Reference string `json:"reference"`
	PaidAt    string `json:"paidAt"`
	Note      string `json:"note"`
}

func validateManual(in ManualInput, now time.Time) (time.Time, error) {
	paid, err := time.Parse(time.RFC3339Nano, in.PaidAt)
	if err != nil || paid.After(now) || len(strings.TrimSpace(in.Reference)) < 3 || len(in.Reference) > 160 || len(strings.TrimSpace(in.Note)) < 3 || len(in.Note) > 500 {
		return time.Time{}, ErrInvalid
	}
	return paid.UTC().Truncate(time.Microsecond), nil
}
func (h *Handler) manual(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	var in ManualInput
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	d.DisallowUnknownFields()
	var extra any
	if d.Decode(&in) != nil || !errors.Is(d.Decode(&extra), io.EOF) {
		http.Error(w, `{"error":{"code":"INVALID_REQUEST","message":"Bukti refund tidak valid"}}`, 400)
		return
	}
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"Akses diperlukan"}}`, 401)
		return
	}
	result, err := h.service.CompleteManual(r.Context(), token, strings.ToLower(r.PathValue("orderID")), in)
	if err == nil {
		_ = json.NewEncoder(w).Encode(result)
		return
	}
	status, code := 409, "REFUND_UNAVAILABLE"
	if errors.Is(err, ErrInvalid) {
		status, code = 422, "INVALID_REQUEST"
	} else if errors.Is(err, staffauth.ErrUnauthorized) {
		status, code = 401, "UNAUTHORIZED"
	} else if errors.Is(err, staffauth.ErrForbidden) {
		status, code = 403, "FORBIDDEN"
	} else if !errors.Is(err, ErrUnavailable) {
		status, code = 500, "INTERNAL_ERROR"
		h.logger.ErrorContext(r.Context(), "manual refund failed", "error", err)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": "Refund manual belum dapat diselesaikan"}})
}
func (s *Service) CompleteManual(ctx context.Context, token, id string, in ManualInput) (Result, error) {
	var result Result
	paid, err := validateManual(in, time.Now())
	if err != nil {
		return result, err
	}
	if _, err := hex.DecodeString(id); err != nil || len(id) != 32 {
		return result, ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	p, err := s.staff.AuthenticateTx(ctx, tx, token)
	if err != nil {
		return result, err
	}
	if p.Role != "ADMIN" {
		return result, staffauth.ErrForbidden
	}
	if _, err := eventstate.ForOrder(ctx, tx, id, true); err != nil {
		return result, err
	}
	var locked string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM orders WHERE id=? FOR UPDATE", id).Scan(&locked); err != nil {
		return result, err
	}
	var reference string
	var priorPaid sql.NullTime
	err = tx.QueryRowContext(ctx, "SELECT status,amount,reason,manual_reference,manual_paid_at FROM order_refunds WHERE order_id=? FOR UPDATE", id).Scan(&result.Status, &result.Amount, &result.Reason, &reference, &priorPaid)
	if errors.Is(err, sql.ErrNoRows) {
		return result, ErrUnavailable
	}
	if err != nil {
		return result, err
	}
	if result.Status == "SUCCEEDED" && reference == strings.TrimSpace(in.Reference) && priorPaid.Valid && priorPaid.Time.Equal(paid) {
		return result, tx.Commit()
	}
	if result.Status != "MANUAL_REQUIRED" {
		return result, ErrUnavailable
	}
	var settled sql.NullTime
	if err := tx.QueryRowContext(ctx, "SELECT paid_at FROM payments WHERE order_id=? AND status='SUCCEEDED'", id).Scan(&settled); err != nil {
		return result, err
	}
	if !settled.Valid || paid.Before(settled.Time) {
		return result, ErrInvalid
	}
	if _, err := tx.ExecContext(ctx, "UPDATE order_refunds SET manual_reference=?,manual_paid_at=? WHERE order_id=?", strings.TrimSpace(in.Reference), paid, id); err != nil {
		return result, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO order_refund_audit(refund_id,staff_id,action,reason,created_at) SELECT id,?,'MANUAL_CONFIRMED',?,UTC_TIMESTAMP(6) FROM order_refunds WHERE order_id=?`, p.ID, in.Note, id); err != nil {
		return result, err
	}
	if err := s.finishTx(ctx, tx, id, true, ""); err != nil {
		return result, err
	}
	result.Status = "SUCCEEDED"
	return result, tx.Commit()
}
