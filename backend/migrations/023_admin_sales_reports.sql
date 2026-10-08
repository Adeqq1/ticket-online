ALTER TABLE payments
  ADD KEY ix_payments_admin_sales (status, paid_at, order_id);

ALTER TABLE order_refunds
  ADD KEY ix_order_refunds_admin_sales (status, completed_at, order_id);
