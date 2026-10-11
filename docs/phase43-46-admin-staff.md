# Phase 43–46: hierarki admin dan alur petugas

Verifikasi lokal: 11 Oktober 2026. Backend, kontrak API, URL, kewenangan, logo, font, dan lime merek dipertahankan.

| Fase | Hasil |
| --- | --- |
| 43 | Token fungsi dibatasi pada `.admin-layout`: sidebar, konten, kartu, input, teks, batas, dan fokus. Label/form 16px; teks sekunder dan badge minimal 14px. |
| 44 | Peran Admin/Staff terpisah dari badge Aktif/Nonaktif. Aktif memakai hijau lembut, Nonaktif netral. Nama, email, dan penugasan membungkus tanpa pemotongan. |
| 45 | Daftar menjadi tampilan awal. Tombol atas membuka panel; hanya form di dalam panel mengirim create. Tambah dan kelola saling menggantikan. Draft dilindungi saat batal, berpindah akun, logout, dan meninggalkan halaman. |
| 46 | Pemeriksaan alur, kontras ter-render, responsivitas, dan regresi lulus. |

Create sukses membersihkan password, menutup panel, memperbarui daftar, mengumumkan hasil, dan memfokuskan akun baru atau pesan hasil. Refresh gagal setelah create tetap menampilkan keberhasilan dengan tombol Muat ulang pada daftar. Email duplikat mempertahankan form dan input. Respons terputus mempertahankan input dan meminta pemeriksaan daftar sebelum submit dapat diulang; draft dapat dibaca, sementara pengeditan ditahan sampai pemeriksaan selesai. Request penyimpanan menahan submit ganda dan perpindahan panel.

## Pemeriksaan

Jalankan Vite pada port 5173, lalu dari `frontend/`:

```bash
bun test
bun run check
bun run build
for width in 320 390 768 1440; do
  python3 scripts/staff-review-check.py http://127.0.0.1:5173 "$width"
done
python3 scripts/admin-ui-check.py --phase 46 --phase35 --output-dir /tmp/phase43-46/admin-regression
```

- 115 unit tests lulus; Svelte/TypeScript tanpa error atau warning; production build lulus, termasuk salinan `_redirects`.
- Alur petugas lulus pada 320, 390, 768, dan 1440px: tambah, batal, fokus, navigasi keyboard, menu mobile, draft tambah/edit, logout, dan dialog native sebelum meninggalkan halaman.
- Create sukses, submit bersamaan, email duplikat, katalog loading/gagal/retry, sesi berakhir, koneksi gagal, JSON terputus, serta refresh gagal sesudah create lulus dengan fixture API.
- Pembesaran teks 200% dan simulasi keyboard melalui viewport setinggi 420px tidak menimbulkan overflow horizontal. Ini pengujian browser desktop; keyboard OS pada perangkat fisik belum diuji.
- Kontras minimum pada keadaan petugas yang diaudit: teks **6,81:1**, batas kontrol/badge **3,25:1**, fokus **10,52:1**. Audit menggunakan warna komputasi browser, mencakup status, hover, disabled, input, dan fokus keyboard. Ukuran tanda logo dipertahankan.
- Regresi admin/scanner: **150 skenario**, 312 screenshot; tanpa error JavaScript, API fixture tak tertangani, atau redirect tak terduga.
- Perbandingan sebelum/sesudah untuk petugas, konser, pesanan, halaman utama publik, dan scanner pada empat lebar: perubahan visual terbatas pada admin. Delapan perbandingan publik/scanner **identik per piksel**.

Artefak lokal berada di `/tmp/phase43-46/`: `workflow/checks-*.json` dan screenshot alur, `admin-regression/manifest.json`, screenshot `before/` dan `after/`, serta `comparison.json`. Request browser memakai fixture dan tidak menulis data backend.
