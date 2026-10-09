ALTER TABLE event_change_orders
  ADD COLUMN next_attempt_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  ADD KEY ix_event_change_due (processed,next_attempt_at,change_id,order_id);

ALTER TABLE event_refund_rights
  ADD COLUMN next_attempt_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  ADD KEY ix_event_refund_due (requested,next_attempt_at,order_id);

ALTER TABLE conversion_journeys
  ADD COLUMN telemetry_received BOOLEAN NOT NULL DEFAULT TRUE;
