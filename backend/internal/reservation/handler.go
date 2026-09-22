package reservation

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type Reservation struct {
	ID        string         `json:"id"`
	Status    string         `json:"status"`
	ExpiresAt string         `json:"expiresAt"`
	Event     EventSummary   `json:"event"`
	Items     []ResponseItem `json:"items"`
	Subtotal  uint64         `json:"subtotal"`
}
type EventSummary struct {
	ID     string `json:"id"`
	Artist string `json:"artist"`
}
type ResponseItem struct {
	TierID    string `json:"tierId"`
	Name      string `json:"name"`
	Quantity  uint64 `json:"quantity"`
	UnitPrice uint64 `json:"unitPrice"`
	LineTotal uint64 `json:"lineTotal"`
}
type Handler struct {
	repository *Repository
	logger     *slog.Logger
}

func NewHandler(repository *Repository, logger *slog.Logger) *Handler {
	return &Handler{repository: repository, logger: logger}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(key) < 16 || len(key) > 100 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Idempotency-Key tidak valid")
		return
	}
	var request Request
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "JSON request tidak valid")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "JSON request tidak valid")
		return
	}
	request = NormalizeRequest(request)
	if err := ValidateRequest(request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Data reservasi tidak valid")
		return
	}
	value, err := h.repository.Create(r.Context(), request, key)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	value, err := h.repository.Get(r.Context(), r.PathValue("reservationID"))
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	value, err := h.repository.Cancel(r.Context(), r.PathValue("reservationID"))
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrEventNotFound):
		writeError(w, http.StatusNotFound, "EVENT_NOT_FOUND", "Event tidak ditemukan")
	case errors.Is(err, ErrTierNotFound):
		writeError(w, http.StatusNotFound, "TIER_NOT_FOUND", "Tier tiket tidak ditemukan")
	case errors.Is(err, ErrInsufficientStock):
		writeError(w, http.StatusConflict, "INSUFFICIENT_STOCK", "Stok tiket tidak mencukupi")
	case errors.Is(err, ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "Idempotency-Key sudah digunakan untuk request berbeda")
	case errors.Is(err, ErrOrderLimitExceeded):
		writeError(w, http.StatusUnprocessableEntity, "ORDER_LIMIT_EXCEEDED", "Jumlah tiket melebihi batas pemesanan")
	case errors.Is(err, ErrReservationNotFound):
		writeError(w, http.StatusNotFound, "RESERVATION_NOT_FOUND", "Reservasi tidak ditemukan")
	case errors.Is(err, ErrReservationExpired):
		writeError(w, http.StatusGone, "RESERVATION_EXPIRED", "Reservasi sudah kedaluwarsa")
	case errors.Is(err, ErrReservationConverted):
		writeError(w, http.StatusConflict, "RESERVATION_CONVERTED", "Reservasi sudah dikonversi")
	default:
		h.logger.ErrorContext(r.Context(), "reservation database error", "request_id", r.Header.Get("X-Request-ID"), "error", err)
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
