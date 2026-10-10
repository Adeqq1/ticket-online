# Phase 30 — Navigasi dan Kerangka Halaman

## Hasil

Admin memiliki satu kerangka bersama untuk sembilan halaman: Konser, Kelola petugas, Pesanan, Masalah, Operasional, Penjualan dan refund, Kehadiran, Konversi, dan Riwayat check-in. Sidebar desktop mengelompokkan tujuan berdasarkan pengelolaan, operasional, dan laporan. Pada mobile, tombol Menu membuka dialog dengan penanda halaman aktif, fokus awal, perangkap Tab, Escape, dan pengembalian fokus.

Judul, penjelasan, tombol keluar, tindakan utama, dan skip-link sekarang konsisten. Tindakan memakai handler atau form yang sudah ada. Label tujuan `/admin/reports` memakai **Penjualan dan refund**, pengecualian preservation yang disetujui untuk menyamakan label antarhalaman.

Scanner tetap memiliki kerangka petugas sendiri. Header menampilkan nama, sesi, event dan gate dari penugasan, serta logout. Keterangan tujuan login di README cocok dengan implementasi: ADMIN ke `/admin/staff`, STAFF ke `/admin/scan`.

Formatter jam pada laporan kehadiran kini memakai kombinasi opsi Intl yang valid. Header scanner dan detail tiket juga membungkus dengan benar pada lebar kecil.

## Checklist dan checkpoint

- [x] Satukan kerangka dan navigasi di sembilan halaman admin.
- [x] Tampilkan kelompok Pengelolaan, Operasional, dan Laporan pada sidebar desktop.
- [x] Sediakan menu mobile dengan halaman aktif, fokus keyboard, tombol tutup, dan pengembalian fokus.
- [x] Tampilkan judul, penjelasan, serta tindakan utama yang memakai API/form yang tersedia.
- [x] Selaraskan dokumentasi tujuan login tanpa mengubah rute atau alur autentikasi.
- [x] Pisahkan kerangka scanner dan tampilkan identitas, status sesi, event/gate, serta logout petugas.

Checkpoint Phase 30: seluruh halaman admin dapat ditemukan dari navigasi bersama; menu mobile dapat digunakan dengan keyboard; scanner hanya menampilkan alur petugas.

## Verifikasi

Capture fixture lokal tersedia di `docs/phase30/screenshots/manifest.json` dan mencakup 106 keadaan pada 11 rute, 178 gambar, desktop 1440×900, mobile 390×844, serta checkpoint 320px. Skenario memuat normal, loading, kosong, error, dan akses ditolak.

- Tidak ada redirect tak terduga, API fixture yang tidak dipetakan, error runtime, atau overflow horizontal sampai 320px.
- Sembilan halaman admin lolos pemeriksaan menu mobile: tombol membuka dialog, fokus masuk dan tetap di dialog, Escape menutup, fokus kembali, halaman aktif tunggal, serta dialog menutup saat viewport menjadi desktop.
- Delapan belas pemeriksaan desktop/mobile memastikan setiap tindakan utama ada dan tombol submit terhubung ke form tujuan yang benar.
- `bun test`: 107 lulus. `bun run check`: 0 error dan warning. `bun run build`: lulus.

Jalankan capture dari `frontend/` dengan Vite aktif: `python3 scripts/admin-ui-check.py --base-url http://127.0.0.1:5174 --output-dir docs/phase30/screenshots --phase 30`. Tanpa `--output-dir`, runner tetap menulis ke baseline Phase 29.
