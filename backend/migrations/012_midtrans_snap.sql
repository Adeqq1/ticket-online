ALTER TABLE payments
  MODIFY status ENUM('PENDING', 'FAILED', 'SUCCEEDED') NOT NULL,
  ADD COLUMN gateway_order_id VARCHAR(50) NULL,
  ADD COLUMN gateway_reference VARCHAR(100) NULL,
  ADD COLUMN redirect_url TEXT NULL,
  ADD UNIQUE KEY uq_payments_gateway_order_id (gateway_order_id),
  ADD UNIQUE KEY uq_payments_gateway_reference (gateway_reference);

CREATE TABLE payment_reconciliation_cases (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  order_id CHAR(32) NOT NULL,
  gateway_order_id VARCHAR(50) NOT NULL,
  amount BIGINT UNSIGNED NOT NULL,
  provider_status VARCHAR(32) NOT NULL,
  status ENUM('OPEN', 'RESOLVED') NOT NULL DEFAULT 'OPEN',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_payment_reconciliation_gateway_order (gateway_order_id),
  KEY idx_payment_reconciliation_status_created (status, created_at),
  CONSTRAINT fk_payment_reconciliation_order FOREIGN KEY (order_id) REFERENCES orders (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT
);
