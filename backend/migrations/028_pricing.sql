CREATE TABLE IF NOT EXISTS checkout_settings (
  id TINYINT PRIMARY KEY CHECK (id = 1),
  admin_fee BIGINT UNSIGNED NOT NULL CHECK (admin_fee <= 1000000000),
  revision BIGINT UNSIGNED NOT NULL DEFAULT 1
);
INSERT IGNORE INTO checkout_settings (id, admin_fee) VALUES (1, 7500);

CREATE TABLE IF NOT EXISTS vouchers (
  code VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
  kind ENUM('PERCENT','FIXED') NOT NULL,
  value BIGINT UNSIGNED NOT NULL,
  starts_at DATETIME(6) NOT NULL,
  ends_at DATETIME(6) NOT NULL,
  active BOOLEAN NOT NULL,
  event_id VARCHAR(64) NULL,
  minimum_subtotal BIGINT UNSIGNED NOT NULL,
  quota BIGINT UNSIGNED NOT NULL,
  revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
  FOREIGN KEY (event_id) REFERENCES events(id),
  CHECK (value > 0 AND value <= 1000000000 AND (kind <> 'PERCENT' OR value <= 100)),
  CHECK (ends_at > starts_at AND quota > 0 AND quota <= 1000000000 AND minimum_subtotal <= 1000000000000)
);
INSERT IGNORE INTO vouchers (code,kind,value,starts_at,ends_at,active,event_id,minimum_subtotal,quota)
VALUES ('HEMAT10','PERCENT',10,'2020-01-01','2099-01-01',TRUE,NULL,0,1000000000);

CREATE TABLE IF NOT EXISTS order_vouchers (
  order_id CHAR(32) PRIMARY KEY,
  code VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  kind ENUM('PERCENT','FIXED') NOT NULL,
  value BIGINT UNSIGNED NOT NULL,
  consumed BOOLEAN NOT NULL DEFAULT FALSE,
  FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE,
  FOREIGN KEY (code) REFERENCES vouchers(code),
  INDEX ix_order_vouchers_usage (code,consumed)
);
INSERT IGNORE INTO order_vouchers (order_id,code,kind,value,consumed)
SELECT id,'HEMAT10','PERCENT',10,status IN ('PAID','REFUND_PENDING','REFUNDED') FROM orders WHERE discount > 0;
