package checkin

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/Adeqq1/ticket-online/backend/internal/eventstate"
	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) CheckIn(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Autentikasi petugas diperlukan")
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
	result, err := h.service.CheckIn(r.Context(), token, request)
	if err != nil {
		h.respondError(w, r, err, result)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) TicketStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Autentikasi petugas diperlukan")
		return
	}
	result, err := h.service.GetTicketStatus(r.Context(), token, r.URL.Query().Get("code"))
	if err != nil {
		h.respondError(w, r, err, Result{})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Autentikasi administrator diperlukan")
		return
	}
	query := r.URL.Query()
	for name, values := range query {
		if (name != "eventId" && name != "gate" && name != "q" && name != "beforeId") || len(values) != 1 {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Filter riwayat check-in tidak valid")
			return
		}
	}
	result, err := h.service.ListHistory(r.Context(), token, HistoryFilter{
		EventID: query.Get("eventId"), Gate: query.Get("gate"), Query: query.Get("q"), BeforeID: query.Get("beforeId"),
	})
	if err != nil {
		if errors.Is(err, ErrInvalidRequest) {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Filter riwayat check-in tidak valid")
		} else if errors.Is(err, staffauth.ErrUnauthorized) {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Sesi administrator tidak valid atau sudah berakhir")
		} else if errors.Is(err, staffauth.ErrForbidden) {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "Endpoint ini hanya dapat diakses administrator")
		} else {
			h.logger.ErrorContext(r.Context(), "list check-in history failed", "request_id", r.Header.Get("X-Request-ID"), "error", err)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Terjadi kesalahan pada server")
		}
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, err error, result Result) {
	switch {
	case errors.Is(err, eventstate.ErrClosed):
		writeError(w, 409, "EVENT_CHANGED", eventstate.ErrClosed.Error())
	case errors.Is(err, ErrInvalidRequest):
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Kode atau data check-in tidak valid")
	case errors.Is(err, staffauth.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Sesi petugas tidak valid atau sudah berakhir")
	case errors.Is(err, staffauth.ErrForbidden):
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Akses petugas ke event atau gate ini ditolak")
	case errors.Is(err, ErrTicketNotFound):
		writeError(w, http.StatusNotFound, "TICKET_NOT_FOUND", "Tiket tidak ditemukan")
	case errors.Is(err, ErrWrongGate):
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]string{"code": "WRONG_GATE", "message": "Tiket berlaku di gate lain", "expectedGate": result.Ticket.Gate}})
	case errors.Is(err, ErrOrderNotPaid):
		writeError(w, http.StatusConflict, "ORDER_NOT_PAID", "Order tiket belum dibayar")
	case errors.Is(err, ErrTicketUsed):
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":       map[string]string{"code": "TICKET_ALREADY_USED", "message": "Tiket sudah digunakan"},
			"status":      result.Status,
			"ticket":      result.Ticket,
			"checkedInAt": result.CheckedInAt,
		})
	default:
		h.logger.ErrorContext(r.Context(), "ticket check-in failed", "request_id", r.Header.Get("X-Request-ID"), "error", err)
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
