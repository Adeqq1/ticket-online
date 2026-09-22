INSERT INTO events (id, artist, city, venue, address, starts_at, genre, status, image_url, description, created_at, updated_at) VALUES
  ('nusa-malam', 'Nusa Malam', 'Jakarta', 'Ruang Selatan', 'Jl. Musik Raya, Jakarta', '2027-08-24 12:30:00', 'Indie', 'EARLY_BIRD', 'https://picsum.photos/seed/nusa-malam/900/1100', 'Nusa Malam hadir dalam konser contoh dengan pilihan tiket untuk area panggung dan penonton.', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)),
  ('ruang-senja', 'Ruang Senja', 'Bandung', 'Gudang Bunyi', 'Jl. Musik Raya, Bandung', '2027-08-30 12:30:00', 'Rock', 'PRESALE', 'https://picsum.photos/seed/ruang-senja/900/1100', 'Ruang Senja hadir dalam konser contoh dengan pilihan tiket untuk area panggung dan penonton.', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)),
  ('dini-hari', 'Dini Hari', 'Yogyakarta', 'Balai Irama', 'Jl. Musik Raya, Yogyakarta', '2027-09-08 12:30:00', 'Pop', 'PRESALE', 'https://picsum.photos/seed/dini-hari/900/1100', 'Dini Hari hadir dalam konser contoh dengan pilihan tiket untuk area panggung dan penonton.', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)),
  ('laut-kaca', 'Laut Kaca', 'Surabaya', 'Panggung Timur', 'Jl. Musik Raya, Surabaya', '2027-09-14 12:30:00', 'Indie', 'SOLD_OUT', 'https://picsum.photos/seed/laut-kaca/900/1100', 'Laut Kaca hadir dalam konser contoh dengan pilihan tiket untuk area panggung dan penonton.', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)),
  ('ritme-kota', 'Ritme Kota', 'Jakarta', 'Aula Tengah', 'Jl. Musik Raya, Jakarta', '2027-09-21 12:30:00', 'Pop', 'EARLY_BIRD', 'https://picsum.photos/seed/ritme-kota/900/1100', 'Ritme Kota hadir dalam konser contoh dengan pilihan tiket untuk area panggung dan penonton.', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)),
  ('bara-utara', 'Bara Utara', 'Bandung', 'Pabrik Nada', 'Jl. Musik Raya, Bandung', '2027-09-27 12:30:00', 'Rock', 'PRESALE', 'https://picsum.photos/seed/bara-utara/900/1100', 'Bara Utara hadir dalam konser contoh dengan pilihan tiket untuk area panggung dan penonton.', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)),
  ('kamar-biru', 'Kamar Biru', 'Yogyakarta', 'Teras Suara', 'Jl. Musik Raya, Yogyakarta', '2027-10-05 12:30:00', 'Indie', 'EARLY_BIRD', 'https://picsum.photos/seed/kamar-biru/900/1100', 'Kamar Biru hadir dalam konser contoh dengan pilihan tiket untuk area panggung dan penonton.', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)),
  ('gelombang-pagi', 'Gelombang Pagi', 'Surabaya', 'Titik Temu', 'Jl. Musik Raya, Surabaya', '2027-10-13 12:30:00', 'Pop', 'PRESALE', 'https://picsum.photos/seed/gelombang-pagi/900/1100', 'Gelombang Pagi hadir dalam konser contoh dengan pilihan tiket untuk area panggung dan penonton.', UTC_TIMESTAMP(6), UTC_TIMESTAMP(6));

INSERT INTO event_zones (event_id, slug, name, description)
SELECT e.id, z.slug, z.name, z.description
FROM events e
JOIN (SELECT 'vip-a' AS slug, 'VIP A' AS name, 'Area depan panggung' AS description UNION ALL SELECT 'vip-b', 'VIP B', 'Area samping panggung' UNION ALL SELECT 'festival', 'Festival', 'Area berdiri umum' UNION ALL SELECT 'tribune', 'Tribune', 'Area duduk bertingkat') z;

INSERT INTO event_lineups (event_id, position, name)
SELECT id, 1, artist FROM events
UNION ALL SELECT id, 2, 'Pembuka Sore' FROM events
UNION ALL SELECT id, 3, 'Tamu Spesial' FROM events;

INSERT INTO ticket_tiers (event_id, slug, name, zone_slug, price, capacity, available_quantity, max_per_order, benefit, gate, seating_mode, created_at, updated_at)
SELECT e.id, t.slug, t.name, t.slug,
  CASE WHEN t.slug = 'festival' THEN CASE e.id WHEN 'nusa-malam' THEN 225000 WHEN 'ruang-senja' THEN 175000 WHEN 'dini-hari' THEN 350000 WHEN 'laut-kaca' THEN 275000 WHEN 'ritme-kota' THEN 450000 WHEN 'bara-utara' THEN 300000 WHEN 'kamar-biru' THEN 150000 ELSE 500000 END ELSE t.price END,
  t.capacity, CASE WHEN e.status = 'SOLD_OUT' THEN 0 ELSE t.capacity END, t.max_per_order, t.benefit, t.gate, t.seating_mode, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)
FROM events e
JOIN (SELECT 'vip-a' AS slug, 'VIP A' AS name, 650000 AS price, 5 AS capacity, 4 AS max_per_order, 'Akses area depan panggung' AS benefit, 'Gate A' AS gate, 'FREE_STANDING' AS seating_mode UNION ALL SELECT 'vip-b', 'VIP B', 475000, 18, 4, 'Akses area samping panggung', 'Gate A', 'FREE_STANDING' UNION ALL SELECT 'festival', 'Festival', 275000, 42, 6, 'Area berdiri umum', 'Gate B', 'FREE_STANDING' UNION ALL SELECT 'tribune', 'Tribune', 350000, 24, 4, 'Area duduk bertingkat', 'Gate C', 'ASSIGNED') t;
