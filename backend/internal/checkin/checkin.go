package checkin

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
)

var (
	ErrInvalidRequest = errors.New("invalid check-in request")
	ErrTicketNotFound = errors.New("ticket not found")
	ErrWrongGate      = errors.New("ticket belongs to another gate")
	ErrOrderNotPaid   = errors.New("ticket order is not paid")
	ErrTicketUsed     = errors.New("ticket already checked in")
)

type Request struct {
	EventID string `json:"eventId"`
	Gate    string `json:"gate"`
	Code    string `json:"code"`
}

type Ticket struct {
	ID           string `json:"id"`
	Code         string `json:"code"`
	AttendeeName string `json:"attendeeName"`
	TierName     string `json:"tierName"`
	EventID      string `json:"eventId"`
	Gate         string `json:"gate"`
}

type Result struct {
	Status      string    `json:"status"`
	Ticket      Ticket    `json:"ticket"`
	CheckedInAt time.Time `json:"checkedInAt"`
}

type TicketStatus struct {
	Status      string     `json:"status"`
	Ticket      Ticket     `json:"ticket"`
	OrderStatus string     `json:"orderStatus"`
	CheckedInAt *time.Time `json:"checkedInAt"`
}

type HistoryFilter struct {
	EventID  string
	Gate     string
	Query    string
	BeforeID string
}

type HistoryItem struct {
	ID          string       `json:"id"`
	Code        *string      `json:"code"`
	EventID     *string      `json:"eventId"`
	EventName   *string      `json:"eventName"`
	Gate        *string      `json:"gate"`
	Staff       HistoryStaff `json:"staff"`
	Outcome     string       `json:"outcome"`
	RecordedAt  time.Time    `json:"recordedAt"`
	CheckedInAt *time.Time   `json:"checkedInAt"`
}

type HistoryStaff struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type HistoryEvent struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Gates []string `json:"gates"`
}

type HistoryPage struct {
	Items         []HistoryItem  `json:"items"`
	NextCursor    *string        `json:"nextCursor"`
	FilterOptions []HistoryEvent `json:"filterOptions"`
}

type Service struct {
	db    *sql.DB
	staff *staffauth.Service
}

func NewService(db *sql.DB, staff *staffauth.Service) *Service {
	return &Service{db: db, staff: staff}
}

func Normalize(request Request) (Request, error) {
	request.EventID = strings.TrimSpace(request.EventID)
	request.Gate = strings.TrimSpace(request.Gate)
	request.Code = strings.ToUpper(strings.TrimSpace(request.Code))
	if request.EventID == "" || len(request.EventID) > 64 || request.Gate == "" || len(request.Gate) > 100 ||
		request.Code == "" {
		return Request{}, ErrInvalidRequest
	}
	code, err := NormalizeTicketCode(request.Code)
	if err != nil {
		return Request{}, ErrInvalidRequest
	}
	request.Code = code
	return request, nil
}

func NormalizeTicketCode(code string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 35 || !strings.HasPrefix(code, "ET-") {
		return "", ErrInvalidRequest
	}
	if _, err := hex.DecodeString(code[3:]); err != nil {
		return "", ErrInvalidRequest
	}
	return code, nil
}

func (s *Service) GetTicketStatus(ctx context.Context, token, code string) (TicketStatus, error) {
	code, err := NormalizeTicketCode(code)
	if err != nil {
		return TicketStatus{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return TicketStatus{}, fmt.Errorf("begin ticket status: %w", err)
	}
	defer tx.Rollback()
	principal, err := s.staff.AuthenticateTx(ctx, tx, token)
	if err != nil {
		return TicketStatus{}, err
	}
	ticketID := strings.ToLower(code[3:])
	var orderID string
	if err := tx.QueryRowContext(ctx, "SELECT order_id FROM etickets WHERE id = ?", ticketID).Scan(&orderID); errors.Is(err, sql.ErrNoRows) {
		return TicketStatus{}, ErrTicketNotFound
	} else if err != nil {
		return TicketStatus{}, fmt.Errorf("find ticket status order: %w", err)
	}
	var orderStatus, eventID string
	if err := tx.QueryRowContext(ctx, `SELECT o.status, r.event_id FROM orders o
		JOIN reservations r ON r.id = o.reservation_id WHERE o.id = ? FOR SHARE`, orderID).Scan(&orderStatus, &eventID); errors.Is(err, sql.ErrNoRows) {
		return TicketStatus{}, ErrTicketNotFound
	} else if err != nil {
		return TicketStatus{}, fmt.Errorf("lock ticket status order: %w", err)
	}
	var snapshot []byte
	if err := tx.QueryRowContext(ctx, "SELECT snapshot FROM etickets WHERE id = ? FOR SHARE", ticketID).Scan(&snapshot); errors.Is(err, sql.ErrNoRows) {
		return TicketStatus{}, ErrTicketNotFound
	} else if err != nil {
		return TicketStatus{}, fmt.Errorf("lock ticket status snapshot: %w", err)
	}
	var ticket Ticket
	if err := json.Unmarshal(snapshot, &ticket); err != nil {
		return TicketStatus{}, fmt.Errorf("decode ticket status snapshot: %w", err)
	}
	if ticket.ID != ticketID || ticket.EventID != eventID {
		return TicketStatus{}, ErrTicketNotFound
	}
	if err := s.staff.AuthorizeTicketStatusTx(ctx, tx, principal, eventID, ticket.Gate); err != nil {
		return TicketStatus{}, err
	}
	result := TicketStatus{Status: "NOT_CHECKED_IN", Ticket: ticket, OrderStatus: orderStatus}
	var checkedInAt sql.NullTime
	if err := tx.QueryRowContext(ctx, "SELECT checked_in_at FROM ticket_checkins WHERE ticket_id = ?", ticketID).Scan(&checkedInAt); err == nil {
		result.Status = "CHECKED_IN"
		checkedInAtUTC := checkedInAt.Time.UTC()
		result.CheckedInAt = &checkedInAtUTC
	} else if !errors.Is(err, sql.ErrNoRows) {
		return TicketStatus{}, fmt.Errorf("find ticket status check-in: %w", err)
	}
	return result, nil
}

func (s *Service) CheckIn(ctx context.Context, token string, request Request) (Result, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Result{}, fmt.Errorf("begin check-in: %w", err)
	}
	defer tx.Rollback()

	principal, err := s.staff.AuthenticateTx(ctx, tx, token)
	if err != nil {
		return Result{}, err
	}
	if principal.Role != "STAFF" {
		return Result{}, staffauth.ErrForbidden
	}
	submitted := request
	request, err = Normalize(request)
	if err != nil {
		return Result{}, s.finishAttempt(ctx, tx, principal, submitted, "INVALID_REQUEST", nil, err)
	}
	if err := s.staff.AuthorizeTicketStatusTx(ctx, tx, principal, request.EventID, request.Gate); err != nil {
		if errors.Is(err, staffauth.ErrForbidden) {
			return Result{}, s.finishAttempt(ctx, tx, principal, request, "FORBIDDEN", nil, err)
		}
		return Result{}, err
	}
	ticketID := strings.ToLower(request.Code[3:])
	var orderID string
	err = tx.QueryRowContext(ctx, "SELECT order_id FROM etickets WHERE id = ?", ticketID).Scan(&orderID)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, s.finishAttempt(ctx, tx, principal, request, "TICKET_NOT_FOUND", nil, ErrTicketNotFound)
	}
	if err != nil {
		return Result{}, fmt.Errorf("find ticket order: %w", err)
	}
	var orderStatus string
	var eventID string
	err = tx.QueryRowContext(ctx, `SELECT o.status, r.event_id FROM orders o
		JOIN reservations r ON r.id = o.reservation_id WHERE o.id = ? FOR UPDATE`, orderID).Scan(&orderStatus, &eventID)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, s.finishAttempt(ctx, tx, principal, request, "TICKET_NOT_FOUND", nil, ErrTicketNotFound)
	}
	if err != nil {
		return Result{}, fmt.Errorf("lock ticket order: %w", err)
	}
	if orderStatus != "PAID" {
		return Result{}, s.finishAttempt(ctx, tx, principal, request, "ORDER_NOT_PAID", nil, ErrOrderNotPaid)
	}
	if eventID != request.EventID {
		return Result{}, s.finishAttempt(ctx, tx, principal, request, "TICKET_NOT_FOUND", nil, ErrTicketNotFound)
	}

	var snapshot []byte
	err = tx.QueryRowContext(ctx, "SELECT snapshot FROM etickets WHERE id = ? FOR UPDATE", ticketID).Scan(&snapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, s.finishAttempt(ctx, tx, principal, request, "TICKET_NOT_FOUND", nil, ErrTicketNotFound)
	}
	if err != nil {
		return Result{}, fmt.Errorf("lock ticket for check-in: %w", err)
	}
	var ticket Ticket
	if err := json.Unmarshal(snapshot, &ticket); err != nil {
		return Result{}, fmt.Errorf("decode ticket snapshot: %w", err)
	}
	if ticket.ID != ticketID || ticket.EventID != eventID {
		return Result{}, s.finishAttempt(ctx, tx, principal, request, "TICKET_NOT_FOUND", nil, ErrTicketNotFound)
	}
	if ticket.Gate != request.Gate {
		return Result{Ticket: ticket}, s.finishAttempt(ctx, tx, principal, request, "WRONG_GATE", nil, ErrWrongGate)
	}
	var checkedInAt sql.NullTime
	err = tx.QueryRowContext(ctx, "SELECT checked_in_at FROM ticket_checkins WHERE ticket_id = ?", ticketID).Scan(&checkedInAt)
	if err == nil {
		checkedInAtUTC := checkedInAt.Time.UTC()
		result := Result{Status: "ALREADY_USED", Ticket: ticket, CheckedInAt: checkedInAtUTC}
		return result, s.finishAttempt(ctx, tx, principal, request, "TICKET_ALREADY_USED", &checkedInAtUTC, ErrTicketUsed)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Result{}, fmt.Errorf("find ticket check-in: %w", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := tx.ExecContext(ctx, `INSERT INTO ticket_checkins
		(ticket_id, staff_id, event_id, gate, checked_in_at) VALUES (?, ?, ?, ?, ?)`, ticketID, principal.ID, eventID, request.Gate, now); err != nil {
		return Result{}, fmt.Errorf("record ticket check-in: %w", err)
	}
	if err := insertAttempt(ctx, tx, principal, request, "CHECKED_IN", ticketID, &now); err != nil {
		return Result{}, fmt.Errorf("record check-in attempt: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("commit check-in: %w", err)
	}
	return Result{Status: "CHECKED_IN", Ticket: ticket, CheckedInAt: now}, nil
}

func (s *Service) finishAttempt(ctx context.Context, tx *sql.Tx, principal staffauth.Principal, request Request, outcome string, checkedInAt *time.Time, result error) error {
	if err := insertAttempt(ctx, tx, principal, request, outcome, "", checkedInAt); err != nil {
		return fmt.Errorf("record check-in attempt: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit check-in attempt: %w", err)
	}
	return result
}

func insertAttempt(ctx context.Context, tx *sql.Tx, principal staffauth.Principal, request Request, outcome, ticketID string, checkedInAt *time.Time) error {
	var code, eventID, gate, checkedAt any
	if normalized, err := NormalizeTicketCode(request.Code); err == nil {
		code = normalized
	}
	if value := strings.TrimSpace(request.EventID); value != "" && len(value) <= 64 {
		eventID = value
	}
	if value := strings.TrimSpace(request.Gate); value != "" && len(value) <= 100 {
		gate = value
	}
	if checkedInAt != nil {
		checkedAt = checkedInAt.UTC()
	}
	var checkedTicket any
	if ticketID != "" {
		checkedTicket = ticketID
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO checkin_attempts
		(checked_in_ticket_id, ticket_code, event_id, gate, staff_id, staff_name, outcome, recorded_at, checked_in_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, UTC_TIMESTAMP(6), ?)`, checkedTicket, code, eventID, gate, principal.ID, principal.Name, outcome, checkedAt)
	return err
}

func (s *Service) ListHistory(ctx context.Context, token string, filter HistoryFilter) (HistoryPage, error) {
	var result HistoryPage
	if len(filter.EventID) > 64 || len(filter.Gate) > 100 || len(filter.Query) > 35 {
		return result, ErrInvalidRequest
	}
	var before uint64
	if filter.BeforeID != "" {
		parsed, err := strconv.ParseUint(filter.BeforeID, 10, 64)
		if err != nil || parsed == 0 || strconv.FormatUint(parsed, 10) != filter.BeforeID {
			return result, ErrInvalidRequest
		}
		before = parsed
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return result, fmt.Errorf("begin check-in history: %w", err)
	}
	defer tx.Rollback()
	principal, err := s.staff.AuthenticateTx(ctx, tx, token)
	if err != nil {
		return result, err
	}
	if principal.Role != "ADMIN" {
		return result, staffauth.ErrForbidden
	}
	result.Items = make([]HistoryItem, 0, 51)
	rows, err := tx.QueryContext(ctx, `SELECT a.id, a.ticket_code, a.event_id, COALESCE(e.artist, a.event_id), a.gate,
		a.staff_id, a.staff_name, a.outcome, a.recorded_at, a.checked_in_at
		FROM checkin_attempts a LEFT JOIN events e ON e.id = a.event_id
		WHERE (? = '' OR a.event_id = ?) AND (? = '' OR a.gate = ?)
		AND (? = '' OR INSTR(UPPER(COALESCE(a.ticket_code, '')), UPPER(?)) > 0)
		AND (? = 0 OR a.id < ?)
		ORDER BY a.id DESC LIMIT 51`, filter.EventID, filter.EventID, filter.Gate, filter.Gate, filter.Query, filter.Query, before, before)
	if err != nil {
		return result, fmt.Errorf("query check-in history: %w", err)
	}
	for rows.Next() {
		var item HistoryItem
		var code, eventID, eventName, gate sql.NullString
		var checkedAt sql.NullTime
		var id uint64
		if err := rows.Scan(&id, &code, &eventID, &eventName, &gate, &item.Staff.ID, &item.Staff.Name, &item.Outcome, &item.RecordedAt, &checkedAt); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan check-in history: %w", err)
		}
		item.ID = strconv.FormatUint(id, 10)
		if code.Valid {
			item.Code = &code.String
		}
		if eventID.Valid {
			item.EventID = &eventID.String
			if eventName.Valid {
				item.EventName = &eventName.String
			}
		}
		if gate.Valid {
			item.Gate = &gate.String
		}
		item.RecordedAt = item.RecordedAt.UTC()
		if checkedAt.Valid {
			value := checkedAt.Time.UTC()
			item.CheckedInAt = &value
		}
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("iterate check-in history: %w", err)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close check-in history: %w", err)
	}
	if len(result.Items) > 50 {
		result.Items = result.Items[:50]
		cursor := result.Items[len(result.Items)-1].ID
		result.NextCursor = &cursor
	}
	result.FilterOptions = make([]HistoryEvent, 0)
	rows, err = tx.QueryContext(ctx, `SELECT DISTINCT a.event_id, COALESCE(e.artist, a.event_id), a.gate
		FROM checkin_attempts a LEFT JOIN events e ON e.id = a.event_id
		WHERE a.event_id IS NOT NULL ORDER BY a.event_id, a.gate`)
	if err != nil {
		return result, fmt.Errorf("query check-in history filters: %w", err)
	}
	byEvent := make(map[string]int)
	for rows.Next() {
		var eventID, eventName string
		var gate sql.NullString
		if err := rows.Scan(&eventID, &eventName, &gate); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan check-in history filters: %w", err)
		}
		index, exists := byEvent[eventID]
		if !exists {
			index = len(result.FilterOptions)
			byEvent[eventID] = index
			result.FilterOptions = append(result.FilterOptions, HistoryEvent{ID: eventID, Name: eventName, Gates: make([]string, 0)})
		}
		if gate.Valid {
			result.FilterOptions[index].Gates = append(result.FilterOptions[index].Gates, gate.String)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("iterate check-in history filters: %w", err)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close check-in history filters: %w", err)
	}
	return result, nil
}
