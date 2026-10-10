# Phase 24: Tiket, Pemulihan, dan Panduan

## Progress

- [x] Tiket Saya memakai kartu ringkas dengan acara, jadwal terkini, peserta, status, kategori/gate, dan akses e-ticket. Jalur pemulihan berada di atas daftar pada kondisi kosong maupun terisi.
- [x] E-ticket menampilkan status, perubahan acara, peserta, jadwal, gate, dan QR sebelum rincian tambahan. Rincian penerbitan dan pesanan dapat dilipat.
- [x] QR tetap hitam di atas putih dengan ruang kosong. QR tidak dirender saat backend menandai tiket tidak aktif.
- [x] Cetak/PDF membuka rincian lengkap, lalu mengembalikan keadaan panel setelah dialog cetak ditutup.
- [x] Pemulihan menjelaskan email pembeli, kode pesanan `TO-…`, perbedaan kode tiket `ET-…`, dan langkah membuka tautan.
- [x] Token privat, respons umum, validasi, batas permintaan, penanganan tautan sekali pakai, dan keadaan penyimpanan gagal tetap dipertahankan.
- [x] Panduan disusun per topik accordion dan menjelaskan alur pembayaran terkonfirmasi, QR, akses, dan cetak tanpa klaim simulasi atau QR yang sudah usang.

## Pemeriksaan

- `bun test`: lulus, 101 tes.
- `bun run check`: lulus, tanpa error atau peringatan.
- `bun run build`: lulus.
- `python3 scripts/public-ui-check.py --current --output-dir docs/phase24/screenshots`: lulus, 69 screenshot desktop/mobile. Pemeriksaan mencakup layar 320px, QR aktif/nonaktif, perubahan acara, cetak, accordion panduan, dan validasi pemulihan.

Screenshot dan manifest tersimpan di `docs/phase24/screenshots/`. API, URL, dan format penyimpanan tidak berubah.
