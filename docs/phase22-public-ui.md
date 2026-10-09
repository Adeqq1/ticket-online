# Phase 22: Detail Konser dan Pemilihan Tiket

## Progress

- [x] Utamakan judul, jadwal efektif, venue/kota, harga awal, dan status perubahan acara.
- [x] Pindahkan pemilihan tiket sebelum panel deskripsi, lineup, lokasi, dan ketentuan.
- [x] Jadikan peta zona panel “Lihat area”, tertutup pada mobile dan terbuka pada desktop.
- [x] Perjelas harga per tiket, manfaat, stok, batas pembelian, dan kontrol jumlah.
- [x] Pertahankan ringkasan desktop dan tampilkan bilah aksi mobile sejak nol tiket.
- [x] Jaga safe area, fokus keyboard, dan area gulir agar bilah bawah tidak menutup tindakan.
- [x] Verifikasi status acara, tiket, hak refund, error, dan alur menuju checkout.

## Perilaku UI

Poster detail memakai rasio 4:5 tanpa backdrop buram. Jadwal utama mengikuti `currentEvent`, termasuk pesan jika tanggal terbaru belum tersedia. Informasi perubahan dan hak refund tampil di atas kategori tiket. Harga awal dihitung dari tier yang dikembalikan API.

Kategori tetap terlihat saat penjualan ditutup; jumlah dan checkout dikunci. Peta zona dibuka melalui `<details>` dan tetap terhubung dengan kategori tiket. Deskripsi, lineup, lokasi, dan ketentuan juga memakai panel native yang bisa dibuka.

Bilah mobile menampilkan jumlah dan subtotal sejak nol tiket, serta menonaktifkan checkout sampai pilihan valid dan penjualan tersedia. Tinggi aktual bilah mengatur ruang gulir dan fokus; fokus keyboard digulir langsung melewati bilah tanpa mengganggu interaksi sentuh. API, URL checkout, query kategori, dan penyimpanan tetap memakai kontrak yang ada.

## Screenshot dan Pemeriksaan

Capture Phase 22 tersimpan di `docs/phase22/screenshots/` dan tidak mengubah baseline Phase 20 atau hasil Phase 21. Runner mengambil semua rute publik, lalu memeriksa ringkasan awal, pemilihan tier/subtotal, parameter checkout, posisi fokus terakhir terhadap bilah, jadwal berubah beserta tenggat refund, pembatalan, penjualan habis, tier kosong, dan retry error.

Verifikasi: `bun test`, `bun run check`, `bun run build`, dan `python3 scripts/public-ui-check.py --current --output-dir docs/phase22/screenshots`. Build menyalin `_redirects`; runner menguji overflow 320px, mode gelap, font Manrope, isolasi admin, serta checkpoint Phase 21 pada 360px.
