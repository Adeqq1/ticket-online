package payment

import (
	"bytes"
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func midtransSnapURL(environment string) string {
	if environment == "production" {
		return "https://app.midtrans.com/snap/v1/transactions"
	}
	return "https://app.sandbox.midtrans.com/snap/v1/transactions"
}

func midtransRedirectHost(environment string) string {
	if environment == "production" {
		return "app.midtrans.com"
	}
	return "app.sandbox.midtrans.com"
}

var midtransHTTPClient = &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}

type snapRequest struct {
	TransactionDetails struct {
		OrderID     string `json:"order_id"`
		GrossAmount uint64 `json:"gross_amount"`
	} `json:"transaction_details"`
	EnabledPayments []string `json:"enabled_payments"`
	CustomerDetails struct {
		FirstName string `json:"first_name"`
		Email     string `json:"email"`
		Phone     string `json:"phone"`
	} `json:"customer_details"`
	Callbacks struct {
		Finish string `json:"finish"`
	} `json:"callbacks"`
	Expiry struct {
		StartTime string `json:"start_time"`
		Unit      string `json:"unit"`
		Duration  int64  `json:"duration"`
	} `json:"expiry"`
}

type snapResponse struct {
	Token       string `json:"token"`
	RedirectURL string `json:"redirect_url"`
}

func (r *Repository) BeginSnap(ctx context.Context, orderID, method string) (id string, amount uint64, gatewayOrderID, redirect string, err error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return "", 0, "", "", fmt.Errorf("begin snap payment: %w", err)
	}
	defer tx.Rollback()
	var status string
	var expires time.Time
	if err = tx.QueryRowContext(ctx, "SELECT status, total, expires_at FROM orders WHERE id = ? FOR UPDATE", orderID).Scan(&status, &amount, &expires); errors.Is(err, sql.ErrNoRows) {
		return "", 0, "", "", ErrOrderNotFound
	} else if err != nil {
		return "", 0, "", "", err
	}
	if status != "PENDING" || !time.Now().UTC().Before(expires) {
		return "", 0, "", "", ErrOrderNotPayable
	}
	var methodInDB string
	err = tx.QueryRowContext(ctx, "SELECT id, method, amount, status, COALESCE(gateway_order_id, ''), COALESCE(redirect_url, '') FROM payments WHERE order_id = ?", orderID).Scan(&id, &methodInDB, &amount, &status, &gatewayOrderID, &redirect)
	if err == nil {
		if status == "SUCCEEDED" {
			return "", 0, "", "", ErrPaymentConflict
		}
		if status == "PENDING" && methodInDB != method {
			return "", 0, "", "", ErrPaymentConflict
		}
		if status == "PENDING" && redirect != "" {
			if err = tx.Commit(); err != nil {
				return "", 0, "", "", err
			}
			return id, amount, gatewayOrderID, redirect, nil
		}
		if status != "PENDING" && status != "FAILED" {
			return "", 0, "", "", ErrPaymentConflict
		}
		if status == "FAILED" {
			attempt, randomErr := randomID()
			if randomErr != nil {
				return "", 0, "", "", randomErr
			}
			gatewayOrderID = orderID + "-" + attempt[:12]
			if _, err = tx.ExecContext(ctx, "UPDATE payments SET method = ?, amount = ?, status = 'PENDING', paid_at = NULL, gateway_order_id = ?, gateway_reference = NULL, redirect_url = NULL, updated_at = ? WHERE id = ?", method, amount, gatewayOrderID, time.Now().UTC(), id); err != nil {
				return "", 0, "", "", err
			}
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		id, err = randomID()
		if err != nil {
			return "", 0, "", "", err
		}
		gatewayOrderID = orderID + "-" + id[:12]
		now := time.Now().UTC()
		if _, err = tx.ExecContext(ctx, "INSERT INTO payments (id, order_id, method, amount, status, gateway_order_id, created_at, updated_at) VALUES (?, ?, ?, ?, 'PENDING', ?, ?, ?)", id, orderID, method, amount, gatewayOrderID, now, now); err != nil {
			return "", 0, "", "", err
		}
	} else {
		return "", 0, "", "", err
	}
	if err = tx.Commit(); err != nil {
		return "", 0, "", "", err
	}
	return id, amount, gatewayOrderID, "", nil
}

func (r *Repository) SetSnapURL(ctx context.Context, id, gatewayOrderID, token, redirect string) (string, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE payments
		SET gateway_reference = COALESCE(gateway_reference, ?), redirect_url = COALESCE(redirect_url, ?), updated_at = ?
		WHERE id = ? AND gateway_order_id = ? AND status = 'PENDING'`, token, redirect, time.Now().UTC(), id, gatewayOrderID)
	if err != nil {
		return "", err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return "", err
	}
	var currentGatewayOrderID, currentStatus string
	var currentRedirect sql.NullString
	err = r.db.QueryRowContext(ctx, "SELECT gateway_order_id, status, redirect_url FROM payments WHERE id = ?", id).Scan(&currentGatewayOrderID, &currentStatus, &currentRedirect)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrPaymentAttemptChanged
		}
		return "", err
	}
	if currentGatewayOrderID != gatewayOrderID || currentStatus != "PENDING" {
		return "", ErrPaymentAttemptChanged
	}
	if affected == 0 && (!currentRedirect.Valid || currentRedirect.String == "") {
		return "", ErrPaymentAttemptChanged
	}
	if !currentRedirect.Valid || currentRedirect.String == "" {
		return "", ErrPaymentAttemptChanged
	}
	return currentRedirect.String, nil
}

func (r *Repository) ApplyNotification(ctx context.Context, n midtransNotification) error {
	return r.applyGatewayStatus(ctx, n.OrderID, n.GrossAmount, n.TransactionStatus, n.FraudStatus)
}

func (r *Repository) applyGatewayStatus(ctx context.Context, gatewayOrderID, grossAmount, transactionStatus, fraudStatus string) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := r.ApplyGatewayStatusTx(ctx, tx, gatewayOrderID, grossAmount, transactionStatus, fraudStatus); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) ApplyGatewayStatusTx(ctx context.Context, tx *sql.Tx, gatewayOrderID, grossAmount, transactionStatus, fraudStatus string) error {
	var orderID, orderStatus, paymentStatus string
	var amount uint64
	var err error
	if err = tx.QueryRowContext(ctx, "SELECT o.id, o.status, p.status, p.amount FROM orders o JOIN payments p ON p.order_id = o.id WHERE p.gateway_order_id = ? FOR UPDATE", gatewayOrderID).Scan(&orderID, &orderStatus, &paymentStatus, &amount); errors.Is(err, sql.ErrNoRows) {
		return ErrOrderNotFound
	} else if err != nil {
		return err
	}
	if fmt.Sprintf("%d.00", amount) != grossAmount {
		return ErrInvalidRequest
	}
	newStatus := "PENDING"
	succeeded := transactionStatus == "settlement" || (transactionStatus == "capture" && (fraudStatus == "" || fraudStatus == "accept"))
	if succeeded {
		newStatus = "SUCCEEDED"
	} else if transactionStatus == "deny" || transactionStatus == "cancel" || transactionStatus == "expire" {
		newStatus = "FAILED"
	}
	if paymentStatus == "SUCCEEDED" {
		return nil
	}
	if paymentStatus == "FAILED" && newStatus == "PENDING" {
		return nil
	}
	now := time.Now().UTC()
	var paidAt any
	if succeeded {
		if orderStatus != "PENDING" {
			_, err = tx.ExecContext(ctx, `INSERT INTO payment_reconciliation_cases
				(order_id, gateway_order_id, amount, provider_status, created_at, updated_at)
				VALUES (?, ?, ?, ?, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))
				ON DUPLICATE KEY UPDATE provider_status = VALUES(provider_status)`, orderID, gatewayOrderID, amount, transactionStatus)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, "UPDATE payments SET status = 'SUCCEEDED', paid_at = UTC_TIMESTAMP(6), updated_at = UTC_TIMESTAMP(6) WHERE order_id = ?", orderID); err != nil {
				return err
			}
			return nil
		}
		paidAt = now
		if _, err = tx.ExecContext(ctx, "UPDATE orders SET status = 'PAID', updated_at = ? WHERE id = ? AND status = 'PENDING'", now, orderID); err != nil {
			return err
		}
		if _, err = issueAndLoadTickets(ctx, tx, orderID); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE payments SET status = ?, paid_at = ?, updated_at = ? WHERE order_id = ?", newStatus, paidAt, now, orderID); err != nil {
		return err
	}
	return nil
}

type midtransNotification struct {
	OrderID           string `json:"order_id"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	SignatureKey      string `json:"signature_key"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
}

func (h *Handler) CreateSnap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	orderID := strings.ToLower(r.PathValue("orderID"))
	if !validID(orderID) {
		writeError(w, 400, "INVALID_REQUEST", "ID order tidak valid")
		return
	}
	if err := h.authorizeOrder(w, r, orderID); err != nil {
		return
	}
	var input struct {
		Method    string `json:"method"`
		ReturnURL string `json:"returnUrl"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil {
		writeError(w, 400, "INVALID_JSON", "JSON request tidak valid")
		return
	}
	input.Method = strings.ToUpper(strings.TrimSpace(input.Method))
	methodPayment := map[string]string{"QRIS": "qris", "VIRTUAL_ACCOUNT": "bank_transfer", "GOPAY": "gopay"}[input.Method]
	if methodPayment == "" || !h.validReturnURL(input.ReturnURL) {
		writeError(w, 422, "INVALID_REQUEST", "Metode atau URL kembali tidak valid")
		return
	}
	id, amount, gatewayOrderID, redirect, err := h.repository.BeginSnap(r.Context(), orderID, input.Method)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	if redirect != "" {
		if !h.validProviderRedirect(redirect) {
			writeError(w, 502, "PAYMENT_PROVIDER_ERROR", "Sesi pembayaran tidak valid.")
			return
		}
		writeJSON(w, 200, map[string]string{"redirectUrl": redirect})
		return
	}
	var expiresAt time.Time
	if err := h.repository.db.QueryRowContext(r.Context(), "SELECT expires_at FROM orders WHERE id = ?", orderID).Scan(&expiresAt); err != nil {
		h.respondError(w, r, err)
		return
	}
	minutes := int64(time.Until(expiresAt).Minutes())
	if minutes < 1 {
		writeError(w, http.StatusConflict, "ORDER_NOT_PAYABLE", "Order tidak dapat dibayar")
		return
	}
	var payload snapRequest
	payload.TransactionDetails.OrderID, payload.TransactionDetails.GrossAmount = gatewayOrderID, amount
	payload.EnabledPayments = []string{methodPayment}
	payload.CustomerDetails.FirstName, payload.CustomerDetails.Email, payload.CustomerDetails.Phone = "", "", ""
	payload.Callbacks.Finish = input.ReturnURL
	localNow := time.Now().In(time.FixedZone("WIB", 7*60*60))
	payload.Expiry.StartTime = localNow.Format("2006-01-02 15:04:05 -0700")
	payload.Expiry.Unit, payload.Expiry.Duration = "minute", minutes
	// Customer data is intentionally limited to fields required by Midtrans.
	var name, email, phone string
	if err := h.repository.db.QueryRowContext(r.Context(), "SELECT name, email, phone FROM order_buyers WHERE order_id = ?", orderID).Scan(&name, &email, &phone); err != nil {
		h.respondError(w, r, err)
		return
	}
	payload.CustomerDetails.FirstName, payload.CustomerDetails.Email, payload.CustomerDetails.Phone = name, email, phone
	response, err := h.callSnap(r.Context(), payload)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "midtrans snap create failed", "request_id", r.Header.Get("X-Request-ID"), "error", err)
		writeError(w, 502, "PAYMENT_PROVIDER_ERROR", "Sesi pembayaran belum dapat dibuat. Coba lagi.")
		return
	}
	if response.Token == "" || response.RedirectURL == "" {
		writeError(w, 502, "PAYMENT_PROVIDER_ERROR", "Sesi pembayaran tidak valid.")
		return
	}
	providerURL, err := url.Parse(response.RedirectURL)
	if err != nil || providerURL.Scheme != "https" || providerURL.Host != midtransRedirectHost(h.environment) || !strings.HasPrefix(providerURL.Path, "/snap/") || providerURL.User != nil {
		writeError(w, 502, "PAYMENT_PROVIDER_ERROR", "Sesi pembayaran tidak valid.")
		return
	}
	redirectURL, err := h.repository.SetSnapURL(r.Context(), id, gatewayOrderID, response.Token, response.RedirectURL)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]string{"redirectUrl": redirectURL})
}

// Kept separate so configuration, return URL and provider calls remain server-owned.
func (h *Handler) validReturnURL(value string) bool {
	got, err := url.Parse(value)
	if err != nil {
		return false
	}
	base, err := url.Parse(h.frontendURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || got.Scheme != base.Scheme || got.Host != base.Host || got.User != nil || got.Fragment != "" || !strings.HasPrefix(got.Path, "/checkout/") {
		return false
	}
	return true
}

func (h *Handler) validProviderRedirect(value string) bool {
	providerURL, err := url.Parse(value)
	return err == nil && providerURL.Scheme == "https" && providerURL.Host == midtransRedirectHost(h.environment) && strings.HasPrefix(providerURL.Path, "/snap/") && providerURL.User == nil
}

func (h *Handler) callSnap(ctx context.Context, payload snapRequest) (snapResponse, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, midtransSnapURL(h.environment), bytes.NewReader(body))
	if err != nil {
		return snapResponse{}, err
	}
	req.SetBasicAuth(h.serverKey, "")
	req.Header.Set("Content-Type", "application/json")
	resp, err := midtransHTTPClient.Do(req)
	if err != nil {
		return snapResponse{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return snapResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return snapResponse{}, fmt.Errorf("provider status %d", resp.StatusCode)
	}
	var result snapResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return snapResponse{}, err
	}
	return result, nil
}

func (h *Handler) MidtransNotification(w http.ResponseWriter, r *http.Request) {
	var n midtransNotification
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&n); err != nil {
		writeError(w, 400, "INVALID_JSON", "Notifikasi tidak valid")
		return
	}
	hash := sha512.Sum512([]byte(n.OrderID + n.StatusCode + n.GrossAmount + h.serverKey))
	expected, err := hex.DecodeString(n.SignatureKey)
	if err != nil || len(expected) != len(hash) || subtle.ConstantTimeCompare(expected, hash[:]) != 1 {
		writeError(w, 401, "INVALID_SIGNATURE", "Tanda tangan notifikasi tidak valid")
		return
	}
	if err := h.repository.ApplyNotification(r.Context(), n); err != nil {
		h.respondError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
