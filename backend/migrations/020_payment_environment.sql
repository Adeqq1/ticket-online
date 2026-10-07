CREATE TABLE payment_environment (
  singleton TINYINT UNSIGNED NOT NULL PRIMARY KEY CHECK (singleton = 1),
  environment ENUM('sandbox', 'production') NOT NULL,
  created_at DATETIME(6) NOT NULL
);

INSERT IGNORE INTO payment_environment (singleton, environment, created_at)
SELECT 1, 'sandbox', UTC_TIMESTAMP(6)
WHERE EXISTS (SELECT 1 FROM payments WHERE gateway_order_id IS NOT NULL);
