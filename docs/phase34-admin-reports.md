# Phase 34 — Laporan dan Kondisi Operasional

## Hasil

- [x] Susun laporan dengan urutan filter → periode aktif → ringkasan → rincian → ekspor.
- [x] Jelaskan istilah metrik, waktu pembaruan, data kosong, serta data yang belum tersedia.
- [x] Pada mobile, prioritaskan metrik utama dan tampilkan rincian saat dibuka.
- [x] Batasi geser horizontal pada wadah tabel yang dapat difokuskan dan diberi label aksesibel.
- [x] Dahulukan kondisi operasional yang perlu tindakan beserta tautan penanganan; ringkasan worker menjadi detail tambahan.
- [x] Pertahankan hitungan dan ekspor laporan dari backend. Tidak menambah endpoint, perhitungan klien, atau kontrol konfigurasi.

## Perubahan

Penjualan dan kehadiran menampilkan filter serta snapshot yang sedang aktif, metrik utama, rincian yang dapat dibuka, lalu ekspor CSV di akhir halaman. Jika filter berubah, laporan dan ekspor tetap menunjuk snapshot terakhir sampai filter diterapkan. Saat refresh gagal, snapshot terakhir tetap tampil dengan status yang jelas.

Laporan konversi menjelaskan jendela observasi dari nilai API, keadaan tanpa data, atribusi yang tidak tersedia, dan hambatan yang belum dikenali tanpa menampilkan kode teknis sebagai label utama. Tidak ada kontrol ekspor karena API tidak menyediakan ekspor konversi.

Halaman operasional menaruh peringatan dan tautan ke penanganan yang sudah tersedia sebelum metrik. Worker dan angka teknis dapat dibuka saat pemeriksaan lebih lanjut. Refresh gagal mempertahankan snapshot terakhir.

## Audit tampilan

Audit menggunakan fixture API lokal dengan tema gelap, zona waktu Asia/Jakarta, dan viewport desktop 1440×900 serta mobile 390×844. Runner juga menangkap lebar 320px dan akses ditolak untuk halaman admin.

- [Penjualan: manifest dan tangkapan desktop/mobile](phase34/screenshots/sales/manifest.json) — termasuk loading, kosong, error, akses ditolak, dan verifikasi ekspor CSV.
- [Kehadiran: manifest dan tangkapan desktop/mobile](phase34/screenshots/attendance/manifest.json) — termasuk loading, kosong, error, akses ditolak, dan verifikasi ekspor CSV.
- [Konversi: manifest dan tangkapan desktop/mobile](phase34/screenshots/conversion/manifest.json) — termasuk loading, kosong, error, nilai atribusi tidak tersedia, dan akses ditolak.
- [Operasional: manifest dan tangkapan desktop/mobile](phase34/screenshots/operations/manifest.json) — termasuk loading, kosong, error, snapshot lama setelah refresh gagal, dan akses ditolak.

Tidak ada error halaman atau permintaan API fixture yang tidak dipetakan. Semua layar tidak meluber pada 390px dan 320px; teks sekunder terendah 14px, teks isi 16px, dan target tindakan minimal 44px. Tabel lebar tetap berada di wadah gulir masing-masing.

## Audit taste

- **Arah visual:** preserve; logo, aksen lime, font, label, URL, filter, dan kontrak API tetap dipertahankan.
- **Hierarki:** filter dan hasil utama muncul sebelum rincian; data teknis menjadi pilihan lanjutan.
- **Responsif:** desktop membuka rincian secara default; mobile menutup rincian agar ringkasan utama mendapat ruang awal.
- **Gerakan:** N/A, halaman laporan tidak memerlukan animasi untuk menyelesaikan tugas.
- **Pola landing page dan konversi pemasaran:** N/A, ini alat operasional admin, bukan halaman akuisisi. Tidak ada hero pemasaran, testimoni, atau CTA penjualan yang relevan.
- **Kontrol backend tambahan:** N/A; tidak ada kebutuhan yang dapat disimpan tanpa endpoint.
