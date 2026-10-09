CREATE TABLE IF NOT EXISTS conversion_journeys (
  id CHAR(32) PRIMARY KEY,
  event_id VARCHAR(64) NOT NULL,
  device ENUM('mobile', 'desktop', 'unknown') NOT NULL,
  started_at DATETIME(6) NOT NULL,
  detail_viewed_at DATETIME(6) NULL,
  KEY ix_conversion_event_started (event_id, started_at),
  CONSTRAINT fk_conversion_journey_event FOREIGN KEY (event_id) REFERENCES events (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT
);

CREATE TABLE IF NOT EXISTS conversion_reservations (
  reservation_id CHAR(32) PRIMARY KEY,
  journey_id CHAR(32) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  KEY ix_conversion_reservation_journey (journey_id),
  CONSTRAINT fk_conversion_reservation_reservation FOREIGN KEY (reservation_id) REFERENCES reservations (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT fk_conversion_reservation_journey FOREIGN KEY (journey_id) REFERENCES conversion_journeys (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT
);

CREATE TABLE IF NOT EXISTS conversion_events (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  journey_id CHAR(32) NOT NULL,
  kind ENUM('VALIDATION_FAILED', 'RESERVATION_EXPIRED', 'STOCK_UNAVAILABLE', 'PAYMENT_FAILURE', 'SERVICE_FAILURE') NOT NULL,
  reason ENUM('BUYER_DATA', 'ATTENDEE_DATA', 'RESERVATION', 'PAYMENT_PROVIDER', 'SERVICE') NOT NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_conversion_event_once (journey_id, kind, reason),
  KEY ix_conversion_event_journey (journey_id),
  CONSTRAINT fk_conversion_event_journey FOREIGN KEY (journey_id) REFERENCES conversion_journeys (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT
);

ALTER TABLE payments ADD COLUMN started_at DATETIME(6) NULL AFTER status;
