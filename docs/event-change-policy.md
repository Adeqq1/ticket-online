# Pembatalan dan perubahan jadwal

Aturan Phase 11 yang disepakati:

| Keputusan | Penjualan dan check-in | Tiket | Refund |
| --- | --- | --- | --- |
| Pembatalan | Ditutup permanen | Tidak berlaku; snapshot dan check-in dipertahankan | Semua order lunas, termasuk sudah check-in, mendapat total order penuh termasuk biaya admin |
| Penundaan tanpa tanggal | Dihentikan | QR dipertahankan, belum aktif untuk masuk | Pembeli lama boleh meminta refund penuh; belum ada tenggat |
| Jadwal pengganti | Penjualan dibuka setelah reservasi/pembayaran lama ditangani; check-in hanya hari konser terbaru dalam WIB | QR sama; tiket yang sudah dipakai tetap terpakai | Pembeli sebelum keputusan boleh meminta refund penuh sampai tenggat admin |

Tenggat refund pada jadwal pengganti wajib masih berlaku, sebelum waktu konser baru, dan tidak boleh memperpendek tenggat yang sudah diberikan. Batas minimum berasal dari seluruh riwayat keputusan, termasuk sebelum penundaan tanpa tanggal; selama penundaan hak refund aktif tetap tanpa tenggat. Perubahan berulang disimpan sebagai keputusan baru. Pembatalan final tidak dapat dibuka kembali. Lokasi tetap mengikuti penguncian yang sudah ada.

Reservasi aktif dibatalkan dan stok dikembalikan sekali. Order belum lunas dihentikan. Worker memakai status/cancel Midtrans yang sudah tersedia; saat provider belum dapat memastikan hasil, stok tetap ditahan dan penjualan pengganti belum dibuka. Settlement terlambat tidak menerbitkan tiket baru dan mendapat refund penuh otomatis atau manual.

Pekerjaan perubahan acara dan pengajuan refund yang gagal dijadwalkan ulang setelah 30 detik. Worker memilih pekerjaan yang sudah jatuh tempo berdasarkan jadwal lalu ID, sehingga pekerjaan yang belum dicoba tetap mendapat giliran. Gangguan tetap tampil pada progres keputusan sampai percobaan berikutnya berhasil.

Refund otomatis hanya memakai allowlist merchant yang sudah diverifikasi: QRIS sampai tujuh hari, GoPay sampai 45 hari. Kanal lain, transaksi di luar jendela, dan penolakan pasti masuk `MANUAL_REQUIRED`. Hasil `UNKNOWN` wajib direkonsiliasi sebelum refund manual. Admin mencatat referensi transfer, waktu, dan catatan bukti setelah transfer penuh berhasil; aplikasi tidak melakukan transfer bank manual. Nominal selalu berasal dari database. Permintaan atau konfirmasi berulang tidak menggandakan refund maupun stok.

Masa akses order dipertahankan selama penundaan tanpa tanggal, pembayaran lama belum pasti, atau refund belum selesai. Jadwal baru mempertahankan batas lama dan minimal sampai tengah malam WIB setelah hari konser terbaru. Pembatalan menyediakan minimal 90 hari sejak keputusan; refund berhasil menyediakan minimal 90 hari sejak konfirmasi final. Order yang sudah refund tidak lagi menunggu jadwal acara. Token tetap privat, tautan pemulihan tetap singkat dan sekali pakai. `accessExpiresAt: null` berarti belum ada tanggal kedaluwarsa final.

Admin menggunakan panel **Pembatalan dan perubahan jadwal** pada `/admin/events`: isi keputusan dan pengumuman, periksa dampak, lalu konfirmasi. Dampak yang berubah mengharuskan pratinjau baru. Setelah keputusan, muat ulang progres untuk melihat pekerjaan yang belum selesai dan gangguan. Refund manual ditangani pada detail order `/admin/orders`, filter **Refund diproses**. Email gagal dapat dikirim ulang dari `/admin/issues`.

Informasi terkini ditampilkan pada acara, order, tiket, dan scanner. Snapshot penerbitan serta check-in tidak ditulis ulang. Cetakan lama dapat memuat jadwal lama; scanner selalu memeriksa keputusan backend terbaru. Email perubahan menunjuk ke halaman order yang memakai informasi terbaru.

## Verifikasi dan peluncuran

- [x] Kebijakan, pratinjau ADMIN, konfirmasi, idempotensi, serta audit keputusan tersedia.
- [x] Integrasi MySQL memverifikasi pembatalan, penundaan, jadwal pengganti, settlement/check-in bersamaan, penolakan pratinjau kedaluwarsa, refund penuh, pemulihan akses, dan pelepasan stok sekali.
- [x] Provider tiruan memverifikasi antrean otomatis, hasil UNKNOWN yang menghalangi refund manual, penolakan pasti, jendela provider kedaluwarsa, dan retry saat provider belum tersedia.
- [x] SMTP lokal memverifikasi pemberitahuan keputusan berulang memakai jadwal terbaru dan tautan privat order yang berfungsi.
- [x] Migrasi 026 dapat dilanjutkan setelah interupsi pada seluruh sembilan titik statement; OpenAPI dapat dibaca tanpa duplikasi dan seluruh referensinya valid.
- [x] Migrasi 027 dapat dilanjutkan setelah interupsi pada ketiga titik statement.
- [x] Verifikasi lokal setelah perbaikan review PR #25: seluruh tes backend dengan MySQL dan akun aplikasi terbatas (`go test -p 1 ./...`), `go vet`, build API/migration, 94 tes frontend, pemeriksaan Svelte/TypeScript tanpa peringatan, build Vite, dan SPA fallback lulus.
- [ ] Verifikasi aplikasi di staging dan kemampuan refund akun merchant pada Midtrans sandbox sebelum rollout.

- Uji lokal memakai database MySQL sementara yang terpisah dari staging/production.
- Pengujian provider memakai respons uji; kemampuan refund akun merchant tetap perlu dibuktikan pada Midtrans sandbox.
- Migrasi 027 menambahkan jadwal retry work/refund dan penanda telemetry pertama. Jalankan runner migration sebelum API versi baru menerima traffic; migrasi 025/026 historis tidak diubah.
- Sebelum rollout, jalankan migrasi sampai 027 di staging, lalu uji pembatalan, penundaan, jadwal baru, SMTP, pembayaran tertunda, dan refund pada kanal merchant yang tersedia.
- Allowlist production tetap kosong sampai verifikasi merchant selesai. Jangan menandai verifikasi sandbox sebagai selesai berdasarkan tes provider lokal.
