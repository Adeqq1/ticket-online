ALTER TABLE reservations
  ADD COLUMN order_reference VARCHAR(32) NULL,
  ADD UNIQUE KEY uq_reservations_order_reference (order_reference);
