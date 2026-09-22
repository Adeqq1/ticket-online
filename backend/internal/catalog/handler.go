package catalog

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

type Event struct {
	ID          string       `json:"id"`
	Artist      string       `json:"artist"`
	City        string       `json:"city"`
	Venue       string       `json:"venue"`
	Address     string       `json:"address"`
	StartsAt    string       `json:"startsAt"`
	Genre       string       `json:"genre"`
	Status      string       `json:"status"`
	Image       string       `json:"image"`
	Description string       `json:"description"`
	Lineup      []string     `json:"lineup"`
	Price       uint64       `json:"price"`
	Zones       []Zone       `json:"zones"`
	TicketTiers []TicketTier `json:"ticketTiers"`
}

type Zone struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type TicketTier struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	ZoneID            string `json:"zoneId"`
	Price             uint64 `json:"price"`
	AvailableQuantity uint64 `json:"availableQuantity"`
	MaxPerOrder       uint64 `json:"maxPerOrder"`
	Benefit           string `json:"benefit"`
	Gate              string `json:"gate"`
	Seating           string `json:"seating"`
}

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	events, err := h.service.ListEvents(r.Context())
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("eventID"))
	if id == "" {
		writeError(w, http.StatusNotFound, "EVENT_NOT_FOUND", "Event tidak ditemukan")
		return
	}
	event, err := h.service.GetEvent(r.Context(), id)
	if errors.Is(err, ErrEventNotFound) {
		writeError(w, http.StatusNotFound, "EVENT_NOT_FOUND", "Event tidak ditemukan")
		return
	}
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, event)
}

func (h *Handler) internalError(w http.ResponseWriter, r *http.Request, err error) {
	h.logger.ErrorContext(r.Context(), "catalog database error", "request_id", r.Header.Get("X-Request-ID"), "error", err)
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
