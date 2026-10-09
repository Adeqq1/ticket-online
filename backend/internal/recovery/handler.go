package recovery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/eventstate"
	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
)

type Handler struct {
	db             *sql.DB
	access         *orderaccess.Access
	logger         *slog.Logger
	trustedProxies []netip.Prefix
}

func NewHandler(db *sql.DB, access *orderaccess.Access, logger *slog.Logger, trustedProxies ...netip.Prefix) *Handler {
	return &Handler{db: db, access: access, logger: logger, trustedProxies: trustedProxies}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/ticket-recovery", h.Request)
	mux.HandleFunc("POST /api/v1/ticket-recovery/verify", h.Verify)
	mux.HandleFunc("POST /api/v1/orders/{orderID}/resend-email", h.Resend)
}

// Request queues a recovery email. Matching happens in the email worker so the response never reveals a match.
func (h *Handler) Request(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var body struct{ Email, Reference string }
	if !decode(w, r, &body) {
		return
	}
	email, reference := NormalizeEmail(body.Email), NormalizeReference(body.Reference)
	if email == "" || reference == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Email atau reference pesanan tidak valid")
		return
	}
	pair := PairKey(h.access, email, reference)
	blocked, wait, err := allow(r.Context(), h.db,
		limit{h.access.Digest("limit/recovery-ip", clientIP(r, h.trustedProxies)), 10, 15 * time.Minute},
		limit{h.access.Digest("limit/recovery-pair-cooldown", pair), 1, time.Minute},
		limit{h.access.Digest("limit/recovery-pair", pair), 3, 15 * time.Minute},
	)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	if blocked == 0 {
		tooMany(w, wait)
		return
	}
	if blocked < 0 {
		if err := h.enqueueRecovery(r.Context(), reference, pair); err != nil {
			h.internal(w, r, err)
			return
		}
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"message": AcceptedMessage})
}

func (h *Handler) enqueueRecovery(ctx context.Context, reference, pair string) error {
	requestID, err := NewID()
	if err != nil {
		return err
	}
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin recovery request: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO recovery_requests (id, reference, pair_key, expires_at, created_at)
		VALUES (?, ?, ?, DATE_ADD(UTC_TIMESTAMP(6), INTERVAL ? SECOND), UTC_TIMESTAMP(6))`,
		requestID, reference, pair, int(RequestTTL.Seconds())); err != nil {
		return fmt.Errorf("insert recovery request: %w", err)
	}
	// The recipient stays empty until the worker confirms the pair and reads the stored buyer email.
	if _, err := tx.ExecContext(ctx, `INSERT INTO email_queue
		(id, kind, dedupe_key, recovery_request_id, recipient, status, attempts, next_attempt_at, last_error, created_at, updated_at)
		VALUES (?, 'RECOVERY', ?, ?, '', 'PENDING', 0, UTC_TIMESTAMP(6), '', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`,
		requestID, "recovery:"+requestID, requestID); err != nil {
		return fmt.Errorf("queue recovery email: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit recovery request: %w", err)
	}
	return nil
}

type VerifyResult struct {
	OrderID         string   `json:"orderId"`
	AccessToken     string   `json:"accessToken"`
	Reference       string   `json:"reference"`
	ReservationID   string   `json:"reservationId"`
	ExpiresAt       string   `json:"expiresAt"`
	AccessExpiresAt *string  `json:"accessExpiresAt"`
	ChangedEvent    bool     `json:"changedEvent"`
	TicketIDs       []string `json:"ticketIds"`
}

var errLinkInvalid = errors.New("recovery link invalid")

// Verify consumes a recovery link once and returns order access for the browser to store.
func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var body struct{ Token string }
	if !decode(w, r, &body) {
		return
	}
	blocked, wait, err := allow(r.Context(), h.db, limit{h.access.Digest("limit/recovery-verify-ip", clientIP(r, h.trustedProxies)), 30, 15 * time.Minute})
	if err != nil {
		h.internal(w, r, err)
		return
	}
	if blocked >= 0 {
		tooMany(w, wait)
		return
	}
	if !validToken(body.Token) {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Tautan pemulihan tidak valid")
		return
	}
	result, err := h.consume(r.Context(), HashToken(body.Token))
	if errors.Is(err, errLinkInvalid) {
		writeError(w, http.StatusGone, "RECOVERY_LINK_INVALID", "Tautan pemulihan tidak berlaku lagi. Minta tautan baru.")
		return
	}
	if err != nil {
		h.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) consume(ctx context.Context, tokenHash string) (VerifyResult, error) {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("begin recovery verify: %w", err)
	}
	defer tx.Rollback()
	var requestID, orderID string
	var usable bool
	// Locking the shared request row serializes verifies of sibling tokens from retried emails.
	err = tx.QueryRowContext(ctx, `SELECT t.request_id, t.order_id,
		t.used_at IS NULL AND r.used_at IS NULL AND t.expires_at > UTC_TIMESTAMP(6) AND r.expires_at > UTC_TIMESTAMP(6)
		FROM recovery_tokens t JOIN recovery_requests r ON r.id = t.request_id
		WHERE t.token_hash = ? FOR UPDATE`, tokenHash).Scan(&requestID, &orderID, &usable)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !usable) {
		return VerifyResult{}, errLinkInvalid
	}
	if err != nil {
		return VerifyResult{}, fmt.Errorf("find recovery token: %w", err)
	}
	order, found, err := FindOrderByID(ctx, tx, orderID)
	if err != nil {
		return VerifyResult{}, err
	}
	if !found || !order.Eligible(time.Now()) {
		return VerifyResult{}, errLinkInvalid
	}
	rows, err := tx.QueryContext(ctx, "SELECT id FROM etickets WHERE order_id = ? ORDER BY issued_at, id", orderID)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("list recovered tickets: %w", err)
	}
	result := VerifyResult{OrderID: order.ID, AccessToken: h.access.Token(order.ID), Reference: order.Reference, ReservationID: order.ReservationID,
		ExpiresAt: order.ExpiresAt.UTC().Format(time.RFC3339Nano), AccessExpiresAt: eventstate.TimeJSON(order.AccessDeadline), ChangedEvent: order.Changed, TicketIDs: []string{}}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return VerifyResult{}, fmt.Errorf("scan recovered ticket: %w", err)
		}
		result.TicketIDs = append(result.TicketIDs, id)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return VerifyResult{}, fmt.Errorf("list recovered tickets: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE recovery_requests SET used_at = UTC_TIMESTAMP(6) WHERE id = ?", requestID); err != nil {
		return VerifyResult{}, fmt.Errorf("consume recovery request: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE recovery_tokens SET used_at = UTC_TIMESTAMP(6) WHERE request_id = ? AND used_at IS NULL", requestID); err != nil {
		return VerifyResult{}, fmt.Errorf("consume recovery tokens: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return VerifyResult{}, fmt.Errorf("commit recovery verify: %w", err)
	}
	return result, nil
}

var orderIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Resend reschedules the order's ticket email to the stored buyer address.
func (h *Handler) Resend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	orderID := strings.ToLower(r.PathValue("orderID"))
	if !orderIDPattern.MatchString(orderID) {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "ID order tidak valid")
		return
	}
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		writeError(w, http.StatusUnauthorized, "ACCESS_TOKEN_REQUIRED", "Token akses diperlukan")
		return
	}
	if _, err := h.access.AuthorizeOrder(r.Context(), orderID, token); err != nil {
		switch {
		case errors.Is(err, orderaccess.ErrUnauthorized), errors.Is(err, orderaccess.ErrNotFound):
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Data tidak ditemukan")
		case errors.Is(err, orderaccess.ErrExpired):
			writeError(w, http.StatusGone, "ACCESS_TOKEN_EXPIRED", "Token akses sudah kedaluwarsa")
		default:
			h.internal(w, r, err)
		}
		return
	}
	order, found, err := FindOrderByID(r.Context(), h.db, orderID)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	if !found || !order.Eligible(time.Now()) || !order.TicketActive {
		writeError(w, http.StatusConflict, "ORDER_NOT_ELIGIBLE", "Email tiket hanya dapat dikirim ulang untuk pesanan lunas dengan tiket lengkap")
		return
	}
	blocked, wait, err := allow(r.Context(), h.db, limit{h.access.Digest("limit/resend-order", orderID), 1, time.Minute})
	if err != nil {
		h.internal(w, r, err)
		return
	}
	if blocked >= 0 {
		tooMany(w, wait)
		return
	}
	// An active job is left alone; a finished one restarts with a fresh attempt budget.
	// status is assigned last so every IF() reads the old status.
	if _, err := h.db.ExecContext(r.Context(), `INSERT INTO email_queue
		(id, kind, dedupe_key, order_id, recipient, status, attempts, next_attempt_at, last_error, created_at, updated_at)
		VALUES (?, 'TICKETS', ?, ?, ?, 'PENDING', 0, UTC_TIMESTAMP(6), '', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))
		ON DUPLICATE KEY UPDATE
		recipient = IF(status IN ('SENT', 'FAILED'), ?, recipient),
		attempts = IF(status IN ('SENT', 'FAILED'), 0, attempts),
		next_attempt_at = IF(status IN ('SENT', 'FAILED'), UTC_TIMESTAMP(6), next_attempt_at),
		sent_at = IF(status IN ('SENT', 'FAILED'), NULL, sent_at),
		last_error = IF(status IN ('SENT', 'FAILED'), '', last_error),
		updated_at = IF(status IN ('SENT', 'FAILED'), UTC_TIMESTAMP(6), updated_at),
		status = IF(status IN ('SENT', 'FAILED'), 'PENDING', status)`,
		orderID, "tickets:"+orderID, orderID, order.BuyerEmail, order.BuyerEmail); err != nil {
		h.internal(w, r, fmt.Errorf("queue ticket resend: %w", err))
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"message": "Email tiket dijadwalkan untuk dikirim ulang ke email pembeli."})
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	var extra any
	if err := decoder.Decode(target); err != nil || !errors.Is(decoder.Decode(&extra), io.EOF) {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "JSON request tidak valid")
		return false
	}
	return true
}

func tooMany(w http.ResponseWriter, wait int) {
	w.Header().Set("Retry-After", strconv.Itoa(wait))
	writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Terlalu banyak permintaan. Coba lagi nanti.")
}

func (h *Handler) internal(w http.ResponseWriter, r *http.Request, err error) {
	h.logger.ErrorContext(r.Context(), "ticket recovery error", "request_id", r.Header.Get("X-Request-ID"), "error", err)
	writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Terjadi kesalahan pada server")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
