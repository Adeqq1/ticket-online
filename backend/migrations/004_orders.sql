CREATE TABLE IF NOT EXISTS orders (
  id CHAR(32) PRIMARY KEY,
  reference VARCHAR(32) NOT NULL,
  reservation_id CHAR(32) NOT NULL,
  status ENUM('PENDING', 'PAID', 'CANCELLED', 'EXPIRED') NOT NULL DEFAULT 'PENDING',
  subtotal BIGINT UNSIGNED NOT NULL,
  admin_fee BIGINT UNSIGNED NOT NULL DEFAULT 0,
  discount BIGINT UNSIGNED NOT NULL DEFAULT 0,
  total BIGINT UNSIGNED GENERATED ALWAYS AS (subtotal + admin_fee - discount) STORED,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_orders_reference (reference),
  UNIQUE KEY uq_orders_reservation (reservation_id),
  CONSTRAINT fk_orders_reservation FOREIGN KEY (reservation_id) REFERENCES reservations (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT chk_orders_subtotal CHECK (subtotal > 0),
  CONSTRAINT chk_orders_discount CHECK (discount <= subtotal)
);

CREATE TABLE IF NOT EXISTS order_items (
  order_id CHAR(32) NOT NULL,
  ticket_tier_id BIGINT UNSIGNED NOT NULL,
  tier_name VARCHAR(100) NOT NULL,
  quantity INT UNSIGNED NOT NULL,
  unit_price BIGINT UNSIGNED NOT NULL,
  line_total BIGINT UNSIGNED GENERATED ALWAYS AS (quantity * unit_price) STORED,
  PRIMARY KEY (order_id, ticket_tier_id),
  CONSTRAINT fk_order_items_order FOREIGN KEY (order_id) REFERENCES orders (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT fk_order_items_ticket_tier FOREIGN KEY (ticket_tier_id) REFERENCES ticket_tiers (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT chk_order_items_quantity CHECK (quantity > 0),
  CONSTRAINT chk_order_items_unit_price CHECK (unit_price > 0)
);

CREATE TABLE IF NOT EXISTS order_buyers (
  order_id CHAR(32) PRIMARY KEY,
  name VARCHAR(80) NOT NULL,
  email VARCHAR(254) NOT NULL,
  phone VARCHAR(32) NOT NULL,
  identity VARCHAR(20) NOT NULL,
  CONSTRAINT fk_order_buyers_order FOREIGN KEY (order_id) REFERENCES orders (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT chk_order_buyers_name CHECK (CHAR_LENGTH(TRIM(name)) >= 2),
  CONSTRAINT chk_order_buyers_email CHECK (CHAR_LENGTH(TRIM(email)) > 0),
  CONSTRAINT chk_order_buyers_phone CHECK (CHAR_LENGTH(TRIM(phone)) > 0),
  CONSTRAINT chk_order_buyers_identity CHECK (identity REGEXP '^[0-9]{12,20}$')
);
