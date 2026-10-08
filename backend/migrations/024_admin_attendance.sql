ALTER TABLE ticket_checkins
  ADD KEY ix_ticket_checkins_admin_attendance (event_id, gate, checked_in_at);
