package email

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
)

const (
	maxAttempts = 3
	leaseTime   = time.Minute
)

type Config struct {
	Host, Username, Password, From, TLSMode, FrontendURL string
	Port                                                 int
	AccessSecret                                         []byte
}

type Service struct {
	db     *sql.DB
	config Config
	logger *slog.Logger
}

func NewService(db *sql.DB, config Config, logger *slog.Logger) *Service {
	return &Service{db: db, config: config, logger: logger}
}

func (s *Service) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		s.process(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) process(ctx context.Context) {
	if err := s.enqueuePaid(ctx); err != nil && ctx.Err() == nil {
		s.logger.ErrorContext(ctx, "reconcile paid order email queue", "error", err)
		return
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE email_queue SET status = 'FAILED', lease_until = NULL,
		claim_token = NULL, last_error = 'pengiriman berhenti sebelum selesai', updated_at = UTC_TIMESTAMP(6)
		WHERE status = 'PROCESSING' AND attempts >= ? AND lease_until <= UTC_TIMESTAMP(6)`, maxAttempts); err != nil && ctx.Err() == nil {
		s.logger.ErrorContext(ctx, "expire exhausted email claims", "error", err)
		return
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM rate_limits WHERE reset_at < UTC_TIMESTAMP(6) LIMIT 1000"); err != nil && ctx.Err() == nil {
		s.logger.ErrorContext(ctx, "delete expired rate limit buckets", "error", err)
	}
	if err := s.expireRecovery(ctx); err != nil && ctx.Err() == nil {
		s.logger.ErrorContext(ctx, "expire recovery email jobs", "error", err)
	}
	if s.config.Host == "" || ctx.Err() != nil {
		return
	}
	for range 20 {
		if ctx.Err() != nil {
			return
		}
		job, found, err := s.claim(ctx)
		if err != nil {
			s.logger.ErrorContext(ctx, "claim order email", "error", err)
			return
		}
		if !found {
			return
		}
		if err := s.send(ctx, job); err != nil {
			if ctx.Err() == nil {
				s.logger.WarnContext(ctx, "email failed", "job_id", job.id, "kind", job.kind, "attempt", job.attempts, "error", err)
			}
			continue
		}
		s.logger.InfoContext(ctx, "email sent", "job_id", job.id, "kind", job.kind, "attempt", job.attempts)
	}
}

func (s *Service) enqueuePaid(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `INSERT IGNORE INTO email_queue
		(id, kind, dedupe_key, order_id, recipient, status, attempts, next_attempt_at, last_error, created_at, updated_at)
		SELECT o.id, 'TICKETS', CONCAT('tickets:', o.id), o.id, b.email, 'PENDING', 0, UTC_TIMESTAMP(6), '', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)
		FROM orders o
		JOIN payments p ON p.order_id = o.id AND p.status = 'SUCCEEDED' AND p.paid_at >= (
			SELECT applied_at FROM schema_migrations WHERE version = 13
		)
		JOIN order_buyers b ON b.order_id = o.id
		JOIN (SELECT order_id, COUNT(*) AS issued FROM etickets GROUP BY order_id) e ON e.order_id = o.id
		JOIN (SELECT order_id, SUM(quantity) AS expected FROM order_items GROUP BY order_id) i ON i.order_id = o.id
		WHERE o.status = 'PAID' AND e.issued = i.expected
		AND NOT EXISTS (SELECT 1 FROM email_queue q WHERE q.dedupe_key = CONCAT('tickets:', o.id))`)
	if err != nil {
		return fmt.Errorf("insert newly paid orders into email queue: %w", err)
	}
	return nil
}

type job struct {
	id, kind, orderID, requestID, recipient, claimToken string
	attempts                                            int
}

func (s *Service) claim(ctx context.Context) (job, bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return job{}, false, fmt.Errorf("begin email claim: %w", err)
	}
	defer tx.Rollback()
	var result job
	var orderID, requestID sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT id, kind, order_id, recovery_request_id, recipient, attempts
		FROM email_queue WHERE attempts < ? AND ((status = 'PENDING' AND next_attempt_at <= UTC_TIMESTAMP(6))
		OR (status = 'PROCESSING' AND lease_until <= UTC_TIMESTAMP(6)))
		ORDER BY next_attempt_at, id LIMIT 1 FOR UPDATE SKIP LOCKED`, maxAttempts).Scan(&result.id, &result.kind, &orderID, &requestID, &result.recipient, &result.attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return job{}, false, tx.Commit()
	}
	if err != nil {
		return job{}, false, fmt.Errorf("select email job: %w", err)
	}
	result.orderID, result.requestID = orderID.String, requestID.String
	token, err := randomID()
	if err != nil {
		return job{}, false, err
	}
	result.attempts++
	result.claimToken = token
	if _, err := tx.ExecContext(ctx, `UPDATE email_queue SET status = 'PROCESSING', attempts = ?,
		lease_until = DATE_ADD(UTC_TIMESTAMP(6), INTERVAL ? SECOND), claim_token = ?, updated_at = UTC_TIMESTAMP(6)
		WHERE id = ?`, result.attempts, int(leaseTime.Seconds()), token, result.id); err != nil {
		return job{}, false, fmt.Errorf("update email claim: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return job{}, false, fmt.Errorf("commit email claim: %w", err)
	}
	return result, true, nil
}

type orderSummary struct {
	buyer, reference, artist, city, venue, address string
	startsAt                                       time.Time
	subtotal, fee, discount, total                 uint64
}

type line struct {
	name         string
	quantity     uint64
	unit, amount uint64
}

type ticket struct {
	ID           string `json:"id"`
	Code         string `json:"code"`
	AttendeeName string `json:"attendeeName"`
	TierName     string `json:"tierName"`
	Gate         string `json:"gate"`
	link         string
}

func (s *Service) send(ctx context.Context, current job) error {
	if current.kind == "RECOVERY" {
		return s.sendRecovery(ctx, current)
	}
	if !mailbox(current.recipient) {
		return s.finishFailure(ctx, current, "alamat penerima tidak valid")
	}
	var summary orderSummary
	err := s.db.QueryRowContext(ctx, `SELECT b.name, o.reference, e.artist, e.city, e.venue, e.address, e.starts_at,
		o.subtotal, o.admin_fee, o.discount, o.total FROM orders o
		JOIN order_buyers b ON b.order_id = o.id JOIN reservations r ON r.id = o.reservation_id
		JOIN events e ON e.id = r.event_id WHERE o.id = ? AND o.status = 'PAID'`, current.orderID).Scan(
		&summary.buyer, &summary.reference, &summary.artist, &summary.city, &summary.venue, &summary.address,
		&summary.startsAt, &summary.subtotal, &summary.fee, &summary.discount, &summary.total)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return s.finishFailure(ctx, current, "ringkasan pesanan tidak tersedia")
		}
		return s.retry(ctx, current, "ringkasan pesanan tidak dapat dibaca")
	}
	var items []line
	rows, err := s.db.QueryContext(ctx, `SELECT tier_name, quantity, unit_price, line_total
		FROM order_items WHERE order_id = ? ORDER BY ticket_tier_id`, current.orderID)
	if err != nil {
		return s.retry(ctx, current, "data tiket tidak dapat dibaca")
	}
	for rows.Next() {
		var item line
		if err := rows.Scan(&item.name, &item.quantity, &item.unit, &item.amount); err != nil {
			rows.Close()
			return s.retry(ctx, current, "data tiket tidak dapat dibaca")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return s.retry(ctx, current, "data tiket tidak dapat dibaca")
	}
	if err := rows.Close(); err != nil {
		return s.retry(ctx, current, "data tiket tidak dapat dibaca")
	}
	rows, err = s.db.QueryContext(ctx, "SELECT snapshot FROM etickets WHERE order_id = ? ORDER BY issued_at, id", current.orderID)
	if err != nil {
		return s.retry(ctx, current, "snapshot tiket tidak dapat dibaca")
	}
	var tickets []ticket
	for rows.Next() {
		var raw []byte
		var value ticket
		if err := rows.Scan(&raw); err != nil || json.Unmarshal(raw, &value) != nil || value.ID == "" {
			rows.Close()
			return s.retry(ctx, current, "snapshot tiket tidak valid")
		}
		tickets = append(tickets, value)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return s.retry(ctx, current, "snapshot tiket tidak dapat dibaca")
	}
	if err := rows.Close(); err != nil {
		return s.retry(ctx, current, "snapshot tiket tidak dapat dibaca")
	}
	if len(tickets) == 0 || len(items) == 0 {
		return s.finishFailure(ctx, current, "tiket pesanan tidak tersedia")
	}
	linkExpiry := orderaccess.Expiry(summary.startsAt)
	if !time.Now().Before(linkExpiry) {
		return s.finishFailure(ctx, current, "akses tiket telah kedaluwarsa")
	}
	access := orderaccess.New(s.db, s.config.AccessSecret).Token(current.orderID)
	for i := range tickets {
		tickets[i].link = s.config.FrontendURL + "/tiket/" + url.PathEscape(tickets[i].ID) + "#access_token=" + url.QueryEscape(access)
	}
	body, from, to, err := renderMessage(s.config, current, summary, items, tickets)
	if err != nil {
		return s.retry(ctx, current, err.Error())
	}
	if err := s.deliver(ctx, body, from, to); err != nil {
		return s.retry(ctx, current, err.Error())
	}
	return s.markSent(ctx, current)
}

func (s *Service) markSent(ctx context.Context, current job) error {
	result, err := s.db.ExecContext(ctx, `UPDATE email_queue SET status = 'SENT', sent_at = UTC_TIMESTAMP(6),
		lease_until = NULL, claim_token = NULL, last_error = '', updated_at = UTC_TIMESTAMP(6)
		WHERE id = ? AND status = 'PROCESSING' AND claim_token = ?`, current.id, current.claimToken)
	if err != nil {
		return fmt.Errorf("record sent email: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return errors.New("email claim expired before sent status was saved")
	}
	return nil
}

func (s *Service) retry(ctx context.Context, current job, reason string) error {
	status := "PENDING"
	delay := time.Minute
	if current.attempts == 2 {
		delay = 5 * time.Minute
	} else if current.attempts >= maxAttempts {
		status = "FAILED"
		delay = 0
	}
	_, err := s.db.ExecContext(ctx, `UPDATE email_queue SET status = ?, next_attempt_at = DATE_ADD(UTC_TIMESTAMP(6), INTERVAL ? SECOND),
		lease_until = NULL, claim_token = NULL, last_error = ?, updated_at = UTC_TIMESTAMP(6)
		WHERE id = ? AND status = 'PROCESSING' AND claim_token = ?`, status, int(delay.Seconds()), truncate(reason, 512), current.id, current.claimToken)
	if err != nil {
		return fmt.Errorf("record email failure: %w", err)
	}
	return errors.New(reason)
}

func (s *Service) finishFailure(ctx context.Context, current job, reason string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE email_queue SET status = 'FAILED', lease_until = NULL,
		claim_token = NULL, last_error = ?, updated_at = UTC_TIMESTAMP(6)
		WHERE id = ? AND status = 'PROCESSING' AND claim_token = ?`, truncate(reason, 512), current.id, current.claimToken)
	if err != nil {
		return fmt.Errorf("record permanent email failure: %w", err)
	}
	return errors.New(reason)
}

func randomID() (string, error) {
	var id [16]byte
	if _, err := io.ReadFull(rand.Reader, id[:]); err != nil {
		return "", fmt.Errorf("generate email claim token: %w", err)
	}
	return fmt.Sprintf("%x", id), nil
}

func mailbox(value string) bool {
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value && !strings.ContainsAny(value, "\r\n")
}

func truncate(value string, length int) string {
	if len(value) <= length {
		return value
	}
	return value[:length]
}

func writeQuotedPrintable(w io.Writer, value string) error {
	writer := quotedprintable.NewWriter(w)
	if _, err := io.WriteString(writer, value); err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
}

func money(value uint64) string {
	raw := strconv.FormatUint(value, 10)
	for i := len(raw) - 3; i > 0; i -= 3 {
		raw = raw[:i] + "." + raw[i:]
	}
	return "Rp" + raw
}

func eventTime(value time.Time) string {
	months := [...]string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	local := value.In(time.FixedZone("WIB", 7*60*60))
	return fmt.Sprintf("%02d %s %d, %02d:%02d WIB", local.Day(), months[local.Month()-1], local.Year(), local.Hour(), local.Minute())
}

func safeURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil && parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment == "" && !strings.ContainsAny(value, "\r\n")
}

func formatAddress(value string) (string, error) {
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address == "" || strings.ContainsAny(parsed.Address, "\r\n") {
		return "", errors.New("email address is invalid")
	}
	return parsed.Address, nil
}

func escaped(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;").Replace(value)
}

func smtpHost(host string) bool {
	return net.ParseIP(host) != nil || !strings.ContainsAny(host, " \t\r\n")
}

func subject(value string) string { return mime.QEncoding.Encode("UTF-8", value) }

// messageID is unique per delivery attempt; mail clients may hide a resend that reuses an earlier Message-ID.
func messageID(jobID, claimToken, frontendURL string) string {
	parsed, _ := url.Parse(frontendURL)
	return "<ticket-" + jobID + "-" + claimToken + "@" + parsed.Hostname() + ">"
}

func cleanError(err error, phase string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("SMTP %s gagal", phase)
}

func (s *Service) deliver(ctx context.Context, body []byte, from, to string) error {
	endpoint := net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp", endpoint)
	if err != nil {
		return cleanError(err, "koneksi")
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return cleanError(err, "batas waktu")
	}
	if s.config.TLSMode == "tls" {
		secure := tls.Client(connection, &tls.Config{ServerName: s.config.Host, MinVersion: tls.VersionTLS12})
		if err := secure.HandshakeContext(ctx); err != nil {
			return cleanError(err, "TLS")
		}
		connection = secure
	}
	client := textproto.NewConn(connection)
	defer client.Close()
	if _, _, err := client.ReadResponse(220); err != nil {
		return cleanError(err, "salam server")
	}
	command := func(expected int, format string, args ...any) error {
		if err := client.PrintfLine(format, args...); err != nil {
			return err
		}
		_, _, err := client.ReadResponse(expected)
		return err
	}
	if err := command(250, "EHLO ticket-online"); err != nil {
		return cleanError(err, "EHLO")
	}
	if s.config.TLSMode == "starttls" {
		if err := command(220, "STARTTLS"); err != nil {
			return cleanError(err, "STARTTLS")
		}
		secure := tls.Client(connection, &tls.Config{ServerName: s.config.Host, MinVersion: tls.VersionTLS12})
		if err := secure.HandshakeContext(ctx); err != nil {
			return cleanError(err, "STARTTLS")
		}
		connection = secure
		client = textproto.NewConn(connection)
		if err := command(250, "EHLO ticket-online"); err != nil {
			return cleanError(err, "EHLO setelah STARTTLS")
		}
	}
	if s.config.Username != "" {
		if s.config.TLSMode == "none" {
			return errors.New("autentikasi SMTP tanpa TLS ditolak")
		}
		credentials := base64.StdEncoding.EncodeToString([]byte("\x00" + s.config.Username + "\x00" + s.config.Password))
		if err := command(235, "AUTH PLAIN %s", credentials); err != nil {
			return cleanError(err, "autentikasi")
		}
	}
	if err := command(250, "MAIL FROM:<%s>", from); err != nil {
		return cleanError(err, "pengirim")
	}
	if err := command(250, "RCPT TO:<%s>", to); err != nil {
		return cleanError(err, "penerima")
	}
	if err := command(354, "DATA"); err != nil {
		return cleanError(err, "isi pesan")
	}
	data := client.DotWriter()
	if _, err := data.Write(body); err != nil {
		_ = data.Close()
		return cleanError(err, "kirim isi pesan")
	}
	if err := data.Close(); err != nil {
		return cleanError(err, "kirim isi pesan")
	}
	if _, _, err := client.ReadResponse(250); err != nil {
		return cleanError(err, "konfirmasi pesan")
	}
	return nil
}

func renderMessage(config Config, current job, summary orderSummary, items []line, tickets []ticket) ([]byte, string, string, error) {
	date := eventTime(summary.startsAt)
	var plain strings.Builder
	fmt.Fprintf(&plain, "Halo %s,\n\nPembayaran pesanan %s berhasil. Berikut ringkasan tiketmu.\n\n%s\n%s\n%s\n%s\n%s\n\nRincian pesanan:\n", summary.buyer, summary.reference, summary.artist, summary.venue, summary.address, summary.city, date)
	for _, item := range items {
		fmt.Fprintf(&plain, "- %s × %d — %s (subtotal %s)\n", item.name, item.quantity, money(item.unit), money(item.amount))
	}
	fmt.Fprintf(&plain, "\nSubtotal: %s\nBiaya admin: %s\nDiskon: %s\nTotal: %s\n", money(summary.subtotal), money(summary.fee), money(summary.discount), money(summary.total))
	fmt.Fprintln(&plain, "\nBuka e-ticket setiap peserta:")
	for _, value := range tickets {
		fmt.Fprintf(&plain, "- %s · %s · Gate %s · Kode %s\n  %s\n", value.AttendeeName, value.TierName, value.Gate, value.Code, value.link)
	}
	fmt.Fprintln(&plain, "\nTunjukkan QR atau kode e-ticket kepada petugas di gate.")
	var html strings.Builder
	fmt.Fprintf(&html, "<!doctype html><html lang=\"id\"><meta charset=\"utf-8\"><body><p>Halo %s,</p><p>Pembayaran pesanan <strong>%s</strong> berhasil.</p><h2>%s</h2><p>%s, %s<br>%s<br>%s</p><h3>Ringkasan pesanan</h3><ul>", escaped(summary.buyer), escaped(summary.reference), escaped(summary.artist), escaped(summary.venue), escaped(summary.city), escaped(summary.address), escaped(date))
	for _, item := range items {
		fmt.Fprintf(&html, "<li>%s × %d — %s (subtotal %s)</li>", escaped(item.name), item.quantity, escaped(money(item.unit)), escaped(money(item.amount)))
	}
	fmt.Fprintf(&html, "</ul><p>Subtotal: %s<br>Biaya admin: %s<br>Diskon: %s<br><strong>Total: %s</strong></p><h3>E-ticket peserta</h3><ul>", escaped(money(summary.subtotal)), escaped(money(summary.fee)), escaped(money(summary.discount)), escaped(money(summary.total)))
	for _, value := range tickets {
		fmt.Fprintf(&html, "<li>%s — %s, Gate %s · Kode %s · <a href=\"%s\">Buka e-ticket</a></li>", escaped(value.AttendeeName), escaped(value.TierName), escaped(value.Gate), escaped(value.Code), escaped(value.link))
	}
	fmt.Fprintln(&html, "</ul><p>Tunjukkan QR atau kode e-ticket kepada petugas di gate.</p></body></html>")
	return compose(config, current, "E-ticket pesanan "+summary.reference, plain.String(), html.String())
}

// compose validates the envelope and wraps plain and HTML bodies in one multipart/alternative message.
func compose(config Config, current job, subjectText, plain, html string) ([]byte, string, string, error) {
	from, err := formatAddress(config.From)
	if err != nil || !safeURL(config.FrontendURL) || !smtpHost(config.Host) {
		return nil, "", "", errors.New("konfigurasi pengiriman email tidak valid")
	}
	to, err := formatAddress(current.recipient)
	if err != nil {
		return nil, "", "", errors.New("alamat penerima tidak valid")
	}
	var body strings.Builder
	multipartWriter := multipart.NewWriter(&body)
	var headers strings.Builder
	fmt.Fprintf(&headers, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=%q\r\n\r\n",
		(&mail.Address{Address: from}).String(), (&mail.Address{Address: to}).String(), subject(subjectText), time.Now().UTC().Format(time.RFC1123Z), messageID(current.id, current.claimToken, config.FrontendURL), multipartWriter.Boundary())
	if _, err := body.WriteString(headers.String()); err != nil {
		return nil, "", "", errors.New("pesan email tidak dapat dibuat")
	}
	for _, part := range []struct{ contentType, value string }{{"text/plain; charset=utf-8", plain}, {"text/html; charset=utf-8", html}} {
		partHeaders := make(textproto.MIMEHeader)
		partHeaders.Set("Content-Type", part.contentType)
		partHeaders.Set("Content-Transfer-Encoding", "quoted-printable")
		writer, err := multipartWriter.CreatePart(partHeaders)
		if err != nil || writeQuotedPrintable(writer, part.value) != nil {
			return nil, "", "", errors.New("pesan email tidak dapat dibuat")
		}
	}
	if err := multipartWriter.Close(); err != nil {
		return nil, "", "", errors.New("pesan email tidak dapat dibuat")
	}
	return []byte(body.String()), from, to, nil
}
