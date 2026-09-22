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

Catalog event dan reservation menggunakan database. Payment serta e-ticket tetap simulasi browser sesuai scope issue.

## Requirements

- Go 1.26 atau lebih baru
- Bun
- Docker Engine dan Docker Compose

## Menjalankan Seluruh Stack

```bash
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
export MYSQL_DSN='ticket:ticket@tcp(127.0.0.1:3306)/ticket_online?parseTime=true&loc=UTC'
go run ./cmd/migrate
go run ./cmd/api
```

Environment variable backend:

| Variable | Default | Keterangan |
|---|---|---|
| `HTTP_ADDR` | `:8080` | Alamat HTTP API |
| `MYSQL_DSN` | wajib | DSN MySQL aplikasi |
| `STATIC_DIR` | kosong | Direktori frontend production |
| `RESERVATION_TTL` | `10m` | Lama reservation |
| `EXPIRY_INTERVAL` | `15s` | Interval expiry worker |

`go run ./cmd/api` juga menjalankan migration sebelum menerima traffic. `go run ./cmd/migrate` aman dijalankan berulang kali.

## Menjalankan Frontend Terpisah

```bash
cd frontend
bun install
bun run dev
```

Vite mem-proxy `/api` ke `http://localhost:8080`. Untuk target lain:

```bash
VITE_API_PROXY_TARGET=http://api:8080 bun run dev -- --host 0.0.0.0
```

## Verification

Backend:

```bash
cd backend
go test ./...
go vet ./...
go build ./cmd/api ./cmd/migrate
```

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
- Payment, order permanen, penerbitan tiket, QR code, email, login, dan refund belum menjadi fitur production.
