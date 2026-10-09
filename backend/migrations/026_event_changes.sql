ALTER TABLE events
  ADD COLUMN lifecycle_status ENUM('SCHEDULED','POSTPONED','RESCHEDULED','CANCELLED') NOT NULL DEFAULT 'SCHEDULED',
  ADD COLUMN change_version BIGINT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN sales_paused BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE orders ADD COLUMN access_deadline DATETIME(6) NULL;
UPDATE orders o JOIN reservations r ON r.id=o.reservation_id JOIN events e ON e.id=r.event_id
SET o.access_deadline=DATE(e.starts_at + INTERVAL 7 HOUR) + INTERVAL 1 DAY - INTERVAL 7 HOUR
WHERE o.access_deadline IS NULL AND e.starts_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS event_changes (
  id CHAR(32) PRIMARY KEY,
  event_id VARCHAR(64) NOT NULL,
  version BIGINT UNSIGNED NOT NULL,
  action ENUM('POSTPONED','RESCHEDULED','CANCELLED') NOT NULL,
  reason VARCHAR(500) NOT NULL,
  announcement VARCHAR(2000) NOT NULL,
  previous_starts_at DATETIME(6) NULL,
  starts_at DATETIME(6) NULL,
  refund_deadline DATETIME(6) NULL,
  staff_id CHAR(32) NOT NULL,
  idempotency_key VARCHAR(100) NOT NULL,
  request_hash CHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_event_change_version (event_id,version),
  UNIQUE KEY uq_event_change_key (event_id,idempotency_key),
  FOREIGN KEY (event_id) REFERENCES events(id),
  FOREIGN KEY (staff_id) REFERENCES staff_users(id)
);

CREATE TABLE IF NOT EXISTS event_change_orders (
  change_id CHAR(32) NOT NULL,
  order_id CHAR(32) NOT NULL,
  stop_pending BOOLEAN NOT NULL,
  processed BOOLEAN NOT NULL DEFAULT FALSE,
  last_error VARCHAR(512) NOT NULL DEFAULT '',
  PRIMARY KEY (change_id,order_id),
  KEY ix_event_change_work (processed,change_id,order_id),
  FOREIGN KEY (change_id) REFERENCES event_changes(id),
  FOREIGN KEY (order_id) REFERENCES orders(id)
);

CREATE TABLE IF NOT EXISTS event_refund_rights (
  order_id CHAR(32) PRIMARY KEY,
  change_id CHAR(32) NOT NULL,
  requested BOOLEAN NOT NULL DEFAULT FALSE,
  deadline DATETIME(6) NULL,
  FOREIGN KEY (order_id) REFERENCES orders(id),
  FOREIGN KEY (change_id) REFERENCES event_changes(id)
);

ALTER TABLE order_refunds
  MODIFY status ENUM('REQUESTED','PROCESSING','SUCCEEDED','FAILED','UNKNOWN','MANUAL_REQUIRED') NOT NULL,
  ADD COLUMN manual_reference VARCHAR(160) NOT NULL DEFAULT '',
  ADD COLUMN manual_paid_at DATETIME(6) NULL;

ALTER TABLE email_queue
  MODIFY kind ENUM('TICKETS','RECOVERY','REFUND','EVENT_CHANGE') NOT NULL,
  ADD COLUMN event_change_id CHAR(32) NULL,
  DROP CHECK chk_email_queue_target,
  ADD CONSTRAINT chk_email_queue_target CHECK (
    (kind='TICKETS' AND order_id IS NOT NULL AND recovery_request_id IS NULL) OR
    (kind='REFUND' AND order_id IS NOT NULL AND recovery_request_id IS NULL AND refund_snapshot IS NOT NULL) OR
    (kind='EVENT_CHANGE' AND order_id IS NOT NULL AND recovery_request_id IS NULL AND event_change_id IS NOT NULL) OR
    (kind='RECOVERY' AND order_id IS NULL AND recovery_request_id IS NOT NULL)
  );

ALTER TABLE checkin_attempts MODIFY outcome ENUM('CHECKED_IN','TICKET_ALREADY_USED','INVALID_REQUEST','TICKET_NOT_FOUND','ORDER_NOT_PAID','WRONG_GATE','FORBIDDEN','EVENT_CHANGED') NOT NULL;
