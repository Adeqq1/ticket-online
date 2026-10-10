# Phase 31 — Pengelolaan Konser dan Petugas

## Perubahan

- [x] Form konser dikelompokkan mengikuti urutan field yang ada; daftar dan editor diberi status perubahan belum disimpan.
- [x] Pergantian konser, zona, kategori, editor petugas, dan logout menjaga draf melalui konfirmasi; navigasi/penutupan tab memakai peringatan browser.
- [x] Jadwal, lokasi, dan gate yang terkunci menjelaskan alasan serta alur yang tersedia.
- [x] Konser, zona, dan kategori baru mendapat saran ID dari nama. Saran berhenti mengikuti nama setelah ID diedit; konflik lokal/API dapat dikoreksi.
- [x] URL poster menampilkan pratinjau dan pesan ketika gambar gagal dimuat.
- [x] Profil, penugasan, dan reset password tetap tersimpan lewat tindakan terpisah dengan konsekuensi akses/sesi yang dijelaskan.
- [x] Daftar petugas dan penugasan memakai nama event/gate dari katalog API.
- [x] Tidak ada endpoint, route, nama field, atau kontrol simpan baru.

## Verifikasi

- `bun test`: 109 lulus.
- `bun run check`: 0 error, 0 warning.
- `bun run build`: berhasil.
- Runner UI dengan fixture lokal mencakup keadaan normal, loading, kosong, error, dan role ditolak pada desktop 1440×900 serta mobile 390×844. Ukuran 320px juga diperiksa.
- Manifest events dan staff mencatat nol redirect tak terduga, nol API fixture yang tidak dipetakan, nol runtime error, nol overflow, dan nol target interaktif di bawah 44px.
- Hasil dan gambar: [events](phase31/screenshots/events/manifest.json), [petugas](phase31/screenshots/staff/manifest.json).

## Batas audit taste

Mode Preserve dipakai; logo, lime, font, URL, label navigasi, nama/urutan field, dan kebijakan dipertahankan. Hero, promosi, CTA pemasaran, dan aturan landing page diberi N/A karena layar ini menjalankan tugas admin. Upload poster, voucher, biaya admin, secret, dan konfigurasi payment tetap di luar cakupan API.
