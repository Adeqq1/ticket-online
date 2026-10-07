ALTER TABLE order_refunds
  ADD COLUMN next_attempt_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  ADD COLUMN last_checked_at DATETIME(6) NULL,
  ADD COLUMN last_check_error VARCHAR(512) NOT NULL DEFAULT '',
  ADD COLUMN claim_token CHAR(32) NULL,
  ADD COLUMN lease_until DATETIME(6) NULL,
  ADD COLUMN retry_deadline DATETIME(6) NULL,
  ADD KEY ix_order_refunds_due (status, next_attempt_at, id);
