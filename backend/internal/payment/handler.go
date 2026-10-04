package payment

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
)

type Handler struct {
	repository *Repository
	logger     *slog.Logger
	access     *orderaccess.Access
}

func NewHandler(repository *Repository, logger *slog.Logger) *Handler {
	return &Handler{repository: repository, logger: logger}
}

func NewHandlerWithAccess(repository *Repository, logger *slog.Logger, access *orderaccess.Access) *Handler {
	return &Handler{repository: repository, logger: logger, access: access}
}

func (h *Handler) Simulate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	orderID := strings.ToLower(r.PathValue("orderID"))
	if !validID(orderID) {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "ID order tidak valid")
		return
	}
	if err := h.authorizeOrder(w, r, orderID); err != nil {
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
	payment, reused, err := h.repository.Simulate(r.Context(), orderID, request)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	status := http.StatusCreated
	if reused {
		status = http.StatusOK
	}
	writeJSON(w, status, payment)
}

func (h *Handler) TicketsForOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	orderID := strings.ToLower(r.PathValue("orderID"))
	if !validID(orderID) {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "ID order tidak valid")
		return
	}
	if err := h.authorizeOrder(w, r, orderID); err != nil {
		return
	}
	tickets, err := h.repository.TicketsForOrder(r.Context(), orderID)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tickets": tickets})
}

func (h *Handler) GetTicket(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	ticketID := strings.ToLower(r.PathValue("ticketID"))
	if !validID(ticketID) {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "ID e-ticket tidak valid")
		return
	}
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		writeError(w, http.StatusUnauthorized, "ACCESS_TOKEN_REQUIRED", "Token akses diperlukan")
		return
	}
	if _, err := h.access.AuthorizeTicket(r.Context(), ticketID, token); err != nil {
		h.respondAccessError(w, r, err)
		return
	}
	ticket, err := h.repository.Ticket(r.Context(), ticketID)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ticket)
}

func (h *Handler) authorizeOrder(w http.ResponseWriter, r *http.Request, orderID string) error {
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		writeError(w, http.StatusUnauthorized, "ACCESS_TOKEN_REQUIRED", "Token akses diperlukan")
		return orderaccess.ErrUnauthorized
	}
	if _, err := h.access.AuthorizeOrder(r.Context(), orderID, token); err != nil {
		h.respondAccessError(w, r, err)
		return err
	}
	return nil
}

func (h *Handler) respondAccessError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, orderaccess.ErrUnauthorized):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Data tidak ditemukan")
	case errors.Is(err, orderaccess.ErrExpired):
		writeError(w, http.StatusGone, "ACCESS_TOKEN_EXPIRED", "Token akses sudah kedaluwarsa")
	case errors.Is(err, orderaccess.ErrNotFound), errors.Is(err, ErrOrderNotFound), errors.Is(err, ErrTicketNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Data tidak ditemukan")
	default:
		h.logger.ErrorContext(r.Context(), "ticket access database error", "request_id", r.Header.Get("X-Request-ID"), "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Terjadi kesalahan pada server")
	}
}

func validID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "Data pembayaran tidak valid")
	case errors.Is(err, ErrOrderNotFound):
		writeError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "Order tidak ditemukan")
	case errors.Is(err, ErrOrderNotPayable):
		writeError(w, http.StatusConflict, "ORDER_NOT_PAYABLE", "Order tidak dapat dibayar")
	case errors.Is(err, ErrPaymentConflict):
		writeError(w, http.StatusConflict, "PAYMENT_CONFLICT", "Pembayaran berhasil dan tidak dapat diubah")
	case errors.Is(err, ErrTicketNotFound):
		writeError(w, http.StatusNotFound, "TICKET_NOT_FOUND", "E-ticket tidak ditemukan")
	case errors.Is(err, ErrIncompleteTickets):
		h.logger.ErrorContext(r.Context(), "incomplete e-ticket set", "request_id", r.Header.Get("X-Request-ID"), "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Terjadi kesalahan pada server")
	default:
		h.logger.ErrorContext(r.Context(), "payment database error", "request_id", r.Header.Get("X-Request-ID"), "error", err)
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
