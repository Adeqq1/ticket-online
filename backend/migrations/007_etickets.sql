CREATE TABLE IF NOT EXISTS order_attendees (
  order_id CHAR(32) NOT NULL,
  ticket_tier_id BIGINT UNSIGNED NOT NULL,
  ticket_number INT UNSIGNED NOT NULL,
  name VARCHAR(80) NOT NULL,
  PRIMARY KEY (order_id, ticket_tier_id, ticket_number),
  CONSTRAINT fk_order_attendees_item FOREIGN KEY (order_id, ticket_tier_id) REFERENCES order_items (order_id, ticket_tier_id)
    ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT chk_order_attendees_ticket_number CHECK (ticket_number > 0),
  CONSTRAINT chk_order_attendees_name CHECK (CHAR_LENGTH(TRIM(name)) >= 2)
);

CREATE TABLE IF NOT EXISTS etickets (
  id CHAR(32) PRIMARY KEY,
  order_id CHAR(32) NOT NULL,
  ticket_tier_id BIGINT UNSIGNED NOT NULL,
  ticket_number INT UNSIGNED NOT NULL,
  snapshot JSON NOT NULL,
  issued_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_etickets_order_slot (order_id, ticket_tier_id, ticket_number),
  CONSTRAINT fk_etickets_attendee FOREIGN KEY (order_id, ticket_tier_id, ticket_number)
    REFERENCES order_attendees (order_id, ticket_tier_id, ticket_number)
    ON DELETE RESTRICT ON UPDATE RESTRICT
);
