# Phase 21: Navigasi, Beranda, dan Katalog

## Progress

- [x] Rapikan hierarki header dengan mempertahankan logo, label, navigasi, dan URL yang ada.
- [x] Ringkas hero; utamakan pencarian dan poster event dari API, hilangkan foto dekoratif dan contoh tiket.
- [x] Simpan seluruh hasil event untuk pencarian beranda; tetap tampilkan empat event awal dan tautan ke katalog.
- [x] Samakan isi kartu konser untuk nama, tanggal WIB, venue/kota, harga awal tier, dan ketersediaan.
- [x] Letakkan pencarian dan jumlah hasil katalog sebelum filter tambahan yang dapat dilipat, dengan ringkasan dan reset.
- [x] Ganti carousel beranda dengan grid desktop dan satu kolom kartu horizontal di mobile.
- [x] Verifikasi layar 360px, interaksi filter, pencarian event kelima, dan hasil build.

## Perilaku UI

Hero beranda menggunakan teks ringkas dan CTA menuju daftar konser. Pencarian beranda tetap memakai tombol atau Enter; daftar awal berisi hingga empat event, sedangkan pencarian memakai seluruh hasil API yang sudah dimuat. Kartu beranda dan katalog memakai poster event, tanggal efektif bila ada perubahan jadwal, serta harga terendah dari tier tiket.

Katalog menampilkan kolom pencarian dan jumlah hasil sebelum filter. Filter genre, kota, dan anggaran memakai elemen `details` native: terbuka di desktop dan tertutup pada mobile. Ringkasan kondisi filter dan Reset tetap terlihat saat panel tertutup. Navigasi, kontrak API, rute, dan penyimpanan tidak berubah.

## Screenshot dan Pemeriksaan

Runner Phase 20 menerima `--output-dir` agar screenshot fase berikutnya tidak menimpa baseline. Hasil Phase 21 ada di `docs/phase21/screenshots/`; cakupan penuh desktop/mobile mengikuti runner publik, ditambah capture dan pemeriksaan interaksi pada 360×800px. Fixture Playwright menyediakan lima event sehingga tes browser memastikan pencarian menemukan event kelima. Skenario tambahan memeriksa hasil kosong, acara habis, tier kosong, perubahan jadwal, dan poster gagal.

Pemeriksaan: `bun test`, `bun run check`, `bun run build`, dan `python3 scripts/public-ui-check.py --current --output-dir docs/phase21/screenshots`. Runner memeriksa overflow 320px, isolasi admin, Manrope, dark mode, pencarian dan hasil kosong beranda, reset filter katalog, filter desktop yang terbuka, fallback poster, status tiket, dan tampilan hasil pada 360px. Build produksi menyalin `_redirects`.
