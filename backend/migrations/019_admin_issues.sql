ALTER TABLE payment_reconciliation_cases
  ADD COLUMN reason VARCHAR(80) NOT NULL DEFAULT 'PAYMENT_SUCCEEDED_AFTER_ORDER_CLOSED',
  ADD COLUMN last_checked_at DATETIME(6) NULL,
  ADD COLUMN last_check_error VARCHAR(512) NOT NULL DEFAULT '',
  ADD COLUMN check_token CHAR(32) NULL,
  ADD COLUMN check_lease_until DATETIME(6) NULL;

ALTER TABLE email_queue
  ADD COLUMN superseded_by CHAR(32) NULL,
  ADD KEY ix_email_queue_admin_failed (status, updated_at, id);
