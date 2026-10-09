package adminissues

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/Adeqq1/ticket-online/backend/internal/payment"
	"github.com/Adeqq1/ticket-online/backend/internal/recovery"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
)

const pageSize = 50

var (
	ErrInvalidRequest      = errors.New("invalid admin issue request")
	ErrNotFound            = errors.New("admin issue not found")
	ErrConflict            = errors.New("admin issue state conflict")
	ErrRateLimited         = errors.New("admin issue action rate limited")
	ErrProviderUnavailable = errors.New("payment provider is not configured")
	ErrProviderFailure     = errors.New("payment provider check failed")
	caseIDPattern          = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)
	jobIDPattern           = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

type Service struct {
	db        *sql.DB
	staff     *staffauth.Service
	access    *orderaccess.Access
	payments  *payment.Repository
	serverKey string
}

func NewService(db *sql.DB, staff *staffauth.Service, access *orderaccess.Access, payments *payment.Repository, serverKey string) *Service {
	return &Service{db: db, staff: staff, access: access, payments: payments, serverKey: serverKey}
}

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/payment-cases", h.listCases)
	mux.HandleFunc("GET /api/v1/admin/payment-cases/{caseID}", h.caseDetail)
	mux.HandleFunc("POST /api/v1/admin/payment-cases/{caseID}/recheck", h.recheck)
	mux.HandleFunc("POST /api/v1/admin/payment-cases/{caseID}/notes", h.addCaseNote)
	mux.HandleFunc("POST /api/v1/admin/payment-cases/{caseID}/resolve", h.resolveCase)
	mux.HandleFunc("GET /api/v1/admin/email-jobs", h.listEmailJobs)
	mux.HandleFunc("GET /api/v1/admin/email-jobs/{jobID}", h.emailJobDetail)
	mux.HandleFunc("POST /api/v1/admin/email-jobs/{jobID}/retry", h.retryEmailJob)
}

type cursor struct {
	At string `json:"at"`
	ID string `json:"id"`
}

func encodeCursor(at time.Time, id string) (string, error) {
	value, err := json.Marshal(cursor{At: at.UTC().Format(time.RFC3339Nano), ID: id})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func decodeCursor(value string, numericID bool) (cursor, time.Time, error) {
	var result cursor
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) > 256 || json.Unmarshal(decoded, &result) != nil {
		return result, time.Time{}, ErrInvalidRequest
	}
	if (numericID && !caseIDPattern.MatchString(result.ID)) || (!numericID && !jobIDPattern.MatchString(result.ID)) {
		return result, time.Time{}, ErrInvalidRequest
	}
	at, err := time.Parse(time.RFC3339Nano, result.At)
	if err != nil || at.UTC().Format(time.RFC3339Nano) != result.At {
		return result, time.Time{}, ErrInvalidRequest
	}
	return result, at.UTC(), nil
}

type CaseFilter struct{ Query, Status, Cursor string }

type PaymentCase struct {
	ID              string     `json:"id"`
	Status          string     `json:"status"`
	OrderID         string     `json:"orderId"`
	Reference       string     `json:"reference"`
	EventName       string     `json:"eventName"`
	OrderStatus     string     `json:"orderStatus"`
	PaymentStatus   string     `json:"paymentStatus"`
	ProviderStatus  string     `json:"providerStatus"`
	Reason          string     `json:"reason"`
	Amount          uint64     `json:"amount"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	LastCheckedAt   *time.Time `json:"lastCheckedAt"`
	LastCheckError  string     `json:"lastCheckError"`
	CheckInProgress bool       `json:"checkInProgress"`
}

type casePage struct {
	Items      []PaymentCase `json:"items"`
	NextCursor *string       `json:"nextCursor"`
}

func (s *Service) ListCases(ctx context.Context, token string, filter CaseFilter) (casePage, error) {
	var result casePage
	filter.Query = strings.TrimSpace(filter.Query)
	if len(filter.Query) > 32 || len(filter.Cursor) > 256 {
		return result, ErrInvalidRequest
	}
	if filter.Status == "" {
		filter.Status = "OPEN"
	}
	if filter.Status != "OPEN" && filter.Status != "RESOLVED" {
		return result, ErrInvalidRequest
	}
	var cursorAt any
	cursorID := ""
	if filter.Cursor != "" {
		value, at, err := decodeCursor(filter.Cursor, true)
		if err != nil {
			return result, err
		}
		cursorAt, cursorID = at, value.ID
	}
	if _, err := s.authenticate(ctx, token); err != nil {
		return result, err
	}
	result.Items = make([]PaymentCase, 0, pageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT c.id, o.id, o.reference, e.artist, o.status, p.status, c.provider_status,
		c.reason, c.amount, c.created_at, c.updated_at, c.last_checked_at, c.last_check_error,
		COALESCE(c.check_lease_until > UTC_TIMESTAMP(6), FALSE)
		FROM payment_reconciliation_cases c JOIN orders o ON o.id = c.order_id
		JOIN reservations r ON r.id = o.reservation_id JOIN events e ON e.id = r.event_id
		JOIN payments p ON p.order_id = o.id
		WHERE c.status = ? AND (? = '' OR INSTR(UPPER(o.reference), UPPER(?)) > 0)
		AND (? IS NULL OR c.created_at < ? OR (c.created_at = ? AND c.id < ?))
		ORDER BY c.created_at DESC, c.id DESC LIMIT 51`, filter.Status, filter.Query, filter.Query,
		cursorAt, cursorAt, cursorAt, cursorID)
	if err != nil {
		return result, fmt.Errorf("query admin payment cases: %w", err)
	}
	for rows.Next() {
		var item PaymentCase
		var id uint64
		var checkedAt sql.NullTime
		if err := rows.Scan(&id, &item.OrderID, &item.Reference, &item.EventName, &item.OrderStatus, &item.PaymentStatus,
			&item.ProviderStatus, &item.Reason, &item.Amount, &item.CreatedAt, &item.UpdatedAt, &checkedAt,
			&item.LastCheckError, &item.CheckInProgress); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan admin payment case: %w", err)
		}
		item.ID = strconv.FormatUint(id, 10)
		item.Status = filter.Status
		item.CreatedAt, item.UpdatedAt = item.CreatedAt.UTC(), item.UpdatedAt.UTC()
		if checkedAt.Valid {
			value := checkedAt.Time.UTC()
			item.LastCheckedAt = &value
		}
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("iterate admin payment cases: %w", err)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close admin payment cases: %w", err)
	}
	if len(result.Items) > pageSize {
		last := result.Items[pageSize-1]
		result.Items = result.Items[:pageSize]
		id := last.ID
		value, err := encodeCursor(last.CreatedAt, id)
		if err != nil {
			return result, err
		}
		result.NextCursor = &value
	}
	return result, nil
}

type AuditEntry struct {
	Action    string          `json:"action"`
	ActorName string          `json:"actorName"`
	CreatedAt time.Time       `json:"createdAt"`
	Data      json.RawMessage `json:"data"`
}

type CaseDetail struct {
	PaymentCase
	Total           uint64       `json:"total"`
	ExpiresAt       time.Time    `json:"expiresAt"`
	GatewayOrderID  string       `json:"gatewayOrderId"`
	CanRecheck      bool         `json:"canRecheck"`
	CanResolve      bool         `json:"canResolve"`
	CheckLeaseUntil *time.Time   `json:"-"`
	Notes           []AuditEntry `json:"notes"`
	History         []AuditEntry `json:"history"`
}

func (s *Service) CaseDetail(ctx context.Context, token, id string) (CaseDetail, error) {
	var result CaseDetail
	if !caseIDPattern.MatchString(id) {
		return result, ErrInvalidRequest
	}
	if _, err := s.authenticate(ctx, token); err != nil {
		return result, err
	}
	var numericID uint64
	if _, err := fmt.Sscan(id, &numericID); err != nil {
		return result, ErrInvalidRequest
	}
	var checkedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT c.id, o.id, o.reference, e.artist, o.status, p.status, c.provider_status,
		c.reason, c.amount, c.created_at, c.updated_at, c.last_checked_at, c.last_check_error,
		COALESCE(c.check_lease_until > UTC_TIMESTAMP(6), FALSE), o.total, o.expires_at, p.gateway_order_id, c.status, c.check_lease_until
		FROM payment_reconciliation_cases c JOIN orders o ON o.id = c.order_id
		JOIN reservations r ON r.id = o.reservation_id JOIN events e ON e.id = r.event_id
		JOIN payments p ON p.order_id = o.id WHERE c.id = ?`, numericID).Scan(&result.ID, &result.OrderID,
		&result.Reference, &result.EventName, &result.OrderStatus, &result.PaymentStatus, &result.ProviderStatus,
		&result.Reason, &result.Amount, &result.CreatedAt, &result.UpdatedAt, &checkedAt, &result.LastCheckError,
		&result.CheckInProgress, &result.Total, &result.ExpiresAt, &result.GatewayOrderID, &result.Status, &result.CheckLeaseUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return result, ErrNotFound
	}
	if err != nil {
		return result, fmt.Errorf("read admin payment case: %w", err)
	}
	result.CreatedAt, result.UpdatedAt, result.ExpiresAt = result.CreatedAt.UTC(), result.UpdatedAt.UTC(), result.ExpiresAt.UTC()
	if checkedAt.Valid {
		value := checkedAt.Time.UTC()
		result.LastCheckedAt = &value
	}
	result.CanRecheck = result.Status == "OPEN" && !result.CheckInProgress && s.serverKey != ""
	result.CanResolve = result.Status == "OPEN" && !result.CheckInProgress
	result.History, result.Notes, err = s.audit(ctx, "PAYMENT_CASE", id)
	return result, err
}

func (s *Service) audit(ctx context.Context, objectType, objectID string) ([]AuditEntry, []AuditEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT action, actor_name, created_at, after_json FROM admin_audit_log
		WHERE object_type = ? AND object_id = ? ORDER BY id`, objectType, objectID)
	if err != nil {
		return nil, nil, fmt.Errorf("query admin issue audit: %w", err)
	}
	history := make([]AuditEntry, 0)
	notes := make([]AuditEntry, 0)
	for rows.Next() {
		var item AuditEntry
		if err := rows.Scan(&item.Action, &item.ActorName, &item.CreatedAt, &item.Data); err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("scan admin issue audit: %w", err)
		}
		item.CreatedAt = item.CreatedAt.UTC()
		history = append(history, item)
		if item.Action == "NOTE" {
			notes = append(notes, item)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, fmt.Errorf("iterate admin issue audit: %w", err)
	}
	return history, notes, rows.Close()
}

type EmailFilter struct{ Query, Kind, Cursor string }

type EmailJob struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	Status       string    `json:"status"`
	Reference    string    `json:"reference"`
	Recipient    string    `json:"recipient"`
	Attempts     uint8     `json:"attempts"`
	LastError    string    `json:"lastError"`
	UpdatedAt    time.Time `json:"updatedAt"`
	SupersededBy *string   `json:"supersededBy"`
}

type emailPage struct {
	Items      []EmailJob `json:"items"`
	NextCursor *string    `json:"nextCursor"`
}

func (s *Service) ListEmailJobs(ctx context.Context, token string, filter EmailFilter) (emailPage, error) {
	var result emailPage
	filter.Query = strings.TrimSpace(filter.Query)
	if len(filter.Query) > 32 || len(filter.Cursor) > 256 || (filter.Kind != "" && filter.Kind != "TICKETS" && filter.Kind != "RECOVERY" && filter.Kind != "REFUND" && filter.Kind != "EVENT_CHANGE") {
		return result, ErrInvalidRequest
	}
	var cursorAt any
	cursorID := ""
	if filter.Cursor != "" {
		value, at, err := decodeCursor(filter.Cursor, false)
		if err != nil {
			return result, err
		}
		cursorAt, cursorID = at, value.ID
	}
	if _, err := s.authenticate(ctx, token); err != nil {
		return result, err
	}
	result.Items = make([]EmailJob, 0, pageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT q.id, q.kind, q.status, COALESCE(o.reference, rr.reference),
		IF(q.recipient = '', '', CONCAT(LEFT(q.recipient, 1), '***', SUBSTRING(q.recipient, LOCATE('@', q.recipient)))),
		q.attempts, q.last_error, q.updated_at, q.superseded_by
		FROM email_queue q LEFT JOIN orders o ON o.id = q.order_id
		LEFT JOIN recovery_requests rr ON rr.id = q.recovery_request_id
		WHERE q.status = 'FAILED' AND q.superseded_by IS NULL AND (? = '' OR q.kind = ?)
		AND (? = '' OR INSTR(UPPER(COALESCE(o.reference, rr.reference, '')), UPPER(?)) > 0)
		AND (? IS NULL OR q.updated_at < ? OR (q.updated_at = ? AND q.id < ?))
		ORDER BY q.updated_at DESC, q.id DESC LIMIT 51`, filter.Kind, filter.Kind, filter.Query, filter.Query,
		cursorAt, cursorAt, cursorAt, cursorID)
	if err != nil {
		return result, fmt.Errorf("query failed admin email jobs: %w", err)
	}
	for rows.Next() {
		var item EmailJob
		var superseded sql.NullString
		if err := rows.Scan(&item.ID, &item.Kind, &item.Status, &item.Reference, &item.Recipient, &item.Attempts, &item.LastError, &item.UpdatedAt, &superseded); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan failed admin email job: %w", err)
		}
		item.UpdatedAt = item.UpdatedAt.UTC()
		if superseded.Valid {
			item.SupersededBy = &superseded.String
		}
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("iterate failed admin email jobs: %w", err)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close failed admin email jobs: %w", err)
	}
	if len(result.Items) > pageSize {
		last := result.Items[pageSize-1]
		result.Items = result.Items[:pageSize]
		value, err := encodeCursor(last.UpdatedAt, last.ID)
		if err != nil {
			return result, err
		}
		result.NextCursor = &value
	}
	return result, nil
}

type EmailDetail struct {
	EmailJob
	OrderStatus string       `json:"orderStatus"`
	CanRetry    bool         `json:"canRetry"`
	RetryReason string       `json:"retryReason"`
	RetryJobID  *string      `json:"retryJobId"`
	History     []AuditEntry `json:"history"`
}

type refundEmailSnapshot struct {
	Status    string `json:"status"`
	Amount    uint64 `json:"amount"`
	Reason    string `json:"reason"`
	Reference string `json:"reference"`
}

func decodeRefundEmailSnapshot(data []byte, target *refundEmailSnapshot) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ErrInvalidRequest
	}
	if (target.Status != "SUCCEEDED" && target.Status != "FAILED") || target.Amount == 0 || len(target.Reason) < 3 || len(target.Reason) > 500 || target.Reference == "" {
		return ErrInvalidRequest
	}
	return nil
}

func (s *Service) EmailDetail(ctx context.Context, token, id string) (EmailDetail, error) {
	var result EmailDetail
	if !jobIDPattern.MatchString(id) {
		return result, ErrInvalidRequest
	}
	if _, err := s.authenticate(ctx, token); err != nil {
		return result, err
	}
	var superseded sql.NullString
	var orderID, requestID sql.NullString
	var requestUsed sql.NullTime
	var refundSnapshot []byte
	var buyerEmail sql.NullString
	var orderTotal sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT q.id, q.kind, q.status, COALESCE(o.reference, rr.reference), q.recipient,
		q.attempts, q.last_error, q.updated_at, q.superseded_by, COALESCE(o.id, ''), COALESCE(rr.id, ''),
		rr.used_at, COALESCE(o.status, ''), q.refund_snapshot, b.email, CAST(o.total AS CHAR)
		FROM email_queue q LEFT JOIN orders o ON o.id = q.order_id
		LEFT JOIN order_buyers b ON b.order_id = o.id LEFT JOIN recovery_requests rr ON rr.id = q.recovery_request_id WHERE q.id = ?`, id).
		Scan(&result.ID, &result.Kind, &result.Status, &result.Reference, &result.Recipient, &result.Attempts,
			&result.LastError, &result.UpdatedAt, &superseded, &orderID, &requestID, &requestUsed,
			&result.OrderStatus, &refundSnapshot, &buyerEmail, &orderTotal)
	if errors.Is(err, sql.ErrNoRows) {
		return result, ErrNotFound
	}
	if err != nil {
		return result, fmt.Errorf("read admin email job: %w", err)
	}
	result.UpdatedAt = result.UpdatedAt.UTC()
	if superseded.Valid {
		result.SupersededBy, result.RetryJobID = &superseded.String, &superseded.String
	}
	if result.Status != "FAILED" || superseded.Valid {
		result.RetryReason = "Job tidak lagi tersedia untuk dikirim ulang."
	} else if result.Kind == "TICKETS" {
		order, found, err := recovery.FindOrderByID(ctx, s.db, orderID.String)
		if err != nil {
			return result, err
		}
		if found && order.Eligible(time.Now()) && order.TicketActive {
			result.CanRetry = true
		} else {
			result.RetryReason = "Pesanan belum lunas, tiket belum lengkap, atau akses pesanan sudah kedaluwarsa."
		}
	} else if result.Kind == "EVENT_CHANGE" {
		var valid bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM email_queue q JOIN event_changes c ON c.id=q.event_change_id
  JOIN orders o ON o.id=q.order_id JOIN reservations r ON r.id=o.reservation_id AND r.event_id=c.event_id
  JOIN order_buyers b ON b.order_id=o.id WHERE q.id=? AND b.email=q.recipient)`, id).Scan(&valid); err != nil {
			return result, err
		}
		result.CanRetry = valid
		if !valid {
			result.RetryReason = "Keputusan atau penerima tidak dapat diverifikasi."
		}
	} else if result.Kind == "REFUND" {
		var snapshot refundEmailSnapshot
		total, parseErr := strconv.ParseUint(orderTotal.String, 10, 64)
		if !orderID.Valid || !buyerEmail.Valid || buyerEmail.String != result.Recipient || !orderTotal.Valid || parseErr != nil || total == 0 ||
			decodeRefundEmailSnapshot(refundSnapshot, &snapshot) != nil || snapshot.Reference != result.Reference || total != snapshot.Amount {
			result.RetryReason = "Snapshot hasil refund atau tujuan pesanan tidak dapat diverifikasi."
		} else {
			result.CanRetry = true
		}
	} else if result.Kind == "RECOVERY" {
		requestUsedAt := requestUsed.Valid
		order, found, err := recovery.FindOrderByReference(ctx, s.db, result.Reference)
		if err != nil {
			return result, err
		}
		if requestUsedAt {
			result.RetryReason = "Permintaan pemulihan ini sudah digunakan."
		} else if !found || !order.Eligible(time.Now()) {
			result.RetryReason = "Data permintaan tidak dapat diverifikasi atau pesanan sudah kedaluwarsa."
		} else {
			var pairKey string
			if err := s.db.QueryRowContext(ctx, "SELECT pair_key FROM recovery_requests WHERE id = ?", requestID.String).Scan(&pairKey); err != nil {
				return result, fmt.Errorf("read recovery request pair: %w", err)
			}
			if subtle.ConstantTimeCompare([]byte(pairKey), []byte(recovery.PairKey(s.access, order.BuyerEmail, result.Reference))) == 1 {
				result.CanRetry = true
			} else {
				result.RetryReason = "Reference dan email permintaan tidak cocok dengan pesanan."
			}
		}
	} else {
		result.RetryReason = "Jenis email tidak dikenal."
	}
	result.History, _, err = s.audit(ctx, "EMAIL_JOB", id)
	return result, err
}

func (s *Service) authenticate(ctx context.Context, token string) (staffauth.Principal, error) {
	principal, err := s.staff.Authenticate(ctx, token)
	if err != nil {
		return principal, err
	}
	if principal.Role != "ADMIN" {
		return principal, staffauth.ErrForbidden
	}
	return principal, nil
}

func newID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func appendAudit(ctx context.Context, tx *sql.Tx, actor staffauth.Principal, action, objectType, objectID string, before, after any) error {
	var beforeValue any
	if before != nil {
		value, err := json.Marshal(before)
		if err != nil {
			return err
		}
		beforeValue = value
	}
	afterValue, err := json.Marshal(after)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO admin_audit_log
		(actor_id, actor_name, action, object_type, object_id, before_json, after_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, UTC_TIMESTAMP(6))`, actor.ID, actor.Name, action, objectType, objectID, beforeValue, afterValue)
	return err
}

func (h *Handler) listCases(w http.ResponseWriter, r *http.Request) {
	filter, ok := parseQuery(w, r, map[string]bool{"q": true, "status": true, "cursor": true})
	if !ok {
		return
	}
	result, err := h.service.ListCases(r.Context(), bearer(r), CaseFilter{Query: first(filter, "q"), Status: first(filter, "status"), Cursor: first(filter, "cursor")})
	h.respond(w, r, result, err)
}

func (h *Handler) caseDetail(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.CaseDetail(r.Context(), bearer(r), r.PathValue("caseID"))
	h.respond(w, r, result, err)
}

func (h *Handler) listEmailJobs(w http.ResponseWriter, r *http.Request) {
	filter, ok := parseQuery(w, r, map[string]bool{"q": true, "kind": true, "cursor": true})
	if !ok {
		return
	}
	result, err := h.service.ListEmailJobs(r.Context(), bearer(r), EmailFilter{Query: first(filter, "q"), Kind: first(filter, "kind"), Cursor: first(filter, "cursor")})
	h.respond(w, r, result, err)
}

func first(values map[string][]string, key string) string {
	if len(values[key]) == 0 {
		return ""
	}
	return values[key][0]
}

func (h *Handler) emailJobDetail(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.EmailDetail(r.Context(), bearer(r), r.PathValue("jobID"))
	h.respond(w, r, result, err)
}

func (h *Handler) retryEmailJob(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.RetryEmailJob(r.Context(), bearer(r), r.PathValue("jobID"))
	h.respond(w, r, result, err)
}

func parseQuery(w http.ResponseWriter, r *http.Request, accepted map[string]bool) (map[string][]string, bool) {
	w.Header().Set("Cache-Control", "no-store")
	query := r.URL.Query()
	for name, values := range query {
		if !accepted[name] || len(values) != 1 {
			writeError(w, 400, "INVALID_REQUEST", "Filter tidak valid")
			return nil, false
		}
	}
	return query, true
}

func bearer(r *http.Request) string {
	token, _ := orderaccess.Bearer(r.Header.Get("Authorization"))
	return token
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, value any, err error) {
	w.Header().Set("Cache-Control", "no-store")
	if err == nil {
		writeJSON(w, 200, value)
		return
	}
	switch {
	case errors.Is(err, staffauth.ErrUnauthorized):
		writeError(w, 401, "UNAUTHORIZED", "Sesi administrator tidak valid atau sudah berakhir")
	case errors.Is(err, staffauth.ErrForbidden):
		writeError(w, 403, "FORBIDDEN", "Akses penanganan masalah hanya tersedia untuk admin")
	case errors.Is(err, ErrInvalidRequest):
		writeError(w, 400, "INVALID_REQUEST", "Data permintaan tidak valid")
	case errors.Is(err, ErrNotFound):
		writeError(w, 404, "NOT_FOUND", "Data tidak ditemukan")
	case errors.Is(err, ErrConflict):
		writeError(w, 409, "STATE_CONFLICT", "Status data berubah. Muat ulang sebelum mencoba lagi.")
	case errors.Is(err, ErrRateLimited):
		wait := 60
		var limited rateLimitError
		if errors.As(err, &limited) && limited.wait > 0 {
			wait = limited.wait
		}
		w.Header().Set("Retry-After", strconv.Itoa(wait))
		writeError(w, 429, "RATE_LIMITED", "Aksi ini dibatasi. Coba lagi nanti.")
	case errors.Is(err, ErrProviderUnavailable):
		writeError(w, 503, "PROVIDER_UNAVAILABLE", "Pemeriksaan pembayaran belum tersedia")
	case errors.Is(err, ErrProviderFailure):
		writeError(w, 502, "PROVIDER_FAILURE", "Provider pembayaran belum dapat diperiksa")
	default:
		h.logger.ErrorContext(r.Context(), "admin issue API error", "request_id", r.Header.Get("X-Request-ID"), "error", err)
		writeError(w, 500, "INTERNAL_ERROR", "Terjadi kesalahan pada server")
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
