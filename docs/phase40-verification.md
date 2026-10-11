# Phase 40 — Verifikasi operasional dengan data dummy

Tanggal eksekusi: **11 Oktober 2026**. Acuan kode: `c78a0ffe0772e327d6d65784effbcdd3a74a0a86` beserta perubahan tes Phase 40 pada working tree.

**Status: verifikasi otomatis lokal dengan data dummy lulus.** Backend, MySQL, AWS SDK, HTTP, dan browser benar-benar dijalankan. Penyimpanan memakai server HTTP fixture S3; pembayaran memakai simulator development. Seluruh akun/pembeli/pengunjung adalah fixture, bukan peserta manusia. Hasil ini melengkapi bukti teknis Phase 35–39; checkpoint penggunaan mandiri oleh manusia serta AWS staging tetap terbuka.

## Data dan lingkungan

| Item | Fixture / lingkungan |
| --- | --- |
| Database | MySQL lokal, schema terpisah `ticket_online_phase40_check`; database aplikasi tidak digunakan |
| Konser | ID acak `poster-roundtrip-*`, Jakarta, Test Venue, jadwal 48 jam setelah eksekusi |
| Inventori | Zona Main, gate Main, 2 tiket @ Rp10.000, batas 2 per order |
| Poster | PNG 8×10 awal dan pengganti dengan isi berbeda; JPEG fixture Nusa Malam untuk browser |
| ADMIN 1 | Membuat draft, zona, tiket, menerbitkan, mengunggah dan menyimpan poster |
| ADMIN 2 | Mengganti poster, mengulang upload setelah koneksi gagal, membuat dua akun STAFF dan penugasannya |
| STAFF 1 / STAFF 2 | Dua akun dan sesi berbeda; masing-masing check-in satu tiket; STAFF 2 juga mencoba tiket yang sudah dipakai |
| Pembeli / pengunjung | Dummy Buyer, Dummy Guest 1/2; alamat email `example.test`; pembayaran QRIS simulasi |
| Browser | Google Chrome headless via Python Playwright; poster 1440/390/320×900; petugas 1440/390×900, sentuh pada 390px |

Akun ADMIN disiapkan oleh fixture melalui service dan perubahan role pada database uji saja. Login dan seluruh tugas operasional berikutnya memakai endpoint HTTP aplikasi. Data fixture dibersihkan setelah tes, termasuk order, tiket, check-in, sesi, akun dan audit terkait; kegagalan cleanup menggagalkan tes. Tidak ada akun dummy yang ditambahkan ke produksi.

## Hasil yang dieksekusi

| Skenario | Hasil / bukti |
| --- | --- |
| ADMIN 1 menyiapkan konser sampai terbit | API pembuatan draft/zona/tiket 201, publikasi 200 |
| Upload → simpan gagal → simpan berhasil | Upload 201; simpan invalid 422 mempertahankan URL lama; simpan valid 200; admin/katalog/detail memuat URL dan deskripsi yang benar |
| ADMIN 2 mengganti poster saat storage terputus | Fixture menutup koneksi PUT; API mengembalikan 503 tanpa URL; konser tetap memakai poster sebelumnya |
| Retry dan penggantian poster | Upload ulang 201 menghasilkan hash/URL berbeda; save 200; aset lama tetap bisa dibaca |
| Restart backend | Server API ditutup; seluruh handler dan client SDK dibuat ulang dengan DB/storage yang sama; katalog dan kedua URL media tetap dapat dibaca |
| Header poster setelah restart | Byte PNG cocok; `nosniff` dan cache `immutable` tersedia; GET media tanpa sesi berhasil |
| Pembelian dummy sampai tiket | Reservasi 201, checkout 201, pembayaran simulasi 201 menerbitkan dua tiket |
| Dua petugas | STAFF 1 check-in 201; STAFF 2 menerima 409 `TICKET_ALREADY_USED` untuk tiket pertama, membaca `CHECKED_IN`, lalu check-in tiket kedua 201 |
| Batas akses | Upload oleh STAFF 403, anonim 401; check-in dengan sesi STAFF kedaluwarsa 401 |
| Browser upload kehilangan koneksi | Request upload dibatalkan sebagai `internetdisconnected`; pesan koneksi muncul; preview/file dipertahankan; simpan diblokir; retry berhasil |
| Browser upload/save lainnya | Lulus file/URL pada konser baru/lama, save gagal setelah upload berhasil, klik berulang, pergantian konser/sumber/file saat upload, cleanup object URL |
| Permukaan publik | Poster tersimpan terlihat dan selesai dimuat pada beranda, katalog, detail; pratinjau admin juga memakai URL hasil simpan |
| Browser kelola petugas | Save penugasan tertunda tidak mengubah pilihan/profil draft; batal konfirmasi pindah mempertahankan draft, setujui konfirmasi membuka petugas berikutnya |
| Browser scanner | Dua scan dengan keyboard dan langkah “Scan berikutnya”; fokus pada hasil; submit bersamaan diblokir |
| Hasil scan belum diketahui | Koneksi gagal, JSON terpotong dan HTTP 500 mempertahankan kode/instruksi; gate ditahan; tidak retry otomatis; scan berikutnya terkunci |

Tes integrasi backend: [posters_integration_test.go](../backend/internal/platform/posters_integration_test.go), `TestPhase40DummyConcertPosterPurchaseAndTwoStaff`. Tes UI: [poster-ui-check.py](../frontend/scripts/poster-ui-check.py) dan [staff-review-check.py](../frontend/scripts/staff-review-check.py). Tes browser memakai respons API fixture; alur HTTP/MySQL diuji terpisah oleh tes integrasi backend. Ini belum merupakan satu sesi browser yang terhubung ke AWS staging.

## Hambatan dan perbaikan

Runner petugas lama masih mengharapkan endpoint katalog publik untuk halaman ADMIN, perpindahan draft tanpa konfirmasi, dan input scan yang langsung aktif setelah hasil. Runner diperbarui untuk `/admin/events`, dialog perubahan belum tersimpan, serta penguncian hasil/fokus dan langkah “Scan berikutnya” yang berlaku saat ini. Tes ulang desktop/mobile lulus. Tidak ditemukan kegagalan aplikasi yang memerlukan perubahan perilaku produk pada skenario yang dijalankan.

## Menjalankan ulang

Gunakan database sementara terpisah yang boleh dimigrasi dan diisi fixture. Jangan gunakan DSN produksi. Dari `backend/`, dengan `MYSQL_TEST_DSN` menunjuk schema tersebut:

```sh
go test -count=1 -run '^TestPhase40' -v ./internal/platform
go test -race -count=1 -run '^TestPhase40' -v ./internal/platform
go test -p 1 -count=1 ./...
```

Dari `frontend/`:

```sh
bun test
bun run check
bun run build
bun run dev -- --host 127.0.0.1 --port 5175 --strictPort
```

Terminal lain, tetap dari `frontend/` (memerlukan Playwright dan Google Chrome yang sudah terpasang):

```sh
python3 scripts/poster-ui-check.py --base-url http://127.0.0.1:5175
python3 scripts/staff-review-check.py http://127.0.0.1:5175 1440
python3 scripts/staff-review-check.py http://127.0.0.1:5175 390
```

| Pemeriksaan 11 Oktober 2026 | Hasil |
| --- | --- |
| Suite backend dengan `MYSQL_TEST_DSN` lokal terpisah | Lulus semua paket |
| Tes Phase 40 dengan race detector | Lulus |
| Frontend unit test | 113 lulus, 0 gagal, 477 assertion |
| Svelte / TypeScript | 0 error, 0 warning |
| Build Vite | Lulus |
| Chrome poster 1440/390/320px | Lulus ketiga ukuran |
| Chrome petugas 1440/390px | Lulus kedua ukuran |

`MYSQL_MIGRATION_TEST_DSN` dan `POSTER_S3_TEST_BUCKET` tidak diisi pada putaran ini; tes migrasi destruktif khusus dan S3 staging dilewati. Bukti suite migrasi sebelumnya tersedia di [Phase 36–39](phase36-39-posters.md).

## Checkpoint dan tindak lanjut

- [x] Alur dummy ADMIN → konser/poster → katalog → pembelian/tiket → dua STAFF/check-in dijalankan.
- [x] Penggantian poster, koneksi upload terputus, retry, save gagal, sesi kedaluwarsa, dan restart handler backend diverifikasi lokal.
- [x] Pemeriksaan browser desktop dan emulasi ponsel dijalankan; runner yang tertinggal diperbaiki.
- [ ] AWS staging aktual: upload melalui sesi ADMIN, private bucket/IAM/versioning, katalog setelah save, redeploy container dan baca ulang aset. Ikuti [runbook poster](phase36-39-posters.md).
- [ ] Picker OS desktop, Android Chrome dan iOS Safari fisik; HEIC tetap perlu dikonversi ke JPEG/PNG.
- [ ] Dua admin dan dua petugas manusia menjalankan tugas tanpa bantuan developer; waktu, bantuan dan hambatan dicatat dari observasi.

Restart handler menguji pemisahan state aplikasi dari storage, belum membuktikan retensi bucket atau redeploy infrastruktur. Sesi akun otomatis dan viewport sentuh belum membuktikan kemudahan penggunaan manusia/perangkat fisik. Data bisnis riil tidak diperlukan untuk mengulang skenario ini.

Tindak lanjut Phase 41–42 telah diimplementasikan setelah putaran Phase 40: biaya admin dan voucher tersimpan di database, pengaturan khusus ADMIN dengan pratinjau/konfirmasi/audit, snapshot order dan kuota transaksional. Aturan lifecycle kuota serta bukti pengujian tersedia di [Phase 41–42](phase41-42-pricing.md). Checkpoint eksternal Phase 40 di atas tetap terbuka.
