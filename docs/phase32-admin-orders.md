# Phase 32 — Pesanan, Refund, dan Penanganan Masalah

## Status

- [x] Pencarian pesanan, status, nominal, dan tindakan relevan tampil pada ringkasan daftar.
- [x] Desktop memakai daftar dan detail berdampingan; mobile memakai kartu, mempertahankan filter, dan membuka detail terpilih.
- [x] Status pesanan, pembayaran, provider, refund, kasus, dan email memakai label operasional serta fallback aman.
- [x] Riwayat audit menampilkan ringkasan; data payload teknis berada dalam panel tambahan.
- [x] Pengecekan pembayaran, pencatatan catatan, penutupan kasus, refund, pencatatan transfer manual, dan pengiriman ulang email dibedakan.
- [x] Konfirmasi dan pemulihan hasil mutasi yang belum pasti dipertahankan tanpa mengulang POST otomatis.
- [x] URL/query, field filter dan kebijakan transfer/refund, endpoint, serta kontrak API dipertahankan.

## Batas kontrak

Halaman memakai endpoint dan izin `canRecheck`, `canResolve`, serta `canRetry` dari API. Endpoint preview refund tidak tersedia; tinjauan konfirmasi memakai nominal dari GET detail pesanan, sedangkan server tetap menghitung dan memvalidasi nominal saat POST. Respons audit hanya memuat payload setelah tindakan, sehingga ringkasan tidak mengklaim nilai sebelum/sesudah yang tidak tersedia.

Jika hasil refund, transfer manual, catatan, pemeriksaan, penutupan, atau retry tidak pasti, halaman melakukan GET detail dan menahan pengulangan sampai hasil dapat dibuktikan. Status yang belum dikenali tetap diberi label aman dan payload teknis tetap bisa dibuka.

## Verifikasi

- `bun test`: 111 lulus.
- `bun run check`: 0 error, 0 warning.
- `bun run build`: berhasil.
- Runner UI memeriksa pesanan dan penanganan masalah pada desktop 1440×900, mobile 390×844, dan lebar 320px; fixture mencakup normal/detail, loading, kosong, error, dan role ditolak.
- Manifest melaporkan nol runtime error, redirect tak terduga, permintaan API tanpa fixture, overflow viewport, atau target interaktif di bawah 44px.
- Screenshot: [pesanan](phase32/screenshots/orders/manifest.json), [penanganan masalah](phase32/screenshots/issues/manifest.json).

## Audit taste

Mode Preserve: logo, aksen lime, keluarga font, URL, navigasi, field, dan teks kebijakan dipertahankan. Hero, promosi, CTA pemasaran, React, dan Tailwind N/A karena halaman admin menjalankan tugas operasional dan memakai Svelte 5 dengan CSS native.
