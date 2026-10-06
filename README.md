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

Pembeli yang berganti browser dapat membuka `/pulihkan-tiket`, mengisi email dan reference pesanan, lalu menerima tautan pemulihan di email pembeli yang tersimpan. Tautan berlaku 15 menit, dipakai sekali setelah tombol **Pulihkan tiket** ditekan, dan menyimpan akses order di browser tersebut. Respons `POST /api/v1/ticket-recovery` selalu sama untuk data yang cocok maupun tidak; pencocokan dilakukan worker dan hanya pesanan `PAID` dengan tiket lengkap serta akses aktif yang dikirimi email. Database hanya menyimpan hash SHA-256 token. Halaman `/pesanan/{id}` menampilkan ringkasan, tautan tiket, dan tombol **Kirim ulang email** (`POST /api/v1/orders/{orderID}/resend-email`, satu kali per menit per order). Batas permintaan disimpan di tabel `rate_limits`, sehingga tetap berlaku setelah restart. Migration 014 mengubah primary key `email_queue`; hentikan API lama sebelum menjalankan versi ini agar worker lama tidak ikut memproses antrean.

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
| `MIDTRANS_SERVER_KEY` | kosong | Server key Midtrans sandbox; mengaktifkan Snap dan webhook bertanda tangan |
| `FRONTEND_URL` | `http://localhost:5173` | Origin frontend untuk validasi URL kembali Snap |
| `SMTP_HOST` | kosong | Host server email; kosong menonaktifkan pengiriman email |
| `SMTP_PORT` | `587` | Port SMTP |
| `SMTP_USERNAME` | kosong | Username SMTP; harus diisi bersama password |
| `SMTP_PASSWORD` | kosong | Password SMTP |
| `SMTP_FROM` | kosong | Alamat pengirim; wajib saat `SMTP_HOST` diisi |
| `SMTP_TLS_MODE` | `starttls` | `starttls` atau `tls`; `none` hanya untuk development tanpa kredensial |
| `STATIC_DIR` | kosong | Direktori frontend production |
| `RESERVATION_TTL` | `10m` | Lama reservation |
| `EXPIRY_INTERVAL` | `15s` | Interval expiry worker |

`go run ./cmd/api` juga menjalankan migration sebelum menerima traffic. `go run ./cmd/migrate` aman dijalankan berulang kali. Backend terpisah juga memerlukan `ORDER_ACCESS_SECRET` yang sama setiap kali proses API dijalankan.

Untuk mengaktifkan email, set `SMTP_HOST`, `SMTP_FROM`, dan kredensial yang diberikan server SMTP. `SMTP_USERNAME` dan `SMTP_PASSWORD` harus hadir bersama. Set `FRONTEND_URL` ke origin aplikasi ber-HTTPS sebelum mengaktifkan SMTP pada production. Compose meneruskan variabel ini ke API; simpan password melalui secret environment deployment, bukan Git.

Untuk sandbox, isi `MIDTRANS_SERVER_KEY` dengan Server Key sandbox dari dashboard Midtrans dan set `FRONTEND_URL` ke origin aplikasi, lalu jalankan Compose. Daftarkan notifikasi Midtrans ke `POST /api/v1/payments/midtrans/notification`. Checkout meminta sesi Snap melalui `POST /api/v1/orders/{orderID}/payments`, lalu mengarahkan pembeli ke halaman pembayaran Midtrans. Order hanya ditandai lunas dan e-ticket diterbitkan setelah notifikasi bertanda tangan valid dengan nominal yang cocok; status order dapat dimuat ulang setelah pembeli kembali ke checkout. Worker memeriksa status Midtrans sebelum melepas stok order dengan sesi aktif; jika provider tidak dapat dihubungi, stok tetap ditahan sampai status dapat dipastikan. Settlement yang datang setelah stok telanjur dilepas dicatat sebagai kasus rekonsiliasi durable dan tidak menerbitkan tiket melebihi kapasitas. Snap sandbox memakai QRIS, transfer bank, dan GoPay. Jangan gunakan server key production untuk sandbox.

Endpoint simulasi lokal tetap tersedia hanya ketika `APP_ENV=development` dan dapat diaktifkan dengan `VITE_ENABLE_PAYMENT_SIMULATION=true`. Pembayaran simulasi memerlukan token privat order dari checkout. Detail order dan e-ticket tetap tersedia melalui `GET /api/v1/orders/{orderID}`, `GET /api/v1/orders/{orderID}/tickets`, dan `GET /api/v1/tickets/{ticketID}`.

## Akun Petugas

Buat admin pertama sekali setelah database berjalan. Jalankan dari root repo. Beri nama 2–80 karakter, email valid (misalnya `admin@example.com`), dan password 12–128 byte. Password dibaca tersembunyi lalu diteruskan melalui stdin, bukan argumen proses:

```bash
read -rsp 'Password admin (12–128 byte): ' STAFF_BOOTSTRAP_PASSWORD
printf '\n'
printf '%s\n' "$STAFF_BOOTSTRAP_PASSWORD" | docker compose run --rm -T --no-deps --entrypoint /app/ticket-staff api bootstrap-admin --name 'Admin Event' --email admin@example.com
unset STAFF_BOOTSTRAP_PASSWORD
```

Jika command mengembalikan `invalid staff input`, periksa nama, format email, dan panjang password. Akun petugas STAFF dibuat dari halaman `/admin/staff`; bootstrap hanya untuk admin pertama dan gagal jika admin sudah ada. Buka `http://localhost:5173/admin/login` (atau `/admin/login` pada host Anda). Admin masuk ke `/admin/staff`, sedangkan akun STAFF masuk ke `/admin/scan`.

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

Tes integrasi checkout, pembayaran, expiry, sesi petugas, dan check-in/riwayat memakai database MySQL sementara melalui `MYSQL_TEST_DSN`; tanpa variabel tersebut, tes integrasi dilewati. Gunakan database tes yang dapat dibuang, lalu jalankan dari `backend/`:

```bash
MYSQL_TEST_DSN="$MYSQL_DSN" go test -v ./...
```

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

## Production Notes

- Production image backend menyajikan API dan SPA fallback dari `STATIC_DIR`.
- Unknown browser routes dikembalikan ke `index.html`; unknown `/api/*` routes tetap JSON `404`.
- Final backend image berjalan sebagai non-root user.
- Gunakan password, DSN, dan secret berbeda untuk production melalui secret manager atau environment deployment.
- Jangan commit `.env`, password production, generated binary, atau database volume.
- QR e-ticket berisi kode `ET-…` saja; pembayaran memakai simulasi development. Payment gateway asli, pengiriman email, login pembeli, pemulihan lintas perangkat, dan refund belum tersedia.
