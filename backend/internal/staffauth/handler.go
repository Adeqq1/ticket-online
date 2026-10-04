package staffauth

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
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/staff/login", h.Login)
	mux.HandleFunc("GET /api/v1/staff/me", h.Me)
	mux.HandleFunc("POST /api/v1/staff/logout", h.Logout)
	mux.HandleFunc("GET /api/v1/admin/staff", h.ListStaff)
	mux.HandleFunc("POST /api/v1/admin/staff", h.CreateStaff)
	mux.HandleFunc("PATCH /api/v1/admin/staff/{staffID}", h.UpdateStaff)
	mux.HandleFunc("PUT /api/v1/admin/staff/{staffID}/assignments", h.ReplaceAssignments)
	mux.HandleFunc("PUT /api/v1/admin/staff/{staffID}/password", h.ResetPassword)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	private(w)
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &request) {
		return
	}
	if len(request.Email) > 254 || len(request.Password) > 128 {
		staffError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "Data login tidak valid")
		return
	}
	session, err := h.service.Login(r.Context(), request.Email, request.Password, r.RemoteAddr)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	staffJSON(w, http.StatusOK, session)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	private(w)
	principal, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	staff, err := h.service.Current(r.Context(), principal)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	staffJSON(w, http.StatusOK, map[string]any{"staff": staff})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	private(w)
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		staffError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Autentikasi petugas diperlukan")
		return
	}
	if err := h.service.Logout(r.Context(), token); err != nil {
		h.respondError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListStaff(w http.ResponseWriter, r *http.Request) {
	private(w)
	if _, ok := h.admin(w, r); !ok {
		return
	}
	staff, err := h.service.ListStaff(r.Context())
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	staffJSON(w, http.StatusOK, map[string]any{"staff": staff})
}

func (h *Handler) CreateStaff(w http.ResponseWriter, r *http.Request) {
	private(w)
	principal, ok := h.admin(w, r)
	if !ok {
		return
	}
	var request struct {
		Name        string       `json:"name"`
		Email       string       `json:"email"`
		Password    string       `json:"password"`
		Assignments []Assignment `json:"assignments"`
	}
	if !decode(w, r, &request) {
		return
	}
	staff, err := h.service.CreateStaff(r.Context(), request.Name, request.Email, request.Password, request.Assignments)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	h.audit(r, principal, staff.ID, "create_staff")
	staffJSON(w, http.StatusCreated, staff)
}

func (h *Handler) UpdateStaff(w http.ResponseWriter, r *http.Request) {
	private(w)
	principal, ok := h.admin(w, r)
	if !ok {
		return
	}
	staffID, ok := parseID(r.PathValue("staffID"))
	if !ok {
		staffError(w, http.StatusBadRequest, "INVALID_REQUEST", "ID petugas tidak valid")
		return
	}
	var request struct {
		Name   *string `json:"name"`
		Active *bool   `json:"active"`
	}
	if !decode(w, r, &request) {
		return
	}
	if err := h.service.UpdateStaff(r.Context(), staffID, request.Name, request.Active); err != nil {
		h.respondError(w, r, err)
		return
	}
	h.audit(r, principal, staffID, "update_staff")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ReplaceAssignments(w http.ResponseWriter, r *http.Request) {
	private(w)
	principal, ok := h.admin(w, r)
	if !ok {
		return
	}
	staffID, ok := parseID(r.PathValue("staffID"))
	if !ok {
		staffError(w, http.StatusBadRequest, "INVALID_REQUEST", "ID petugas tidak valid")
		return
	}
	var request struct {
		Assignments []Assignment `json:"assignments"`
	}
	if !decode(w, r, &request) {
		return
	}
	if request.Assignments == nil {
		staffError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "Penugasan harus berupa array")
		return
	}
	if err := h.service.ReplaceAssignments(r.Context(), staffID, request.Assignments); err != nil {
		h.respondError(w, r, err)
		return
	}
	h.audit(r, principal, staffID, "replace_staff_assignments")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	private(w)
	principal, ok := h.admin(w, r)
	if !ok {
		return
	}
	staffID, ok := parseID(r.PathValue("staffID"))
	if !ok {
		staffError(w, http.StatusBadRequest, "INVALID_REQUEST", "ID petugas tidak valid")
		return
	}
	var request struct {
		Password string `json:"password"`
	}
	if !decode(w, r, &request) {
		return
	}
	if err := h.service.ResetPassword(r.Context(), staffID, request.Password); err != nil {
		h.respondError(w, r, err)
		return
	}
	h.audit(r, principal, staffID, "reset_staff_password")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) authenticate(w http.ResponseWriter, r *http.Request) (Principal, bool) {
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		staffError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Autentikasi petugas diperlukan")
		return Principal{}, false
	}
	principal, err := h.service.Authenticate(r.Context(), token)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			staffError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Sesi petugas tidak valid atau sudah berakhir")
		} else {
			h.respondError(w, r, err)
		}
		return Principal{}, false
	}
	return principal, true
}

func (h *Handler) admin(w http.ResponseWriter, r *http.Request) (Principal, bool) {
	principal, ok := h.authenticate(w, r)
	if !ok {
		return Principal{}, false
	}
	if principal.Role != "ADMIN" {
		staffError(w, http.StatusForbidden, "FORBIDDEN", "Akses administrator diperlukan")
		return Principal{}, false
	}
	return principal, true
}

func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		staffError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", "Data petugas tidak valid")
	case errors.Is(err, ErrCredentials), errors.Is(err, ErrUnauthorized):
		staffError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Email atau password tidak valid")
	case errors.Is(err, ErrForbidden):
		staffError(w, http.StatusForbidden, "FORBIDDEN", "Akses administrator diperlukan")
	case errors.Is(err, ErrNotFound):
		staffError(w, http.StatusNotFound, "NOT_FOUND", "Petugas tidak ditemukan")
	case errors.Is(err, ErrDuplicateEmail):
		staffError(w, http.StatusConflict, "DUPLICATE_EMAIL", "Email sudah digunakan")
	case errors.Is(err, ErrAdminExists):
		staffError(w, http.StatusConflict, "ADMIN_EXISTS", "Administrator sudah tersedia")
	case errors.Is(err, ErrRateLimited):
		staffError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Terlalu banyak percobaan login")
	default:
		h.logger.ErrorContext(r.Context(), "staff API error", "request_id", r.Header.Get("X-Request-ID"), "error", err)
		staffError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Terjadi kesalahan pada server")
	}
}

func (h *Handler) audit(r *http.Request, actor Principal, targetID, action string) {
	h.logger.InfoContext(r.Context(), "staff admin action", "request_id", r.Header.Get("X-Request-ID"), "actor_id", actor.ID, "target_id", targetID, "action", action, "result", "success")
}

func private(w http.ResponseWriter) { w.Header().Set("Cache-Control", "no-store") }

func staffJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func staffError(w http.ResponseWriter, status int, code, message string) {
	staffJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		staffError(w, http.StatusBadRequest, "INVALID_JSON", "JSON request tidak valid")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		staffError(w, http.StatusBadRequest, "INVALID_JSON", "JSON request tidak valid")
		return false
	}
	return true
}

func parseID(value string) (string, bool) {
	value = strings.ToLower(value)
	if len(value) != 32 {
		return "", false
	}
	_, err := hex.DecodeString(value)
	return value, err == nil
}
