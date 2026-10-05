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

Frontend checkout, pembayaran simulasi, dan e-ticket membaca order serta snapshot tiket dari API. Setiap peserta menerima satu kode `ET-…` dengan data gate dan snapshot acara dari server. Tiket Saya memuat order yang aksesnya tersimpan pada browser yang sama; login pembeli, email, dan pemulihan lintas perangkat belum tersedia. Snapshot demo lama tetap tersimpan, tetapi tidak dianggap tiket backend.

Order `PENDING` memakai deadline reservasi, tersedia sebagai `expiresAt` pada respons checkout/detail order. Worker mengubah order yang lewat deadline menjadi `EXPIRED` dan mengembalikan stok tepat sekali; pembayaran terlambat ditolak meskipun worker belum berjalan. Deadline ini berbeda dari masa berlaku token akses.

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
| `STATIC_DIR` | kosong | Direktori frontend production |
| `RESERVATION_TTL` | `10m` | Lama reservation |
| `EXPIRY_INTERVAL` | `15s` | Interval expiry worker |

`go run ./cmd/api` juga menjalankan migration sebelum menerima traffic. `go run ./cmd/migrate` aman dijalankan berulang kali. Backend terpisah juga memerlukan `ORDER_ACCESS_SECRET` yang sama setiap kali proses API dijalankan.

Untuk memakai endpoint pembayaran simulasi di Docker Compose, aktifkan kedua batas development: `APP_ENV=development VITE_ENABLE_PAYMENT_SIMULATION=true docker compose up --build`. Frontend default menonaktifkan kontrol simulasi; backend hanya mendaftarkan endpoint pembayaran ketika `APP_ENV=development`. Endpoint `POST /api/v1/orders/{orderID}/simulate-payment` menerima `{"method":"QRIS","result":"SUCCEEDED"}`; metode yang didukung ialah `QRIS`, `VIRTUAL_ACCOUNT`, dan `GOPAY`, sedangkan hasil yang didukung ialah `SUCCEEDED` dan `FAILED`. Nominal dibaca server dari order. Pembayaran gagal dapat dicoba lagi; setelah gangguan jaringan frontend membaca status order sebelum retry. Setelah berhasil, pembayaran tidak dapat diubah dan retry mengembalikan e-ticket yang sama. Baca detail order lewat `GET /api/v1/orders/{orderID}`, daftar tiket lewat `GET /api/v1/orders/{orderID}/tickets`, atau satu tiket lewat `GET /api/v1/tickets/{ticketID}`. Ketiga endpoint tersebut dan pembayaran simulasi memerlukan token privat order dari checkout.

## Akun Petugas

Buat admin pertama sekali setelah database berjalan. Jalankan dari root repo. Beri nama 2–80 karakter, email valid (misalnya `admin@example.com`), dan password 12–128 byte. Password dibaca tersembunyi lalu diteruskan melalui stdin, bukan argumen proses:

```bash
read -rsp 'Password admin (12–128 byte): ' STAFF_BOOTSTRAP_PASSWORD
printf '\n'
printf '%s\n' "$STAFF_BOOTSTRAP_PASSWORD" | docker compose run --rm -T --no-deps --entrypoint /app/ticket-staff api bootstrap-admin --name 'Admin Event' --email admin@example.com
unset STAFF_BOOTSTRAP_PASSWORD
```

Jika command mengembalikan `invalid staff input`, periksa nama, format email, dan panjang password. Admin berikutnya dibuat dari halaman `/admin/staff`; bootstrap hanya untuk admin pertama dan gagal jika admin sudah ada. Buka `http://localhost:5173/admin/login` (atau `/admin/login` pada host Anda). Admin masuk ke `/admin/staff`, sedangkan akun STAFF masuk ke `/admin/scan`.

Di `/admin/staff`, admin dapat membuat petugas, mengubah nama dan status aktif, mengganti password, serta menetapkan event dan gate. Pilihan gate bersumber dari tier tiket event. Menonaktifkan akun atau mengganti password mencabut semua sesi petugas terkait. Sesi berlangsung 8 jam dan profil/penugasan dibaca lewat `GET /api/v1/staff/me`. Tombol Keluar memanggil `POST /api/v1/staff/logout` dan menghapus token petugas tab ini. Jika jaringan gagal, token lokal tetap dibersihkan dan halaman menyatakan bahwa pencabutan sesi di server belum dapat dipastikan.

STAFF memilih penugasan event/gate lalu memasukkan kode e-ticket individual (`ET-` diikuti ID 32 digit heksadesimal); scanner yang bertindak sebagai keyboard juga dapat digunakan. Check-in dikirim ke `POST /api/v1/staff/check-ins`. Server hanya menerima order berstatus PAID dan gate yang sesuai snapshot tiket. Hasil menampilkan tiket berhasil, telah dipakai (`TICKET_ALREADY_USED`), gate salah (`WRONG_GATE`, beserta gate yang benar), tiket tidak ditemukan, order belum dibayar, atau akses ditolak. Jika koneksi putus setelah pengiriman, tampil “Hasil belum diketahui”; gate harus tetap ditahan dan petugas tidak boleh retry otomatis. Minta admin memeriksa status tiket sebelum melakukan tindakan lanjutan. Penghitung scanner hanya mencatat aktivitas sesi petugas di tab itu, bukan total pengunjung.

Di production, layani API melalui HTTPS agar password dan token petugas terlindungi saat transit.

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

Tes integrasi checkout, pembayaran, expiry, dan sesi petugas memakai database MySQL sementara melalui `MYSQL_TEST_DSN`; tanpa variabel tersebut, tes integrasi dilewati. Gunakan database tes yang dapat dibuang, lalu jalankan dari `backend/`:

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
- Kode `ET-…` saat ini bukan QR yang dapat dipindai. Pembayaran memakai simulasi development; payment gateway asli, pengiriman email, login pembeli, pemulihan lintas perangkat, dan refund belum tersedia.
