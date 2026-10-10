# Phase 29 — Baseline dan Aturan Operasional Admin/Petugas

## Ringkasan

Pembacaan desain: aplikasi operasional admin/petugas untuk administrator dan petugas gate, dengan bahasa visual Preserve yang mempertahankan logo, lime, dan keluarga font. Hierarki, ukuran, kelompok kerja, dan umpan balik menjadi fokus. Gunakan Svelte 5 dan CSS native sesuai proyek.

Mode tampilan baseline adalah dark, sama dengan permukaan tetap `.scan-shell`. Bukan landing page atau pemasaran, sehingga hero, CTA pemasaran, SEO, serta rekomendasi paket React/Tailwind diberi N/A. Motion dibatasi pada umpan balik keadaan scanner; hormati `prefers-reduced-motion` yang sudah ada.

## Preservation dan desain saat ini

- Rute: `/admin/login`, `/admin/events`, `/admin/staff`, `/admin/orders`, `/admin/issues`, `/admin/operations`, `/admin/reports`, `/admin/reports/attendance`, `/admin/reports/conversion`, `/admin/check-ins`, dan `/admin/scan`.
- Query: `/admin/orders?orderId=<32 karakter hex>` membuka detail order. Filter halaman lain disimpan pada state halaman dan tidak mengubah URL.
- Navigasi saat ini memakai label Konser, Pesanan, Masalah, Operasional, Penjualan dan refund, Kehadiran, Konversi, Kelola petugas, Riwayat check-in. Visibilitas berbeda menurut layar; scanner petugas bukan bagian navigasi admin yang sama.
- Anchor lewati-konten dipertahankan per layar; ID utama mencakup `event-admin-content`, `staff-admin-content`, `orders-content`, `issues-content`, `operations-content`, `reports-content`, `attendance-report-content`, `conversion-content`, `checkin-history-content`, dan `scan-content`. Login memakai anchor pada status sesi bila diperlukan.
- Merek memakai wordmark Tiket Online dan tanda `TO`; lime admin `#b9f36d`; stack admin Arial/Helvetica, Courier New untuk metadata. Skala radius saat ini dominan siku. Scanner memakai permukaan gelap dan lime.
- Nama field dan urutannya mengikuti form saat ini. Manifest menyimpan label, `name`, `id`, tipe, anchor, dan tautan per rute. Poster memakai field `URL poster`; tidak ada field upload. Halaman admin tidak memiliki consent atau cookie copy (N/A). Copy operasional yang dijaga mencakup “Nominal dihitung server. Tiket akan ditahan sampai Midtrans mengonfirmasi hasil.”, “Transfer {nominal} sudah berhasil dan bukti sudah diperiksa.”, aturan refund saat jadwal berubah, dan konfirmasi “Nonaktifkan {nama}? Semua sesi petugas ini akan dicabut.”
- Standar redesign fase berikutnya: teks isi dan form ≥16px; informasi sekunder ≥14px; target interaktif ≥44×44px. Audit mendapati banyak metadata, pesan bantu, tombol teks, dan beberapa nilai tabel di bawah standar itu.
- Pemeriksaan akses merekam pengalihan seluruh sembilan rute admin ke `/admin/scan` saat dibuka oleh akun STAFF. Identitas logo, label, dan pesan aktual juga tercatat di screenshot.

## Tugas layar dan matriks tindakan/API

| Layar dan tugas utama | Tindakan → API yang sudah tersedia | Sukses → bila gagal → langkah berikutnya |
| --- | --- | --- |
| Login: masuk sebagai admin/petugas | Kirim email/password → `POST /api/v1/staff/login`; validasi sesi → `GET /api/v1/staff/me` | Sesi valid menuju halaman sesuai peran; kredensial salah atau sesi tak tervalidasi menampilkan pesan dan retry. |
| Konser: menyiapkan event dan kategori | Baca `/api/v1/admin/events`; CRUD konser, zona, tier; pratinjau/commit perubahan di `/api/v1/admin/events/{eventID}/changes*` | Konfirmasi simpan/pratinjau dan daftar terbarui; konflik jadwal, validasi, atau jaringan ditampilkan tanpa menganggap simpan berhasil. Periksa kembali data. |
| Petugas: membuat akun dan mengatur akses | `GET/POST /api/v1/admin/staff`; `PATCH /{staffID}`; `PUT /{staffID}/assignments` dan `/password`; opsi event dari `/api/v1/admin/events` | Profil/penugasan terbarui dan sesi dicabut bila akun dinonaktifkan/password diganti; gagal ditampilkan, lalu muat ulang sebelum mencoba lagi. |
| Pesanan: mencari order, memeriksa tiket dan refund | `GET /api/v1/admin/orders`, `GET /{orderID}`, `POST /{orderID}/refund`, `/refund/manual` | Status order, pembayaran, tiket, dan refund tampil; error/hasil tak pasti ditahan untuk pemeriksaan ulang. Refund penuh otomatis/manual memakai kontrak yang ada. |
| Masalah: rekonsiliasi pembayaran dan email | `GET /api/v1/admin/payment-cases`, `/email-jobs`, detail masing-masing; `POST` recheck/note/resolve/retry | Detail dan riwayat audit diperbarui; kegagalan mempertahankan kasus untuk pemeriksaan/admin. Retry email membuat job sesuai aturan backend. |
| Operasional: membaca layanan dan worker | `GET /api/v1/admin/operations` | Snapshot/alert tampil; 503 berarti data tidak tersedia dan tombol muat ulang memberi langkah berikutnya. |
| Laporan: membaca penjualan/refund, kehadiran, konversi | `GET /api/v1/admin/reports/sales`, `/attendance`, `/conversion`; ekspor `.csv` sales/attendance | Filter, snapshot, tabel, dan CSV mengikuti data server; validasi filter atau kegagalan load/export tampil dengan retry. |
| Riwayat: audit hasil check-in | `GET /api/v1/admin/check-ins` dengan filter/cursor | Halaman hasil final scan; kosong berarti belum ada catatan yang cocok. Error dapat dicoba lagi. |
| Scanner: memutuskan tiket boleh masuk | `GET /api/v1/staff/me`, `GET /api/v1/events`, `POST /api/v1/staff/check-ins`, `GET /api/v1/staff/ticket-status` | Hasil sukses/ditolak menyebut keputusan server; jika hasil tak diketahui, tahan gate, periksa status, minta admin bila belum pasti. Jangan kirim ulang otomatis. |

### Batas kemampuan

API saat ini mendukung pengelolaan event, zona/tier, staf dan penugasan, pencarian order/refund, antrean masalah/email, status operasional, laporan/CSV, riwayat check-in, scanner, dan sesi. Tidak ada dasar untuk menambah kontrol voucher, biaya admin, upload poster, secret, atau konfigurasi payment environment dalam redesign. Jika diperlukan, catat sebagai pekerjaan backend terpisah. Poster event saat ini berupa URL/gambar pada input API, bukan alur upload.

## Hasil baseline dan checklist

Runner `frontend/scripts/admin-ui-check.py` mencegat semua request API memakai fixture lokal; request yang tidak dipetakan menggagalkan capture. Tidak ada data pembeli nyata atau backend staging. Manifest berisi commit, viewport, peran, skenario, URL akhir, detail field/nav/anchor, ukuran teks, target sentuh, fokus, overflow, request API, error runtime, dan daftar screenshot. Baseline mencakup 106 keadaan pada 11 rute dan 185 gambar, termasuk potongan halaman dan checkpoint 320px. Cakupan meliputi desktop 1440×900, mobile 390×844, normal, loading, kosong, error, dan role-denied. Login memakai submit fixture untuk menangkap loading/error; form kosong adalah tampilan awal.

Temuan: teks terukur mulai dari 10px; runner mencatat 263 observasi target interaktif di bawah 44px pada 22 capture normal. Tidak ada overflow dokumen pada desktop. Overflow horizontal muncul di scanner pada 390px, lalu laporan penjualan, laporan kehadiran, dan scanner pada 320px. Tab pertama mencapai skip-link di 20 layar; pada scanner setelah scan sukses, fokus berpindah ke tindakan “Scan berikutnya”.

Laporan kehadiran gagal saat fixture berisi jam check-in: browser mencatat `Invalid option` dari formatter `hour()` karena menggabungkan `dateStyle` dengan opsi `hour`/`minute`. Error terekam pada capture normal desktop dan mobile di manifest. Layar ini belum memiliki capture data normal yang berhasil; perbaiki formatter sebelum memakai laporan tersebut sebagai baseline visual hasil.

Dark mode adalah satu-satunya permukaan admin saat ini (`.scan-shell` menetapkan palet gelap), sehingga tema terang N/A sebagai tema yang tidak tersedia. Reduced motion memakai aturan global serta aturan khusus scanner/auth. Print N/A untuk halaman operasi admin; aturan print di proyek hanya berlaku untuk halaman publik/tiket.

- [x] Rekam seluruh rute/keadaan desktop dan mobile; manifest tidak menunjukkan redirect tak terduga.
- [x] Selesaikan inventaris URL/query, label, field/urutan, anchor, logo, dan teks kebijakan/konfirmasi.
- [x] Petakan tugas utama dan kriteria selesai per layar.
- [x] Lengkapi matriks tindakan → endpoint → sukses → gagal → langkah berikutnya berdasarkan UI, API, dan OpenAPI.
- [x] Catat baseline ukuran teks, kontrol sentuh, overflow 320px, fokus, live region, reduced motion, tema, dan print.
- [x] Tandai kebutuhan API tersedia vs. pekerjaan backend tambahan; tidak ada kontrol simpan tanpa endpoint.

Checkpoint Phase 29: setiap layar memiliki tugas dan definisi selesai; URL/label/field/merek/kebijakan preservation tercatat; tidak ada kontrol baru yang mengaku menyimpan data tanpa endpoint.

## Verifikasi

Jalankan dari `frontend/` dengan Vite tersedia: `python3 scripts/admin-ui-check.py --base-url http://127.0.0.1:5174`. Runner membutuhkan Playwright dan Chrome. Artefak disimpan di `docs/phase29/screenshots/`.
