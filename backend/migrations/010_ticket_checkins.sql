CREATE TABLE IF NOT EXISTS ticket_checkins (
  ticket_id CHAR(32) PRIMARY KEY,
  staff_id CHAR(32) NOT NULL,
  event_id VARCHAR(64) NOT NULL,
  gate VARCHAR(100) NOT NULL,
  checked_in_at DATETIME(6) NOT NULL,
  CONSTRAINT fk_ticket_checkins_ticket FOREIGN KEY (ticket_id) REFERENCES etickets (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT fk_ticket_checkins_staff FOREIGN KEY (staff_id) REFERENCES staff_users (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT fk_ticket_checkins_event FOREIGN KEY (event_id) REFERENCES events (id)
    ON DELETE RESTRICT ON UPDATE RESTRICT
);
