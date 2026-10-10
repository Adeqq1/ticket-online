# Phase 35 — Verifikasi dan Uji Tugas

## Status

- [ ] Backend staging: belum tersedia untuk pengujian.
- [ ] Uji tugas dengan dua admin dan dua petugas: belum dijadwalkan.
- [ ] Uji perangkat nyata, keyboard virtual, kamera, landscape, dan pembesaran browser: belum dilakukan.
- [x] Pemeriksaan lokal, fixture, desktop/mobile, tablet, 320px, landscape, viewport pendek, serta simulasi teks 200%: selesai.

Checkpoint Phase 35 tetap terbuka sampai alur staging dan sesi pengguna/perangkat nyata dapat dijalankan. Pemeriksaan browser lokal memakai fixture; hasilnya tidak membuktikan kontrak layanan staging atau perilaku kamera perangkat nyata.

## Pemeriksaan lokal

Semua command dijalankan dari `frontend/`.

| Pemeriksaan | Hasil |
| --- | --- |
| `bun test` | 111 lulus, 0 gagal. |
| `bun run check` | 0 error, 0 warning. |
| `bun run build` | Berhasil. |
| `git diff --check` | Berhasil. |
| `python3 scripts/admin-ui-check.py --base-url http://127.0.0.1:5174 --phase 35 --phase35 --output-dir docs/phase35/screenshots` | 140 capture pada 11 rute, 287 file gambar. Nol runtime error, request API fixture yang belum dipetakan, redirect tak terduga, overflow 320px, atau overflow pada viewport tambahan. |
| `python3 scripts/public-ui-check.py --current --phase25 --base-url http://127.0.0.1:5174 --output-dir /tmp/phase35-public-final` | 69 tangkapan; pemeriksaan responsive, kontras token, target sentuh, input, reduced motion, font, dan isolasi admin lulus. |

Vite memakai port 5174 karena port 5173 telah digunakan proses lain. Seluruh API pada audit admin dan publik dicegat dan dijawab fixture lokal. Tidak ada data staging atau transaksi nyata.

## Cakupan audit UI

Runner admin menangkap 11 rute admin/petugas pada desktop 1440×900 dan mobile 390×844, termasuk keadaan normal, loading, kosong, error, serta akses ditolak. Scanner juga menguji tiket ditolak, hasil check-in belum pasti, kamera berhenti ketika keluar/sesi berakhir, dan dua submit bersamaan hanya mengirim satu POST.

Setiap halaman normal turut diperiksa pada tablet 768×1024, landscape 844×390, viewport pendek 390×500 sebagai simulasi ruang keyboard, dan teks 200% yang disimulasikan di browser. Layar mobile juga dicek pada 320×740. Simulasi viewport pendek tidak membuka keyboard OS; pembesaran CSS tidak menggantikan zoom browser atau pengaturan aksesibilitas sistem. Capture normal dan keadaan interaktif scanner tetap memakai viewport desktop/mobile; variasi tambahan menangkap tampilan normal masing-masing rute.

Artefak screenshot dan hasil per rute: [manifest audit Phase 35](phase35/screenshots/manifest.json). Baseline preservation: [inventaris Phase 29](phase29-admin-ui.md) dan tangkapan sebelum redesign pada commit `b6c8349`.

## Audit Preservation dan Brand Fidelity

Tidak ada perubahan URL/query, label navigasi, nama atau urutan field, anchor, logo, teks kebijakan/persetujuan, maupun kontrak API pada Phase 35. Pengecualian label navigasi yang tercatat sebelumnya tetap hanya perubahan Phase 30 untuk konsistensi label **Penjualan dan refund**. Tidak ada kontrol penyimpanan atau endpoint baru. Perbandingan sumber terhadap commit sebelum Phase 35 (`f4dde1e`) tidak menunjukkan perubahan pada halaman publik atau stylesheet publik. Matrix publik 69 tangkapan lulus; isolasi admin memastikan halaman login admin tidak memakai wrapper visual publik.

Logo Tiket Online, aksen lime `#b9f36d`, keluarga Arial/Helvetica admin, permukaan gelap, dan bentuk tegas tetap dipakai. CSS Phase 35 memperbaiki pembungkusan teks refund, judul kasus, tombol panjang, metrik laporan pada layar kecil, dan ringkasan sesi scanner. CSS baru tetap dibatasi ke area admin.

## Audit taste dan Pre-Flight Section 14

Pembacaan desain: redesign preserve untuk alat kerja admin dan petugas gate. Arah mempertahankan merek dan struktur tugas. Dial yang dipakai untuk evaluasi: variance mengikuti baseline (3), motion ringan (1), kepadatan sesuai data operasional (5). Stack tetap Svelte 5 dan CSS native.

| Pemeriksaan | Hasil |
| --- | --- |
| Em-dash/en-dash yang tampil | 0 pada semua capture desktop/mobile dan keadaan interaktif; assertion otomatis menggagalkan capture bila salah satunya terlihat. |
| Tema dan aksen | Dark adalah tema admin yang tersedia; aksen lime dipertahankan. Tema terang N/A karena tidak tersedia pada shell admin. |
| Keterbacaan dan sentuh | Ukuran teks, kontrol, dan fokus terekam pada manifest desktop/mobile; 320px dan teks 200% diperiksa untuk overflow. Teks refund, judul kasus/tombol panjang, metrik laporan, dan ringkasan sesi scanner membungkus tanpa mendorong halaman melebar. |
| Loading, kosong, error, status, reduced motion | N/A untuk sebagian aturan marketing visual; keadaan aplikasi dan reduced motion yang relevan diperiksa oleh fixture/runner. |
| Hero, CTA marketing, testimonial, logo wall, bento, SEO akuisisi, gambar promosi, sticky/horizontal-pan editorial, marquee, animasi scroll | N/A: layar adalah alat operasional, bukan landing page atau halaman akuisisi. |
| React, Tailwind, sistem desain eksternal, font/aset baru | N/A: proyek memakai Svelte 5, CSS native, serta brand/font yang sudah ada. Tidak ada dependensi baru. |
| Perhitungan backend dan staging | Hasil CSV tetap berasal dari fixture API pada audit; verifikasi angka dan tindakan staging menunggu backend staging. |

## Prosedur uji eksternal yang masih menunggu

1. Jalankan alur cari order, lihat status/detail, periksa pembayaran, refund, catat transfer manual, dan tutup kasus pada staging. Ulangi dengan jaringan terputus dan respons yang hasilnya belum pasti.
2. Jalankan login dan sesi kedaluwarsa untuk admin serta petugas. Pastikan akses admin ditolak ke petugas dan tujuan kembali sesuai peran.
3. Pada dua perangkat petugas, scan tiket yang valid, sudah digunakan, salah gate, dan hasil jaringan tak pasti secara bersamaan. Pastikan backend mencegah check-in ganda dan UI tidak menganggap hasil tak pasti sebagai sukses.
4. Ulangi tugas utama bersama dua admin dan dua petugas. Catat keberhasilan, bantuan, waktu, dan kesalahan pemahaman.
5. Ulangi pada perangkat fisik 320px atau lebih lebar, tablet, landscape, keyboard virtual terbuka, kamera, serta zoom teks/browser 200%. Periksa fokus keyboard dan scroll hanya pada wadah tabel yang memang memerlukannya.

Jangan masukkan secret atau token staging ke screenshot, manifest, atau dokumen hasil.
