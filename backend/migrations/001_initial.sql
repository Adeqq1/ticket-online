CREATE TABLE IF NOT EXISTS events (
  id VARCHAR(64) PRIMARY KEY,
  artist VARCHAR(160) NOT NULL,
  city VARCHAR(100) NOT NULL,
  venue VARCHAR(160) NOT NULL,
  address VARCHAR(255) NOT NULL,
  starts_at DATETIME(6) NOT NULL,
  genre VARCHAR(40) NOT NULL,
  status ENUM('EARLY_BIRD', 'PRESALE', 'SOLD_OUT') NOT NULL,
  image_url VARCHAR(500) NOT NULL,
  description TEXT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL
);

CREATE TABLE IF NOT EXISTS event_zones (
  event_id VARCHAR(64) NOT NULL,
  slug VARCHAR(64) NOT NULL,
  name VARCHAR(100) NOT NULL,
  description VARCHAR(255) NOT NULL,
  PRIMARY KEY (event_id, slug),
  CONSTRAINT fk_event_zones_event FOREIGN KEY (event_id) REFERENCES events (id)
);

CREATE TABLE IF NOT EXISTS event_lineups (
  event_id VARCHAR(64) NOT NULL,
  position SMALLINT UNSIGNED NOT NULL,
  name VARCHAR(160) NOT NULL,
  PRIMARY KEY (event_id, position),
  CONSTRAINT fk_event_lineups_event FOREIGN KEY (event_id) REFERENCES events (id)
);

CREATE TABLE IF NOT EXISTS ticket_tiers (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  event_id VARCHAR(64) NOT NULL,
  slug VARCHAR(64) NOT NULL,
  name VARCHAR(100) NOT NULL,
  zone_slug VARCHAR(64) NOT NULL,
  price BIGINT UNSIGNED NOT NULL,
  capacity INT UNSIGNED NOT NULL,
  available_quantity INT UNSIGNED NOT NULL,
  max_per_order INT UNSIGNED NOT NULL,
  benefit VARCHAR(255) NOT NULL,
  gate VARCHAR(100) NOT NULL,
  seating_mode ENUM('ASSIGNED', 'FREE_STANDING') NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_ticket_tiers_event_slug (event_id, slug),
  CONSTRAINT fk_ticket_tiers_event FOREIGN KEY (event_id) REFERENCES events (id),
  CONSTRAINT fk_ticket_tiers_zone FOREIGN KEY (event_id, zone_slug) REFERENCES event_zones (event_id, slug),
  CONSTRAINT chk_ticket_tiers_price CHECK (price > 0),
  CONSTRAINT chk_ticket_tiers_capacity CHECK (capacity > 0),
  CONSTRAINT chk_ticket_tiers_available CHECK (available_quantity <= capacity),
  CONSTRAINT chk_ticket_tiers_max_order CHECK (max_per_order > 0)
);

CREATE TABLE IF NOT EXISTS reservations (
  id CHAR(32) PRIMARY KEY,
  event_id VARCHAR(64) NOT NULL,
  status ENUM('ACTIVE', 'EXPIRED', 'CANCELLED', 'CONVERTED') NOT NULL,
  idempotency_key VARCHAR(100) NOT NULL,
  request_hash CHAR(64) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_reservations_idempotency_key (idempotency_key),
  KEY ix_reservations_status_expires (status, expires_at),
  CONSTRAINT fk_reservations_event FOREIGN KEY (event_id) REFERENCES events (id)
);

CREATE TABLE IF NOT EXISTS reservation_items (
  reservation_id CHAR(32) NOT NULL,
  ticket_tier_id BIGINT UNSIGNED NOT NULL,
  quantity INT UNSIGNED NOT NULL,
  unit_price BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (reservation_id, ticket_tier_id),
  CONSTRAINT fk_reservation_items_reservation FOREIGN KEY (reservation_id) REFERENCES reservations (id),
  CONSTRAINT fk_reservation_items_tier FOREIGN KEY (ticket_tier_id) REFERENCES ticket_tiers (id),
  CONSTRAINT chk_reservation_items_quantity CHECK (quantity > 0),
  CONSTRAINT chk_reservation_items_price CHECK (unit_price > 0)
);
