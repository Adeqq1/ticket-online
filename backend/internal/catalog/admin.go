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
	ID                string   `json:"id"`
	Artist            string   `json:"artist"`
	City              string   `json:"city"`
	Venue             string   `json:"venue"`
	Address           string   `json:"address"`
	StartsAt          string   `json:"startsAt"`
	Genre             string   `json:"genre"`
	Status            string   `json:"status"`
	PublicationStatus string   `json:"publicationStatus"`
	Image             string   `json:"image"`
	Description       string   `json:"description"`
	Lineup            []string `json:"lineup"`
	startsAt          *time.Time
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
	mux.HandleFunc("POST /api/v1/admin/events/{eventID}/zones", h.createZone)
	mux.HandleFunc("PUT /api/v1/admin/events/{eventID}/zones/{zoneID}", h.updateZone)
	mux.HandleFunc("POST /api/v1/admin/events/{eventID}/ticket-tiers", h.createTier)
	mux.HandleFunc("PUT /api/v1/admin/events/{eventID}/ticket-tiers/{tierID}", h.updateTier)
}

func (h *AdminHandler) authorize(w http.ResponseWriter, r *http.Request) (staffauth.Principal, bool) {
	w.Header().Set("Cache-Control", "no-store")
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		adminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Autentikasi petugas diperlukan")
		return staffauth.Principal{}, false
	}
	principal, err := h.staff.Authenticate(r.Context(), token)
	if errors.Is(err, staffauth.ErrUnauthorized) {
		adminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Sesi petugas tidak valid atau sudah berakhir")
		return staffauth.Principal{}, false
	}
	if err != nil {
		h.internalError(w, r, err)
		return staffauth.Principal{}, false
	}
	if principal.Role != "ADMIN" {
		adminError(w, http.StatusForbidden, "FORBIDDEN", "Akses administrator diperlukan")
		return staffauth.Principal{}, false
	}
	return principal, true
}

func (h *AdminHandler) list(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r); !ok {
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
	principal, ok := h.authorize(w, r)
	if !ok {
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
	if err := h.repository.CreateEvent(r.Context(), input.repositoryInput(), AdminActor{ID: principal.ID, Name: principal.Name}); err != nil {
		h.respondError(w, r, err)
		return
	}
	event, err := h.repository.adminDetail(r.Context(), input.ID)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	adminJSON(w, http.StatusCreated, event)
}

func (h *AdminHandler) update(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.authorize(w, r)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("eventID"))
	if len(id) > 64 || !eventSlug.MatchString(id) {
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
	if err := h.repository.UpdateEvent(r.Context(), id, input.repositoryInput(), AdminActor{ID: principal.ID, Name: principal.Name}); err != nil {
		h.respondError(w, r, err)
		return
	}
	event, err := h.repository.adminDetail(r.Context(), id)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	adminJSON(w, http.StatusOK, event)
}

type adminZoneInput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
type adminTierInput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ZoneID      string `json:"zoneId"`
	Price       uint64 `json:"price"`
	Capacity    uint64 `json:"capacity"`
	MaxPerOrder uint64 `json:"maxPerOrder"`
	Benefit     string `json:"benefit"`
	Gate        string `json:"gate"`
	Seating     string `json:"seating"`
}

func (h *AdminHandler) zone(w http.ResponseWriter, r *http.Request, create bool) {
	principal, ok := h.authorize(w, r)
	if !ok {
		return
	}
	eventID := r.PathValue("eventID")
	if len(eventID) > 64 || !eventSlug.MatchString(eventID) {
		adminError(w, 400, "INVALID_REQUEST", "ID konser tidak valid")
		return
	}
	var in adminZoneInput
	if !decodeAdmin(w, r, &in) {
		return
	}
	if create {
		in.ID = strings.ToLower(strings.TrimSpace(in.ID))
	} else {
		in.ID = r.PathValue("zoneID")
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	if !eventSlug.MatchString(in.ID) || len(in.ID) > 64 || utf8.RuneCountInString(in.Name) < 1 || utf8.RuneCountInString(in.Name) > 100 || utf8.RuneCountInString(in.Description) > 255 {
		adminError(w, 422, "INVALID_REQUEST", "Data zona tidak valid")
		return
	}
	if err := h.repository.SaveZone(r.Context(), eventID, ZoneInput{ID: in.ID, Name: in.Name, Description: in.Description}, create, AdminActor{ID: principal.ID, Name: principal.Name}); err != nil {
		h.respondError(w, r, err)
		return
	}
	adminJSON(w, map[bool]int{true: 201, false: 200}[create], map[string]any{"id": in.ID, "name": in.Name, "description": in.Description})
}
func (h *AdminHandler) createZone(w http.ResponseWriter, r *http.Request) { h.zone(w, r, true) }
func (h *AdminHandler) updateZone(w http.ResponseWriter, r *http.Request) { h.zone(w, r, false) }

func (h *AdminHandler) tier(w http.ResponseWriter, r *http.Request, create bool) {
	principal, ok := h.authorize(w, r)
	if !ok {
		return
	}
	eventID := r.PathValue("eventID")
	if len(eventID) > 64 || !eventSlug.MatchString(eventID) {
		adminError(w, 400, "INVALID_REQUEST", "ID konser tidak valid")
		return
	}
	var in adminTierInput
	if !decodeAdmin(w, r, &in) {
		return
	}
	if create {
		in.ID = strings.ToLower(strings.TrimSpace(in.ID))
	} else {
		in.ID = r.PathValue("tierID")
	}
	for _, p := range []*string{&in.Name, &in.ZoneID, &in.Benefit, &in.Gate, &in.Seating} {
		*p = strings.TrimSpace(*p)
	}
	if !eventSlug.MatchString(in.ID) || len(in.ID) > 64 || utf8.RuneCountInString(in.Name) < 1 || utf8.RuneCountInString(in.Name) > 100 || !eventSlug.MatchString(in.ZoneID) || in.Price == 0 || in.Price > 9007199254740991 || in.Capacity > 4294967295 || in.MaxPerOrder == 0 || in.MaxPerOrder > 4294967295 || utf8.RuneCountInString(in.Benefit) > 255 || utf8.RuneCountInString(in.Gate) < 1 || utf8.RuneCountInString(in.Gate) > 100 || (in.Seating != "assigned" && in.Seating != "free-standing") {
		adminError(w, 422, "INVALID_REQUEST", "Data kategori tiket tidak valid")
		return
	}
	seating := map[string]string{"assigned": "ASSIGNED", "free-standing": "FREE_STANDING"}[in.Seating]
	err := h.repository.SaveTier(r.Context(), eventID, TierInput{ID: in.ID, Name: in.Name, ZoneID: in.ZoneID, Price: in.Price, Capacity: in.Capacity, MaxPerOrder: in.MaxPerOrder, Benefit: in.Benefit, Gate: in.Gate, Seating: seating}, create, AdminActor{ID: principal.ID, Name: principal.Name})
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	event, err := h.repository.adminDetail(r.Context(), eventID)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	for _, tier := range event.TicketTiers {
		if tier.ID == in.ID {
			adminJSON(w, map[bool]int{true: 201, false: 200}[create], tier)
			return
		}
	}
}
func (h *AdminHandler) createTier(w http.ResponseWriter, r *http.Request) { h.tier(w, r, true) }
func (h *AdminHandler) updateTier(w http.ResponseWriter, r *http.Request) { h.tier(w, r, false) }

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
	if in.Genre == "" {
		in.Genre = "Pop"
	}
	if in.Status == "" {
		in.Status = "Presale"
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
	if utf8.RuneCountInString(in.Artist) < 2 || utf8.RuneCountInString(in.Artist) > 160 || utf8.RuneCountInString(in.City) > 100 || utf8.RuneCountInString(in.Venue) > 160 || utf8.RuneCountInString(in.Address) > 255 || utf8.RuneCountInString(in.Description) > 16000 || len(in.Lineup) > 100 {
		return false
	}
	if in.StartsAt != "" {
		value, err := time.Parse(time.RFC3339, in.StartsAt)
		if err != nil {
			return false
		}
		parsed := value.UTC()
		in.startsAt = &parsed
	}
	if in.Image != "" {
		poster, err := url.ParseRequestURI(in.Image)
		if err != nil || (poster.Scheme != "http" && poster.Scheme != "https") || poster.Host == "" || poster.User != nil {
			return false
		}
	}
	if len(in.Image) > 500 {
		return false
	}
	for i := range in.Lineup {
		in.Lineup[i] = strings.TrimSpace(in.Lineup[i])
		if in.Lineup[i] == "" || utf8.RuneCountInString(in.Lineup[i]) > 160 {
			return false
		}
	}
	return true
}

func (in adminEventInput) repositoryInput() EventInput {
	status := map[string]string{"Early Bird": "EARLY_BIRD", "Presale": "PRESALE", "Sold Out": "SOLD_OUT"}[in.Status]
	return EventInput{ID: in.ID, Artist: in.Artist, City: in.City, Venue: in.Venue, Address: in.Address, StartsAt: in.startsAt, Genre: in.Genre, Status: status, PublicationStatus: in.PublicationStatus, Image: in.Image, Description: in.Description, Lineup: in.Lineup}
}

func (h *AdminHandler) respondError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrEventNotFound):
		adminError(w, http.StatusNotFound, "EVENT_NOT_FOUND", "Event tidak ditemukan")
	case errors.Is(err, ErrDuplicateEvent):
		adminError(w, http.StatusConflict, "DUPLICATE_EVENT", "ID konser sudah digunakan")
	case errors.Is(err, ErrScheduleLocked):
		adminError(w, http.StatusConflict, "SCHEDULE_LOCKED", "Jadwal terkunci karena konser sudah memiliki pesanan")
	case errors.Is(err, ErrCapacityBelowBound):
		adminError(w, http.StatusConflict, "CAPACITY_BELOW_BOUND", "Kapasitas tidak boleh di bawah tiket terjual atau tertahan")
	case errors.Is(err, ErrGateLocked):
		adminError(w, http.StatusConflict, "GATE_LOCKED", "Gate terkunci setelah reservasi pertama")
	case errors.Is(err, ErrPublicationIncomplete):
		adminError(w, http.StatusUnprocessableEntity, "PUBLICATION_INCOMPLETE", "Lengkapi informasi konser, lineup, zona, dan kategori tiket sebelum publikasi")
	case errors.Is(err, ErrInvalidZone):
		adminError(w, http.StatusUnprocessableEntity, "INVALID_ZONE", "Zona kategori tidak ditemukan untuk konser ini")
	case errors.Is(err, ErrEventHasReservations):
		adminError(w, http.StatusConflict, "EVENT_HAS_RESERVATIONS", "Informasi jadwal dan lokasi tidak dapat dikosongkan setelah reservasi dibuat")
	case errors.Is(err, ErrLocationLocked):
		adminError(w, http.StatusConflict, "LOCATION_LOCKED", "Lokasi tidak dapat diubah setelah reservasi pertama")
	case errors.Is(err, ErrDuplicateTier):
		adminError(w, http.StatusConflict, "DUPLICATE_TIER", "ID kategori tiket sudah digunakan")
	case errors.Is(err, ErrDuplicateZone):
		adminError(w, http.StatusConflict, "DUPLICATE_ZONE", "ID zona sudah digunakan")
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
