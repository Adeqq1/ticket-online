# Phase 27 — Berbagi Acara dan Menemukan Lokasi

- [x] Tombol berbagi memakai Web Share API, dengan aksi salin tautan jika API tidak tersedia atau berbagi gagal.
- [x] Tautan publik hanya memuat origin dan path konser; query keranjang, fragment, dan token tidak ikut dibagikan.
- [x] Tautan Google Maps memakai venue, alamat, dan kota dari API; kota yang sudah tercantum di alamat tidak digandakan.
- [x] Berbagi dan lokasi tampil sebagai aksi sekunder di bawah ringkasan; CTA pembelian dan pilihan tiket tetap terjaga.
- [x] Pembatalan berbagi tidak menampilkan error. Clipboard gagal atau tidak tersedia menampilkan URL siap salin manual.
- [x] Nama dan lokasi panjang membungkus pada mobile; lokasi kosong menampilkan status tanpa tautan peta.

## Verifikasi

- `bun test`: 107 lulus.
- `bun run check`: 0 error dan 0 warning.
- `bun run build`: berhasil; `dist/_redirects` sama dengan `public/_redirects`.
- Runner browser: 69 tangkapan. Fixture memverifikasi Web Share berhasil/dibatalkan/gagal, salin berhasil/gagal/tidak tersedia, URL peta aman, pilihan tiket tetap tampil, lokasi kosong, dan teks panjang pada 320px.
