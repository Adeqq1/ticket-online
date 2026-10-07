package adminorders

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/Adeqq1/ticket-online/backend/internal/payment"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
)

const pageSize = 50

var ErrInvalidRequest = errors.New("invalid admin order request")

var orderIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

type Service struct {
	db    *sql.DB
	staff *staffauth.Service
}

func NewService(db *sql.DB, staff *staffauth.Service) *Service { return &Service{db: db, staff: staff} }

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/orders", h.list)
	mux.HandleFunc("GET /api/v1/admin/orders/{orderID}", h.detail)
}

type Filter struct {
	Query, EventID, Status, DateFrom, DateTo, Cursor string
}

type filterOptions struct {
	Events []eventOption `json:"events"`
}

type eventOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type OrderItem struct {
	TierID    string `json:"tierId"`
	Name      string `json:"name"`
	Quantity  uint64 `json:"quantity"`
	UnitPrice uint64 `json:"unitPrice"`
	LineTotal uint64 `json:"lineTotal"`
}

type Payment struct {
	Method string     `json:"method"`
	Amount uint64     `json:"amount"`
	Status string     `json:"status"`
	PaidAt *time.Time `json:"paidAt"`
}

type Ticket struct {
	ID           string     `json:"id"`
	Code         string     `json:"code"`
	AttendeeName string     `json:"attendeeName"`
	TierName     string     `json:"tierName"`
	Gate         string     `json:"gate"`
	Status       string     `json:"status"`
	CheckedInAt  *time.Time `json:"checkedInAt"`
	CheckedInBy  *string    `json:"checkedInBy"`
}

type Order struct {
	ID          string    `json:"id"`
	Reference   string    `json:"reference"`
	Status      string    `json:"status"`
	EventID     string    `json:"eventId"`
	EventName   string    `json:"eventName"`
	BuyerName   string    `json:"buyerName"`
	CreatedAt   time.Time `json:"createdAt"`
	TicketCount uint64    `json:"ticketCount"`
	Total       uint64    `json:"total"`
}

type Detail struct {
	Order
	Buyer struct {
		Name           string `json:"name"`
		Email          string `json:"email"`
		Phone          string `json:"phone"`
		IdentityMasked string `json:"identityMasked"`
	} `json:"buyer"`
	Subtotal  uint64      `json:"subtotal"`
	AdminFee  uint64      `json:"adminFee"`
	Discount  uint64      `json:"discount"`
	ExpiresAt time.Time   `json:"expiresAt"`
	Items     []OrderItem `json:"items"`
	Payment   *Payment    `json:"payment"`
	Tickets   []Ticket    `json:"tickets"`
}

type page struct {
	Items         []Order       `json:"items"`
	NextCursor    *string       `json:"nextCursor"`
	FilterOptions filterOptions `json:"filterOptions"`
}

type cursor struct {
	CreatedAt string `json:"createdAt"`
	ID        string `json:"id"`
}

func (s *Service) List(r *http.Request, token string, filter Filter) (page, error) {
	var result page
	query, err := parseFilter(filter)
	if err != nil {
		return result, err
	}
	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return result, fmt.Errorf("begin admin order list: %w", err)
	}
	defer tx.Rollback()
	principal, err := s.staff.AuthenticateTx(r.Context(), tx, token)
	if err != nil {
		return result, err
	}
	if principal.Role != "ADMIN" {
		return result, staffauth.ErrForbidden
	}
	result.Items = make([]Order, 0, pageSize)
	rows, err := tx.QueryContext(r.Context(), `SELECT o.id, o.reference, o.status, e.id, e.artist,
		b.name, o.created_at, COALESCE(SUM(oi.quantity), 0), o.total
		FROM orders o JOIN reservations r ON r.id = o.reservation_id JOIN events e ON e.id = r.event_id
		JOIN order_buyers b ON b.order_id = o.id JOIN order_items oi ON oi.order_id = o.id
		WHERE (? = '' OR INSTR(UPPER(o.reference), UPPER(?)) > 0) AND (? = '' OR e.id = ?)
		AND (? = '' OR o.status = ?) AND (? IS NULL OR o.created_at >= ?) AND (? IS NULL OR o.created_at < ?)
		AND (? IS NULL OR o.created_at < ? OR (o.created_at = ? AND o.id < ?))
		GROUP BY o.id, o.reference, o.status, e.id, e.artist, b.name, o.created_at, o.total
		ORDER BY o.created_at DESC, o.id DESC LIMIT 51`, query.Query, query.Query, query.EventID, query.EventID,
		query.Status, query.Status, query.From, query.From, query.To, query.To,
		query.CursorAt, query.CursorAt, query.CursorAt, query.CursorID)
	if err != nil {
		return result, fmt.Errorf("query admin orders: %w", err)
	}
	for rows.Next() {
		var item Order
		if err := rows.Scan(&item.ID, &item.Reference, &item.Status, &item.EventID, &item.EventName, &item.BuyerName, &item.CreatedAt, &item.TicketCount, &item.Total); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan admin order: %w", err)
		}
		item.CreatedAt = item.CreatedAt.UTC()
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("iterate admin orders: %w", err)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close admin orders: %w", err)
	}
	if len(result.Items) > pageSize {
		last := result.Items[pageSize-1]
		result.Items = result.Items[:pageSize]
		encoded, err := encodeCursor(last)
		if err != nil {
			return result, err
		}
		result.NextCursor = &encoded
	}
	result.FilterOptions.Events = make([]eventOption, 0)
	rows, err = tx.QueryContext(r.Context(), `SELECT DISTINCT e.id, e.artist FROM orders o
		JOIN reservations r ON r.id = o.reservation_id JOIN events e ON e.id = r.event_id ORDER BY e.artist, e.id`)
	if err != nil {
		return result, fmt.Errorf("query admin order filter options: %w", err)
	}
	for rows.Next() {
		var option eventOption
		if err := rows.Scan(&option.ID, &option.Name); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan admin order filter option: %w", err)
		}
		result.FilterOptions.Events = append(result.FilterOptions.Events, option)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("iterate admin order filter options: %w", err)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close admin order filter options: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("commit admin order list: %w", err)
	}
	return result, nil
}

type parsedFilter struct {
	Filter
	From, To, CursorAt any
	CursorID           string
}

func parseFilter(filter Filter) (parsedFilter, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	if len(filter.Query) > 32 || len(filter.EventID) > 64 || len(filter.Cursor) > 256 {
		return parsedFilter{}, ErrInvalidRequest
	}
	switch filter.Status {
	case "", "PENDING", "PAID", "CANCELLED", "EXPIRED", "REFUND_PENDING", "REFUNDED":
	default:
		return parsedFilter{}, ErrInvalidRequest
	}
	result := parsedFilter{Filter: filter}
	if filter.DateFrom != "" {
		from, err := time.Parse("2006-01-02", filter.DateFrom)
		if err != nil || from.Format("2006-01-02") != filter.DateFrom {
			return parsedFilter{}, ErrInvalidRequest
		}
		result.From = from.Add(-7 * time.Hour)
	}
	if filter.DateTo != "" {
		to, err := time.Parse("2006-01-02", filter.DateTo)
		if err != nil || to.Format("2006-01-02") != filter.DateTo {
			return parsedFilter{}, ErrInvalidRequest
		}
		result.To = to.AddDate(0, 0, 1).Add(-7 * time.Hour)
	}
	if filter.DateFrom != "" && filter.DateTo != "" && filter.DateFrom > filter.DateTo {
		return parsedFilter{}, ErrInvalidRequest
	}
	if filter.Cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(filter.Cursor)
		if err != nil {
			return parsedFilter{}, ErrInvalidRequest
		}
		var value cursor
		if json.Unmarshal(decoded, &value) != nil || !orderIDPattern.MatchString(value.ID) {
			return parsedFilter{}, ErrInvalidRequest
		}
		createdAt, err := time.Parse(time.RFC3339Nano, value.CreatedAt)
		if err != nil || createdAt.UTC().Format(time.RFC3339Nano) != value.CreatedAt {
			return parsedFilter{}, ErrInvalidRequest
		}
		result.CursorAt, result.CursorID = createdAt.UTC(), value.ID
	}
	return result, nil
}

func encodeCursor(item Order) (string, error) {
	value, err := json.Marshal(cursor{CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), ID: item.ID})
	if err != nil {
		return "", fmt.Errorf("encode admin order cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (s *Service) Detail(r *http.Request, token, id string) (Detail, error) {
	var detail Detail
	if !orderIDPattern.MatchString(id) {
		return detail, ErrInvalidRequest
	}
	// AuthenticateTx takes shared locks so session revocation cannot race this read.
	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return detail, fmt.Errorf("begin admin order detail: %w", err)
	}
	defer tx.Rollback()
	principal, err := s.staff.AuthenticateTx(r.Context(), tx, token)
	if err != nil {
		return detail, err
	}
	if principal.Role != "ADMIN" {
		return detail, staffauth.ErrForbidden
	}
	var identity string
	err = tx.QueryRowContext(r.Context(), `SELECT o.id, o.reference, o.status, e.id, e.artist, b.name, o.created_at,
		(SELECT COALESCE(SUM(oi.quantity), 0) FROM order_items oi WHERE oi.order_id = o.id), o.total,
		b.name, b.email, b.phone, b.identity, o.subtotal, o.admin_fee, o.discount, o.expires_at
		FROM orders o JOIN reservations r ON r.id = o.reservation_id JOIN events e ON e.id = r.event_id
		JOIN order_buyers b ON b.order_id = o.id WHERE o.id = ?`, id).Scan(&detail.ID, &detail.Reference, &detail.Status,
		&detail.EventID, &detail.EventName, &detail.BuyerName, &detail.CreatedAt, &detail.TicketCount, &detail.Total,
		&detail.Buyer.Name, &detail.Buyer.Email, &detail.Buyer.Phone, &identity, &detail.Subtotal, &detail.AdminFee, &detail.Discount, &detail.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return detail, sql.ErrNoRows
	}
	if err != nil {
		return detail, fmt.Errorf("read admin order: %w", err)
	}
	detail.Buyer.IdentityMasked = maskIdentity(identity)
	detail.CreatedAt, detail.ExpiresAt = detail.CreatedAt.UTC(), detail.ExpiresAt.UTC()
	detail.Items = make([]OrderItem, 0)
	rows, err := tx.QueryContext(r.Context(), `SELECT tt.slug, oi.tier_name, oi.quantity, oi.unit_price, oi.line_total
		FROM order_items oi JOIN ticket_tiers tt ON tt.id = oi.ticket_tier_id WHERE oi.order_id = ? ORDER BY tt.slug`, id)
	if err != nil {
		return detail, fmt.Errorf("query admin order items: %w", err)
	}
	for rows.Next() {
		var item OrderItem
		if err := rows.Scan(&item.TierID, &item.Name, &item.Quantity, &item.UnitPrice, &item.LineTotal); err != nil {
			rows.Close()
			return detail, fmt.Errorf("scan admin order item: %w", err)
		}
		detail.Items = append(detail.Items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return detail, fmt.Errorf("iterate admin order items: %w", err)
	}
	if err := rows.Close(); err != nil {
		return detail, fmt.Errorf("close admin order items: %w", err)
	}
	var paymentRow Payment
	var paidAt sql.NullTime
	err = tx.QueryRowContext(r.Context(), "SELECT method, amount, status, paid_at FROM payments WHERE order_id = ?", id).Scan(&paymentRow.Method, &paymentRow.Amount, &paymentRow.Status, &paidAt)
	if err == nil {
		if paidAt.Valid {
			value := paidAt.Time.UTC()
			paymentRow.PaidAt = &value
		}
		detail.Payment = &paymentRow
	} else if !errors.Is(err, sql.ErrNoRows) {
		return detail, fmt.Errorf("read admin order payment: %w", err)
	}
	detail.Tickets = make([]Ticket, 0)
	rows, err = tx.QueryContext(r.Context(), `SELECT et.id, et.snapshot, tc.checked_in_at, su.name
		FROM etickets et LEFT JOIN ticket_checkins tc ON tc.ticket_id = et.id
		LEFT JOIN staff_users su ON su.id = tc.staff_id WHERE et.order_id = ? ORDER BY et.issued_at, et.id`, id)
	if err != nil {
		return detail, fmt.Errorf("query admin order tickets: %w", err)
	}
	for rows.Next() {
		var ticket Ticket
		var snapshot []byte
		var checkedInAt sql.NullTime
		var checkedInBy sql.NullString
		if err := rows.Scan(&ticket.ID, &snapshot, &checkedInAt, &checkedInBy); err != nil {
			rows.Close()
			return detail, fmt.Errorf("scan admin order ticket: %w", err)
		}
		var snapshotTicket payment.Ticket
		if err := json.Unmarshal(snapshot, &snapshotTicket); err != nil {
			rows.Close()
			return detail, fmt.Errorf("decode admin order ticket snapshot: %w", err)
		}
		ticket.Code, ticket.AttendeeName, ticket.TierName, ticket.Gate = snapshotTicket.Code, snapshotTicket.AttendeeName, snapshotTicket.TierName, snapshotTicket.Gate
		ticket.Status = "NOT_CHECKED_IN"
		if checkedInAt.Valid {
			value := checkedInAt.Time.UTC()
			ticket.CheckedInAt, ticket.Status = &value, "CHECKED_IN"
		}
		if checkedInBy.Valid {
			value := checkedInBy.String
			ticket.CheckedInBy = &value
		}
		detail.Tickets = append(detail.Tickets, ticket)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return detail, fmt.Errorf("iterate admin order tickets: %w", err)
	}
	if err := rows.Close(); err != nil {
		return detail, fmt.Errorf("close admin order tickets: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return detail, fmt.Errorf("commit admin order detail: %w", err)
	}
	return detail, nil
}

func maskIdentity(identity string) string {
	if len(identity) <= 4 {
		return identity
	}
	return strings.Repeat("•", len(identity)-4) + identity[len(identity)-4:]
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		writeError(w, 401, "UNAUTHORIZED", "Autentikasi administrator diperlukan")
		return
	}
	query := r.URL.Query()
	for name, values := range query {
		if !strings.Contains("|q|eventId|status|dateFrom|dateTo|cursor|", "|"+name+"|") || len(values) != 1 {
			writeError(w, 400, "INVALID_REQUEST", "Filter pesanan tidak valid")
			return
		}
	}
	filter := Filter{Query: query.Get("q"), EventID: query.Get("eventId"), Status: query.Get("status"), DateFrom: query.Get("dateFrom"), DateTo: query.Get("dateTo"), Cursor: query.Get("cursor")}
	result, err := h.service.List(r, token, filter)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}

func (h *Handler) detail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		writeError(w, 401, "UNAUTHORIZED", "Autentikasi administrator diperlukan")
		return
	}
	result, err := h.service.Detail(r, token, r.PathValue("orderID"))
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}

func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, staffauth.ErrUnauthorized):
		writeError(w, 401, "UNAUTHORIZED", "Sesi administrator tidak valid atau sudah berakhir")
	case errors.Is(err, staffauth.ErrForbidden):
		writeError(w, 403, "FORBIDDEN", "Akses pesanan hanya tersedia untuk admin")
	case errors.Is(err, sql.ErrNoRows):
		writeError(w, 404, "ORDER_NOT_FOUND", "Pesanan tidak ditemukan")
	case errors.Is(err, ErrInvalidRequest):
		writeError(w, 400, "INVALID_REQUEST", "Filter atau ID pesanan tidak valid")
	default:
		h.logger.ErrorContext(r.Context(), "admin order API error", "request_id", r.Header.Get("X-Request-ID"), "error", err)
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
