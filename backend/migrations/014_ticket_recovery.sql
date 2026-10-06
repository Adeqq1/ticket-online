CREATE TABLE IF NOT EXISTS recovery_requests (
  id CHAR(32) PRIMARY KEY,
  reference VARCHAR(32) NOT NULL,
  pair_key CHAR(64) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  used_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL
);

CREATE TABLE IF NOT EXISTS recovery_tokens (
  token_hash CHAR(64) PRIMARY KEY,
  request_id CHAR(32) NOT NULL,
  order_id CHAR(32) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  used_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  KEY ix_recovery_tokens_request (request_id),
  CONSTRAINT fk_recovery_tokens_request FOREIGN KEY (request_id) REFERENCES recovery_requests (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT fk_recovery_tokens_order FOREIGN KEY (order_id) REFERENCES orders (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT
);

CREATE TABLE IF NOT EXISTS rate_limits (
  bucket CHAR(64) PRIMARY KEY,
  hits INT UNSIGNED NOT NULL,
  reset_at DATETIME(6) NOT NULL,
  KEY ix_rate_limits_reset (reset_at)
);

ALTER TABLE email_queue DROP FOREIGN KEY fk_email_queue_order;

ALTER TABLE email_queue
  ADD COLUMN id CHAR(32) NULL FIRST,
  ADD COLUMN kind ENUM('TICKETS', 'RECOVERY') NOT NULL DEFAULT 'TICKETS' AFTER id,
  ADD COLUMN dedupe_key VARCHAR(80) NULL AFTER kind,
  ADD COLUMN recovery_request_id CHAR(32) NULL AFTER order_id;

UPDATE email_queue SET id = order_id, dedupe_key = CONCAT('tickets:', order_id);

ALTER TABLE email_queue
  DROP PRIMARY KEY,
  MODIFY id CHAR(32) NOT NULL,
  ADD PRIMARY KEY (id),
  MODIFY dedupe_key VARCHAR(80) NOT NULL,
  ADD UNIQUE KEY uq_email_queue_dedupe (dedupe_key),
  MODIFY order_id CHAR(32) NULL,
  ADD KEY ix_email_queue_order (order_id),
  ADD KEY ix_email_queue_recovery (recovery_request_id);

ALTER TABLE email_queue
  ADD CONSTRAINT fk_email_queue_order FOREIGN KEY (order_id) REFERENCES orders (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT,
  ADD CONSTRAINT fk_email_queue_recovery FOREIGN KEY (recovery_request_id) REFERENCES recovery_requests (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT,
  ADD CONSTRAINT chk_email_queue_target CHECK (
    (kind = 'TICKETS' AND order_id IS NOT NULL AND recovery_request_id IS NULL)
    OR (kind = 'RECOVERY' AND order_id IS NULL AND recovery_request_id IS NOT NULL)
  )
