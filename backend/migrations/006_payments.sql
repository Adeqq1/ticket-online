CREATE TABLE IF NOT EXISTS payments (
  id CHAR(32) PRIMARY KEY,
  order_id CHAR(32) NOT NULL,
  method ENUM('QRIS', 'VIRTUAL_ACCOUNT', 'GOPAY') NOT NULL,
  amount BIGINT UNSIGNED NOT NULL,
  status ENUM('FAILED', 'SUCCEEDED') NOT NULL,
  paid_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_payments_order (order_id),
  CONSTRAINT fk_payments_order FOREIGN KEY (order_id) REFERENCES orders (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT chk_payments_amount CHECK (amount > 0)
);
