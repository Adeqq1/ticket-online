package checkout

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

const adminFee uint64 = 7500

var (
	ErrInvalidRequest       = errors.New("invalid checkout request")
	ErrInvalidVoucher       = errors.New("invalid voucher")
	ErrReservationNotFound  = errors.New("reservation not found")
	ErrReservationExpired   = errors.New("reservation expired")
	ErrReservationCancelled = errors.New("reservation cancelled")
	ErrReservationConverted = errors.New("reservation already converted")
	ErrIdempotencyConflict  = errors.New("checkout request conflicts with existing order")
)

type Buyer struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Identity string `json:"identity"`
}

type Request struct {
	Buyer       Buyer  `json:"buyer"`
	VoucherCode string `json:"voucherCode,omitempty"`
}

type Item struct {
	TierID       string `json:"tierId"`
	ticketTierID uint64
	Name         string `json:"name"`
	Quantity     uint64 `json:"quantity"`
	UnitPrice    uint64 `json:"unitPrice"`
	LineTotal    uint64 `json:"lineTotal"`
}

type Order struct {
	ID            string `json:"id"`
	Reference     string `json:"reference"`
	ReservationID string `json:"reservationId"`
	Status        string `json:"status"`
	Subtotal      uint64 `json:"subtotal"`
	AdminFee      uint64 `json:"adminFee"`
	Discount      uint64 `json:"discount"`
	Total         uint64 `json:"total"`
	Items         []Item `json:"items"`
}

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func Validate(request Request) (Request, error) {
	request.Buyer.Name = strings.TrimSpace(request.Buyer.Name)
	request.Buyer.Email = strings.TrimSpace(request.Buyer.Email)
	request.Buyer.Phone = strings.TrimSpace(request.Buyer.Phone)
	request.Buyer.Identity = strings.TrimSpace(request.Buyer.Identity)
	if utf8.RuneCountInString(request.Buyer.Name) < 2 || utf8.RuneCountInString(request.Buyer.Name) > 80 || len(request.Buyer.Email) > 254 {
		return Request{}, ErrInvalidRequest
	}
	address, err := mail.ParseAddress(request.Buyer.Email)
	if err != nil || address.Address != request.Buyer.Email || !strings.Contains(strings.SplitN(request.Buyer.Email, "@", 2)[1], ".") {
		return Request{}, ErrInvalidRequest
	}
	request.Buyer.Phone = strings.NewReplacer(" ", "", "-", "").Replace(request.Buyer.Phone)
	phone := strings.TrimPrefix(request.Buyer.Phone, "+")
	if len(phone) < 9 || len(phone) > 15 || !digits(phone) || len(request.Buyer.Identity) < 12 || len(request.Buyer.Identity) > 20 || !digits(request.Buyer.Identity) {
		return Request{}, ErrInvalidRequest
	}
	request.VoucherCode = strings.ToUpper(strings.TrimSpace(request.VoucherCode))
	if request.VoucherCode != "" && request.VoucherCode != "HEMAT10" {
		return Request{}, ErrInvalidVoucher
	}
	return request, nil
}

func digits(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func requestHash(reservationID string, request Request) (string, error) {
	data, err := json.Marshal(struct {
		ReservationID string `json:"reservationId"`
		Buyer         Buyer  `json:"buyer"`
		VoucherCode   string `json:"voucherCode"`
	}{reservationID, request.Buyer, request.VoucherCode})
	if err != nil {
		return "", fmt.Errorf("marshal checkout hash: %w", err)
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func (r *Repository) Create(ctx context.Context, reservationID string, request Request) (Order, bool, error) {
	request, err := Validate(request)
	if err != nil {
		return Order{}, false, err
	}
	hash, err := requestHash(reservationID, request)
	if err != nil {
		return Order{}, false, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Order{}, false, fmt.Errorf("begin checkout: %w", err)
	}
	defer tx.Rollback()

	var status string
	var expiresAt time.Time
	err = tx.QueryRowContext(ctx, "SELECT status, expires_at FROM reservations WHERE id = ? FOR UPDATE", reservationID).Scan(&status, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, false, ErrReservationNotFound
	}
	if err != nil {
		return Order{}, false, fmt.Errorf("lock reservation: %w", err)
	}

	var existing Order
	var existingHash sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT id, reference, reservation_id, status, subtotal, admin_fee, discount, total, request_hash
		FROM orders WHERE reservation_id = ?`, reservationID).Scan(&existing.ID, &existing.Reference, &existing.ReservationID, &existing.Status, &existing.Subtotal, &existing.AdminFee, &existing.Discount, &existing.Total, &existingHash)
	if err == nil {
		if !existingHash.Valid || existingHash.String != hash {
			return Order{}, false, ErrIdempotencyConflict
		}
		if err := loadItems(ctx, tx, &existing); err != nil {
			return Order{}, false, err
		}
		return existing, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Order{}, false, fmt.Errorf("find existing order: %w", err)
	}
	switch status {
	case "CANCELLED":
		return Order{}, false, ErrReservationCancelled
	case "EXPIRED":
		return Order{}, false, ErrReservationExpired
	case "CONVERTED":
		return Order{}, false, ErrReservationConverted
	case "ACTIVE":
		if !time.Now().UTC().Before(expiresAt) {
			return Order{}, false, ErrReservationExpired
		}
	default:
		return Order{}, false, ErrInvalidRequest
	}

	rows, err := tx.QueryContext(ctx, `SELECT ri.ticket_tier_id, tt.slug, tt.name, ri.quantity, ri.unit_price
		FROM reservation_items ri JOIN ticket_tiers tt ON tt.id = ri.ticket_tier_id
		WHERE ri.reservation_id = ? ORDER BY ri.ticket_tier_id`, reservationID)
	if err != nil {
		return Order{}, false, fmt.Errorf("read reservation items: %w", err)
	}
	order := Order{ReservationID: reservationID, Status: "PENDING", AdminFee: adminFee}
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ticketTierID, &item.TierID, &item.Name, &item.Quantity, &item.UnitPrice); err != nil {
			rows.Close()
			return Order{}, false, fmt.Errorf("scan reservation item: %w", err)
		}
		if item.UnitPrice == 0 || item.Quantity == 0 || item.Quantity > math.MaxUint64/item.UnitPrice {
			rows.Close()
			return Order{}, false, ErrInvalidRequest
		}
		item.LineTotal = item.Quantity * item.UnitPrice
		if math.MaxUint64-order.Subtotal < item.LineTotal {
			rows.Close()
			return Order{}, false, ErrInvalidRequest
		}
		order.Subtotal += item.LineTotal
		order.Items = append(order.Items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Order{}, false, fmt.Errorf("iterate reservation items: %w", err)
	}
	if err := rows.Close(); err != nil {
		return Order{}, false, fmt.Errorf("close reservation items: %w", err)
	}
	if len(order.Items) == 0 || math.MaxUint64-order.Subtotal < order.AdminFee {
		return Order{}, false, ErrInvalidRequest
	}
	if request.VoucherCode == "HEMAT10" {
		order.Discount = order.Subtotal / 10
	}
	order.Total = order.Subtotal + order.AdminFee - order.Discount
	order.ID, err = randomID()
	if err != nil {
		return Order{}, false, err
	}
	order.Reference, err = randomReference()
	if err != nil {
		return Order{}, false, err
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `INSERT INTO orders
		(id, reference, reservation_id, status, subtotal, admin_fee, discount, created_at, updated_at, request_hash)
		VALUES (?, ?, ?, 'PENDING', ?, ?, ?, ?, ?, ?)`, order.ID, order.Reference, reservationID, order.Subtotal, order.AdminFee, order.Discount, now, now, hash); err != nil {
		return Order{}, false, fmt.Errorf("insert order: %w", err)
	}
	for _, item := range order.Items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_items
			(order_id, ticket_tier_id, tier_name, quantity, unit_price) VALUES (?, ?, ?, ?, ?)`, order.ID, item.ticketTierID, item.Name, item.Quantity, item.UnitPrice); err != nil {
			return Order{}, false, fmt.Errorf("insert order item: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO order_buyers (order_id, name, email, phone, identity) VALUES (?, ?, ?, ?, ?)`, order.ID, request.Buyer.Name, request.Buyer.Email, request.Buyer.Phone, request.Buyer.Identity); err != nil {
		return Order{}, false, fmt.Errorf("insert order buyer: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE reservations SET status = 'CONVERTED', order_reference = ?, updated_at = ? WHERE id = ? AND status = 'ACTIVE'", order.Reference, now, reservationID); err != nil {
		return Order{}, false, fmt.Errorf("convert reservation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Order{}, false, fmt.Errorf("commit checkout: %w", err)
	}
	return order, false, nil
}

func loadItems(ctx context.Context, tx *sql.Tx, order *Order) error {
	rows, err := tx.QueryContext(ctx, `SELECT oi.ticket_tier_id, tt.slug, oi.tier_name, oi.quantity, oi.unit_price, oi.line_total
		FROM order_items oi JOIN ticket_tiers tt ON tt.id = oi.ticket_tier_id WHERE oi.order_id = ? ORDER BY oi.ticket_tier_id`, order.ID)
	if err != nil {
		return fmt.Errorf("read existing order items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ticketTierID, &item.TierID, &item.Name, &item.Quantity, &item.UnitPrice, &item.LineTotal); err != nil {
			return fmt.Errorf("scan existing order item: %w", err)
		}
		order.Items = append(order.Items, item)
	}
	return rows.Err()
}

func randomID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate order id: %w", err)
	}
	return hex.EncodeToString(bytes[:]), nil
}

func randomReference() (string, error) {
	var bytes [10]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate order reference: %w", err)
	}
	return "TO-" + hex.EncodeToString(bytes[:]), nil
}
