# Phase 36–39: poster lokal dan URL

## Implementasi

- Kontrak konser tetap `image: URL`. Tidak ada migrasi database, perubahan URL halaman, pembayaran, stok, tiket, atau kebijakan publikasi.
- `POST /api/v1/admin/posters`: sesi bearer ADMIN diperiksa sebelum membaca body. Tepat satu part file bernama `file`; JPEG/PNG maksimal 5 MiB, seluruh multipart termasuk epilogue maksimal 6 MiB. Decoder memeriksa format, dimensi maksimal 6000 per sisi/20 megapiksel sebelum decode penuh, lalu memeriksa kerusakan. Nama pengguna tidak digunakan sebagai key.
- SHA-256 byte file menentukan nama `.jpg`/`.png`. URL dikembalikan setelah conditional `PutObject` (`If-None-Match: *`) berhasil atau S3 mengonfirmasi key hash sudah ada (412). Upload identik, termasuk upload bersamaan/retry setelah respons hilang, memakai objek dan URL yang sama tanpa versi duplikat. Konflik 409 dan error izin tetap menghasilkan error storage; tidak ada fallback tulis tanpa syarat. Dua upload bersamaan per proses; kelebihan mendapat 429/Retry-After. Timeout 30 detik sejak awal handler mencakup autentikasi, pembacaan, dan S3. Deadline socket dari context yang sama dipasang melalui ResponseController sebelum membaca multipart; error timeout body menghasilkan 504, dan slot dilepas. Context diperiksa sebelum/selepas decode; SDK maksimal dua percobaan. Server membaca maksimal 35 detik dan menulis maksimal 45 detik agar respons timeout upload bisa dikirim.
- `GET /api/v1/media/posters/{filename}` membaca bucket privat melalui backend tanpa sesi/token URL. Content-Type ditentukan ekstensi terverifikasi; nosniff, ETag, cache immutable satu tahun, conditional GET/range. GET dengan satu ETag yang cocok persis, tanpa If-Match/range, mengonfirmasi objek melalui GetObject lalu mengembalikan 304 tanpa membaca body penuh dan selalu menutup body S3. Header conditional lainnya tetap melalui ServeContent; belum ada HeadObject khusus revalidasi. Respons gagal tidak disimpan dalam cache. Origin publik berasal dari `FRONTEND_URL`, HTTPS wajib pada production.
- Bucket kosong menonaktifkan upload dengan 503 `POSTERS_UNAVAILABLE`. Missing credentials/S3 gagal menghasilkan 503 `POSTER_STORAGE_FAILED`, deadline 504 `POSTER_TIMEOUT`. Autentikasi tidak tersedia menghasilkan 503 `AUTH_UNAVAILABLE`; kegagalan memasang deadline baca menghasilkan 503 `POSTER_READ_DEADLINE_FAILED` tanpa membaca body. Error lain dijelaskan di [OpenAPI](openapi.yaml).
- Form Phase 35 memakai radio native, file picker tunggal, pratinjau object URL, nama/ukuran file, status live, retry, dan tombol minimal 44px. Konser baru default upload; poster lama tampil hingga pengganti dipilih. File pilihan menjadi perubahan belum tersimpan, termasuk file yang tidak valid. Simpan diblokir selama file pengganti pending/upload. Beralih ke URL membatalkan file pilihan dan mempertahankan URL draft terakhir. URL eksternal tidak diunduh backend.
- Upload sukses hanya memperbarui draft; kegagalan simpan mempertahankan URL terunggah dan field lain. Mengganti file/sumber/konser membatalkan request dan mengabaikan respons lama. Object URL dibersihkan saat diganti/component dihancurkan; browser membersihkan resource dokumen saat tab ditutup.
- Tidak ada crop, kompresi, galeri, atau multi-upload. Semua file terunggah adalah aset publik sebelum publikasi. Objek lama tetap disimpan; cleanup memerlukan pelacakan referensi yang aman.

## Provisioning dan konfigurasi

[deploy/poster-buckets.yaml](../deploy/poster-buckets.yaml) membuat dua bucket AWS S3 terpisah, staging/production, yang tidak digunakan untuk backup SQL. Nama bucket dibuat CloudFormation. Keduanya privat dengan Block Public Access, ACL dinonaktifkan, enkripsi AES256, versioning aktif, akses hanya melalui TLS, dan Retain saat stack dihapus/diganti. Tidak ada lifecycle expiration untuk poster atau versi lama.

Jalankan dari root dengan profile operator yang berwenang:

```sh
aws cloudformation deploy --profile YOUR_OPERATOR_PROFILE --region ap-southeast-1 \
  --stack-name ticket-online-posters --template-file deploy/poster-buckets.yaml
aws cloudformation describe-stacks --profile YOUR_OPERATOR_PROFILE --region ap-southeast-1 \
  --stack-name ticket-online-posters --query 'Stacks[0].Outputs'
```

Masukkan output sesuai lingkungan pada `.env.staging`/`.env.production`, mencontoh file `.example`:

| Backend | Compose staging | Compose production | Makna |
| --- | --- | --- | --- |
| POSTER_S3_BUCKET | STAGING_POSTER_S3_BUCKET | PRODUCTION_POSTER_S3_BUCKET | Bucket khusus lingkungan; kosong menonaktifkan upload |
| POSTER_S3_REGION | STAGING_POSTER_S3_REGION | PRODUCTION_POSTER_S3_REGION | Region bucket |
| POSTER_S3_ENDPOINT | STAGING_POSTER_S3_ENDPOINT | PRODUCTION_POSTER_S3_ENDPOINT | Opsional S3-compatible, path style otomatis; gunakan TLS |
| POSTER_S3_PREFIX | STAGING_POSTER_S3_PREFIX | PRODUCTION_POSTER_S3_PREFIX | Default `posters` |
| FRONTEND_URL | Diturunkan dari STAGING_DOMAIN | Diturunkan dari PRODUCTION_DOMAIN | Origin HTTPS stabil endpoint media |

Kredensial memakai default credential chain AWS SDK; utamakan instance/workload IAM role. SDK juga mendukung shared profile pada proses backend yang dijalankan langsung, atau environment credentials backend. Compose menyediakan `STAGING_POSTER_AWS_ACCESS_KEY_ID/SECRET_ACCESS_KEY/SESSION_TOKEN` dan padanan `PRODUCTION_`, diteruskan hanya pada service API sebagai variabel AWS standar. Variabel backup `AWS_PROFILE` tidak diteruskan ke API. Jangan menaruh kredensial dalam VITE_*, URL, Git, atau log. Compose lokal menerima POSTER_S3_* serta AWS_* backend untuk pengembangan; gunakan `APP_ENV=development` untuk origin HTTP lokal.

Role staging hanya boleh mengakses bucket staging; production hanya bucket production. Policy minimum untuk prefix default (ganti nama bucket, buat policy terpisah per lingkungan):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {"Effect": "Allow", "Action": ["s3:GetObject", "s3:PutObject"], "Resource": "arn:aws:s3:::YOUR_POSTER_BUCKET/posters/*"},
    {"Effect": "Allow", "Action": "s3:ListBucket", "Resource": "arn:aws:s3:::YOUR_POSTER_BUCKET"}
  ]
}
```

ListBucket memungkinkan S3 membedakan key yang hilang (404) dari akses terlarang (403); backend tidak menyediakan endpoint daftar objek. Role aplikasi tidak memerlukan DeleteObject, pengaturan bucket, atau izin backup SQL. Profile penguji staging juga memerlukan `s3:GetBucketVersioning`.

## Ketahanan, pemulihan, kapasitas

Objek berada di S3 dan tidak terpengaruh rebuild image/redeploy container. Pertahankan bucket, prefix, dan origin media yang sama antarrelease; perubahan origin membutuhkan redirect domain lama untuk URL tersimpan. Untuk S3-compatible, terapkan private bucket, encryption, versioning, dan retensi melalui kontrol provider yang setara sebelum digunakan. Pastikan provider mendukung conditional PutObject `If-None-Match: *` dan respons 412; backend tidak menurunkan syarat write jika fitur tersebut tidak didukung.

Pemulihan oleh operator: cari versi key melalui `aws s3api list-object-versions`, unduh versi baik memakai `get-object --version-id`, lalu unggah byte asli ke key yang sama. Verifikasi hash file sama dengan nama key dan Content-Type sesuai ekstensi. Untuk delete marker, pulihkan versi baik lewat salinan ulang; jangan mengubah isi di bawah key hash lama. Tes GET endpoint media setelah pemulihan. Hak baca versi dan pemulihan hanya untuk operator; backup database saja tidak memulihkan objek poster.

Pantau metrik harian AWS/S3 `BucketSizeBytes` (seluruh StorageType relevan termasuk versi lama) dan `NumberOfObjects`, serta biaya storage/egress. Aktifkan budget/alarm sesuai batas operasional lingkungan. Pantau log `poster upload succeeded`/`poster upload failed` (kode, status, staff ID, hash, ukuran; tanpa kredensial/isi/nama file pengguna), 429, 503, 504, dan latensi HTTP endpoint upload/media. Dua upload maksimal per proses membatasi decoder gambar sekitar 160 MB per decode pada kasus PNG 16-bit 20 MP, selain buffer dan runtime; sesuaikan batas memori container dan kapasitas replika berdasarkan pengukuran. Versi dan aset tanpa referensi akan bertambah karena belum ada cleanup.

## Verifikasi otomatis

Dari `backend/`, jalankan suite terhadap MySQL sementara (jangan database aplikasi):

```sh
MYSQL_TEST_DSN='...' MYSQL_MIGRATION_TEST_DSN='...' go test -p 1 ./...
go vet ./...
go build ./cmd/api ./cmd/migrate ./cmd/staff
```

Paket posters memakai AWS SDK asli dengan server HTTP S3 fixture: JPEG/PNG, MIME palsu, path filename, kosong/rusak/SVG/GIF, ukuran/dimensi/megapiksel, file lebih dari satu, field salah, multipart chunked/epilogue berlebih, autentikasi sebelum body dibaca, unconfigured, kegagalan/timeout S3, dua upload bersamaan/429, headers/cache/404/conditional GET. Tes MySQL memeriksa STAFF ditolak, ADMIN berhasil, sesi kedaluwarsa ditolak. Tes konfigurasi memeriksa origin, region, endpoint, prefix, dan HTTPS production.

Dari `frontend/`:

```sh
bun test
bun run check
bun run build
bun run dev -- --host 127.0.0.1 --port 5174
# Terminal lain; memerlukan Playwright dan Chrome dari tooling UI existing:
python3 scripts/poster-ui-check.py
```

API unit test memeriksa multipart/session/AbortSignal dan respons tidak valid/error. Browser check memakai API fixture pada 1440, 390, dan 320px: pratinjau file, pending/dirty, upload gagal/retry, upload sukses lalu simpan gagal, klik berulang, pergantian konser/sumber, URL, validasi HEIC/ukuran, cleanup object URL, ukuran tombol/overflow, serta poster tersimpan pada admin/beranda/katalog/detail. Pengujian fixture ini tidak membuktikan konektivitas bucket staging.

Tes S3 staging terpisah, memakai bucket staging privat yang sudah diprovision, bukan production:

```sh
export AWS_PROFILE=YOUR_STAGING_TEST_PROFILE
export POSTER_S3_TEST_BUCKET=YOUR_STAGING_POSTER_BUCKET
export POSTER_S3_TEST_REGION=ap-southeast-1
export POSTER_S3_TEST_PREFIX=posters
# Opsional: POSTER_S3_TEST_ENDPOINT untuk S3-compatible.
cd backend
go test -count=1 -run '^TestStagingS3$' -v ./internal/posters
```

Tes memeriksa versioning, upload SDK, dan baca melalui handler baru yang mensimulasikan restart. Fixture kecil berbasis hash dibiarkan sebagai aset uji; tidak dihapus otomatis. Jika variabel bucket tidak ada, tes dilewati secara eksplisit.

## Pemeriksaan staging/perangkat yang masih diperlukan

Putaran lanjutan dengan data dummy, dua ADMIN/dua STAFF, koneksi upload terputus, restart backend dan browser desktop/mobile tercatat pada [Phase 40](phase40-verification.md). Putaran lokal tersebut melengkapi bukti otomatis; pengujian AWS dan perangkat fisik berikut tetap diperlukan.

- Provisioning AWS aktual, pengisian konfigurasi per lingkungan, policy IAM, budget/alarm kapasitas, serta eksekusi tes S3 di atas membutuhkan akun/profile AWS dan domain staging yang dipilih operator. Lingkungan pengerjaan belum menyediakan AWS CLI/kredensial/konfigurasi poster.
- Setelah deploy, gunakan sesi ADMIN nyata untuk upload, simpan konser, lalu verifikasi poster pada empat permukaan dari perangkat berbeda. Pastikan GET tanpa sesi ke backend berhasil, sedangkan akses anonim langsung ke key bucket ditolak. Ulangi sesudah redeploy, saat sesi berakhir, dan ketika izin S3 sementara dicabut pada staging.
- Desktop Chrome otomatis mencakup set_input_files dan rendering, bukan dialog OS fisik. Ulangi picker di desktop, Android Chrome, iOS Safari: ambil file kamera/galeri JPEG/PNG, cancel picker, ganti file saat upload, rotasi, keyboard, dan pindah konser. Pilihan accept adalah petunjuk; perangkat bisa tetap menawarkan format lain.
- HEIC/HEIF dari iPhone tidak didukung; ekspor/konversi ke JPEG/PNG terlebih dahulu. Jika sistem otomatis mengonversi gambar menjadi JPEG, file hasil tetap harus lolos decoder backend. Jangan mengklaim emulasi Chromium sebagai pengujian Safari/iOS atau picker Android fisik.

## Hasil lokal (10 Oktober 2026)

- Lulus seluruh `go test -p 1 ./...` dengan MYSQL_TEST_DSN dan MYSQL_MIGRATION_TEST_DSN pada schema sementara terpisah, termasuk sesi poster dan seluruh skenario interupsi migrasi. Paket migrasi selesai dalam sekitar 398 detik.
- Lulus `go vet ./...` dan build binary API/migrate/staff.
- Lulus tes integrasi route lengkap: sesi ADMIN → upload ke fixture S3 → simpan konser gagal (poster lama tetap ada) → simpan berhasil → URL terbaca di admin/katalog/detail dan byte poster tersedia tanpa sesi.
- Lulus 113 unit test frontend (477 assertion), Svelte/TypeScript tanpa error/warning, dan build Vite; `_redirects` tersedia dalam dist.
- Lulus Chrome pada 1440/390/320px termasuk kedua sumber pada konser baru/lama, penggantian file saat upload, retry, simpan gagal, stale response, cleanup, serta empat permukaan poster.
- YAML OpenAPI/security schemes, template dua bucket, dan konfigurasi Compose backend diperiksa. Tidak ada kredensial AWS pada frontend.
- Tes staging S3 dilewati karena POSTER_S3_TEST_BUCKET belum tersedia. Nama `adeqq1` diberikan operator. Berkas `~/Unduhan/ubuntu-aws.pem` tersedia sebagai kunci SSH, tetapi belum ada profile kredensial AWS SDK. Host/IP server, region, dan domain staging belum diberikan. Provisioning AWS, alarm aktual, dan pengujian Android/iOS fisik belum dijalankan.

## Perbaikan review PR #30 (11 Oktober 2026)

- Deadline baca socket mengikuti sisa budget context operasi; body multipart/header/file maupun epilogue yang terhenti berakhir 504, termasuk chunked. Fixture koneksi HTTP nyata memakai budget parent 300 ms dan ReadTimeout global 2 detik untuk membedakan kedua deadline, memastikan dua slot kembali tersedia dan S3 tidak dipanggil untuk body timeout. Tes timeout SDK 40 ms tetap dipertahankan.
- Revalidasi satu ETag persis cocok menutup body tanpa membaca penuh; objek hilang tetap 404, If-Match tetap didahulukan, dan weak/list/range memakai ServeContent.
- Conditional PUT menghindari versi duplikat untuk file identik, upload bersamaan, dan retry setelah respons hilang. Fixture membedakan 412 yang berhasil dari 403/404/409 yang tetap gagal. Objek dan versi lama tidak dihapus.

Validasi perbaikan: lulus `MYSQL_TEST_DSN=... go test -p 1 ./...` pada schema tes terpisah, `go vet ./...`, build API/migrate/staff, serta race detector untuk deadline body, revalidasi ETag, dan upload bersamaan. Tes integrasi route memakai server HTTP nyata agar ResponseController mencapai socket melalui middleware Unwrap. Tes S3 staging belum dijalankan.
