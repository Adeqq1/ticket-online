package reservation

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"
)

type Repository struct {
	db  *sql.DB
	ttl time.Duration
}

type tier struct {
	id                            uint64
	slug, name                    string
	price, available, maxPerOrder uint64
}
type reservationRecord struct {
	id, eventID, status, requestHash string
	eventArtist                      string
	expiresAt                        time.Time
	items                            []ResponseItem
}

func NewRepository(db *sql.DB, ttl time.Duration) *Repository { return &Repository{db: db, ttl: ttl} }

func (r *Repository) Create(ctx context.Context, request Request, idempotencyKey string) (Reservation, error) {
	request = NormalizeRequest(request)
	hash, err := RequestHash(request)
	if err != nil {
		return Reservation{}, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Reservation{}, fmt.Errorf("begin reservation: %w", err)
	}
	defer tx.Rollback()
	var existingHash string
	var existingID string
	err = tx.QueryRowContext(ctx, "SELECT id, request_hash FROM reservations WHERE idempotency_key = ? FOR UPDATE", idempotencyKey).Scan(&existingID, &existingHash)
	if err == nil {
		if existingHash != hash {
			return Reservation{}, ErrIdempotencyConflict
		}
		return r.getTx(ctx, tx, existingID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Reservation{}, fmt.Errorf("check idempotency: %w", err)
	}

	var eventArtist string
	if err := tx.QueryRowContext(ctx, "SELECT artist FROM events WHERE id = ?", request.EventID).Scan(&eventArtist); errors.Is(err, sql.ErrNoRows) {
		return Reservation{}, ErrEventNotFound
	} else if err != nil {
		return Reservation{}, fmt.Errorf("check event: %w", err)
	}
	items := append([]ItemRequest(nil), request.Items...)
	sort.Slice(items, func(i, j int) bool { return items[i].TierID < items[j].TierID })
	placeholders := make([]byte, 0, len(items)*2-1)
	args := []any{request.EventID}
	for index, item := range items {
		if index > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args = append(args, item.TierID)
	}
	rows, err := tx.QueryContext(ctx, "SELECT id, slug, name, price, available_quantity, max_per_order FROM ticket_tiers WHERE event_id = ? AND slug IN ("+string(placeholders)+") ORDER BY id FOR UPDATE", args...)
	if err != nil {
		return Reservation{}, fmt.Errorf("lock ticket tiers: %w", err)
	}
	defer rows.Close()
	tiers := make(map[string]tier, len(items))
	for rows.Next() {
		var value tier
		if err := rows.Scan(&value.id, &value.slug, &value.name, &value.price, &value.available, &value.maxPerOrder); err != nil {
			return Reservation{}, fmt.Errorf("scan ticket tier: %w", err)
		}
		tiers[value.slug] = value
	}
	if err := rows.Err(); err != nil {
		return Reservation{}, fmt.Errorf("iterate ticket tiers: %w", err)
	}
	if len(tiers) != len(items) {
		return Reservation{}, ErrTierNotFound
	}
	for _, item := range items {
		value := tiers[item.TierID]
		if item.Quantity > value.maxPerOrder {
			return Reservation{}, ErrOrderLimitExceeded
		}
		if item.Quantity > value.available {
			return Reservation{}, ErrInsufficientStock
		}
	}
	var idBytes [16]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		return Reservation{}, fmt.Errorf("generate reservation id: %w", err)
	}
	reservationID := hex.EncodeToString(idBytes[:])
	now := time.Now().UTC()
	expiresAt := now.Add(r.ttl)
	if _, err := tx.ExecContext(ctx, "INSERT INTO reservations (id, event_id, status, idempotency_key, request_hash, expires_at, created_at, updated_at) VALUES (?, ?, 'ACTIVE', ?, ?, ?, ?, ?)", reservationID, request.EventID, idempotencyKey, hash, expiresAt, now, now); err != nil {
		return Reservation{}, fmt.Errorf("insert reservation: %w", err)
	}
	responseItems := make([]ResponseItem, 0, len(items))
	subtotal := uint64(0)
	for _, item := range items {
		value := tiers[item.TierID]
		if _, err := tx.ExecContext(ctx, "INSERT INTO reservation_items (reservation_id, ticket_tier_id, quantity, unit_price) VALUES (?, ?, ?, ?)", reservationID, value.id, item.Quantity, value.price); err != nil {
			return Reservation{}, fmt.Errorf("insert reservation item: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE ticket_tiers SET available_quantity = available_quantity - ?, updated_at = ? WHERE id = ? AND available_quantity >= ?", item.Quantity, now, value.id, item.Quantity); err != nil {
			return Reservation{}, fmt.Errorf("decrement ticket stock: %w", err)
		}
		lineTotal := item.Quantity * value.price
		subtotal += lineTotal
		responseItems = append(responseItems, ResponseItem{TierID: value.slug, Name: value.name, Quantity: item.Quantity, UnitPrice: value.price, LineTotal: lineTotal})
	}
	if err := tx.Commit(); err != nil {
		return Reservation{}, fmt.Errorf("commit reservation: %w", err)
	}
	return Reservation{ID: reservationID, Status: "ACTIVE", ExpiresAt: expiresAt.UTC().Format(time.RFC3339), Event: EventSummary{ID: request.EventID, Artist: eventArtist}, Items: responseItems, Subtotal: subtotal}, nil
}

func (r *Repository) Get(ctx context.Context, id string) (Reservation, error) {
	record, err := r.get(ctx, r.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Reservation{}, ErrReservationNotFound
	}
	if err != nil {
		return Reservation{}, err
	}
	if record.status == "ACTIVE" && !time.Now().UTC().Before(record.expiresAt) {
		return Reservation{}, ErrReservationExpired
	}
	return record.response(), nil
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (r *Repository) get(ctx context.Context, db queryer, id string) (reservationRecord, error) {
	var record reservationRecord
	err := db.QueryRowContext(ctx, "SELECT r.id, r.event_id, r.status, r.expires_at, r.request_hash, e.artist FROM reservations r JOIN events e ON e.id = r.event_id WHERE r.id = ?", id).Scan(&record.id, &record.eventID, &record.status, &record.expiresAt, &record.requestHash, &record.eventArtist)
	if err != nil {
		return record, err
	}
	rows, err := db.QueryContext(ctx, "SELECT ri.quantity, ri.unit_price, tt.slug, tt.name FROM reservation_items ri JOIN ticket_tiers tt ON tt.id = ri.ticket_tier_id WHERE ri.reservation_id = ? ORDER BY tt.id", id)
	if err != nil {
		return record, fmt.Errorf("get reservation items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var quantity uint64
		var price uint64
		var slug, name string
		if err := rows.Scan(&quantity, &price, &slug, &name); err != nil {
			return record, err
		}
		record.items = append(record.items, ResponseItem{TierID: slug, Name: name, Quantity: quantity, UnitPrice: price, LineTotal: quantity * price})
	}
	return record, rows.Err()
}
func (r *Repository) getTx(ctx context.Context, tx *sql.Tx, id string) (Reservation, error) {
	record, err := r.get(ctx, tx, id)
	if err != nil {
		return Reservation{}, err
	}
	if record.status == "ACTIVE" && !time.Now().UTC().Before(record.expiresAt) {
		return Reservation{}, ErrReservationExpired
	}
	return record.response(), nil
}
func (r reservationRecord) response() Reservation {
	var subtotal uint64
	for _, item := range r.items {
		subtotal += item.LineTotal
	}
	return Reservation{ID: r.id, Status: r.status, ExpiresAt: r.expiresAt.UTC().Format(time.RFC3339), Event: EventSummary{ID: r.eventID, Artist: r.eventArtist}, Items: r.items, Subtotal: subtotal}
}
