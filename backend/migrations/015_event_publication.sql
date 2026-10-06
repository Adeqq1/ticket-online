ALTER TABLE events
  ADD COLUMN publication_status ENUM('DRAFT', 'PUBLISHED', 'ARCHIVED') NOT NULL DEFAULT 'PUBLISHED';

UPDATE events SET publication_status = 'PUBLISHED';

ALTER TABLE events ALTER COLUMN publication_status SET DEFAULT 'DRAFT';
