# Phase 41–42 — Biaya admin dan voucher

**Status: implementasi Phase 41–42 selesai; pengujian lokal lulus.** Tanggal: **11 Oktober 2026**. Data pengujian dummy memakai database sementara terpisah; tidak ada akun, order, atau voucher uji yang ditambahkan ke produksi.

## Pemakaian ADMIN

Buka **Biaya dan voucher** (`/admin/pricing`). STAFF dan sesi anonim tidak dapat mengakses API pengaturan.

- Biaya admin adalah nominal tetap rupiah **per order**, nilai awal migrasi Rp7.500. Isi nominal, lihat pratinjau subtotal + biaya, lalu konfirmasi penyimpanan. Rentang Rp0–Rp1.000.000.000.
- Buat voucher dengan kode, jenis persentase/nominal, nilai, awal/akhir berlaku, status aktif, cakupan satu konser atau semua konser, minimum subtotal tiket, dan kuota order.
- Kode dinormalisasi ke huruf besar, panjang 2–32 karakter: huruf/angka, `_`, atau `-`; karakter pertama huruf/angka. Kode tersimpan tidak dapat diganti atau dihapus. Nonaktifkan promosi yang selesai.
- Masa berlaku mengikuti zona perangkat pada form, dikirim sebagai RFC3339 dengan zona waktu. Awal termasuk, akhir tidak termasuk. Persentase 1–100; nominal Rp1–Rp1.000.000.000; kuota 1–1.000.000.000.
- Daftar menunjukkan kuota ditahan, dipakai, dan dilepas. Detail order ADMIN menampilkan kode, jenis dan nilai diskon yang tersimpan.
- Perubahan biaya/voucher dan audit pelaku, waktu, sebelum/sesudah disimpan dalam transaksi yang sama. UI menampilkan 100 perubahan terbaru; riwayat database tidak dipangkas.
- Setiap pengaturan memiliki `revision`. Konflik dengan perubahan admin lain menghasilkan 409; muat ulang, periksa nilai terbaru, lalu terapkan ulang perubahan. Kegagalan simpan mempertahankan draft. Konfirmasi diperlukan sebelum membuang perubahan.

## Aturan perhitungan dan kuota

Aturan berikut ditetapkan sebelum implementasi kuota. Satu voucher untuk satu order; diskon hanya atas subtotal tiket, tanpa mengurangi biaya admin. Persentase dihitung dengan integer dan dibulatkan ke bawah ke rupiah; diskon nominal dibatasi subtotal. Minimum pembelian memakai subtotal sebelum diskon, tanpa biaya admin. Kuota dihitung per order, bukan jumlah tiket atau pembeli.

| Kejadian | Pengaruh kuota |
| --- | --- |
| Pratinjau biaya (`quote`) | Memvalidasi ketersediaan; tidak memesan kuota |
| Checkout baru berhasil, order PENDING | Menahan satu kuota dalam transaksi pembuatan order |
| Checkout invalid atau transaksi rollback | Tidak memakai kuota |
| Replay checkout dengan reservasi/payload yang sama | Mengembalikan snapshot order yang sama, tanpa tambahan kuota |
| Pembayaran gagal yang masih bisa diulang | Tetap menahan kuota selama order PENDING |
| Jam kedaluwarsa terlewati, tetapi rekonsiliasi belum selesai | Tetap ditahan; tidak mengasumsikan pembayaran belum berhasil |
| Order resmi EXPIRED/CANCELLED setelah proses expiry/rekonsiliasi | Melepas kuota jika belum dikonsumsi |
| Pembayaran berhasil dan tiket diterbitkan | Mengonsumsi kuota permanen dalam transaksi yang sama |
| Refund pending/selesai | Tidak mengembalikan kuota yang sudah dikonsumsi |
| Pembayaran terlambat setelah order resmi dilepas | Mengikuti rekonsiliasi/refund existing; tidak menerbitkan tiket atau mengambil kembali kuota |

Checkout memakai isolasi READ COMMITTED dan mengunci baris voucher (`FOR UPDATE`) sebelum menghitung penggunaan serta menyimpan order. Dua checkout yang memperebutkan kuota terakhir tidak dapat sama-sama berhasil. Update voucher memakai penguncian yang sama; kuota tidak boleh diturunkan di bawah jumlah ditahan + dikonsumsi. Perubahan status pembayaran/expiry atomik menjaga perpindahan kuota.

Nominal biaya, diskon, total serta kode/jenis/nilai voucher disimpan pada order. Mengubah biaya, aturan atau status voucher hanya memengaruhi order baru; replay, detail, pembayaran dan refund menggunakan snapshot lama. Refund memakai nominal transaksi tersimpan sesuai kebijakan refund existing.

Checkout meminta estimasi server dari reservasi aktif dan kunci aksesnya. Server membaca subtotal dari database. Bila pratinjau gagal, pembayaran order baru diblokir dan UI tidak mengarang nominal. Bila nominal checkout berubah dari estimasi, order/access tetap disimpan dan pembeli diminta meninjau total sebelum memulai pembayaran.

**Batas versi ini:** cakupan satu konser atau semua konser; tidak ada kuota per pembeli, kombinasi voucher, atau order gratis. Biaya admin nol diperbolehkan, tetapi total akhir nol ditolak karena alur pembayaran existing memerlukan nominal positif. Tambahkan alur penerbitan tiket gratis sebelum membuka total nol.

## API dan migrasi

Kontrak lengkap dan kode error ada di [OpenAPI](openapi.yaml).

| Endpoint | Akses / hasil |
| --- | --- |
| `GET /api/v1/admin/pricing` | ADMIN; biaya, voucher/penggunaan, audit |
| `PUT /api/v1/admin/pricing` | ADMIN; `{adminFee, revision}` |
| `POST /api/v1/admin/vouchers` | ADMIN; record lengkap, revision awal 0 |
| `PUT /api/v1/admin/vouchers/{code}` | ADMIN; record lengkap dengan revision terbaru |
| `POST /api/v1/reservations/{reservationID}/quote` | `Idempotency-Key` milik reservasi; `{voucherCode}` opsional |

Pengaturan memberi 401 untuk sesi hilang/kedaluwarsa, 403 untuk STAFF, 400 untuk JSON invalid, 404 untuk record tidak ditemukan, 409 untuk revision/kode/kuota konflik, 422 untuk aturan invalid, dan 500 untuk kegagalan internal. Quote memakai error checkout existing termasuk `INVALID_VOUCHER` (422) akses reservasi tidak cocok (401), serta reservasi tidak aktif/kedaluwarsa (410). Respons privat/pratinjau memakai `Cache-Control: no-store`.

Migrasi [028_pricing.sql](../backend/migrations/028_pricing.sql) menambahkan `checkout_settings`, `vouchers`, dan `order_vouchers`. Seed Rp7.500 dan `HEMAT10` 10% mempertahankan perilaku awal, tetapi nilainya kini data yang dapat dikelola ADMIN. Seed HEMAT10 berlaku 2020–2099, semua konser, minimum nol, kuota satu miliar; tinjau/nonaktifkan lewat ADMIN sesuai kebutuhan promosi. Order lama dengan diskon dibackfill sebagai HEMAT10, sesuai satu-satunya voucher pada versi sebelumnya; order PAID/REFUND_PENDING/REFUNDED ditandai sudah mengonsumsi kuota. Nominal order lama tidak diubah.

Migrasi memakai operasi resumable dan seed `INSERT IGNORE`; semua enam titik interupsi migrasi diuji. Startup backend menjalankan migrasi existing sebelum melayani request. Backup database sebelum rollout dan deploy backend dahulu, kemudian frontend. Tidak ada variabel environment baru. Jangan menghapus tabel snapshot untuk rollback aplikasi; pertahankan data transaksi dan audit. Bila versi lama dipulihkan, hentikan checkout baru karena versi lama masih memakai aturan hard-coded; jangan membuka penjualan sampai versi yang memahami konfigurasi database aktif kembali.

## Bukti pengujian

Tes [pricing_integration_test.go](../backend/internal/platform/pricing_integration_test.go) memakai HTTP aplikasi, MySQL lokal `ticket_online_phase40_check`, akun ADMIN/STAFF dan pembayaran simulasi. Fixture dibersihkan dan biaya/revision awal dikembalikan setelah tes. Browser [pricing-ui-check.py](../frontend/scripts/pricing-ui-check.py) memakai respons API dummy, diuji terpisah dari integrasi database.

| Pemeriksaan | Hasil |
| --- | --- |
| Fee baru, fee nol, snapshot/replay order lama | Lulus |
| Persentase, pembulatan integer, diskon nominal dibatasi subtotal | Lulus |
| Cakupan/minimum, nonaktif, belum berlaku/kedaluwarsa, total nol | Lulus |
| Dua checkout serentak untuk kuota terakhir | Tepat satu berhasil; lulus race detector |
| Quote tidak menahan, pending/payment retry menahan | Lulus |
| Expiry resmi melepas; pembayaran/tiket dan refund tidak melepas | Lulus; workflow refund manual existing dijalankan |
| ADMIN/STAFF/anonim/sesi kedaluwarsa, audit, revision stale, pengurangan kuota invalid | Lulus |
| Migrasi 28 terputus lalu resume/rerun | Lulus keenam titik interupsi pada database disposable |
| Suite backend dengan MySQL terpisah; `go vet` dan `go build` | Lulus semua paket |
| Frontend unit test | 115 lulus, 0 gagal, 499 assertion |
| Svelte / TypeScript; build Vite | 0 error, 0 warning; build lulus |
| OpenAPI | YAML dan 439 referensi lokal valid |
| Regresi poster Chrome 1440/390/320px | Lulus ketiga ukuran |
| Chrome 1440px / emulasi sentuh 390px | Lulus fee preview/konfirmasi/gagal/retry, voucher create/edit/nonaktif/penggunaan, quote gagal dan review perubahan nominal; tanpa error runtime atau overflow |

Pengujian mobile menemukan rincian biaya menutup sendiri setelah quote voucher diperbarui. State buka/tutup diperbaiki dan tes ulang kedua ukuran lulus. Race detector awal menemukan slice attendee yang dibagi oleh dua request fixture; fixture kini membuat payload terpisah seperti dua HTTP request sebenarnya dan tes ulang lulus.

Jalankan ulang dari `backend/`, dengan `MYSQL_TEST_DSN` menunjuk database **sementara terpisah**:

```sh
go test -p 1 -count=1 ./...
go test -race -count=1 -run '^TestPricing' -v ./internal/platform
go vet ./...
go build ./...
```

Untuk migrasi, gunakan `MYSQL_MIGRATION_TEST_DSN` dengan izin membuat/menghapus database disposable (jangan gunakan produksi):

```sh
go test -count=1 -run '^TestMySQLNonAtomicMigrationsResume/version_28_' -v ./migrations
```

Dari `frontend/`:

```sh
bun test
bun run check
bun run build
bun run dev -- --host 127.0.0.1 --port 5175 --strictPort
```

Terminal lain dari `frontend/`, memakai Playwright dan Chrome yang sudah tersedia:

```sh
python3 scripts/pricing-ui-check.py http://127.0.0.1:5175
python3 scripts/poster-ui-check.py --base-url http://127.0.0.1:5175
```

Keterbatasan verifikasi operasional AWS staging, perangkat fisik dan sesi manusia tetap dicatat di [Phase 40](phase40-verification.md). Hasil dummy tidak membuktikan deployment eksternal atau pembayaran bank nyata.
