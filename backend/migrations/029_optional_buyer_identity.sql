ALTER TABLE order_buyers
  DROP CHECK chk_order_buyers_identity,
  ADD CONSTRAINT chk_order_buyers_identity CHECK (identity = '' OR identity REGEXP '^[0-9]{12,20}$');
