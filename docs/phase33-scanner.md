# Phase 33 — Scanner untuk Pekerjaan Gate

## Status

- [x] Judul dekoratif diringkas; penugasan event/gate, kamera, dan hasil memakai ruang utama.
- [x] Kamera dan input manual tetap tersedia sebagai dua cara memasukkan tiket.
- [x] Hasil check-in mendahulukan status, identitas tiket, gate/penugasan, dan tindakan berikutnya.
- [x] Teks, ikon, dan warna membedakan diterima, ditolak, dan hasil belum diketahui.
- [x] Penguncian scan, pemeriksaan status, konfirmasi penanganan, serta pemeriksaan sesi sebelum POST dipertahankan.
- [x] Tombol lanjut tunggal berukuran sentuh minimum 44px; fokus diarahkan ke input manual atau kamera sesuai cara scan.
- [x] Kamera berhenti saat sesi tidak valid, scan terkunci, halaman ditinggalkan, atau komponen dilepas.

## Batas kontrak

Endpoint dan payload tidak berubah. POST check-in tetap hanya dikirim sekali per percobaan. Hasil status yang dibaca lewat GET tidak mengubah respons POST yang belum pasti menjadi sukses; petugas tetap melihat status amber dan konfirmasi yang diperlukan sebelum scan berikutnya. Status pesanan ditampilkan dengan label operasional.

URL `/admin/scan`, logo, aksen lime, font, label, field, dan anchor dipertahankan. Tidak ada pengecualian preservation.

## Verifikasi

- `bun test`: 111 lulus.
- `bun run check`: 0 error, 0 warning.
- `bun run build`: berhasil.
- Runner UI memeriksa desktop 1440×900, mobile 390×844, dan lebar 320px dengan hasil normal, loading, tanpa penugasan, error, tiket ditolak, status check-in terkonfirmasi, kamera, serta logout.
- Simulasi memastikan track kamera berhenti ketika sesi berakhir. Manifest melaporkan nol runtime error, API tanpa fixture, dan redirect tak terduga; audit normal tidak menemukan overflow atau target interaktif di bawah 44px.
- Screenshot dan manifest: [audit scanner](phase33/screenshots/scanner/manifest.json).

## Audit taste

Mode Preserve. N/A: hero pemasaran, promosi, CTA landing page, React, dan Tailwind; scanner adalah alat operasional berbasis Svelte 5 dan CSS native.
