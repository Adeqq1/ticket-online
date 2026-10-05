ALTER TABLE payments
  MODIFY status ENUM('PENDING', 'FAILED', 'SUCCEEDED') NOT NULL,
  ADD COLUMN gateway_order_id VARCHAR(50) NULL,
  ADD COLUMN gateway_reference VARCHAR(100) NULL,
  ADD COLUMN redirect_url TEXT NULL,
  ADD UNIQUE KEY uq_payments_gateway_order_id (gateway_order_id),
  ADD UNIQUE KEY uq_payments_gateway_reference (gateway_reference);
