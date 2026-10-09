-- Run on MySQL 8.4 with a SELECT-only account. Dates are WIB calendar dates.
-- Override these session variables before sourcing this file, or leave them NULL.
-- The end date is exclusive. Never put buyer data in session variables.
SET @phase12_date_from = COALESCE(@phase12_date_from, DATE_FORMAT(DATE(UTC_TIMESTAMP() + INTERVAL 7 HOUR) - INTERVAL 30 DAY, '%Y-%m-%d'));
SET @phase12_date_to_exclusive = COALESCE(@phase12_date_to_exclusive, DATE_FORMAT(DATE(UTC_TIMESTAMP() + INTERVAL 7 HOUR), '%Y-%m-%d'));

SET TRANSACTION ISOLATION LEVEL REPEATABLE READ;
START TRANSACTION WITH CONSISTENT SNAPSHOT, READ ONLY;

WITH dates AS (
    SELECT STR_TO_DATE(@phase12_date_from, '%Y-%m-%d') AS date_from,
        STR_TO_DATE(@phase12_date_to_exclusive, '%Y-%m-%d') AS date_to_exclusive
), period AS (
    SELECT date_from, date_to_exclusive,
        CAST(date_from AS DATETIME) - INTERVAL 7 HOUR AS from_utc,
        CAST(date_to_exclusive AS DATETIME) - INTERVAL 7 HOUR AS to_utc,
        @phase12_date_from REGEXP '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'
            AND @phase12_date_to_exclusive REGEXP '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'
            AND DATE_FORMAT(date_from, '%Y-%m-%d') = @phase12_date_from
            AND DATE_FORMAT(date_to_exclusive, '%Y-%m-%d') = @phase12_date_to_exclusive
            AND date_from < date_to_exclusive
            AND DATEDIFF(date_to_exclusive, date_from) <= 366
            AND date_to_exclusive <= DATE(UTC_TIMESTAMP() + INTERVAL 7 HOUR) AS valid
    FROM dates
), successful_orders AS (
    -- payments.order_id and order_buyers.order_id are unique: one row per order.
    -- ponytail: email is a buyer proxy, replace only when verified buyer accounts exist.
    SELECT p.order_id, p.paid_at,
        CAST(NULLIF(LOWER(TRIM(b.email)), '') AS BINARY) AS buyer_key
    FROM payments p
    LEFT JOIN order_buyers b ON b.order_id = p.order_id
    CROSS JOIN period d
    WHERE d.valid AND p.status = 'SUCCEEDED' AND p.paid_at < d.to_utc
), buyers AS (
    SELECT buyer_key, COUNT(*) AS historical_orders,
        SUM(paid_at >= d.from_utc) AS period_orders
    FROM successful_orders
    CROSS JOIN period d
    WHERE buyer_key IS NOT NULL
    GROUP BY buyer_key
), totals AS (
    SELECT COUNT(*) AS paid_orders,
        COUNT(CASE WHEN buyer_key IS NULL THEN 1 END) AS orders_without_buyer
    FROM successful_orders
    CROSS JOIN period d
    WHERE paid_at >= d.from_utc
), buyer_totals AS (
    SELECT COUNT(*) AS unique_buyers,
        COUNT(CASE WHEN historical_orders >= 2 THEN 1 END) AS repeat_buyers
    FROM buyers WHERE period_orders > 0
)
SELECT @phase12_date_from AS date_from,
    @phase12_date_to_exclusive AS date_to_exclusive,
    'Asia/Jakarta' AS time_zone, UTC_TIMESTAMP(6) AS snapshot_at_utc,
    COALESCE((SELECT environment FROM payment_environment WHERE singleton = 1), 'unconfigured') AS payment_environment,
    CASE WHEN d.valid THEN 'OK' ELSE 'INVALID_PERIOD' END AS period_status,
    CASE WHEN d.valid THEN t.paid_orders END AS paid_orders,
    CASE WHEN d.valid THEN t.orders_without_buyer END AS orders_without_buyer,
    CASE WHEN d.valid THEN b.unique_buyers END AS unique_buyers,
    CASE WHEN d.valid THEN b.repeat_buyers END AS repeat_buyers,
    CASE WHEN d.valid THEN ROUND(100.0 * b.repeat_buyers / NULLIF(b.unique_buyers, 0), 2) END AS repeat_buyer_percent
FROM period d CROSS JOIN totals t CROSS JOIN buyer_totals b;

COMMIT;
