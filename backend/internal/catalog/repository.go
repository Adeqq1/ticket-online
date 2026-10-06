package catalog

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

var (
	ErrEventNotFound         = errors.New("event not found")
	ErrScheduleLocked        = errors.New("event schedule locked after first order")
	ErrDuplicateEvent        = errors.New("duplicate event id")
	ErrEventHasReservations  = errors.New("event fields are required while reservations exist")
	ErrCapacityBelowBound    = errors.New("capacity is below sold or held ticket quantity")
	ErrGateLocked            = errors.New("gate cannot change after a reservation")
	ErrPublicationIncomplete = errors.New("event is incomplete for publication")
	ErrInvalidZone           = errors.New("ticket zone does not belong to event")
	ErrDuplicateTier         = errors.New("duplicate ticket tier id")
	ErrDuplicateZone         = errors.New("duplicate event zone id")
	ErrLocationLocked        = errors.New("event location locked after first reservation")
)

type Repository struct {
	db *sql.DB
}

type AdminActor struct{ ID, Name string }

func recordAdminAudit(ctx context.Context, tx *sql.Tx, actor AdminActor, action, objectType, objectID string, before, after any) error {
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return fmt.Errorf("encode audit before value: %w", err)
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return fmt.Errorf("encode audit after value: %w", err)
	}
	if bytes.Equal(beforeJSON, afterJSON) {
		return nil
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO admin_audit_log (actor_id, actor_name, action, object_type, object_id, before_json, after_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, UTC_TIMESTAMP(6))`, actor.ID, actor.Name, action, objectType, objectID, beforeJSON, afterJSON)
	if err != nil {
		return fmt.Errorf("record admin audit: %w", err)
	}
	return nil
}

func eventAuditSnapshot(ctx context.Context, tx *sql.Tx, id string) (eventAuditValue, error) {
	var value eventAuditValue
	var startsAt sql.NullTime
	err := tx.QueryRowContext(ctx, `SELECT id, artist, city, venue, address, starts_at, genre, status, publication_status, image_url, description FROM events WHERE id = ?`, id).Scan(&value.ID, &value.Artist, &value.City, &value.Venue, &value.Address, &startsAt, &value.Genre, &value.Status, &value.PublicationStatus, &value.Image, &value.Description)
	if err != nil {
		return value, err
	}
	if startsAt.Valid {
		value.StartsAt = startsAt.Time.UTC().Format(time.RFC3339Nano)
	}
	value.Lineup = make([]string, 0)
	rows, err := tx.QueryContext(ctx, `SELECT name FROM event_lineups WHERE event_id = ? ORDER BY position`, id)
	if err != nil {
		return value, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return value, err
		}
		value.Lineup = append(value.Lineup, name)
	}
	return value, rows.Err()
}

type eventAuditValue struct {
	ID                string   `json:"id"`
	Artist            string   `json:"artist"`
	City              string   `json:"city"`
	Venue             string   `json:"venue"`
	Address           string   `json:"address"`
	StartsAt          string   `json:"startsAt"`
	Genre             string   `json:"genre"`
	Status            string   `json:"status"`
	PublicationStatus string   `json:"publicationStatus"`
	Image             string   `json:"image"`
	Description       string   `json:"description"`
	Lineup            []string `json:"lineup"`
}
type tierAuditValue struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	ZoneID            string `json:"zoneId"`
	Price             uint64 `json:"price"`
	Capacity          uint64 `json:"capacity"`
	AvailableQuantity uint64 `json:"availableQuantity"`
	MaxPerOrder       uint64 `json:"maxPerOrder"`
	Benefit           string `json:"benefit"`
	Gate              string `json:"gate"`
	Seating           string `json:"seating"`
}

type eventRecord struct {
	id, artist, city, venue, address                        string
	startsAt                                                sql.NullTime
	genre, status, publicationStatus, imageURL, description string
}

type zoneRecord struct {
	slug, name, description string
}

type tierRecord struct {
	slug, name, zoneSlug, benefit, gate, seating    string
	price, availableQuantity, maxPerOrder, capacity uint64
}

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) ListEvents(ctx context.Context) ([]Event, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, artist, city, venue, address, starts_at, genre, status, publication_status, image_url, description FROM events WHERE publication_status = 'PUBLISHED' ORDER BY starts_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	events := make([]Event, 0)
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

func (r *Repository) adminDetail(ctx context.Context, id string) (AdminEvent, error) {
	event, err := r.adminEvent(ctx, id)
	if err != nil {
		return AdminEvent{}, err
	}
	return r.adminDTO(ctx, event)
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

func (r *Repository) ListAdminEvents(ctx context.Context) ([]AdminEvent, error) {
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
	events := make([]AdminEvent, 0, len(records))
	for _, record := range records {
		event, err := r.buildEvent(ctx, record)
		if err != nil {
			return nil, err
		}
		admin, err := r.adminDTO(ctx, event)
		if err != nil {
			return nil, err
		}
		events = append(events, admin)
	}
	return events, nil
}

type EventInput struct {
	ID, Artist, City, Venue, Address                     string
	StartsAt                                             *time.Time
	Genre, Status, PublicationStatus, Image, Description string
	Lineup                                               []string
}

func (r *Repository) CreateEvent(ctx context.Context, input EventInput, actor AdminActor) error {
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create event: %w", err)
	}
	defer tx.Rollback()
	if input.PublicationStatus == "PUBLISHED" {
		if err := validatePublication(ctx, tx, input.ID, input); err != nil {
			return err
		}
	}
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
	after := eventAuditValue{ID: input.ID, Artist: input.Artist, City: input.City, Venue: input.Venue, Address: input.Address, Genre: input.Genre, Status: input.Status, PublicationStatus: input.PublicationStatus, Image: input.Image, Description: input.Description, Lineup: input.Lineup}
	if input.StartsAt != nil {
		after.StartsAt = input.StartsAt.UTC().Format(time.RFC3339Nano)
	}
	if err := recordAdminAudit(ctx, tx, actor, "CREATE", "EVENT", input.ID, nil, after); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit create event: %w", err)
	}
	return nil
}

func (r *Repository) UpdateEvent(ctx context.Context, id string, input EventInput, actor AdminActor) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin update event: %w", err)
	}
	defer tx.Rollback()
	var existing sql.NullTime
	var city, venue, address string
	err = tx.QueryRowContext(ctx, "SELECT starts_at, city, venue, address FROM events WHERE id = ? FOR UPDATE", id).Scan(&existing, &city, &venue, &address)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrEventNotFound
	}
	if err != nil {
		return fmt.Errorf("lock event: %w", err)
	}
	before, err := eventAuditSnapshot(ctx, tx, id)
	if err != nil {
		return fmt.Errorf("read event audit snapshot: %w", err)
	}
	if existing.Valid != (input.StartsAt != nil) || (existing.Valid && !existing.Time.Equal(*input.StartsAt)) {
		var hasReservations bool
		err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reservations WHERE event_id = ?)`, id).Scan(&hasReservations)
		if err != nil {
			return fmt.Errorf("check event orders: %w", err)
		}
		if hasReservations {
			return ErrScheduleLocked
		}
	}
	if city != input.City || venue != input.Venue || address != input.Address {
		var hasReservations bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reservations WHERE event_id = ?)`, id).Scan(&hasReservations); err != nil {
			return fmt.Errorf("check event location reservations: %w", err)
		}
		if hasReservations {
			return ErrLocationLocked
		}
	}
	if input.Artist == "" || input.City == "" || input.Venue == "" || input.Address == "" || input.StartsAt == nil {
		var hasReservations bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reservations WHERE event_id = ?)`, id).Scan(&hasReservations); err != nil {
			return fmt.Errorf("check event reservations: %w", err)
		}
		if hasReservations {
			return ErrEventHasReservations
		}
	}
	if input.PublicationStatus == "PUBLISHED" {
		if err := validatePublication(ctx, tx, id, input); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE events SET artist = ?, city = ?, venue = ?, address = ?, starts_at = ?, genre = ?, status = ?, publication_status = ?, image_url = ?, description = ?, updated_at = UTC_TIMESTAMP(6) WHERE id = ?`, input.Artist, input.City, input.Venue, input.Address, input.StartsAt, input.Genre, input.Status, input.PublicationStatus, input.Image, input.Description, id)
	if err != nil {
		return fmt.Errorf("update event: %w", err)
	}
	if err := replaceLineup(ctx, tx, id, input.Lineup); err != nil {
		return err
	}
	after, err := eventAuditSnapshot(ctx, tx, id)
	if err != nil {
		return fmt.Errorf("read updated event audit snapshot: %w", err)
	}
	if err := recordAdminAudit(ctx, tx, actor, "UPDATE", "EVENT", id, before, after); err != nil {
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
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reservations WHERE event_id = ?)`, record.id).Scan(&scheduleLocked); err != nil {
		return Event{}, fmt.Errorf("check event schedule lock: %w", err)
	}
	return Event{
		ID: record.id, Artist: record.artist, City: record.city, Venue: record.venue, Address: record.address,
		StartsAt: formatStartsAt(record.startsAt), Genre: record.genre, Status: displayStatus(record.status), PublicationStatus: record.publicationStatus,
		Image: record.imageURL, Description: record.description, Lineup: lineup, Price: price, Zones: zones, TicketTiers: tiers, ScheduleLocked: scheduleLocked,
	}, nil
}

func formatStartsAt(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339Nano)
}

func (r *Repository) adminDTO(ctx context.Context, event Event) (AdminEvent, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT slug, name, zone_slug, price, available_quantity, max_per_order, benefit, gate, seating_mode, capacity,
		EXISTS(SELECT 1 FROM reservation_items ri JOIN reservations r ON r.id = ri.reservation_id WHERE ri.ticket_tier_id = ticket_tiers.id) AS gate_locked
		FROM ticket_tiers WHERE event_id = ? ORDER BY id`, event.ID)
	if err != nil {
		return AdminEvent{}, fmt.Errorf("list admin ticket tiers: %w", err)
	}
	defer rows.Close()
	tiers := make([]AdminTicketTier, 0)
	for rows.Next() {
		var item AdminTicketTier
		var seating string
		if err := rows.Scan(&item.ID, &item.Name, &item.ZoneID, &item.Price, &item.AvailableQuantity, &item.MaxPerOrder, &item.Benefit, &item.Gate, &seating, &item.Capacity, &item.GateLocked); err != nil {
			return AdminEvent{}, err
		}
		item.Seating = displaySeating(seating)
		item.BoundQuantity = item.Capacity - item.AvailableQuantity
		tiers = append(tiers, item)
	}
	if err := rows.Err(); err != nil {
		return AdminEvent{}, err
	}
	return AdminEvent{Event: event, TicketTiers: tiers, LocationLocked: event.ScheduleLocked}, nil
}

type dbQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func validatePublication(ctx context.Context, q dbQueryer, eventID string, input EventInput) error {
	var zones, tiers int
	var invalidZone, invalidTier bool
	if input.StartsAt == nil || strings.TrimSpace(input.Artist) == "" || strings.TrimSpace(input.City) == "" || strings.TrimSpace(input.Venue) == "" || strings.TrimSpace(input.Address) == "" || strings.TrimSpace(input.Description) == "" || strings.TrimSpace(input.Image) == "" || len(input.Lineup) == 0 {
		return ErrPublicationIncomplete
	}
	if err := q.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM event_zones WHERE event_id = ?),
		(SELECT COUNT(*) FROM ticket_tiers WHERE event_id = ?),
		EXISTS(SELECT 1 FROM event_zones WHERE event_id = ? AND CHAR_LENGTH(TRIM(name)) = 0),
		EXISTS(SELECT 1 FROM ticket_tiers WHERE event_id = ? AND (CHAR_LENGTH(TRIM(name)) = 0 OR CHAR_LENGTH(TRIM(gate)) = 0))`, eventID, eventID, eventID, eventID).Scan(&zones, &tiers, &invalidZone, &invalidTier); err != nil {
		return fmt.Errorf("check publication completeness: %w", err)
	}
	if zones == 0 || tiers == 0 || invalidZone || invalidTier {
		return ErrPublicationIncomplete
	}
	return nil
}

type ZoneInput struct{ ID, Name, Description string }
type TierInput struct {
	ID, Name, ZoneID, Benefit, Gate, Seating string
	Price, Capacity, MaxPerOrder             uint64
}

func lockAdminEvent(ctx context.Context, tx *sql.Tx, eventID string) error {
	var id string
	err := tx.QueryRowContext(ctx, "SELECT id FROM events WHERE id = ? FOR UPDATE", eventID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrEventNotFound
	}
	if err != nil {
		return fmt.Errorf("lock admin event: %w", err)
	}
	return nil
}

func (r *Repository) SaveZone(ctx context.Context, eventID string, input ZoneInput, create bool, actor AdminActor) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockAdminEvent(ctx, tx, eventID); err != nil {
		return err
	}
	var before any
	var oldZone zoneRecord
	if !create {
		err = tx.QueryRowContext(ctx, `SELECT slug, name, description FROM event_zones WHERE event_id = ? AND slug = ? FOR UPDATE`, eventID, input.ID).Scan(&oldZone.slug, &oldZone.name, &oldZone.description)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrEventNotFound
		}
		if err != nil {
			return err
		}
		before = ZoneInputAudit{ID: oldZone.slug, Name: oldZone.name, Description: oldZone.description}
	}
	if create {
		_, err = tx.ExecContext(ctx, "INSERT INTO event_zones (event_id, slug, name, description) VALUES (?, ?, ?, ?)", eventID, input.ID, input.Name, input.Description)
	} else {
		result, updateErr := tx.ExecContext(ctx, "UPDATE event_zones SET name = ?, description = ? WHERE event_id = ? AND slug = ?", input.Name, input.Description, eventID, input.ID)
		err = updateErr
		if err == nil {
			n, _ := result.RowsAffected()
			if n == 0 {
				var exists bool
				if e := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM event_zones WHERE event_id = ? AND slug = ?)", eventID, input.ID).Scan(&exists); e != nil {
					return e
				}
				if !exists {
					return ErrEventNotFound
				}
			}
		}
	}
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return ErrDuplicateZone
		}
		return fmt.Errorf("save event zone: %w", err)
	}
	if err := recordAdminAudit(ctx, tx, actor, map[bool]string{true: "CREATE", false: "UPDATE"}[create], "ZONE", eventID+"/"+input.ID, before, ZoneInputAudit{ID: input.ID, Name: input.Name, Description: input.Description}); err != nil {
		return err
	}
	return tx.Commit()
}

type ZoneInputAudit struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (r *Repository) SaveTier(ctx context.Context, eventID string, input TierInput, create bool, actor AdminActor) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockAdminEvent(ctx, tx, eventID); err != nil {
		return err
	}
	var oldCapacity, available uint64
	newAvailable := input.Capacity
	var oldGate string
	var tierID uint64
	var before any
	if !create {
		var old tierAuditValue
		err = tx.QueryRowContext(ctx, "SELECT id, name, zone_slug, price, capacity, available_quantity, max_per_order, benefit, gate, seating_mode FROM ticket_tiers WHERE event_id = ? AND slug = ? FOR UPDATE", eventID, input.ID).Scan(&tierID, &old.Name, &old.ZoneID, &old.Price, &old.Capacity, &old.AvailableQuantity, &old.MaxPerOrder, &old.Benefit, &old.Gate, &old.Seating)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrEventNotFound
		}
		if err != nil {
			return err
		}
		old.ID = input.ID
		before = old
		oldCapacity, available, oldGate = old.Capacity, old.AvailableQuantity, old.Gate
	}
	var zoneExists bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM event_zones WHERE event_id = ? AND slug = ?)", eventID, input.ZoneID).Scan(&zoneExists); err != nil {
		return err
	}
	if !zoneExists {
		return ErrInvalidZone
	}
	if !create {
		var gateLocked bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reservation_items WHERE ticket_tier_id = ?)`, tierID).Scan(&gateLocked); err != nil {
			return err
		}
		if gateLocked && input.Gate != oldGate {
			return ErrGateLocked
		}
		bound := oldCapacity - available
		if input.Capacity < bound {
			return ErrCapacityBelowBound
		}
		newAvailable = input.Capacity - bound
		_, err = tx.ExecContext(ctx, `UPDATE ticket_tiers SET name=?, zone_slug=?, price=?, capacity=?, available_quantity=?, max_per_order=?, benefit=?, gate=?, seating_mode=?, updated_at=UTC_TIMESTAMP(6) WHERE id=?`, input.Name, input.ZoneID, input.Price, input.Capacity, newAvailable, input.MaxPerOrder, input.Benefit, input.Gate, input.Seating, tierID)
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO ticket_tiers (event_id, slug, name, zone_slug, price, capacity, available_quantity, max_per_order, benefit, gate, seating_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`, eventID, input.ID, input.Name, input.ZoneID, input.Price, input.Capacity, input.Capacity, input.MaxPerOrder, input.Benefit, input.Gate, input.Seating)
	}
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return ErrDuplicateTier
		}
		return fmt.Errorf("save ticket tier: %w", err)
	}
	after := tierAuditValue{ID: input.ID, Name: input.Name, ZoneID: input.ZoneID, Price: input.Price, Capacity: input.Capacity, AvailableQuantity: newAvailable, MaxPerOrder: input.MaxPerOrder, Benefit: input.Benefit, Gate: input.Gate, Seating: input.Seating}
	if err := recordAdminAudit(ctx, tx, actor, map[bool]string{true: "CREATE", false: "UPDATE"}[create], "TICKET_TIER", eventID+"/"+input.ID, before, after); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) zones(ctx context.Context, eventID string) ([]Zone, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT slug, name, description FROM event_zones WHERE event_id = ? ORDER BY slug`, eventID)
	if err != nil {
		return nil, fmt.Errorf("list event zones: %w", err)
	}
	defer rows.Close()
	result := make([]Zone, 0)
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
	result := make([]string, 0)
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
	rows, err := r.db.QueryContext(ctx, `SELECT slug, name, zone_slug, price, available_quantity, max_per_order, benefit, gate, seating_mode, capacity FROM ticket_tiers WHERE event_id = ? ORDER BY id`, eventID)
	if err != nil {
		return nil, fmt.Errorf("list ticket tiers: %w", err)
	}
	defer rows.Close()
	result := make([]TicketTier, 0)
	for rows.Next() {
		var tier tierRecord
		if err := rows.Scan(&tier.slug, &tier.name, &tier.zoneSlug, &tier.price, &tier.availableQuantity, &tier.maxPerOrder, &tier.benefit, &tier.gate, &tier.seating, &tier.capacity); err != nil {
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
