package checkin

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
		len(request.Code) != 35 || !strings.HasPrefix(request.Code, "ET-") {
		return Request{}, ErrInvalidRequest
	}
	if _, err := hex.DecodeString(request.Code[3:]); err != nil {
		return Request{}, ErrInvalidRequest
	}
	return request, nil
}

func (s *Service) CheckIn(ctx context.Context, token string, request Request) (Result, error) {
	request, err := Normalize(request)
	if err != nil {
		return Result{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Result{}, fmt.Errorf("begin check-in: %w", err)
	}
	defer tx.Rollback()

	principal, err := s.staff.AuthorizeGateTx(ctx, tx, token, request.EventID, request.Gate)
	if err != nil {
		return Result{}, err
	}
	ticketID := strings.ToLower(request.Code[3:])
	var orderID string
	err = tx.QueryRowContext(ctx, "SELECT order_id FROM etickets WHERE id = ?", ticketID).Scan(&orderID)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, ErrTicketNotFound
	}
	if err != nil {
		return Result{}, fmt.Errorf("find ticket order: %w", err)
	}
	var orderStatus string
	var eventID string
	err = tx.QueryRowContext(ctx, `SELECT o.status, r.event_id FROM orders o
		JOIN reservations r ON r.id = o.reservation_id WHERE o.id = ? FOR UPDATE`, orderID).Scan(&orderStatus, &eventID)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, ErrTicketNotFound
	}
	if err != nil {
		return Result{}, fmt.Errorf("lock ticket order: %w", err)
	}
	if orderStatus != "PAID" {
		return Result{}, ErrOrderNotPaid
	}
	if eventID != request.EventID {
		return Result{}, ErrTicketNotFound
	}

	var snapshot []byte
	err = tx.QueryRowContext(ctx, "SELECT snapshot FROM etickets WHERE id = ? FOR UPDATE", ticketID).Scan(&snapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, ErrTicketNotFound
	}
	if err != nil {
		return Result{}, fmt.Errorf("lock ticket for check-in: %w", err)
	}
	var ticket Ticket
	if err := json.Unmarshal(snapshot, &ticket); err != nil {
		return Result{}, fmt.Errorf("decode ticket snapshot: %w", err)
	}
	if ticket.ID != ticketID || ticket.EventID != eventID {
		return Result{}, ErrTicketNotFound
	}
	if ticket.Gate != request.Gate {
		return Result{Ticket: ticket}, ErrWrongGate
	}
	var checkedInAt sql.NullTime
	err = tx.QueryRowContext(ctx, "SELECT checked_in_at FROM ticket_checkins WHERE ticket_id = ?", ticketID).Scan(&checkedInAt)
	if err == nil {
		return Result{Status: "ALREADY_USED", Ticket: ticket, CheckedInAt: checkedInAt.Time.UTC()}, ErrTicketUsed
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Result{}, fmt.Errorf("find ticket check-in: %w", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := tx.ExecContext(ctx, `INSERT INTO ticket_checkins
		(ticket_id, staff_id, event_id, gate, checked_in_at) VALUES (?, ?, ?, ?, ?)`, ticketID, principal.ID, eventID, request.Gate, now); err != nil {
		return Result{}, fmt.Errorf("record ticket check-in: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("commit check-in: %w", err)
	}
	return Result{Status: "CHECKED_IN", Ticket: ticket, CheckedInAt: now}, nil
}
