CREATE TABLE IF NOT EXISTS admin_audit_log (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  actor_id VARCHAR(64) NOT NULL,
  actor_name VARCHAR(80) NOT NULL,
  action VARCHAR(16) NOT NULL,
  object_type VARCHAR(24) NOT NULL,
  object_id VARCHAR(140) NOT NULL,
  before_json JSON NULL,
  after_json JSON NOT NULL,
  created_at DATETIME(6) NOT NULL,
  INDEX idx_admin_audit_object (object_type, object_id, created_at),
  INDEX idx_admin_audit_actor (actor_id, created_at)
);
