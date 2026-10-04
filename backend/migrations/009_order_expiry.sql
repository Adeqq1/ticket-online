ALTER TABLE orders ADD COLUMN expires_at DATETIME(6) NULL;

UPDATE orders o JOIN reservations r ON r.id = o.reservation_id
SET o.expires_at = r.expires_at;

ALTER TABLE orders
  MODIFY COLUMN expires_at DATETIME(6) NOT NULL,
  ADD KEY idx_orders_expiry (status, expires_at, id);
