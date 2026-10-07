ALTER TABLE orders
  MODIFY status ENUM('PENDING', 'PAID', 'CANCELLED', 'EXPIRED', 'REFUND_PENDING', 'REFUNDED') NOT NULL DEFAULT 'PENDING';

CREATE TABLE order_refunds (
  id CHAR(32) PRIMARY KEY,
  order_id CHAR(32) NOT NULL,
  status ENUM('REQUESTED', 'PROCESSING', 'SUCCEEDED', 'FAILED', 'UNKNOWN') NOT NULL,
  original_order_status ENUM('PAID', 'CANCELLED', 'EXPIRED') NOT NULL,
  amount BIGINT UNSIGNED NOT NULL,
  reason VARCHAR(500) NOT NULL,
  requested_by_staff_id CHAR(32) NOT NULL,
  refund_key VARCHAR(80) NOT NULL,
  gateway_order_id VARCHAR(50) NOT NULL,
  provider_transaction_id VARCHAR(100) NOT NULL DEFAULT '',
  attempts TINYINT UNSIGNED NOT NULL DEFAULT 0,
  last_error VARCHAR(512) NOT NULL DEFAULT '',
  requested_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  completed_at DATETIME(6) NULL,
  UNIQUE KEY uq_order_refunds_order (order_id),
  UNIQUE KEY uq_order_refunds_key (refund_key),
  KEY ix_order_refunds_reconcile (status, updated_at),
  CONSTRAINT fk_order_refunds_order FOREIGN KEY (order_id) REFERENCES orders (id) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT fk_order_refunds_staff FOREIGN KEY (requested_by_staff_id) REFERENCES staff_users (id) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT chk_order_refunds_amount CHECK (amount > 0)
);

CREATE TABLE order_refund_audit (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  refund_id CHAR(32) NOT NULL,
  staff_id CHAR(32) NOT NULL,
  action VARCHAR(32) NOT NULL,
  reason VARCHAR(500) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  KEY ix_order_refund_audit_refund (refund_id, id),
  CONSTRAINT fk_order_refund_audit_refund FOREIGN KEY (refund_id) REFERENCES order_refunds (id) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT fk_order_refund_audit_staff FOREIGN KEY (staff_id) REFERENCES staff_users (id) ON DELETE RESTRICT ON UPDATE RESTRICT
);

ALTER TABLE email_queue
  MODIFY kind ENUM('TICKETS', 'RECOVERY', 'REFUND') NOT NULL,
  ADD COLUMN refund_snapshot JSON NULL,
  DROP CHECK chk_email_queue_target,
  ADD CONSTRAINT chk_email_queue_target CHECK (
    (kind = 'TICKETS' AND order_id IS NOT NULL AND recovery_request_id IS NULL) OR
    (kind = 'REFUND' AND order_id IS NOT NULL AND recovery_request_id IS NULL AND refund_snapshot IS NOT NULL) OR
    (kind = 'RECOVERY' AND order_id IS NULL AND recovery_request_id IS NOT NULL)
  );
