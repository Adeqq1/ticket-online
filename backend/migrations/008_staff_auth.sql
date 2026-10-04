CREATE TABLE IF NOT EXISTS staff_users (
  id CHAR(32) PRIMARY KEY,
  name VARCHAR(80) NOT NULL,
  email VARCHAR(254) NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  role ENUM('ADMIN', 'STAFF') NOT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_staff_users_email (email),
  CONSTRAINT chk_staff_users_name CHECK (CHAR_LENGTH(TRIM(name)) >= 2)
);

CREATE TABLE IF NOT EXISTS staff_sessions (
  token_hash BINARY(32) PRIMARY KEY,
  staff_id CHAR(32) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  KEY ix_staff_sessions_expiry (expires_at),
  CONSTRAINT fk_staff_sessions_user FOREIGN KEY (staff_id) REFERENCES staff_users (id)
    ON DELETE CASCADE ON UPDATE RESTRICT
);

CREATE TABLE IF NOT EXISTS staff_assignments (
  staff_id CHAR(32) NOT NULL,
  event_id VARCHAR(64) NOT NULL,
  gate VARCHAR(100) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (staff_id, event_id, gate),
  CONSTRAINT fk_staff_assignments_user FOREIGN KEY (staff_id) REFERENCES staff_users (id)
    ON DELETE CASCADE ON UPDATE RESTRICT,
  CONSTRAINT fk_staff_assignments_event FOREIGN KEY (event_id) REFERENCES events (id)
    ON DELETE CASCADE ON UPDATE RESTRICT
);
