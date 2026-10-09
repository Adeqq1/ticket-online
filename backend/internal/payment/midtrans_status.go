package payment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/eventstate"
)

type midtransStatus struct {
	OrderID           string `json:"order_id"`
	GrossAmount       string `json:"gross_amount"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
}

type GatewayStatus struct {
	OrderID, GrossAmount, TransactionStatus, FraudStatus string
}

func (r *Repository) readMidtransStatus(ctx context.Context, serverKey, gatewayOrderID string) (midtransStatus, error) {
	endpoint := strings.TrimRight(r.midtransBaseURL, "/") + "/v2/" + url.PathEscape(gatewayOrderID) + "/status"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return midtransStatus{}, err
	}
	req.SetBasicAuth(serverKey, "")
	resp, err := midtransHTTPClient.Do(req)
	if err != nil {
		return midtransStatus{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return midtransStatus{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return midtransStatus{}, fmt.Errorf("Midtrans status API returned %d", resp.StatusCode)
	}
	var result midtransStatus
	if err := json.Unmarshal(body, &result); err != nil {
		return midtransStatus{}, err
	}
	if result.OrderID != gatewayOrderID || result.GrossAmount == "" || result.TransactionStatus == "" {
		return midtransStatus{}, errors.New("Midtrans status response does not match payment attempt")
	}
	return result, nil
}

func (r *Repository) ReadGatewayStatus(ctx context.Context, serverKey, gatewayOrderID string) (GatewayStatus, error) {
	status, err := r.readMidtransStatus(ctx, serverKey, gatewayOrderID)
	if err != nil {
		return GatewayStatus{}, err
	}
	return GatewayStatus{OrderID: status.OrderID, GrossAmount: status.GrossAmount, TransactionStatus: status.TransactionStatus, FraudStatus: status.FraudStatus}, nil
}

func (r *Repository) cancelMidtrans(ctx context.Context, serverKey, gatewayOrderID string) error {
	endpoint := strings.TrimRight(r.midtransBaseURL, "/") + "/v2/" + url.PathEscape(gatewayOrderID) + "/cancel"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader("{}"))
	if err != nil {
		return err
	}
	req.SetBasicAuth(serverKey, "")
	req.Header.Set("Content-Type", "application/json")
	resp, err := midtransHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Midtrans cancel API returned %d", resp.StatusCode)
	}
	return nil
}

func (r *Repository) expireOrderWithMidtrans(ctx context.Context, id, serverKey string) error {
	var orderStatus, paymentStatus, gatewayOrderID string
	var expiresAt time.Time
	var amount uint64
	err := r.db.QueryRowContext(ctx, `SELECT o.status, o.expires_at, COALESCE(p.status, ''),
		COALESCE(p.gateway_order_id, ''), COALESCE(p.amount, 0)
		FROM orders o LEFT JOIN payments p ON p.order_id = o.id WHERE o.id = ?`, id).
		Scan(&orderStatus, &expiresAt, &paymentStatus, &gatewayOrderID, &amount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read order before expiry reconciliation: %w", err)
	}
	if orderStatus != "PENDING" || time.Now().UTC().Before(expiresAt) {
		return nil
	}
	if gatewayOrderID != "" && paymentStatus == "PENDING" {
		if serverKey == "" {
			return errors.New("Midtrans server key unavailable; preserving order stock")
		}
		status, err := r.readMidtransStatus(ctx, serverKey, gatewayOrderID)
		if err != nil {
			return fmt.Errorf("reconcile Midtrans payment before expiry: %w", err)
		}
		if !paymentAmountMatches(amount, status.GrossAmount) {
			return errors.New("Midtrans status amount does not match order; preserving order stock")
		}
		if isGatewaySuccess(status.TransactionStatus, status.FraudStatus) {
			return r.applyGatewayStatus(ctx, gatewayOrderID, status.GrossAmount, status.TransactionStatus, status.FraudStatus)
		}
		if isGatewayFailure(status.TransactionStatus) {
			if err := r.applyGatewayStatus(ctx, gatewayOrderID, status.GrossAmount, status.TransactionStatus, status.FraudStatus); err != nil {
				return err
			}
			return r.expireAfterGatewayFailure(ctx, id, gatewayOrderID)
		}
		if !strings.EqualFold(status.TransactionStatus, "pending") {
			return fmt.Errorf("unrecognized Midtrans status %q; preserving order stock", status.TransactionStatus)
		}
		if err := r.cancelMidtrans(ctx, serverKey, gatewayOrderID); err != nil {
			return fmt.Errorf("cancel pending Midtrans payment before expiry: %w", err)
		}
		status, err = r.readMidtransStatus(ctx, serverKey, gatewayOrderID)
		if err != nil {
			return fmt.Errorf("confirm Midtrans cancellation before expiry: %w", err)
		}
		if !paymentAmountMatches(amount, status.GrossAmount) {
			return errors.New("Midtrans cancellation amount does not match order; preserving order stock")
		}
		if isGatewaySuccess(status.TransactionStatus, status.FraudStatus) {
			return r.applyGatewayStatus(ctx, gatewayOrderID, status.GrossAmount, status.TransactionStatus, status.FraudStatus)
		}
		if !isGatewayFailure(status.TransactionStatus) {
			return fmt.Errorf("Midtrans payment remains %q after cancellation; preserving order stock", status.TransactionStatus)
		}
		if err := r.applyGatewayStatus(ctx, gatewayOrderID, status.GrossAmount, status.TransactionStatus, status.FraudStatus); err != nil {
			return err
		}
		return r.expireAfterGatewayFailure(ctx, id, gatewayOrderID)
	}
	return r.expireOrderLocally(ctx, id, gatewayOrderID)
}

func isGatewaySuccess(status, fraudStatus string) bool {
	return status == "settlement" || (status == "capture" && (fraudStatus == "" || fraudStatus == "accept"))
}

func isGatewayFailure(status string) bool {
	return status == "deny" || status == "cancel" || status == "expire"
}

func paymentAmountMatches(amount uint64, grossAmount string) bool {
	return grossAmount == fmt.Sprintf("%d.00", amount)
}

func (r *Repository) expireOrderLocally(ctx context.Context, id, expectedGatewayOrderID string) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin order expiration: %w", err)
	}
	defer tx.Rollback()
	if _, err := eventstate.ForOrder(ctx, tx, id, true); err != nil {
		return err
	}
	var status, paymentStatus, gatewayOrderID string
	var expiresAt time.Time
	err = tx.QueryRowContext(ctx, `SELECT o.status, o.expires_at, COALESCE(p.status, ''), COALESCE(p.gateway_order_id, '')
		FROM orders o LEFT JOIN payments p ON p.order_id = o.id WHERE o.id = ? FOR UPDATE`, id).
		Scan(&status, &expiresAt, &paymentStatus, &gatewayOrderID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lock order for expiration: %w", err)
	}
	if status != "PENDING" || time.Now().UTC().Before(expiresAt) {
		return nil
	}
	if gatewayOrderID != expectedGatewayOrderID || (gatewayOrderID != "" && paymentStatus == "PENDING") || paymentStatus == "SUCCEEDED" {
		return nil
	}
	if err := expireLockedOrder(ctx, tx, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit order expiration: %w", err)
	}
	return nil
}

func (r *Repository) expireAfterGatewayFailure(ctx context.Context, id, expectedGatewayOrderID string) error {
	return r.expireOrderLocally(ctx, id, expectedGatewayOrderID)
}
