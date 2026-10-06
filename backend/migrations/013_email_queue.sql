CREATE TABLE IF NOT EXISTS email_queue (
  order_id CHAR(32) PRIMARY KEY,
  recipient VARCHAR(254) NOT NULL,
  status ENUM('PENDING', 'PROCESSING', 'SENT', 'FAILED') NOT NULL DEFAULT 'PENDING',
  attempts TINYINT UNSIGNED NOT NULL DEFAULT 0,
  next_attempt_at DATETIME(6) NOT NULL,
  lease_until DATETIME(6) NULL,
  claim_token CHAR(32) NULL,
  sent_at DATETIME(6) NULL,
  last_error VARCHAR(512) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  KEY ix_email_queue_pending (status, next_attempt_at, order_id),
  KEY ix_email_queue_lease (status, lease_until, order_id),
  CONSTRAINT fk_email_queue_order FOREIGN KEY (order_id) REFERENCES orders (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT chk_email_queue_attempts CHECK (attempts <= 3)
);
