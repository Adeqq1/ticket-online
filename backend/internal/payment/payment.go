package payment

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidRequest  = errors.New("invalid payment request")
	ErrOrderNotFound   = errors.New("order not found")
	ErrOrderNotPayable = errors.New("order cannot be paid")
	ErrPaymentConflict = errors.New("payment result conflicts with existing payment")
)

type Request struct {
	Method string `json:"method"`
	Result string `json:"result"`
}

type Payment struct {
	ID          string `json:"id"`
	OrderID     string `json:"orderId"`
	OrderStatus string `json:"orderStatus"`
	Method      string `json:"method"`
	Amount      uint64 `json:"amount"`
	Status      string `json:"status"`
	PaidAt      string `json:"paidAt,omitempty"`
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

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

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
	err = tx.QueryRowContext(ctx, "SELECT status, total FROM orders WHERE id = ? FOR UPDATE", orderID).Scan(&orderStatus, &amount)
	if errors.Is(err, sql.ErrNoRows) {
		return Payment{}, false, ErrOrderNotFound
	}
	if err != nil {
		return Payment{}, false, fmt.Errorf("lock order for payment: %w", err)
	}
	if orderStatus != "PENDING" && orderStatus != "PAID" {
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
				return existing, true, nil
			}
			return Payment{}, false, ErrPaymentConflict
		}
		if existing.Status == "SUCCEEDED" {
			return Payment{}, false, ErrPaymentConflict
		}
		if existing.Method == request.Method && request.Result == "FAILED" {
			existing.OrderStatus = orderStatus
			return existing, true, nil
		}
		now := time.Now().UTC()
		var succeededAt any
		if request.Result == "SUCCEEDED" {
			succeededAt = now
		}
		if _, err := tx.ExecContext(ctx, "UPDATE payments SET method = ?, amount = ?, status = ?, paid_at = ?, updated_at = ? WHERE order_id = ?", request.Method, amount, request.Result, succeededAt, now, orderID); err != nil {
			return Payment{}, false, fmt.Errorf("update payment: %w", err)
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
		} else {
			existing.OrderStatus = orderStatus
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
	if _, err := tx.ExecContext(ctx, `INSERT INTO payments (id, order_id, method, amount, status, paid_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, existing.ID, orderID, request.Method, amount, request.Result, succeededAt, now, now); err != nil {
		return Payment{}, false, fmt.Errorf("insert payment: %w", err)
	}
	existing.OrderID = orderID
	existing.Method = request.Method
	existing.Amount = amount
	existing.Status = request.Result
	existing.PaidAt = formatTime(sql.NullTime{Time: now, Valid: request.Result == "SUCCEEDED"})
	existing.OrderStatus = orderStatus
	if request.Result == "SUCCEEDED" {
		if _, err := tx.ExecContext(ctx, "UPDATE orders SET status = 'PAID', updated_at = ? WHERE id = ? AND status = 'PENDING'", now, orderID); err != nil {
			return Payment{}, false, fmt.Errorf("mark order paid: %w", err)
		}
		existing.OrderStatus = "PAID"
	}
	if err := tx.Commit(); err != nil {
		return Payment{}, false, fmt.Errorf("commit payment: %w", err)
	}
	return existing, false, nil
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
