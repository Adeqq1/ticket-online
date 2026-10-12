# Konsistensi tampilan mobile admin

Tanggal pemeriksaan: 12 Oktober 2026. Referensi interaksi: daftar pesanan. Desktop mempertahankan daftar/tabel lengkap; pola mobile berlaku di bawah 768px.

| Area | Perubahan |
| --- | --- |
| Konser, petugas, kasus pembayaran, email gagal, check-in | Kartu horizontal, maksimal lima per kelompok, Sebelumnya/Berikutnya, fokus dan posisi kartu dipulihkan setelah editor ditutup. |
| Pencarian dan laporan | Filter opsional ditutup pada mobile. Pencarian petugas dan pilihan event wajib pada kehadiran tetap terlihat. |
| Biaya/voucher | Biaya tetap terlihat. Form voucher dibuka melalui Buat/Kelola, dengan konfirmasi pembatalan draft dan fokus kembali ke pemicu. Riwayat dapat dibuka/tutup. |
| Operasional | Peringatan tetap terlihat; rincian worker menjadi pasangan label–nilai pada mobile. |

Kelompok yang belum berisi lima kartu dilengkapi setelah cursor berhasil dimuat; kartu sebelumnya tidak dilewati. Kegagalan cursor mempertahankan kelompok dan data yang tersedia. Teks aksesibel pada kartu di luar layar dibatasi oleh area scroll agar tidak memperlebar viewport perangkat sentuh.

## Tes otomatis

- `bun test`: 127 lulus, 0 gagal; 603 assertion.
- `bun run check`: 0 error dan 0 peringatan.
- `bun run build`: lulus.
- `scripts/admin-ui-check.py --mobile-lists`: lulus pada 320, 390, 768, 1440px; fixture 20 konser/petugas, dua halaman cursor, keyboard, fokus/posisi kembali, kegagalan cursor dan retry, tanpa mutasi dari navigasi daftar.
- `scripts/admin-ui-check.py --workflows`: lulus pada empat lebar tersebut; buat/edit/publikasi konser, draft, detail pesanan, cursor dan hasil yang tidak ganda.
- Pemeriksaan `--only` untuk issues, operations, sales, conversion, attendance, check-ins dengan `--phase35`: lulus. Mencakup kondisi pemuatan/kegagalan, sesi berakhir, 320px, tablet, tinggi layar berkurang dan pembesaran teks.
- `scripts/pricing-ui-check.py`: lulus pada 320, 390, 768, 1440px, termasuk draft/Batal/fokus, teks editor 200%, konflik revisi, kegagalan parsial, hasil simpan belum pasti dan regresi recovery checkout.
- Pemeriksaan petugas: alur buat dan kegagalan pada 320px lulus; `--management-only` pada 320 dan 1440px lulus, termasuk penugasan, nonaktif, reset password, reload fixture dan penguncian hasil belum pasti.

Perintah browser dijalankan dari `frontend/` terhadap `http://localhost:5173`. API browser memakai fixture; penyimpanan/reload fixture bukan bukti integrasi backend nyata. Manifest daftar tersedia di [folder bukti](admin-mobile-consistency/).

## Screenshot

Bukti visual terpisah dari bukti fungsional: [konser 320px](admin-mobile-consistency/concerts-320.png), [petugas 390px](admin-mobile-consistency/staff-390.png), [email 320px](admin-mobile-consistency/emails-320.png), [check-in 390px](admin-mobile-consistency/checkins-390.png), [laporan 390px](admin-mobile-consistency/sales-390.png), [editor voucher 320px dengan teks 200%](admin-mobile-consistency/voucher-text200-320.png).

## Batas verifikasi

Perubahan mobile ini mempertahankan API, URL, izin, aturan bisnis, palet, font dan logo. Halaman publik dan scanner tidak diubah oleh pekerjaan ini. Pengujian backend nyata tidak dijalankan ulang untuk perubahan tampilan ini. Simulasi tinggi layar bukan pengujian keyboard fisik pada perangkat.

UAT manusia fase sebelumnya tetap **belum selesai**: dua peserta admin belum tersedia. Screenshot dan hasil otomatis tidak menutup checkpoint tersebut.
