# ticket-online

Demo alur pemesanan tiket konser dengan Svelte 5, Vite, TypeScript, Bun, Go, MySQL, dan Docker Compose.

## Arsitektur

```text
Browser -> frontend:5173 -> /api proxy -> api:8080 -> db:3306
```

- `frontend/`: SPA Svelte 5 dan Vite.
- `backend/`: modular monolith Go dengan `net/http`, `database/sql`, dan MySQL driver.
- `backend/migrations/`: migration dan seed yang di-embed ke binary Go.
- `compose.yaml`: MySQL, API production, dan Vite development server.

Catalog event, reservation, order, pembayaran, dan e-ticket menggunakan database. Checkout dibuat dari reservasi aktif lewat `POST /api/v1/reservations/{reservationID}/checkout` dengan `buyer`, nama untuk setiap tiket pada `attendees`, dan `voucherCode` opsional. Biaya admin Rp7.500 dan voucher `HEMAT10` dihitung server; request ulang dengan data checkout sama mengembalikan order yang sama. Pembayaran simulasi hanya tersedia di backend development. Pembayaran berhasil menerbitkan snapshot e-ticket untuk setiap tiket dalam transaksi yang sama.

Frontend checkout, pembayaran simulasi, dan e-ticket membaca order serta snapshot tiket dari API. Setiap peserta menerima satu kode `ET-…` dengan data gate dan snapshot acara dari server. Tiket Saya memuat order yang aksesnya tersimpan pada browser yang sama; tautan pada email konfirmasi juga dapat membuka e-ticket di perangkat lain. Snapshot demo lama tetap tersimpan, tetapi tidak dianggap tiket backend.

Order `PENDING` memakai deadline reservasi, tersedia sebagai `expiresAt` pada respons checkout/detail order. Worker mengubah order yang lewat deadline menjadi `EXPIRED` dan mengembalikan stok tepat sekali; pembayaran terlambat ditolak meskipun worker belum berjalan. Deadline ini berbeda dari masa berlaku token akses.

Setelah pembayaran berhasil dan semua e-ticket diterbitkan, worker membuat antrean email dan mengirim ringkasan pesanan beserta tautan untuk setiap peserta. Tautan membawa akses privat pada fragment URL yang dibersihkan dari address bar setelah dibuka; tiket dapat dimuat di perangkat lain selama akses order belum kedaluwarsa. Email dikirim maksimal tiga kali. Status antrean yang dapat dilihat di database adalah `PENDING`, `PROCESSING`, `SENT`, dan `FAILED`; `SENT` berarti server SMTP menerima pesan.

Pembeli yang berganti browser dapat membuka `/pulihkan-tiket`, mengisi email dan reference pesanan, lalu menerima tautan pemulihan di email pembeli yang tersimpan. Tautan berlaku 15 menit, dipakai sekali setelah tombol **Pulihkan tiket** ditekan, dan menyimpan akses order di browser tersebut. Respons `POST /api/v1/ticket-recovery` selalu sama untuk data yang cocok maupun tidak; pencocokan dilakukan worker dan hanya pesanan `PAID` dengan tiket lengkap serta akses aktif yang dikirimi email. Database hanya menyimpan hash SHA-256 token. Halaman `/pesanan/{id}` menampilkan ringkasan, tautan tiket, dan tombol **Kirim ulang email** (`POST /api/v1/orders/{orderID}/resend-email`, satu kali per menit per order). Batas permintaan disimpan di tabel `rate_limits`, sehingga tetap berlaku setelah restart. Migration 014 dapat dilanjutkan setelah interupsi DDL tanpa mengubah checksum SQL historis; jalankan ulang migration runner sebelum API menerima traffic. Migration 014 mengubah primary key `email_queue`; hentikan API lama sebelum menjalankan versi ini agar worker lama tidak ikut memproses antrean.

Checkout juga memerlukan header `Idempotency-Key` yang sama dengan key reservasi. Respons menyertakan `accessToken` privat order serta `accessExpiresAt`. Simpan token dengan aman; token dipakai sebagai `Authorization: Bearer <accessToken>` pada API order dan tiket, serta pembayaran simulasi. Token kedaluwarsa pukul 00.00 WIB setelah tanggal konser. Server memakai `ORDER_ACCESS_SECRET` yang tetap untuk menandatangani token.

## Requirements

- Go 1.26 atau lebih baru
- Bun
- Docker Engine dan Docker Compose

## Menjalankan Seluruh Stack

```bash
docker compose up --build
```

Compose memerlukan `ORDER_ACCESS_SECRET`. Buat sekali dan simpan nilainya sebagai secret deployment:

```bash
export ORDER_ACCESS_SECRET="$(openssl rand -base64 32)"
docker compose up --build
```

Buka:

- Frontend development: `http://localhost:5173`
- API: `http://localhost:8080`
- Health: `http://localhost:8080/api/v1/health`
- Readiness: `http://localhost:8080/api/v1/ready`

API menunggu database sehat, menjalankan migration, lalu menyajikan frontend production dari `/app/public`. Database menggunakan named volume `mysql_data`.

Jika port lokal sudah digunakan, hentikan service tersebut atau ubah mapping port di `compose.yaml`. Jangan memakai password development ini untuk production.

## Menjalankan Backend Terpisah

Jalankan MySQL terlebih dahulu:

```bash
docker compose up -d db
```

Dari root repository:

```bash
cd backend
export MYSQL_DSN='ticket:ticket@tcp(127.0.0.1:8000)/ticket_online?parseTime=true&loc=UTC'
go run ./cmd/migrate
go run ./cmd/api
```

Environment variable backend:

| Variable | Default | Keterangan |
|---|---|---|
| `HTTP_ADDR` | `:8080` | Alamat HTTP API |
| `MYSQL_DSN` | wajib | DSN MySQL aplikasi |
| `ORDER_ACCESS_SECRET` | wajib | Base64 dari 32 byte acak; harus tetap sama setelah restart |
| `APP_ENV` | `production` | Aktifkan endpoint pembayaran simulasi hanya dengan `development` |
| `MIDTRANS_ENV` | `sandbox` | Pilih `sandbox` atau `production`; tidak mengikuti `APP_ENV` |
| `MIDTRANS_SERVER_KEY` | kosong | Server key untuk lingkungan yang dipilih; validasi prefix berbeda untuk sandbox dan production |
| `TRANSACTIONS_ENABLED` | `true` untuk sandbox, `false` untuk production | `true` atau `false`; menghentikan reservasi, checkout, dan sesi pembayaran baru |
| `MIDTRANS_REFUND_METHODS` | kosong | Kanal refund yang sudah diverifikasi pada akun merchant; hanya `QRIS` dan `GOPAY` diterima |
| `FRONTEND_URL` | `http://localhost:5173` | Origin frontend untuk validasi URL kembali Snap |
| `SMTP_HOST` | kosong | Host server email; kosong menonaktifkan pengiriman email |
| `SMTP_PORT` | `587` | Port SMTP |
| `SMTP_USERNAME` | kosong | Username SMTP; harus diisi bersama password |
| `SMTP_PASSWORD` | kosong | Password SMTP |
| `SMTP_FROM` | kosong | Alamat pengirim; wajib saat `SMTP_HOST` diisi |
| `SMTP_TLS_MODE` | `starttls` | `starttls` atau `tls`; `none` hanya untuk development tanpa kredensial |
| `TRUSTED_PROXY_CIDRS` | kosong | Daftar CIDR proxy tepercaya dipisahkan koma untuk limiter pemulihan |
| `STATIC_DIR` | kosong | Direktori frontend production |
| `RESERVATION_TTL` | `10m` | Lama reservation |
| `EXPIRY_INTERVAL` | `15s` | Interval expiry worker |

`go run ./cmd/api` juga menjalankan migration sebelum menerima traffic. `go run ./cmd/migrate` aman dijalankan berulang kali. Backend terpisah juga memerlukan `ORDER_ACCESS_SECRET` yang sama setiap kali proses API dijalankan.

Untuk mengaktifkan email, set `SMTP_HOST`, `SMTP_FROM`, dan kredensial yang diberikan server SMTP. `SMTP_USERNAME` dan `SMTP_PASSWORD` harus hadir bersama. Set `FRONTEND_URL` ke origin aplikasi tanpa subpath, query, userinfo, atau fragment; HTTP hanya diperbolehkan pada development dan HTTPS wajib pada production sebelum mengaktifkan SMTP. Compose meneruskan variabel ini ke API; simpan password melalui secret environment deployment, bukan Git.

Proxy Vite meneruskan IP klien melalui `X-Forwarded-For`. Compose memakai subnet `172.30.18.0/24` dan IP frontend tetap `172.30.18.3`; API hanya mempercayai `172.30.18.3/32` secara default. Untuk backend lokal di belakang Vite lokal, set `TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128`. Pada deployment lain, isi hanya CIDR proxy yang dikendalikan aplikasi dan sesuaikan jika subnet Compose diubah; jangan mempercayai seluruh internet atau jaringan yang juga digunakan klien langsung. Default loader kosong mengabaikan header forwarding. Rantai header diperiksa dari kanan ke kiri sampai IP pertama di luar proxy tepercaya; nilai bukan IP diabaikan dengan memakai alamat koneksi. Batas request dan verifikasi memakai identitas yang sama.

Untuk sandbox, isi `MIDTRANS_ENV=sandbox` dan `MIDTRANS_SERVER_KEY` dengan Server Key sandbox dari dashboard Midtrans, lalu set `FRONTEND_URL` ke origin aplikasi. Compose staging memaksa lingkungan sandbox dan mengaktifkan transaksi. Daftarkan notifikasi ke `POST /api/v1/payments/midtrans/notification`. Checkout meminta sesi Snap lewat `POST /api/v1/orders/{orderID}/payments`, lalu mengarahkan pembeli ke Midtrans. Order ditandai lunas dan e-ticket diterbitkan setelah notifikasi bertanda tangan valid dengan nominal cocok. Worker memeriksa status sebelum melepas stok order yang punya sesi aktif; jika provider tidak dapat dihubungi, stok tetap ditahan. Settlement setelah stok dilepas dicatat sebagai kasus rekonsiliasi dan tidak menerbitkan tiket melebihi kapasitas.

Refund penuh diajukan ADMIN dari detail pesanan melalui `POST /api/v1/admin/orders/{orderID}/refund`. Nominal berasal dari database, alasan dicatat dalam audit, dan tiket dinonaktifkan sambil hasil Midtrans belum diketahui. Allowlist `MIDTRANS_REFUND_METHODS` kosong secara default sampai kemampuan refund akun merchant dan kanal diverifikasi. QRIS dibatasi 7 hari dan GoPay 45 hari; Virtual Account dan kanal lain ditolak. Worker merekonsiliasi hasil belum diketahui melalui status Midtrans tanpa mengirim refund baru.

Untuk konfigurasi production, gunakan file deployment dan secret terpisah dari staging. Salin `.env.production.example` menjadi `.env.production`, isi secret di secret manager/environment deployment, lalu validasi dengan `docker compose --env-file .env.production -f compose.production.yaml config`. Setelah domain HTTPS dan webhook Midtrans diarahkan ke origin production, mulai API memakai `docker compose --env-file .env.production -f compose.production.yaml up -d`. Gunakan database/volume khusus production. API mengikat database ke lingkungan Midtrans saat startup; database yang sudah memproses transaksi sandbox tidak dapat dipakai sebagai production. Production memulai dengan `TRANSACTIONS_ENABLED=false`. Setelah verifikasi eksternal siap, operator mengatur variabel menjadi `true` dan menjalankan ulang service API dengan `--force-recreate`; webhook, rekonsiliasi, expiry, dan akses tiket tetap tersedia saat transaksi dihentikan. Key Midtrans production tetap diwajibkan walaupun transaksi dihentikan agar webhook serta rekonsiliasi bisa berjalan.

Endpoint simulasi lokal tetap tersedia hanya ketika `APP_ENV=development` dan dapat diaktifkan dengan `VITE_ENABLE_PAYMENT_SIMULATION=true`. Pembayaran simulasi memerlukan token privat order dari checkout. Detail order dan e-ticket tetap tersedia melalui `GET /api/v1/orders/{orderID}`, `GET /api/v1/orders/{orderID}/tickets`, dan `GET /api/v1/tickets/{ticketID}`.

## Akun Petugas

Buat admin pertama sekali setelah database berjalan. Jalankan dari root repo. Beri nama 2–80 karakter, email valid (misalnya `admin@example.com`), dan password 12–128 byte. Password dibaca tersembunyi lalu diteruskan melalui stdin, bukan argumen proses:

```bash
read -rsp 'Password admin (12–128 byte): ' STAFF_BOOTSTRAP_PASSWORD
printf '\n'
printf '%s\n' "$STAFF_BOOTSTRAP_PASSWORD" | docker compose run --rm -T --no-deps --entrypoint /app/ticket-staff api bootstrap-admin --name 'Admin Event' --email admin@example.com
unset STAFF_BOOTSTRAP_PASSWORD
```

Jika command mengembalikan `invalid staff input`, periksa nama, format email, dan panjang password. Akun petugas STAFF dibuat dari halaman `/admin/staff`; bootstrap hanya untuk admin pertama dan gagal jika admin sudah ada. Buka `http://localhost:5173/admin/login` (atau `/admin/login` pada host Anda). Admin masuk ke `/admin/events`, sedangkan akun STAFF masuk ke `/admin/scan`.

Di `/admin/events`, admin dapat mengelola informasi konser, zona, kategori tiket, harga, kapasitas, batas pembelian, benefit, dan gate. Draft boleh belum lengkap; publikasi membutuhkan jadwal, lokasi, poster, deskripsi, lineup, zona, dan kategori. Katalog publik hanya memuat konser `PUBLISHED`; reservasi baru ditolak untuk konser tersembunyi. Perubahan kapasitas menjaga stok tiket yang sudah terjual atau tertahan, dan gate kategori terkunci setelah reservasi pertama. Harga yang tersimpan pada reservasi/order dan snapshot e-ticket yang sudah diterbitkan tetap berlaku setelah data konser atau kategori diperbarui. Jadwal dan lokasi terkunci sejak reservasi pertama, sedangkan checkout dari reservasi tersimpan tetap dapat dilanjutkan setelah konser diarsipkan. Endpoint pengelolaan memerlukan sesi ADMIN.

Di `/admin/staff`, admin dapat membuat petugas, mengubah nama dan status aktif, mengganti password, serta menetapkan event dan gate. Pilihan gate bersumber dari tier tiket event. Menonaktifkan akun atau mengganti password mencabut semua sesi petugas terkait. Sesi berlangsung 8 jam dan profil/penugasan dibaca lewat `GET /api/v1/staff/me`. Tombol Keluar memanggil `POST /api/v1/staff/logout` dan menghapus token petugas tab ini. Jika jaringan gagal, token lokal tetap dibersihkan dan halaman menyatakan bahwa pencabutan sesi di server belum dapat dipastikan.

Halaman `/admin/check-ins` menampilkan riwayat hasil final request check-in STAFF, dengan filter event/gate, pencarian potongan kode, waktu check-in, dan nama petugas. Endpoint `GET /api/v1/admin/check-ins` hanya untuk ADMIN dan memuat maksimal 50 hasil per halaman. Migrasi mengisi riwayat keberhasilan lama; percobaan terdahulu yang gagal tidak memiliki catatan. Kode input tidak valid disimpan tanpa payload mentah atau data pribadi. Request tanpa sesi STAFF valid dan kegagalan teknis yang belum memiliki hasil final tidak dicatat.

STAFF memilih penugasan event/gate, lalu scan QR e-ticket dengan kamera atau masukkan kode individual (`ET-` diikuti ID 32 digit heksadesimal). Browser akan meminta izin kamera; browser/perangkat tanpa dukungan kamera dapat memakai input manual atau scanner keyboard. Check-in dikirim ke `POST /api/v1/staff/check-ins`. Sesudah percobaan, kamera dan input manual terkunci sampai petugas memilih “Scan berikutnya”. Server hanya menerima order berstatus PAID dan gate yang sesuai snapshot tiket. Hasil menampilkan tiket berhasil, telah dipakai (`TICKET_ALREADY_USED`), gate salah (`WRONG_GATE`, beserta gate yang benar), tiket tidak ditemukan, order belum dibayar, atau akses ditolak.

Jika koneksi putus setelah pengiriman, halaman menampilkan “Hasil belum diketahui” dan menahan gate. Tombol “Periksa status” memanggil `GET /api/v1/staff/ticket-status?code=ET-…` tanpa melakukan check-in. STAFF hanya dapat membaca tiket pada event/gate penugasannya; ADMIN dapat membaca semua tiket. Jika tiket tercatat masuk, respons menampilkan waktu check-in. “Belum tercatat saat diperiksa” tidak membuktikan permintaan sebelumnya gagal; tahan gate, minta admin memeriksa tiket, lalu konfirmasi penanganan sebelum memilih scan berikutnya. Tidak ada retry check-in otomatis. Penghitung scanner hanya mencatat aktivitas sesi petugas di tab itu, bukan total pengunjung.

Di production, layani API melalui HTTPS agar password, token petugas, dan akses kamera terlindungi.

Contoh body checkout (jumlah nama harus sama dengan jumlah tiket reservasi pada setiap tier):

```json
{
  "buyer": {"name": "Nama Pembeli", "email": "buyer@example.com", "phone": "081234567890", "identity": "123456789012"},
  "attendees": [{"tierId": "festival", "names": ["Nama Peserta Satu", "Nama Peserta Dua"]}]
}
```

## Menjalankan Frontend Terpisah

```bash
cd frontend
bun install
bun run dev
```

Vite mem-proxy `/api` ke `http://localhost:8080`. Untuk target lain:

```bash
VITE_API_PROXY_TARGET=http://api:8080 VITE_ENABLE_PAYMENT_SIMULATION=true bun run dev -- --host 0.0.0.0
```

## Verification

Backend:

```bash
cd backend
go test ./...
go vet ./...
go build ./cmd/api ./cmd/migrate ./cmd/staff
```

Tes integrasi checkout, pembayaran, expiry, sesi petugas, check-in/riwayat, serta email dan pemulihan tiket (SMTP gagal sampai batas tiga percobaan, webhook Midtrans berulang, token kedaluwarsa/dipakai ulang, dan pemulihan dari perangkat tanpa akses tersimpan; SMTP disimulasikan server lokal di dalam tes) memakai database MySQL sementara melalui `MYSQL_TEST_DSN`; tanpa variabel tersebut, tes integrasi dilewati. Gunakan database tes yang dapat dibuang, lalu jalankan dari `backend/`:

```bash
MYSQL_TEST_DSN="$MYSQL_DSN" go test -v ./...
```

Tes interupsi migration 014 memakai `MYSQL_MIGRATION_TEST_DSN` terpisah dengan izin membuat database sementara. Tes membuat dan menghapus database miliknya sendiri pada setiap batas DDL, serta memeriksa job lama, schema akhir, dan checksum; CI menjalankannya otomatis.

CI menjalankan perintah yang sama terhadap service MySQL job, dengan secret order sementara yang dibuat per job.

Frontend:

```bash
cd frontend
bun test
bun run check
bun run build
```

Docker:

```bash
docker build -f backend/Dockerfile -t ticket-online-api:local .
docker build -f frontend/Dockerfile -t ticket-online-frontend:local frontend
docker compose config
```

## Reset Database Development


```bash
docker compose down -v
docker compose up --build
```

## Staging, Monitoring, and Restore Drill

Staging runs on a VPS with Docker Compose, Caddy-issued HTTPS, a dedicated MySQL volume, Midtrans sandbox keys, and an isolated SMTP test account. Keep production credentials and customer data out of this environment. Copy `.env.staging.example` to the ignored `.env.staging`, replace every placeholder, and create `.secrets/mysql-backup.cnf` with mode `0600`:

```ini
[client]
user=ticket_backup
password=THE_STAGING_DB_BACKUP_PASSWORD
host=127.0.0.1
```

On first database initialization, Compose creates the read-only `ticket_backup` account using `STAGING_DB_BACKUP_PASSWORD`. Create the account before the first backup if the existing DB volume predates this setup. Use unique random passwords, a base64 encoding of 32 random bytes for `ORDER_ACCESS_SECRET`, and a real domain whose DNS points to the VPS. Allow inbound TCP 80/443 and outbound access to Midtrans sandbox, the SMTP sandbox, and the backup bucket. Configure the S3-compatible bucket with versioning and a 14-day expiration rule scoped to the full staging backup prefix; `deploy/staging-s3-lifecycle.json` is a template. The bucket credentials must only permit that prefix. Store the age private identity separately from the VPS and bucket.

Commit the release before building: the image script requires a clean worktree and embeds the commit revision. Transfer the image artifact, then load it on the VPS. Run commands from the repository root; `AWS_PROFILE`, `S3_URI`, and `AGE_RECIPIENT` are shell variables used by the scripts, so export them in the operator shell (the example file is for Compose interpolation):

```bash
./scripts/staging-image.sh
# transfer out/ticket-online-api-<commit>.tar to the VPS
docker image load -i out/ticket-online-api-<commit>.tar
set -a; source .env.staging; set +a
export AWS_PROFILE S3_URI AGE_RECIPIENT
export STAGING_IMAGE="ticket-online-api:$(git rev-parse HEAD)"
./scripts/staging-deploy.sh
```

Set up a host firewall and Docker, point the staging DNS record to the VPS, then run the deploy script. It validates Compose configuration, starts/waits for MySQL, creates an encrypted backup before an existing API is stopped, applies migrations, starts API and HTTPS, and checks `/api/v1/ready`. First install has no prior application data to back up. Caddy obtains and renews the HTTPS certificate. Retain the previous image tag until the new release passes the staging flow.

The ADMIN page `/admin/operations` reports recent API 5xx responses, failed and delayed ticket email, open payment reconciliation cases, and worker heartbeat/failure counts. Database-backed metrics refresh every minute; stale data returns `503`. API request errors, worker failures, and changes in alert state are also written to structured container logs. Configure host-level log collection/retention and an external uptime check for `/api/v1/ready`; this repository does not provision a hosted monitoring service or page an operator.

The ADMIN page `/admin/reports` summarizes successful payments by `paid_at` and successful refunds by `completed_at`. The API `GET /api/v1/admin/reports/sales` accepts `eventId`, `dateFrom`, and `dateTo`; dates are inclusive Jakarta calendar days, default to the last 30 days, and allow up to 366 days. Pending refunds and open reconciliation cases describe the database snapshot time, independent of the requested dates.

Run the complete staging acceptance flow using a sandbox buyer and mailbox: publish a test concert, reserve and check out, complete a Midtrans sandbox payment, confirm the ticket email and ticket recovery link, open the ticket, then check it in with the assigned scanner. Also run the MySQL integration suite against a disposable test database to verify last-stock contention, payment versus expiry, repeated webhooks, and simultaneous scans. Keep evidence and timestamps in the deployment record. Do not use live customer or payment data.

For a backup, install AWS CLI, `age`, Docker Compose, and `gzip`; configure `AWS_PROFILE` for the dedicated bucket and provide `S3_URI` plus `AGE_RECIPIENT`. The script stores an encrypted SQL dump, checksum, image digest, source revision, and schema version. It uses the read-only MySQL account and bucket-side encryption. The age recipient encrypts the dump before upload. Uploads are retained for 14 days by the bucket lifecycle rule. Current migrations contain no routines, events, or triggers, so the dump explicitly skips those objects to keep the backup account read-only; if a migration adds one, update its backup flags/grants and test a full dump/restore round trip together.

To practice restore, copy `.env.staging.restore.example` to `.env.staging.restore`, replace its passwords, and create `.secrets/mysql-restore.cnf` mode `0600` with client credentials for `ticket_restore` on `127.0.0.1`. Set shell variables `RESTORE_PROJECT_NAME` (a fresh name beginning `ticket-online-staging-restore-`), `S3_URI`, `AWS_PROFILE`, and `AGE_IDENTITY_FILE` (path to the off-host private age identity); optionally set `S3_ENDPOINT_URL`. Then pass the backup's timestamped name without extension:

```bash
./scripts/staging-restore.sh ticket-online-staging-YYYYMMDDTHHMMSSZ
```

The restore script verifies SHA-256 before decryption and imports into a new Compose project and volume, isolated from the staging DB; its only published database port is `127.0.0.1:13307`. Inspect migration version, table counts, order/ticket relations, and sample ticket snapshots through the isolated container. Record elapsed time and compare to the 60-minute restore target, then remove the restore project and volume after evidence is captured. Never point the restore script at the live staging project or volume.

The staging target is RPO 24 hours with a restore exercise under 60 minutes. Schedule backups at least daily outside the repository, monitor their success, and keep the decryption key off-host. Actual HTTPS issuance, provider sandbox credentials, SMTP delivery, backup bucket policy, and VPS restore acceptance must be verified in the provisioned environment; local repo checks do not establish those external facts.

## Production Notes

- Production image backend menyajikan API dan SPA fallback dari `STATIC_DIR`.
- Unknown browser routes dikembalikan ke `index.html`; unknown `/api/*` routes tetap JSON `404`.
- Final backend image berjalan sebagai non-root user.
- Gunakan password, DSN, dan secret berbeda untuk production melalui secret manager atau environment deployment.
- Jangan commit `.env`, password production, generated binary, atau database volume.
- QR e-ticket berisi kode `ET-…` saja. Midtrans mendukung sandbox dan konfigurasi production; transaksi production dinonaktifkan secara default. Simulasi hanya pada development. Pengiriman email SMTP dan pemulihan tiket lintas perangkat tersedia; refund dan login pembeli belum tersedia.
