# Phase 23: Checkout dan Halaman Pesanan

## Progress

- [x] Pertahankan tiga langkah checkout, nama/urutan field, validasi, idempotensi, dan pemulihan checkout.
- [x] Pertahankan form satu kolom, label terlihat, keyboard sesuai field, dan error dekat input.
- [x] Ringkas indikator langkah tanpa menghapus fokus ke judul tahap.
- [x] Tampilkan total dan countdown mobile pada ringkasan yang mengikuti pengguna saat menggulir.
- [x] Tampilkan rincian biaya terbuka sebelum persetujuan dan konfirmasi pembayaran.
- [x] Tata order berdasarkan status dan tindakan, tiket, hak/tenggat/progres refund, lalu rincian item.
- [x] Pertahankan status order sebagai sumber kebenaran; countdown lokal tidak mengubah status pembayaran.
- [x] Verifikasi pemulihan idempotent dan status pembayaran/refund yang tertunda, gagal, dan belum pasti.

## Perilaku UI

Ringkasan checkout mobile menampilkan estimasi atau total server serta batas reservasi/pembayaran. Rincian biaya dapat dilipat selama pengisian data dan dibuka pada langkah konfirmasi; desktop mempertahankan ringkasan di sisi form. Fokus judul setiap langkah memiliki ruang di bawah ringkasan sticky.

Halaman order menampilkan status order terpisah dari status pembayaran. Untuk order pending, countdown memakai `expiresAt` dari API dan meminta pembeli memeriksa status setelah waktunya lewat. Halaman tidak menganggap order selesai hanya karena pembayaran terdeteksi.

Tiket berada sebelum informasi refund. Hak refund, tenggat, dan progres pengembalian tetap terpisah dan menjelaskan bahwa permintaan belum berarti dana telah kembali. Status yang belum pasti meminta pembeli memeriksa status sebelum mengulang tindakan.

Tidak ada perubahan endpoint, tipe API, URL/query, penyimpanan, idempotency key, atau aturan validasi.

## Screenshot dan Pemeriksaan

Capture Phase 23 tersimpan di `docs/phase23/screenshots/` agar tidak menimpa baseline dan hasil fase sebelumnya. Runner mencakup 69 capture desktop/mobile, viewport sempit, ringkasan biaya, status order pending/expired/cancelled, error tiket, tenggat refund aktif/kedaluwarsa/tanpa batas, progres refund, serta interaksi retry checkout dan pengajuan refund.

Verifikasi: `bun test`, `bun run check`, `bun run build`, dan `python3 scripts/public-ui-check.py --current --output-dir docs/phase23/screenshots`. Pemeriksaan Svelte tanpa error/peringatan; tes unit dan runner browser lulus. Runner memakai API tiruan sesuai kontrak OpenAPI, bukan data pembeli atau backend langsung.
