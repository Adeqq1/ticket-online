package checkout

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/conversion"
	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
)

type Handler struct {
	repository *Repository
	logger     *slog.Logger
	access     *orderaccess.Access
	conversion *conversion.Service
}

func NewHandler(repository *Repository, logger *slog.Logger, metrics ...*conversion.Service) *Handler {
	h := &Handler{repository: repository, logger: logger}
	if len(metrics) > 0 {
		h.conversion = metrics[0]
	}
	return h
}

func NewHandlerWithAccess(repository *Repository, logger *slog.Logger, access *orderaccess.Access, metrics ...*conversion.Service) *Handler {
	h := &Handler{repository: repository, logger: logger, access: access}
	if len(metrics) > 0 {
		h.conversion = metrics[0]
	}
	return h
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 100 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Idempotency-Key tidak valid")
		return
	}
	reservationID := strings.ToLower(r.PathValue("reservationID"))
	if len(reservationID) != 32 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "ID reservasi tidak valid")
		return
	}
	if _, err := hex.DecodeString(reservationID); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "ID reservasi tidak valid")
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	var request Request
	if err := decoder.Decode(&request); err != nil {
		if h.conversion != nil {
			h.conversion.Record(r.Context(), r.Header.Get("X-Conversion-Journey"), "VALIDATION_FAILED", "BUYER_DATA")
		}
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "JSON request tidak valid")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if h.conversion != nil {
			h.conversion.Record(r.Context(), r.Header.Get("X-Conversion-Journey"), "VALIDATION_FAILED", "BUYER_DATA")
		}
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "JSON request tidak valid")
		return
	}
	order, replay, err := h.repository.Create(r.Context(), reservationID, idempotencyKey, request)
	if err != nil {
		if h.conversion != nil && errors.Is(err, ErrInvalidRequest) {
			h.conversion.Record(r.Context(), r.Header.Get("X-Conversion-Journey"), "VALIDATION_FAILED", "BUYER_DATA")
		}
		h.respondError(w, r, err)
		return
	}
	if h.access != nil {
		var expiresAt time.Time
		order.AccessToken, expiresAt, err = h.access.Issue(r.Context(), order.ID)
		if err != nil {
			h.respondError(w, r, err)
			return
		}
		order.AccessExpiresAt = expiresAt.Format(time.RFC3339)
	}
	status := http.StatusCreated
	if replay {
		status = http.StatusOK
	}
	writeJSON(w, status, order)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	orderID := strings.ToLower(r.PathValue("orderID"))
	if !validID(orderID) {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "ID order tidak valid")
		return
	}
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		writeError(w, http.StatusUnauthorized, "ACCESS_TOKEN_REQUIRED", "Token akses diperlukan")
		return
	}
	expiresAt, err := h.access.AuthorizeOrder(r.Context(), orderID, token)
	if err != nil {
		h.respondAccessError(w, r, err)
		return
	}
	detail, err := h.repository.Get(r.Context(), orderID)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	detail.AccessExpiresAt = expiresAt.Format(time.RFC3339)
	writeJSON(w, http.StatusOK, detail)
}

func validID(value string) bool {
	if len(value) != 32 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func (h *Handler) respondAccessError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, orderaccess.ErrUnauthorized):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Data tidak ditemukan")
	case errors.Is(err, orderaccess.ErrExpired):
		writeError(w, http.StatusGone, "ACCESS_TOKEN_EXPIRED", "Token akses sudah kedaluwarsa")
	case errors.Is(err, orderaccess.ErrNotFound), errors.Is(err, ErrOrderNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Data tidak ditemukan")
	default:
		h.logger.ErrorContext(r.Context(), "order access database error", "request_id", r.Header.Get("X-Request-ID"), "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Terjadi kesalahan pada server")
	}
}

func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "Data checkout tidak valid")
	case errors.Is(err, ErrInvalidVoucher):
		writeError(w, http.StatusUnprocessableEntity, "INVALID_VOUCHER", "Kode voucher tidak valid")
	case errors.Is(err, ErrReservationNotFound):
		writeError(w, http.StatusNotFound, "RESERVATION_NOT_FOUND", "Reservasi tidak ditemukan")
	case errors.Is(err, ErrReservationExpired):
		writeError(w, http.StatusGone, "RESERVATION_EXPIRED", "Reservasi sudah kedaluwarsa")
	case errors.Is(err, ErrReservationCancelled):
		writeError(w, http.StatusConflict, "RESERVATION_CANCELLED", "Reservasi sudah dibatalkan")
	case errors.Is(err, ErrReservationConverted):
		writeError(w, http.StatusConflict, "RESERVATION_CONVERTED", "Reservasi sudah dikonversi")
	case errors.Is(err, ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "CHECKOUT_CONFLICT", "Reservasi sudah dipesan dengan data checkout berbeda")
	case errors.Is(err, ErrReservationAccessDenied):
		writeError(w, http.StatusUnauthorized, "RESERVATION_ACCESS_DENIED", "Akses reservasi tidak valid")
	case errors.Is(err, orderaccess.ErrNotFound):
		writeError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "Order tidak ditemukan")
	default:
		h.logger.ErrorContext(r.Context(), "checkout database error", "request_id", r.Header.Get("X-Request-ID"), "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Terjadi kesalahan pada server")
	}
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
