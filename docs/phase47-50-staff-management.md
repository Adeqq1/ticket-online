# Phase 47–50: pengelolaan petugas

Verifikasi lokal: 11 Oktober 2026. Fase 47–49 diterapkan. Fase 50 memisahkan bukti otomatis, screenshot, dan UAT manusia; checkpoint UAT masih terbuka karena dua pengguna admin tidak tersedia.

## Perubahan

- Kelola selalu terlihat untuk STAFF. ADMIN mendapat penjelasan batas pengelolaan. Peran dan status berlabel eksplisit; badge tidak interaktif.
- Panel menampilkan identitas dan tautan ke Profil dan akses, Penugasan, serta Reset password. Petunjuk menjelaskan urutan pemilihan dan kapan akses berlaku. Pilihan event/gate yang belum ditambahkan menghalangi penyimpanan dan ikut dilindungi sebagai draft.
- Pencarian nama/email mengabaikan kapitalisasi dan spasi luar. Filter status, event, dan tanpa penugasan memakai AND dan mempertahankan urutan backend. Jumlah hasil, reset, dua ringkasan penugasan, dan disclosure penugasan tambahan tersedia. Filter tidak menghapus draft; akun terpilih yang tersaring mendapat keterangan.
- Request berjalan menahan submit ganda. Hasil mutasi edit yang belum pasti dikunci per akun/tindakan selama halaman terbuka, termasuk sesudah panel dibuka kembali. Profil/penugasan dibuka kembali hanya ketika daftar terbaru cocok dengan perubahan yang dikirim. Hasil reset password tidak dapat dibuktikan lewat API daftar; pesan meminta pemeriksaan password oleh petugas. Kunci ini tidak bertahan melewati reload penuh.

Palet/token sebelumnya dipertahankan. Perubahan tampilan dibatasi ke halaman petugas dan editor penugasannya; backend, kontrak API, URL halaman, role, izin ADMIN, dan aturan pencabutan sesi tetap.

## Bukti otomatis

Dari `frontend/`, dengan Vite berjalan di `http://127.0.0.1:5173`:

```bash
bun test
bun run check
bun run build
for width in 320 390 768 1440; do
  python3 scripts/staff-review-check.py http://127.0.0.1:5173 "$width"
done
python3 scripts/admin-ui-check.py --only staff --phase 50 --phase35 --output-dir /tmp/phase47-50/admin-regression
```

- Unit: 118 tes, 515 assertion, tanpa kegagalan. Svelte/TypeScript: 0 error dan 0 warning. Production build lulus.
- Fixture 20 akun mencakup ADMIN, STAFF aktif/nonaktif, tanpa penugasan, tujuh penugasan, dan nama/email panjang. Katalog mencakup event tanpa gate. Fixture dan request mutasi dicatat dalam JSON; password dikeluarkan dari catatan.
- Alur browser menguji create → ubah profil → tambah/hapus penugasan → nonaktifkan → reset password. Reload memverifikasi nama, status, dan penugasan dari fixture yang menyimpan perubahan. Reset password memverifikasi request, konfirmasi, pesan hasil, dan pembersihan input; login dengan password baru tidak diuji dengan backend nyata.
- Kasus kegagalan mencakup email duplikat, katalog gagal/retry, sesi berakhir, jaringan gagal, respons terputus, dan refresh gagal sesudah create berhasil. Pemeriksaan juga mencakup pembatalan draft, pilihan yang belum ditambahkan, perpindahan akun, submit ganda, serta rekonsiliasi hasil mutasi yang belum pasti.
- Keyboard memverifikasi fokus judul panel, navigasi bagian, disclosure penugasan, badge di luar urutan fokus, dan fokus kembali sesudah batal/reset filter. Live region dan pesan akun yang tersaring diperiksa di DOM; pengumuman melalui screen reader manusia belum diuji.
- Pembesaran teks 200% dan viewport pendek untuk simulasi keyboard diperiksa oleh browser desktop. Keyboard OS pada perangkat fisik belum diuji.
- Alur petugas dan regresi scanner lulus pada 320, 390, 768, dan 1440px. Audit warna ter-render mencatat kontras minimum teks 6,81:1, batas kontrol 4,49:1, serta fokus 10,52:1; tanpa kegagalan audit atau overflow horizontal. Badge diaudit sebagai teks/informasi, bukan kontrol.
- Regresi admin khusus petugas lulus untuk 12 skenario desktop/mobile, termasuk penolakan role STAFF dengan respons 403. Tidak ada error JavaScript, request fixture yang tak tertangani, atau redirect tak terduga.

Request browser menggunakan fixture lokal dan tidak menulis data backend. Hasil otomatis tidak menggantikan UAT manusia.

## Bukti screenshot

Artefak lokal berada di `/tmp/phase47-50/workflow/`. Untuk setiap lebar 320, 390, 768, dan 1440px:

| Berkas | Isi |
| --- | --- |
| `roster20-list-{width}.png`, `roster20-full-list-{width}.png` | Daftar fixture 20 akun |
| `roster20-create-open-{width}.png` | Panel tambah setelah tombol pembuka dipilih |
| `roster20-event-gate-selection-{width}.png` | Pilihan belum ditambahkan dan penyimpanan ditahan |
| `roster20-manage-{width}.png` | Panel Kelola |
| `roster20-empty-search-{width}.png` | Pencarian/filter tanpa hasil |
| `roster20-filtered-editor-{width}.png` | Editor tetap terbuka ketika akun tersaring |
| `roster20-unknown-password-{width}.png` | Hasil reset belum pasti dan tindakan dikunci |
| `text200-{width}.png`, `keyboard-height-{width}.png` | Pembesaran teks dan simulasi viewport pendek |
| `fixture20-{width}.json` | Data uji |
| `checks-{width}.json`, `management-{width}.json` | Audit kontras/overflow, error browser, dan catatan mutasi |

Regresi admin petugas dicatat terpisah di `/tmp/phase47-50/admin-regression/manifest.json` beserta screenshot. Scanner diperiksa oleh skrip alur petugas. Tidak dilakukan perbandingan piksel baru untuk halaman publik/admin lain pada fase ini; stylesheet bersama tidak berubah.

## UAT manusia — belum dilakukan

Pengguna menyatakan dua admin tidak tersedia. Kedua peserta perlu mengerjakan tugas secara terpisah, tanpa arahan developer tentang tombol atau urutan klik. Gunakan akun dan event uji, lalu catat hambatan serta salah pengertian apa adanya.

1. Temukan petugas tertentu dari 20 akun; sebutkan peran, status, dan gate yang dapat diakses.
2. Perbarui nama seorang petugas, lalu buat akun petugas baru menggunakan data uji yang diberikan.
3. Berikan akun baru akses ke event/gate yang ditentukan; jelaskan kapan akses tersebut mulai berlaku. Ganti gate dan periksa hasil setelah reload.
4. Mulai mengubah akun lain, lalu batalkan perubahan. Pastikan data sebelumnya tetap tersedia.
5. Nonaktifkan petugas yang ditentukan dan periksa status setelah reload; jelaskan dampaknya pada sesi petugas.

| Peserta | Tanggal/perangkat | Tugas selesai mandiri | Salah pengertian status/penyimpanan | Hambatan/catatan | Hasil |
| --- | --- | --- | --- | --- | --- |
| Admin 1 | — | — | — | — | Belum diuji |
| Admin 2 | — | — | — | — | Belum diuji |

Checkpoint manusia baru dapat ditutup ketika kedua admin menyelesaikan tugas tanpa salah memahami status atau penyimpanan. Screenshot dan tes otomatis tidak menutup checkpoint ini.
