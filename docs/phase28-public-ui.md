# Phase 28 — Performa Frontend Publik

- [x] Mengukur baseline dan hasil akhir pada beranda, katalog, detail konser, dan checkout.
- [x] Memuat modul halaman sesuai rute; AdminArea dan scanner tidak masuk ke JavaScript awal halaman publik.
- [x] Mengganti Manrope TTF variabel dengan bobot lokal WOFF2 400, 700, dan 800; lisensi OFL tetap tersedia.
- [x] Memprioritaskan poster pertama di beranda dan katalog, menunda poster lain, serta mempertahankan dimensi gambar.
- [x] Menghapus preconnect Picsum yang tidak lagi memberi manfaat konsisten.
- [x] Menampilkan status saat modul dimuat dan aksi retry setelah kegagalan chunk; menjaga ruang konten dan menyembunyikan footer selama data masih dimuat.

## Metode dan hasil

Build baseline dibuat dari commit Phase 27 `36e0d93`; baseline dan hasil akhir diuji pada fixture API dan gambar lokal yang sama. Lima pengukuran per rute memakai Chrome pada viewport 390×844, jaringan 1,6 Mbps turun / 750 Kbps naik, latensi 150 ms, CPU 4× lebih lambat, dan cache dingin. JS adalah ukuran transfer total JavaScript sampai konten rute siap; server fixture menyajikan aset tanpa kompresi HTTP. CLS memakai jendela sesi standar. Angka di tabel adalah median.

| Rute | FCP sebelum → sesudah | LCP sebelum → sesudah | Siap sebelum → sesudah | JS sebelum → sesudah |
| --- | ---: | ---: | ---: | ---: |
| Beranda | 2.764 → 1.160 ms (-58,0%) | 4.312 → 1.772 ms (-58,9%) | 4.321 → 2.030 ms (-53,0%) | 343,1 → 81,3 KiB (-76,3%) | 0,048 → 0,072 |
| Katalog | 2.780 → 1.160 ms (-58,3%) | 2.780 → 1.704 ms (-38,7%) | 4.254 → 2.061 ms (-51,5%) | 343,1 → 85,7 KiB (-75,0%) | 0,093 → 0,019 |
| Detail | 2.748 → 1.168 ms (-57,5%) | 4.292 → 2.020 ms (-52,9%) | 4.186 → 2.056 ms (-50,9%) | 343,1 → 94,4 KiB (-72,5%) | 0,908 → 0,017 |
| Checkout | 2.740 → 1.172 ms (-57,3%) | 3.436 → 2.600 ms (-24,3%) | 3.754 → 3.054 ms (-18,6%) | 343,1 → 116,8 KiB (-66,0%) | 0,920 → 0,017 |

Beranda CLS naik tipis 0,024 menjadi 0,072 dan tetap di bawah ambang baik 0,1; katalog, detail, dan checkout membaik. Bundle entry turun dari 351,01 KB (103,95 KB gzip) ke 57,23 KB (21,13 KB gzip). Tiga file font WOFF2 berjumlah 92.296 byte, dibanding TTF 164.700 byte sebelumnya. Pengukuran rinci dan skrip benchmark ada di `frontend/scripts/phase28-performance.py`.

## Verifikasi

- `bun test`: 107 lulus.
- `bun run check`: 0 error dan 0 warning.
- `bun run build`: berhasil; `dist/_redirects` sama dengan `public/_redirects`.
- Runner browser publik: 69 tangkapan lulus. Pemeriksaan mencakup rute publik, retry chunk dan API, aksesibilitas/responsif, serta alur checkout.
- Retry chunk gagal lalu muat ulang berhasil; retry API katalog pulih pada permintaan kedua.
