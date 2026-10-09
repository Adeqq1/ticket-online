package email

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/Adeqq1/ticket-online/backend/internal/recovery"
)

const noMatch = "tidak ada pesanan yang memenuhi syarat pemulihan"

// expireRecovery closes recovery jobs whose request deadline passed, including while SMTP is disabled.
func (s *Service) expireRecovery(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE email_queue q JOIN recovery_requests r ON r.id = q.recovery_request_id
		SET q.status = 'FAILED', q.lease_until = NULL, q.claim_token = NULL, q.last_error = 'permintaan pemulihan kedaluwarsa',
		q.updated_at = UTC_TIMESTAMP(6)
		WHERE q.kind = 'RECOVERY' AND q.status = 'PENDING' AND r.expires_at <= UTC_TIMESTAMP(6)`)
	return err
}

func (s *Service) sendRecovery(ctx context.Context, current job) error {
	var reference, pairKey string
	var usable bool
	err := s.db.QueryRowContext(ctx, `SELECT reference, pair_key, used_at IS NULL AND expires_at > UTC_TIMESTAMP(6)
		FROM recovery_requests WHERE id = ?`, current.requestID).Scan(&reference, &pairKey, &usable)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !usable) {
		return s.finishFailure(ctx, current, "permintaan pemulihan kedaluwarsa")
	}
	if err != nil {
		return s.retry(ctx, current, "permintaan pemulihan tidak dapat dibaca")
	}
	order, found, err := recovery.FindOrderByReference(ctx, s.db, reference)
	if err != nil {
		return s.retry(ctx, current, "pesanan tidak dapat dibaca")
	}
	access := orderaccess.New(s.db, s.config.AccessSecret)
	if !found || subtle.ConstantTimeCompare([]byte(recovery.PairKey(access, order.BuyerEmail, reference)), []byte(pairKey)) != 1 || !order.Eligible(time.Now()) {
		return s.finishFailure(ctx, current, noMatch)
	}
	current.recipient = order.BuyerEmail
	if !mailbox(current.recipient) {
		return s.finishFailure(ctx, current, "alamat penerima tidak valid")
	}
	if _, err := s.db.ExecContext(ctx, "UPDATE email_queue SET recipient = ? WHERE id = ? AND claim_token = ?", current.recipient, current.id, current.claimToken); err != nil {
		return s.retry(ctx, current, "penerima tidak dapat disimpan")
	}
	// Each attempt gets a fresh token; earlier ones stay valid until the shared request deadline.
	raw, hash, err := recovery.NewToken()
	if err != nil {
		return s.retry(ctx, current, "token pemulihan tidak dapat dibuat")
	}
	var expiresAt time.Time
	if err := s.db.QueryRowContext(ctx, "SELECT expires_at FROM recovery_requests WHERE id = ?", current.requestID).Scan(&expiresAt); err != nil {
		return s.retry(ctx, current, "masa berlaku pemulihan tidak dapat dibaca")
	}
	if !order.AccessDeadline.IsZero() && order.AccessDeadline.Before(expiresAt) {
		expiresAt = order.AccessDeadline
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO recovery_tokens (token_hash, request_id, order_id, expires_at, created_at)
		VALUES (?, ?, ?, ?, UTC_TIMESTAMP(6))`, hash, current.requestID, order.ID, expiresAt); err != nil {
		return s.retry(ctx, current, "token pemulihan tidak dapat disimpan")
	}
	link := s.config.FrontendURL + "/pulihkan-tiket#token=" + raw
	body, from, to, err := renderRecovery(s.config, current, order.BuyerName, reference, link)
	if err != nil {
		return s.retry(ctx, current, err.Error())
	}
	if err := s.deliver(ctx, body, from, to); err != nil {
		return s.retry(ctx, current, err.Error())
	}
	return s.markSent(ctx, current)
}

func renderRecovery(config Config, current job, buyer, reference, link string) ([]byte, string, string, error) {
	minutes := int(recovery.RequestTTL.Minutes())
	var plain strings.Builder
	fmt.Fprintf(&plain, "Halo %s,\n\nKami menerima permintaan untuk memulihkan akses tiket pesanan %s di browser baru.\n\n", buyer, reference)
	fmt.Fprintf(&plain, "Buka tautan berikut lalu tekan tombol Pulihkan tiket. Tautan berlaku %d menit dan hanya dapat dipakai sekali:\n%s\n\n", minutes, link)
	fmt.Fprintln(&plain, "Jika kamu tidak meminta pemulihan, abaikan email ini. Akses pesananmu tidak berubah.")
	var html strings.Builder
	fmt.Fprintf(&html, "<!doctype html><html lang=\"id\"><meta charset=\"utf-8\"><body><p>Halo %s,</p><p>Kami menerima permintaan untuk memulihkan akses tiket pesanan <strong>%s</strong> di browser baru.</p>", escaped(buyer), escaped(reference))
	fmt.Fprintf(&html, "<p><a href=\"%s\">Pulihkan tiket</a></p><p>Tautan berlaku %d menit dan hanya dapat dipakai sekali.</p>", escaped(link), minutes)
	fmt.Fprintln(&html, "<p>Jika kamu tidak meminta pemulihan, abaikan email ini. Akses pesananmu tidak berubah.</p></body></html>")
	return compose(config, current, "Pulihkan tiket pesanan "+reference, plain.String(), html.String())
}
