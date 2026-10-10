# Phase 26 — Pencarian Konser yang Lebih Berguna

- [x] Pilihan kota dan genre mengikuti nilai unik dari seluruh katalog API.
- [x] Harga awal tidak dibatasi; input rupiah aktif hanya setelah pengguna mencentang batas harga.
- [x] Urutan tanggal terdekat dan harga terendah memakai data katalog yang sudah dimuat.
- [x] Pencarian, filter, urutan, panel, dan posisi gulir dipulihkan setelah kembali dari detail melalui penyimpanan sesi khusus katalog.
- [x] Filter aktif tampil sebagai chip yang dapat dihapus satu per satu.
- [x] Pengurutan dan filter tambahan berada di panel ringkas pada mobile; pencarian dan hasil tetap di luar panel.

## Verifikasi

- `bun test`: 105 lulus.
- `bun run check`: 0 error dan 0 warning.
- `bun run build`: berhasil; `dist/_redirects` sama dengan `public/_redirects`.
- `python3 scripts/public-ui-check.py --current`: 69 screenshot; fixture memverifikasi semua hasil tanpa filter, genre/kota API, harga Rp2,5 juta, kedua urutan, chip, dan kembali dari detail dengan posisi gulir.
