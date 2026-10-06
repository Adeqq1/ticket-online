-- Resumed by resumeAdminCatalog because MySQL commits ALTER TABLE implicitly.
ALTER TABLE events MODIFY starts_at DATETIME(6) NULL;
ALTER TABLE ticket_tiers DROP CHECK chk_ticket_tiers_capacity;
ALTER TABLE ticket_tiers ADD CONSTRAINT chk_ticket_tiers_capacity CHECK (capacity >= 0);
