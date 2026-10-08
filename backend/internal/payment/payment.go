package payment

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidRequest        = errors.New("invalid payment request")
	ErrOrderNotFound         = errors.New("order not found")
	ErrOrderNotPayable       = errors.New("order cannot be paid")
	ErrPaymentConflict       = errors.New("payment result conflicts with existing payment")
	ErrPaymentAttemptChanged = errors.New("payment attempt changed while creating Snap session")
	ErrTicketNotFound        = errors.New("e-ticket not found")
	ErrIncompleteTickets     = errors.New("order has an incomplete e-ticket set")
)

type Request struct {
	Method string `json:"method"`
	Result string `json:"result"`
}

type Payment struct {
	ID          string   `json:"id"`
	OrderID     string   `json:"orderId"`
	OrderStatus string   `json:"orderStatus"`
	Method      string   `json:"method"`
	Amount      uint64   `json:"amount"`
	Status      string   `json:"status"`
	PaidAt      string   `json:"paidAt,omitempty"`
	Tickets     []Ticket `json:"tickets"`
}

type Ticket struct {
	ID             string `json:"id"`
	Code           string `json:"code"`
	AttendeeName   string `json:"attendeeName"`
	OrderReference string `json:"orderReference"`
	EventID        string `json:"eventId"`
	EventArtist    string `json:"eventArtist"`
	EventCity      string `json:"eventCity"`
	EventVenue     string `json:"eventVenue"`
	EventAddress   string `json:"eventAddress"`
	EventStartsAt  string `json:"eventStartsAt"`
	TierName       string `json:"tierName"`
	Gate           string `json:"gate"`
	IssuedAt       string `json:"issuedAt"`
}

func Validate(request Request) (Request, error) {
	request.Method = strings.ToUpper(strings.TrimSpace(request.Method))
	request.Result = strings.ToUpper(strings.TrimSpace(request.Result))
	switch request.Method {
	case "QRIS", "VIRTUAL_ACCOUNT", "GOPAY":
	default:
		return Request{}, ErrInvalidRequest
	}
	if request.Result != "SUCCEEDED" && request.Result != "FAILED" {
		return Request{}, ErrInvalidRequest
	}
	return request, nil
}

type Repository struct {
	db              *sql.DB
	midtransBaseURL string
}

func NewRepository(db *sql.DB) *Repository {
	return NewRepositoryWithMidtransEnvironment(db, "sandbox")
}

func NewRepositoryWithMidtransEnvironment(db *sql.DB, environment string) *Repository {
	baseURL := "https://api.sandbox.midtrans.com"
	if environment == "production" {
		baseURL = "https://api.midtrans.com"
	}
	return &Repository{db: db, midtransBaseURL: baseURL}
}

func (r *Repository) Simulate(ctx context.Context, orderID string, request Request) (Payment, bool, error) {
	request, err := Validate(request)
	if err != nil {
		return Payment{}, false, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Payment{}, false, fmt.Errorf("begin payment: %w", err)
	}
	defer tx.Rollback()

	var orderStatus string
	var amount uint64
	var expiresAt time.Time
	err = tx.QueryRowContext(ctx, "SELECT status, total, expires_at FROM orders WHERE id = ? FOR UPDATE", orderID).Scan(&orderStatus, &amount, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Payment{}, false, ErrOrderNotFound
	}
	if err != nil {
		return Payment{}, false, fmt.Errorf("lock order for payment: %w", err)
	}
	if orderStatus != "PENDING" && orderStatus != "PAID" {
		return Payment{}, false, ErrOrderNotPayable
	}
	if orderStatus == "PENDING" && !time.Now().UTC().Before(expiresAt) {
		if err := expireLockedOrder(ctx, tx, orderID); err != nil {
			return Payment{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return Payment{}, false, fmt.Errorf("commit order expiration during payment: %w", err)
		}
		return Payment{}, false, ErrOrderNotPayable
	}

	var existing Payment
	var paidAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT id, order_id, method, amount, status, paid_at
		FROM payments WHERE order_id = ?`, orderID).Scan(&existing.ID, &existing.OrderID, &existing.Method, &existing.Amount, &existing.Status, &paidAt)
	if err == nil {
		if orderStatus == "PAID" {
			if existing.Status == "SUCCEEDED" && request.Result == "SUCCEEDED" && request.Method == existing.Method {
				existing.OrderStatus = orderStatus
				existing.PaidAt = formatTime(paidAt)
				existing.Tickets, err = issueAndLoadTickets(ctx, tx, orderID)
				if err != nil {
					return Payment{}, false, err
				}
				if err := tx.Commit(); err != nil {
					return Payment{}, false, fmt.Errorf("commit ticket replay: %w", err)
				}
				return existing, true, nil
			}
			return Payment{}, false, ErrPaymentConflict
		}
		if existing.Status == "SUCCEEDED" {
			return Payment{}, false, ErrPaymentConflict
		}
		if existing.Method == request.Method && request.Result == "FAILED" {
			existing.OrderStatus = orderStatus
			existing.Tickets = []Ticket{}
			return existing, true, nil
		}
		now := time.Now().UTC()
		var succeededAt any
		if request.Result == "SUCCEEDED" {
			succeededAt = now
		}
		if _, err := tx.ExecContext(ctx, "UPDATE payments SET method = ?, amount = ?, status = ?, started_at = COALESCE(started_at, ?), paid_at = ?, updated_at = ? WHERE order_id = ?", request.Method, amount, request.Result, now, succeededAt, now, orderID); err != nil {
			return Payment{}, false, fmt.Errorf("update payment: %w", err)
		}
		if request.Result == "FAILED" {
			_, _ = tx.ExecContext(ctx, `INSERT IGNORE INTO conversion_events (journey_id,kind,reason,created_at)
				SELECT cr.journey_id,'PAYMENT_FAILURE','PAYMENT_PROVIDER',UTC_TIMESTAMP(6) FROM conversion_reservations cr JOIN orders o ON o.reservation_id=cr.reservation_id WHERE o.id=?`, orderID)
		}
		existing.Method = request.Method
		existing.Amount = amount
		existing.Status = request.Result
		existing.PaidAt = formatTime(sql.NullTime{Time: now, Valid: request.Result == "SUCCEEDED"})
		if request.Result == "SUCCEEDED" {
			if _, err := tx.ExecContext(ctx, "UPDATE orders SET status = 'PAID', updated_at = ? WHERE id = ? AND status = 'PENDING'", now, orderID); err != nil {
				return Payment{}, false, fmt.Errorf("mark order paid: %w", err)
			}
			existing.OrderStatus = "PAID"
			existing.Tickets, err = issueAndLoadTickets(ctx, tx, orderID)
			if err != nil {
				return Payment{}, false, err
			}
		} else {
			existing.OrderStatus = orderStatus
			existing.Tickets = []Ticket{}
		}
		if err := tx.Commit(); err != nil {
			return Payment{}, false, fmt.Errorf("commit payment: %w", err)
		}
		return existing, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Payment{}, false, fmt.Errorf("find payment: %w", err)
	}
	if orderStatus != "PENDING" {
		return Payment{}, false, ErrOrderNotPayable
	}
	existing.ID, err = randomID()
	if err != nil {
		return Payment{}, false, err
	}
	now := time.Now().UTC()
	var succeededAt any
	if request.Result == "SUCCEEDED" {
		succeededAt = now
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO payments (id, order_id, method, amount, status, started_at, paid_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, existing.ID, orderID, request.Method, amount, request.Result, now, succeededAt, now, now); err != nil {
		return Payment{}, false, fmt.Errorf("insert payment: %w", err)
	}
	if request.Result == "FAILED" {
		_, _ = tx.ExecContext(ctx, `INSERT IGNORE INTO conversion_events (journey_id,kind,reason,created_at)
			SELECT cr.journey_id,'PAYMENT_FAILURE','PAYMENT_PROVIDER',UTC_TIMESTAMP(6) FROM conversion_reservations cr JOIN orders o ON o.reservation_id=cr.reservation_id WHERE o.id=?`, orderID)
	}
	existing.OrderID = orderID
	existing.Method = request.Method
	existing.Amount = amount
	existing.Status = request.Result
	existing.PaidAt = formatTime(sql.NullTime{Time: now, Valid: request.Result == "SUCCEEDED"})
	existing.OrderStatus = orderStatus
	existing.Tickets = []Ticket{}
	if request.Result == "SUCCEEDED" {
		if _, err := tx.ExecContext(ctx, "UPDATE orders SET status = 'PAID', updated_at = ? WHERE id = ? AND status = 'PENDING'", now, orderID); err != nil {
			return Payment{}, false, fmt.Errorf("mark order paid: %w", err)
		}
		existing.OrderStatus = "PAID"
		existing.Tickets, err = issueAndLoadTickets(ctx, tx, orderID)
		if err != nil {
			return Payment{}, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Payment{}, false, fmt.Errorf("commit payment: %w", err)
	}
	return existing, false, nil
}

func issueAndLoadTickets(ctx context.Context, tx *sql.Tx, orderID string) ([]Ticket, error) {
	var expected, existing uint64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(quantity), 0) FROM order_items WHERE order_id = ?", orderID).Scan(&expected); err != nil {
		return nil, fmt.Errorf("count ordered tickets: %w", err)
	}
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM etickets WHERE order_id = ?", orderID).Scan(&existing); err != nil {
		return nil, fmt.Errorf("count e-tickets: %w", err)
	}
	if existing != 0 && existing != expected {
		return nil, ErrIncompleteTickets
	}
	if existing == 0 && expected > 0 {
		var attendeeCount uint64
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM order_attendees WHERE order_id = ?", orderID).Scan(&attendeeCount); err != nil {
			return nil, fmt.Errorf("count order attendees: %w", err)
		}
		if attendeeCount != 0 && attendeeCount != expected {
			return nil, ErrIncompleteTickets
		}
		var buyerName string
		if err := tx.QueryRowContext(ctx, "SELECT name FROM order_buyers WHERE order_id = ?", orderID).Scan(&buyerName); err != nil {
			return nil, fmt.Errorf("read buyer for ticket fallback: %w", err)
		}
		rows, err := tx.QueryContext(ctx, "SELECT oi.ticket_tier_id, oi.quantity, oi.tier_name, tt.gate, r.event_id, o.reference, e.artist, e.city, e.venue, e.address, DATE_FORMAT(e.starts_at, '%Y-%m-%dT%H:%i:%s.%fZ') FROM order_items oi JOIN orders o ON o.id = oi.order_id JOIN reservations r ON r.id = o.reservation_id JOIN events e ON e.id = r.event_id JOIN ticket_tiers tt ON tt.id = oi.ticket_tier_id WHERE oi.order_id = ? ORDER BY oi.ticket_tier_id", orderID)
		if err != nil {
			return nil, fmt.Errorf("read order ticket snapshots: %w", err)
		}
		type itemSnapshot struct {
			tierID, quantity                                             uint64
			tier, gate, eventID, reference, artist, city, venue, address string
			starts                                                       string
		}
		var items []itemSnapshot
		for rows.Next() {
			var item itemSnapshot
			if err := rows.Scan(&item.tierID, &item.quantity, &item.tier, &item.gate, &item.eventID, &item.reference, &item.artist, &item.city, &item.venue, &item.address, &item.starts); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan ticket snapshot source: %w", err)
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("iterate ticket snapshot sources: %w", err)
		}
		if err := rows.Close(); err != nil {
			return nil, fmt.Errorf("close ticket snapshot sources: %w", err)
		}
		for _, item := range items {
			for number := uint64(1); number <= item.quantity; number++ {
				var attendee string
				err := tx.QueryRowContext(ctx, "SELECT name FROM order_attendees WHERE order_id = ? AND ticket_tier_id = ? AND ticket_number = ?", orderID, item.tierID, number).Scan(&attendee)
				if errors.Is(err, sql.ErrNoRows) && attendeeCount == 0 {
					attendee = buyerName
					if _, err := tx.ExecContext(ctx, "INSERT INTO order_attendees (order_id, ticket_tier_id, ticket_number, name) VALUES (?, ?, ?, ?)", orderID, item.tierID, number, attendee); err != nil {
						return nil, fmt.Errorf("save legacy ticket attendee: %w", err)
					}
				} else if errors.Is(err, sql.ErrNoRows) {
					return nil, ErrIncompleteTickets
				} else if err != nil {
					return nil, fmt.Errorf("read ticket attendee: %w", err)
				}
				ticketID, err := randomID()
				if err != nil {
					return nil, err
				}
				now := time.Now().UTC()
				ticket := Ticket{ID: ticketID, Code: "ET-" + strings.ToUpper(ticketID), AttendeeName: attendee, OrderReference: item.reference, EventID: item.eventID, EventArtist: item.artist, EventCity: item.city, EventVenue: item.venue, EventAddress: item.address, EventStartsAt: item.starts, TierName: item.tier, Gate: item.gate, IssuedAt: now.Format(time.RFC3339Nano)}
				snapshot, err := json.Marshal(ticket)
				if err != nil {
					return nil, fmt.Errorf("marshal ticket snapshot: %w", err)
				}
				if _, err := tx.ExecContext(ctx, "INSERT INTO etickets (id, order_id, ticket_tier_id, ticket_number, snapshot, issued_at) VALUES (?, ?, ?, ?, ?, ?)", ticketID, orderID, item.tierID, number, snapshot, now); err != nil {
					return nil, fmt.Errorf("insert e-ticket: %w", err)
				}
			}
		}
	}
	return loadTickets(ctx, tx, "order_id = ?", orderID)
}

func loadTickets(ctx context.Context, tx *sql.Tx, predicate string, args ...any) ([]Ticket, error) {
	rows, err := tx.QueryContext(ctx, "SELECT snapshot FROM etickets WHERE "+predicate+" ORDER BY issued_at, id", args...)
	if err != nil {
		return nil, fmt.Errorf("read e-tickets: %w", err)
	}
	defer rows.Close()
	tickets := []Ticket{}
	for rows.Next() {
		var snapshot []byte
		var ticket Ticket
		if err := rows.Scan(&snapshot); err != nil {
			return nil, fmt.Errorf("scan e-ticket: %w", err)
		}
		if err := json.Unmarshal(snapshot, &ticket); err != nil {
			return nil, fmt.Errorf("decode e-ticket snapshot: %w", err)
		}
		tickets = append(tickets, ticket)
	}
	return tickets, rows.Err()
}

func (r *Repository) TicketsForOrder(ctx context.Context, orderID string) ([]Ticket, error) {
	var status string
	if err := r.db.QueryRowContext(ctx, "SELECT status FROM orders WHERE id = ?", orderID).Scan(&status); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrOrderNotFound
	} else if err != nil {
		return nil, fmt.Errorf("find order for e-tickets: %w", err)
	}
	if status != "PAID" {
		return nil, ErrOrderNotFound
	}
	rows, err := r.db.QueryContext(ctx, "SELECT snapshot FROM etickets WHERE order_id = ? ORDER BY issued_at, id", orderID)
	if err != nil {
		return nil, fmt.Errorf("read order e-tickets: %w", err)
	}
	defer rows.Close()
	return decodeTickets(rows)
}

func (r *Repository) Ticket(ctx context.Context, ticketID string) (Ticket, error) {
	var snapshot []byte
	if err := r.db.QueryRowContext(ctx, "SELECT t.snapshot FROM etickets t JOIN orders o ON o.id=t.order_id WHERE t.id = ? AND o.status='PAID'", ticketID).Scan(&snapshot); errors.Is(err, sql.ErrNoRows) {
		return Ticket{}, ErrTicketNotFound
	} else if err != nil {
		return Ticket{}, fmt.Errorf("read e-ticket: %w", err)
	}
	var ticket Ticket
	if err := json.Unmarshal(snapshot, &ticket); err != nil {
		return Ticket{}, fmt.Errorf("decode e-ticket snapshot: %w", err)
	}
	return ticket, nil
}

func decodeTickets(rows *sql.Rows) ([]Ticket, error) {
	tickets := []Ticket{}
	for rows.Next() {
		var snapshot []byte
		var ticket Ticket
		if err := rows.Scan(&snapshot); err != nil {
			return nil, fmt.Errorf("scan e-ticket: %w", err)
		}
		if err := json.Unmarshal(snapshot, &ticket); err != nil {
			return nil, fmt.Errorf("decode e-ticket snapshot: %w", err)
		}
		tickets = append(tickets, ticket)
	}
	return tickets, rows.Err()
}

func formatTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339Nano)
}

func randomID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate payment id: %w", err)
	}
	return hex.EncodeToString(bytes[:]), nil
}
