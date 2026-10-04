package checkout

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type Handler struct {
	repository *Repository
	logger     *slog.Logger
}

func NewHandler(repository *Repository, logger *slog.Logger) *Handler {
	return &Handler{repository: repository, logger: logger}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
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
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "JSON request tidak valid")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "JSON request tidak valid")
		return
	}
	order, replay, err := h.repository.Create(r.Context(), reservationID, request)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	status := http.StatusCreated
	if replay {
		status = http.StatusOK
	}
	writeJSON(w, status, order)
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
