package orderaccess

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

func Bearer(header string) (string, bool) {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" || strings.ContainsAny(parts[1], " \t\r\n") {
		return "", false
	}
	return parts[1], true
}

var (
	ErrUnauthorized = errors.New("order access denied")
	ErrExpired      = errors.New("order access expired")
	ErrNotFound     = errors.New("order access target not found")
)

type Access struct {
	db     *sql.DB
	secret []byte
}

func New(db *sql.DB, secret []byte) *Access { return &Access{db: db, secret: secret} }

func ParseSecret(value string) ([]byte, error) {
	secret, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(secret) != 32 {
		return nil, errors.New("ORDER_ACCESS_SECRET must be base64 encoding of 32 random bytes")
	}
	return secret, nil
}

func (a *Access) Token(orderID string) string {
	// ponytail: tokens cannot be revoked per order; add stored token versions if individual revocation is needed.
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte("ticket-online/order-access/v1:" + orderID))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Digest returns a hex HMAC for values that must be matched without being stored, such as limiter identities.
func (a *Access) Digest(purpose string, values ...string) string {
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte("ticket-online/" + purpose + "/v1"))
	for _, value := range values {
		_, _ = mac.Write([]byte{0})
		_, _ = mac.Write([]byte(value))
	}
	return hex.EncodeToString(mac.Sum(nil))
}

func (a *Access) Issue(ctx context.Context, orderID string) (string, time.Time, error) {
	var startsAt time.Time
	if err := a.db.QueryRowContext(ctx, `SELECT e.starts_at FROM orders o
		JOIN reservations r ON r.id = o.reservation_id JOIN events e ON e.id = r.event_id
		WHERE o.id = ?`, orderID).Scan(&startsAt); errors.Is(err, sql.ErrNoRows) {
		return "", time.Time{}, ErrNotFound
	} else if err != nil {
		return "", time.Time{}, fmt.Errorf("find order expiry: %w", err)
	}
	return a.Token(orderID), Expiry(startsAt), nil
}

func (a *Access) AuthorizeOrder(ctx context.Context, orderID, token string) (time.Time, error) {
	var startsAt time.Time
	err := a.db.QueryRowContext(ctx, `SELECT e.starts_at FROM orders o
		JOIN reservations r ON r.id = o.reservation_id JOIN events e ON e.id = r.event_id
		WHERE o.id = ?`, orderID).Scan(&startsAt)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, ErrNotFound
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("find order access: %w", err)
	}
	return a.authorize(orderID, startsAt, token)
}

func (a *Access) AuthorizeTicket(ctx context.Context, ticketID, token string) (string, error) {
	var orderID string
	var startsAt time.Time
	err := a.db.QueryRowContext(ctx, `SELECT et.order_id, e.starts_at FROM etickets et
		JOIN orders o ON o.id = et.order_id JOIN reservations r ON r.id = o.reservation_id
		JOIN events e ON e.id = r.event_id WHERE et.id = ?`, ticketID).Scan(&orderID, &startsAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("find ticket access: %w", err)
	}
	if _, err := a.authorize(orderID, startsAt, token); err != nil {
		return "", err
	}
	return orderID, nil
}

func (a *Access) authorize(orderID string, startsAt time.Time, token string) (time.Time, error) {
	want := a.Token(orderID)
	if !hmac.Equal([]byte(want), []byte(token)) {
		return time.Time{}, ErrUnauthorized
	}
	expiresAt := Expiry(startsAt)
	if !time.Now().Before(expiresAt) {
		return expiresAt, ErrExpired
	}
	return expiresAt, nil
}

func Expiry(startsAt time.Time) time.Time {
	wib := time.FixedZone("WIB", 7*60*60)
	// DATETIME stores UTC wall time; rebuild it as UTC if the DSN uses another loc.
	storedUTC := time.Date(startsAt.Year(), startsAt.Month(), startsAt.Day(), startsAt.Hour(), startsAt.Minute(), startsAt.Second(), startsAt.Nanosecond(), time.UTC)
	local := storedUTC.In(wib)
	return time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, wib).UTC()
}
