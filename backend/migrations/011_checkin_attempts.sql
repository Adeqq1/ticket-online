CREATE TABLE IF NOT EXISTS checkin_attempts (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  checked_in_ticket_id CHAR(32) NULL UNIQUE,
  ticket_code VARCHAR(35) NULL,
  event_id VARCHAR(64) NULL,
  gate VARCHAR(100) NULL,
  staff_id CHAR(32) NOT NULL,
  staff_name VARCHAR(80) NOT NULL,
  outcome ENUM('CHECKED_IN', 'TICKET_ALREADY_USED', 'INVALID_REQUEST', 'TICKET_NOT_FOUND', 'ORDER_NOT_PAID', 'WRONG_GATE', 'FORBIDDEN') NOT NULL,
  recorded_at DATETIME(6) NOT NULL,
  checked_in_at DATETIME(6) NULL,
  KEY ix_checkin_attempts_event_gate_id (event_id, gate, id),
  KEY ix_checkin_attempts_ticket_code_id (ticket_code, id),
  CONSTRAINT fk_checkin_attempts_staff FOREIGN KEY (staff_id) REFERENCES staff_users (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT fk_checkin_attempts_ticket FOREIGN KEY (checked_in_ticket_id) REFERENCES etickets (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT
);

INSERT IGNORE INTO checkin_attempts
  (checked_in_ticket_id, ticket_code, event_id, gate, staff_id, staff_name, outcome, recorded_at, checked_in_at)
SELECT c.ticket_id, JSON_UNQUOTE(JSON_EXTRACT(t.snapshot, '$.code')), c.event_id, c.gate,
  c.staff_id, s.name, 'CHECKED_IN', c.checked_in_at, c.checked_in_at
FROM ticket_checkins c
JOIN etickets t ON t.id = c.ticket_id
JOIN staff_users s ON s.id = c.staff_id
ORDER BY c.checked_in_at, c.ticket_id;
