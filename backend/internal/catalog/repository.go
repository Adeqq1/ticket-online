package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
)

var (
	ErrEventNotFound  = errors.New("event not found")
	ErrScheduleLocked = errors.New("event schedule locked after first order")
	ErrDuplicateEvent = errors.New("duplicate event id")
)

type Repository struct {
	db *sql.DB
}

type eventRecord struct {
	id, artist, city, venue, address                        string
	startsAt                                                time.Time
	genre, status, publicationStatus, imageURL, description string
}

type zoneRecord struct {
	slug, name, description string
}

type tierRecord struct {
	slug, name, zoneSlug, benefit, gate, seating string
	price, availableQuantity, maxPerOrder        uint64
}

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) ListEvents(ctx context.Context) ([]Event, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, artist, city, venue, address, starts_at, genre, status, publication_status, image_url, description FROM events WHERE publication_status = 'PUBLISHED' ORDER BY starts_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var events []Event
	var records []eventRecord
	for rows.Next() {
		record, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close events: %w", err)
	}
	for _, record := range records {
		event, err := r.buildEvent(ctx, record)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (r *Repository) GetEvent(ctx context.Context, id string) (Event, error) {
	record, err := scanEvent(r.db.QueryRowContext(ctx, `SELECT id, artist, city, venue, address, starts_at, genre, status, publication_status, image_url, description FROM events WHERE id = ? AND publication_status = 'PUBLISHED'`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrEventNotFound
	}
	if err != nil {
		return Event{}, fmt.Errorf("get event: %w", err)
	}
	return r.buildEvent(ctx, record)
}

func (r *Repository) adminEvent(ctx context.Context, id string) (Event, error) {
	record, err := scanEvent(r.db.QueryRowContext(ctx, `SELECT id, artist, city, venue, address, starts_at, genre, status, publication_status, image_url, description FROM events WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrEventNotFound
	}
	if err != nil {
		return Event{}, fmt.Errorf("get admin event: %w", err)
	}
	return r.buildEvent(ctx, record)
}

func (r *Repository) ReservationEvent(ctx context.Context, reservationID, key string) (Event, error) {
	var eventID string
	err := r.db.QueryRowContext(ctx, "SELECT event_id FROM reservations WHERE id = ? AND idempotency_key = ?", reservationID, key).Scan(&eventID)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrEventNotFound
	}
	if err != nil {
		return Event{}, fmt.Errorf("find reservation event: %w", err)
	}
	return r.adminEvent(ctx, eventID)
}

func (r *Repository) ListAdminEvents(ctx context.Context) ([]Event, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, artist, city, venue, address, starts_at, genre, status, publication_status, image_url, description FROM events ORDER BY starts_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list admin events: %w", err)
	}
	defer rows.Close()
	var records []eventRecord
	for rows.Next() {
		record, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan admin event: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin events: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close admin events: %w", err)
	}
	events := make([]Event, 0, len(records))
	for _, record := range records {
		event, err := r.buildEvent(ctx, record)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

type EventInput struct {
	ID, Artist, City, Venue, Address                     string
	StartsAt                                             time.Time
	Genre, Status, PublicationStatus, Image, Description string
	Lineup                                               []string
}

func (r *Repository) CreateEvent(ctx context.Context, input EventInput) error {
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create event: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO events (id, artist, city, venue, address, starts_at, genre, status, publication_status, image_url, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, input.ID, input.Artist, input.City, input.Venue, input.Address, input.StartsAt, input.Genre, input.Status, input.PublicationStatus, input.Image, input.Description, now, now)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return ErrDuplicateEvent
		}
		return fmt.Errorf("insert event: %w", err)
	}
	if err := replaceLineup(ctx, tx, input.ID, input.Lineup); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit create event: %w", err)
	}
	return nil
}

func (r *Repository) UpdateEvent(ctx context.Context, id string, input EventInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin update event: %w", err)
	}
	defer tx.Rollback()
	var existing time.Time
	err = tx.QueryRowContext(ctx, "SELECT starts_at FROM events WHERE id = ? FOR UPDATE", id).Scan(&existing)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrEventNotFound
	}
	if err != nil {
		return fmt.Errorf("lock event: %w", err)
	}
	if !existing.Equal(input.StartsAt) {
		var ordered bool
		err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM orders o JOIN reservations r ON r.id = o.reservation_id WHERE r.event_id = ?)`, id).Scan(&ordered)
		if err != nil {
			return fmt.Errorf("check event orders: %w", err)
		}
		if ordered {
			return ErrScheduleLocked
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE events SET artist = ?, city = ?, venue = ?, address = ?, starts_at = ?, genre = ?, status = ?, publication_status = ?, image_url = ?, description = ?, updated_at = UTC_TIMESTAMP(6) WHERE id = ?`, input.Artist, input.City, input.Venue, input.Address, input.StartsAt, input.Genre, input.Status, input.PublicationStatus, input.Image, input.Description, id)
	if err != nil {
		return fmt.Errorf("update event: %w", err)
	}
	if err := replaceLineup(ctx, tx, id, input.Lineup); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit update event: %w", err)
	}
	return nil
}

func replaceLineup(ctx context.Context, tx *sql.Tx, eventID string, lineup []string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM event_lineups WHERE event_id = ?", eventID); err != nil {
		return fmt.Errorf("delete event lineup: %w", err)
	}
	for i, name := range lineup {
		if _, err := tx.ExecContext(ctx, "INSERT INTO event_lineups (event_id, position, name) VALUES (?, ?, ?)", eventID, i+1, name); err != nil {
			return fmt.Errorf("insert event lineup: %w", err)
		}
	}
	return nil
}

func scanEvent(scanner interface{ Scan(...any) error }) (eventRecord, error) {
	var record eventRecord
	err := scanner.Scan(&record.id, &record.artist, &record.city, &record.venue, &record.address, &record.startsAt, &record.genre, &record.status, &record.publicationStatus, &record.imageURL, &record.description)
	return record, err
}

func (r *Repository) buildEvent(ctx context.Context, record eventRecord) (Event, error) {
	zones, err := r.zones(ctx, record.id)
	if err != nil {
		return Event{}, err
	}
	lineup, err := r.lineup(ctx, record.id)
	if err != nil {
		return Event{}, err
	}
	tiers, err := r.tiers(ctx, record.id)
	if err != nil {
		return Event{}, err
	}
	price := uint64(0)
	for _, tier := range tiers {
		if price == 0 || tier.Price < price {
			price = tier.Price
		}
	}
	var scheduleLocked bool
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM orders o JOIN reservations r ON r.id = o.reservation_id WHERE r.event_id = ?)`, record.id).Scan(&scheduleLocked); err != nil {
		return Event{}, fmt.Errorf("check event schedule lock: %w", err)
	}
	return Event{
		ID: record.id, Artist: record.artist, City: record.city, Venue: record.venue, Address: record.address,
		StartsAt: record.startsAt.UTC().Format(time.RFC3339), Genre: record.genre, Status: displayStatus(record.status), PublicationStatus: record.publicationStatus,
		Image: record.imageURL, Description: record.description, Lineup: lineup, Price: price, Zones: zones, TicketTiers: tiers, ScheduleLocked: scheduleLocked,
	}, nil
}

func (r *Repository) zones(ctx context.Context, eventID string) ([]Zone, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT slug, name, description FROM event_zones WHERE event_id = ? ORDER BY slug`, eventID)
	if err != nil {
		return nil, fmt.Errorf("list event zones: %w", err)
	}
	defer rows.Close()
	var result []Zone
	for rows.Next() {
		var zone zoneRecord
		if err := rows.Scan(&zone.slug, &zone.name, &zone.description); err != nil {
			return nil, fmt.Errorf("scan event zone: %w", err)
		}
		result = append(result, Zone{ID: zone.slug, Name: zone.name, Description: zone.description})
	}
	return result, rows.Err()
}

func (r *Repository) lineup(ctx context.Context, eventID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT name FROM event_lineups WHERE event_id = ? ORDER BY position`, eventID)
	if err != nil {
		return nil, fmt.Errorf("list event lineup: %w", err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan event lineup: %w", err)
		}
		result = append(result, name)
	}
	return result, rows.Err()
}

func (r *Repository) tiers(ctx context.Context, eventID string) ([]TicketTier, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT slug, name, zone_slug, price, available_quantity, max_per_order, benefit, gate, seating_mode FROM ticket_tiers WHERE event_id = ? ORDER BY id`, eventID)
	if err != nil {
		return nil, fmt.Errorf("list ticket tiers: %w", err)
	}
	defer rows.Close()
	var result []TicketTier
	for rows.Next() {
		var tier tierRecord
		if err := rows.Scan(&tier.slug, &tier.name, &tier.zoneSlug, &tier.price, &tier.availableQuantity, &tier.maxPerOrder, &tier.benefit, &tier.gate, &tier.seating); err != nil {
			return nil, fmt.Errorf("scan ticket tier: %w", err)
		}
		result = append(result, TicketTier{ID: tier.slug, Name: tier.name, ZoneID: tier.zoneSlug, Price: tier.price, AvailableQuantity: tier.availableQuantity, MaxPerOrder: tier.maxPerOrder, Benefit: tier.benefit, Gate: tier.gate, Seating: displaySeating(tier.seating)})
	}
	return result, rows.Err()
}

func displayStatus(status string) string {
	switch status {
	case "EARLY_BIRD":
		return "Early Bird"
	case "PRESALE":
		return "Presale"
	case "SOLD_OUT":
		return "Sold Out"
	default:
		return status
	}
}

func displaySeating(seating string) string {
	if seating == "FREE_STANDING" {
		return "free-standing"
	}
	if seating == "ASSIGNED" {
		return "assigned"
	}
	return seating
}
