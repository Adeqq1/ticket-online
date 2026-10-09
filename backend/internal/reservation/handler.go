package reservation

import (
	"encoding/json"
	"errors"
	"github.com/Adeqq1/ticket-online/backend/internal/conversion"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Adeqq1/ticket-online/backend/internal/eventstate"
)

type Reservation struct {
	ID        string         `json:"id"`
	Status    string         `json:"status"`
	ExpiresAt string         `json:"expiresAt"`
	Event     EventSummary   `json:"event"`
	Items     []ResponseItem `json:"items"`
	Subtotal  uint64         `json:"subtotal"`
	Reference string         `json:"reference,omitempty"`
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
	conversion *conversion.Service
}

func NewHandler(repository *Repository, logger *slog.Logger, metrics ...*conversion.Service) *Handler {
	h := &Handler{repository: repository, logger: logger}
	if len(metrics) > 0 {
		h.conversion = metrics[0]
	}
	return h
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
		if h.conversion != nil {
			journey := r.Header.Get("X-Conversion-Journey")
			if errors.Is(err, ErrInsufficientStock) {
				h.conversion.Record(r.Context(), journey, "STOCK_UNAVAILABLE", "RESERVATION")
			}
			if errors.Is(err, ErrReservationExpired) {
				h.conversion.Record(r.Context(), journey, "RESERVATION_EXPIRED", "RESERVATION")
			}
		}
		h.respondError(w, r, err)
		return
	}
	if h.conversion != nil {
		h.conversion.Link(r.Context(), r.Header.Get("X-Conversion-Journey"), request.EventID, value.ID)
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
func (h *Handler) Convert(w http.ResponseWriter, r *http.Request) {
	value, err := h.repository.Convert(r.Context(), r.PathValue("reservationID"))
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, eventstate.ErrClosed):
		writeError(w, 409, "EVENT_CHANGED", eventstate.ErrClosed.Error())
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
	case errors.Is(err, ErrReservationCancelled):
		writeError(w, http.StatusConflict, "RESERVATION_CANCELLED", "Reservasi sudah dibatalkan")
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
