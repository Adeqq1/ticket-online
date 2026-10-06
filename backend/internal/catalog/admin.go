package catalog

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
)

type adminEventInput struct {
	ID                string    `json:"id"`
	Artist            string    `json:"artist"`
	City              string    `json:"city"`
	Venue             string    `json:"venue"`
	Address           string    `json:"address"`
	StartsAt          time.Time `json:"startsAt"`
	Genre             string    `json:"genre"`
	Status            string    `json:"status"`
	PublicationStatus string    `json:"publicationStatus"`
	Image             string    `json:"image"`
	Description       string    `json:"description"`
	Lineup            []string  `json:"lineup"`
}

type AdminHandler struct {
	repository *Repository
	staff      *staffauth.Service
	logger     *slog.Logger
}

func NewAdminHandler(repository *Repository, staff *staffauth.Service, logger *slog.Logger) *AdminHandler {
	return &AdminHandler{repository: repository, staff: staff, logger: logger}
}

var eventSlug = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func (h *AdminHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/events", h.list)
	mux.HandleFunc("POST /api/v1/admin/events", h.create)
	mux.HandleFunc("PUT /api/v1/admin/events/{eventID}", h.update)
}

func (h *AdminHandler) authorize(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		adminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Autentikasi petugas diperlukan")
		return false
	}
	principal, err := h.staff.Authenticate(r.Context(), token)
	if errors.Is(err, staffauth.ErrUnauthorized) {
		adminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Sesi petugas tidak valid atau sudah berakhir")
		return false
	}
	if err != nil {
		h.internalError(w, r, err)
		return false
	}
	if principal.Role != "ADMIN" {
		adminError(w, http.StatusForbidden, "FORBIDDEN", "Akses administrator diperlukan")
		return false
	}
	return true
}

func (h *AdminHandler) list(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	events, err := h.repository.ListAdminEvents(r.Context())
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	adminJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (h *AdminHandler) create(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	var input adminEventInput
	if !decodeAdmin(w, r, &input) {
		return
	}
	if !input.valid(true) {
		adminError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "Data konser tidak valid")
		return
	}
	if err := h.repository.CreateEvent(r.Context(), input.repositoryInput()); err != nil {
		h.respondError(w, r, err)
		return
	}
	event, err := h.repository.adminEvent(r.Context(), input.ID)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	adminJSON(w, http.StatusCreated, event)
}

func (h *AdminHandler) update(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	id := strings.TrimSpace(r.PathValue("eventID"))
	if !eventSlug.MatchString(id) {
		adminError(w, http.StatusBadRequest, "INVALID_REQUEST", "ID konser tidak valid")
		return
	}
	var input adminEventInput
	if !decodeAdmin(w, r, &input) {
		return
	}
	input.ID = id
	if !input.valid(false) {
		adminError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "Data konser tidak valid")
		return
	}
	if err := h.repository.UpdateEvent(r.Context(), id, input.repositoryInput()); err != nil {
		h.respondError(w, r, err)
		return
	}
	event, err := h.repository.adminEvent(r.Context(), id)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	adminJSON(w, http.StatusOK, event)
}

func (in *adminEventInput) valid(requireID bool) bool {
	in.ID = strings.ToLower(strings.TrimSpace(in.ID))
	for _, value := range []*string{&in.Artist, &in.City, &in.Venue, &in.Address, &in.Genre, &in.Status, &in.Image, &in.Description, &in.PublicationStatus} {
		*value = strings.TrimSpace(*value)
	}
	if in.PublicationStatus == "" {
		in.PublicationStatus = "DRAFT"
	}
	if requireID && (len(in.ID) > 64 || !eventSlug.MatchString(in.ID)) {
		return false
	}
	if in.Genre != "Rock" && in.Genre != "Pop" && in.Genre != "Indie" {
		return false
	}
	if in.Status != "Early Bird" && in.Status != "Presale" && in.Status != "Sold Out" {
		return false
	}
	if in.PublicationStatus != "DRAFT" && in.PublicationStatus != "PUBLISHED" && in.PublicationStatus != "ARCHIVED" {
		return false
	}
	if utf8.RuneCountInString(in.Artist) < 2 || utf8.RuneCountInString(in.Artist) > 160 || utf8.RuneCountInString(in.City) < 2 || utf8.RuneCountInString(in.City) > 100 || utf8.RuneCountInString(in.Venue) < 2 || utf8.RuneCountInString(in.Venue) > 160 || utf8.RuneCountInString(in.Address) < 2 || utf8.RuneCountInString(in.Address) > 255 || utf8.RuneCountInString(in.Description) > 16000 || len(in.Lineup) > 100 {
		return false
	}
	if in.StartsAt.IsZero() {
		return false
	}
	poster, err := url.ParseRequestURI(in.Image)
	if err != nil || (poster.Scheme != "http" && poster.Scheme != "https") || poster.Host == "" || poster.User != nil || len(in.Image) > 500 {
		return false
	}
	for i := range in.Lineup {
		in.Lineup[i] = strings.TrimSpace(in.Lineup[i])
		if in.Lineup[i] == "" || utf8.RuneCountInString(in.Lineup[i]) > 160 {
			return false
		}
	}
	if len(in.Lineup) == 0 {
		in.Lineup = []string{in.Artist}
	}
	return true
}

func (in adminEventInput) repositoryInput() EventInput {
	status := map[string]string{"Early Bird": "EARLY_BIRD", "Presale": "PRESALE", "Sold Out": "SOLD_OUT"}[in.Status]
	return EventInput{ID: in.ID, Artist: in.Artist, City: in.City, Venue: in.Venue, Address: in.Address, StartsAt: in.StartsAt.UTC(), Genre: in.Genre, Status: status, PublicationStatus: in.PublicationStatus, Image: in.Image, Description: in.Description, Lineup: in.Lineup}
}

func (h *AdminHandler) respondError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrEventNotFound):
		adminError(w, http.StatusNotFound, "EVENT_NOT_FOUND", "Event tidak ditemukan")
	case errors.Is(err, ErrDuplicateEvent):
		adminError(w, http.StatusConflict, "DUPLICATE_EVENT", "ID konser sudah digunakan")
	case errors.Is(err, ErrScheduleLocked):
		adminError(w, http.StatusConflict, "SCHEDULE_LOCKED", "Jadwal terkunci karena konser sudah memiliki pesanan")
	default:
		h.internalError(w, r, err)
	}
}

func (h *AdminHandler) internalError(w http.ResponseWriter, r *http.Request, err error) {
	h.logger.ErrorContext(r.Context(), "admin event API error", "request_id", r.Header.Get("X-Request-ID"), "error", err)
	adminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Terjadi kesalahan pada server")
}

func decodeAdmin(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		adminError(w, http.StatusBadRequest, "INVALID_JSON", "JSON request tidak valid")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		adminError(w, http.StatusBadRequest, "INVALID_JSON", "JSON request tidak valid")
		return false
	}
	return true
}

func adminJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func adminError(w http.ResponseWriter, status int, code, message string) {
	adminJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
