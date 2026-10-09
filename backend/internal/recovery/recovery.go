// Package recovery lets a buyer restore order access on a new browser and resend ticket email.
package recovery

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/netip"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/eventstate"
	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/go-sql-driver/mysql"
)

const (
	// RequestTTL bounds every recovery link; tokens are also bounded by the order access deadline.
	RequestTTL      = 15 * time.Minute
	AcceptedMessage = "Jika data sesuai dengan pesanan yang masih berlaku, tautan pemulihan akan dikirim ke email pembeli."
)

var referencePattern = regexp.MustCompile(`^[Tt][Oo]-[0-9A-Fa-f]{20}$`)

// NormalizeReference returns the canonical order reference, or "" when value cannot be one.
func NormalizeReference(value string) string {
	value = strings.TrimSpace(value)
	if !referencePattern.MatchString(value) {
		return ""
	}
	return "TO-" + strings.ToLower(value[3:])
}

// NormalizeEmail returns the lowercase address, or "" when value is not one bare address.
func NormalizeEmail(value string) string {
	value = strings.TrimSpace(value)
	parsed, err := mail.ParseAddress(value)
	if len(value) > 254 || err != nil || parsed.Address != value {
		return ""
	}
	return strings.ToLower(value)
}

// PairKey identifies an email/reference pair so requests can be matched without storing the typed email.
func PairKey(access *orderaccess.Access, email, reference string) string {
	return access.Digest("recovery-pair", strings.ToLower(strings.TrimSpace(email)), reference)
}

// NewToken returns a URL-safe token from 32 random bytes and the SHA-256 hash that is stored instead.
func NewToken() (string, string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", "", fmt.Errorf("generate recovery token: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(bytes[:])
	return raw, HashToken(raw), nil
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func validToken(raw string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	return len(raw) == 43 && err == nil && len(decoded) == 32
}

func NewID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(bytes[:]), nil
}

type Order struct {
	ID, Reference, ReservationID, Status, BuyerName, BuyerEmail string
	ExpiresAt, StartsAt                                         time.Time
	Complete                                                    bool
	Changed, AccessHeld, TicketActive                           bool
	AccessDeadline                                              time.Time
}

// Eligible reports whether the order may be recovered or have its tickets resent.
func (o Order) Eligible(now time.Time) bool {
	if o.Changed {
		return o.AccessHeld || now.Before(o.AccessDeadline)
	}
	return o.Status == "PAID" && o.Complete && now.Before(orderaccess.Expiry(o.StartsAt))
}

type querier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const orderQuery = `SELECT o.id, o.reference, o.reservation_id, o.status, o.expires_at, e.starts_at, b.name, b.email,
	(SELECT COUNT(*) FROM etickets t WHERE t.order_id = o.id) > 0 AND
	(SELECT COUNT(*) FROM etickets t WHERE t.order_id = o.id) = (SELECT COALESCE(SUM(i.quantity), 0) FROM order_items i WHERE i.order_id = o.id)
	FROM orders o JOIN reservations r ON r.id = o.reservation_id JOIN events e ON e.id = r.event_id
	JOIN order_buyers b ON b.order_id = o.id `

func FindOrderByID(ctx context.Context, q querier, id string) (Order, bool, error) {
	return findOrder(ctx, q, orderQuery+"WHERE o.id = ?", id)
}

func FindOrderByReference(ctx context.Context, q querier, reference string) (Order, bool, error) {
	return findOrder(ctx, q, orderQuery+"WHERE o.reference = ?", reference)
}

func findOrder(ctx context.Context, q querier, query, value string) (Order, bool, error) {
	var order Order
	err := q.QueryRowContext(ctx, query, value).Scan(&order.ID, &order.Reference, &order.ReservationID, &order.Status,
		&order.ExpiresAt, &order.StartsAt, &order.BuyerName, &order.BuyerEmail, &order.Complete)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, false, nil
	}
	if err != nil {
		return Order{}, false, fmt.Errorf("find recovery order: %w", err)
	}
	state, err := eventstate.ForOrder(ctx, q, order.ID, false)
	if err != nil {
		return order, false, err
	}
	order.Changed = state.Version > 0
	order.AccessDeadline, err = eventstate.Deadline(ctx, q, order.ID)
	if err != nil {
		return order, false, err
	}
	order.AccessHeld = order.AccessDeadline.IsZero()
	var requested bool
	if err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM event_refund_rights WHERE order_id=? AND requested=TRUE)", order.ID).Scan(&requested); err != nil {
		return order, false, err
	}
	order.TicketActive = order.Status == "PAID" && order.Complete && state.StartsAt != nil && !requested
	return order, true, nil
}

type limit struct {
	bucket string
	max    int
	period time.Duration
}

type RateLimit struct {
	Bucket string
	Max    int
	Period time.Duration
}

// allow counts one hit in each limit in order and stops at the first exceeded one.
// It returns that limit's index (-1 when all pass) and the seconds until its window resets.
func allow(ctx context.Context, db *sql.DB, limits ...limit) (int, int, error) {
	for attempt := 1; ; attempt++ {
		blocked, wait, err := allowOnce(ctx, db, limits)
		var mysqlErr *mysql.MySQLError
		// Concurrent upserts of a new bucket can deadlock; the loser is safe to replay.
		if err != nil && attempt < 3 && errors.As(err, &mysqlErr) && mysqlErr.Number == 1213 {
			continue
		}
		return blocked, wait, err
	}
}

func allowOnce(ctx context.Context, db *sql.DB, limits []limit) (int, int, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, 0, fmt.Errorf("begin rate limit: %w", err)
	}
	defer tx.Rollback()
	public := make([]RateLimit, len(limits))
	for i, current := range limits {
		public[i] = RateLimit{Bucket: current.bucket, Max: current.max, Period: current.period}
	}
	blocked, wait, err := CheckRateLimitsTx(ctx, tx, public...)
	if err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return blocked, wait, nil
}

// CheckRateLimitsTx records hits in the caller's transaction so a protected action and its rate limit commit together.
func CheckRateLimitsTx(ctx context.Context, tx *sql.Tx, limits ...RateLimit) (int, int, error) {
	for index, current := range limits {
		seconds := int(current.Period.Seconds())
		// hits is assigned before reset_at, so both IF() calls still read the old reset_at.
		if _, err := tx.ExecContext(ctx, `INSERT INTO rate_limits (bucket, hits, reset_at)
			VALUES (?, 1, DATE_ADD(UTC_TIMESTAMP(6), INTERVAL ? SECOND))
			ON DUPLICATE KEY UPDATE hits = IF(reset_at <= UTC_TIMESTAMP(6), 1, hits + 1),
			reset_at = IF(reset_at <= UTC_TIMESTAMP(6), DATE_ADD(UTC_TIMESTAMP(6), INTERVAL ? SECOND), reset_at)`,
			current.Bucket, seconds, seconds); err != nil {
			return 0, 0, fmt.Errorf("count rate limit: %w", err)
		}
		var hits, wait int
		if err := tx.QueryRowContext(ctx, `SELECT hits,
			CAST(CEIL(TIMESTAMPDIFF(MICROSECOND, UTC_TIMESTAMP(6), reset_at) / 1000000) AS SIGNED)
			FROM rate_limits WHERE bucket = ? FOR UPDATE`, current.Bucket).Scan(&hits, &wait); err != nil {
			return 0, 0, fmt.Errorf("read rate limit: %w", err)
		}
		if hits > current.Max {
			return index, max(wait, 1), nil
		}
	}
	return -1, 0, nil
}

func clientIP(r *http.Request, trusted []netip.Prefix) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return "unknown"
	}
	peer = peer.Unmap()
	isTrusted := func(ip netip.Addr) bool {
		return slices.ContainsFunc(trusted, func(prefix netip.Prefix) bool { return prefix.Contains(ip) })
	}
	if !isTrusted(peer) || len(r.Header.Values("X-Forwarded-For")) == 0 {
		return peer.String()
	}
	chain := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(chain) - 1; i >= 0; i-- {
		ip, err := netip.ParseAddr(strings.TrimSpace(chain[i]))
		if err != nil || ip.Zone() != "" {
			return peer.String()
		}
		ip = ip.Unmap()
		if !isTrusted(ip) || i == 0 {
			return ip.String()
		}
	}
	return peer.String()
}
