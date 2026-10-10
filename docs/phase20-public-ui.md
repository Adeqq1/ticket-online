# Phase 20: Fondasi Visual Publik dan Pemetaan API

## Progress

- [x] Panduan singkat warna, latar, radius, jarak, tipografi, tombol, input, dan status tercatat di bawah.
- [x] Manrope variable disimpan di `frontend/public/fonts/Manrope-Variable.ttf` dengan lisensi OFL. Font publik memakai `font-display: swap`; logo mempertahankan Arial dan mark yang ada.
- [x] Fondasi tipografi dan token publik dicakup `.public-site`; panel admin tidak memakai kelas atau token publik.
- [x] Matriks komponen dan API client dipetakan ke operation OpenAPI yang ada.
- [x] Screenshot desktop dan mobile bagi semua halaman publik dan keadaan standar, loading, kosong, error, serta status order selesai direkam; periksa indeks dalam `phase20/screenshots/index.json`.

## Panduan visual

| Elemen | Fondasi publik |
|---|---|
| Aksen utama | Biru `#1358d8`; dark mode `#769dff`; logo dan wordmark tetap |
| Netral | Latar terang `#f7f8fa`, permukaan `#fff`, teks `#17211f`, garis `#d5dce4`; warna teks dan permukaan dark mode mempertahankan palet yang ada |
| Tipografi | Manrope variable 200–800; teks isi 16px dengan tinggi baris 1,5; Arial sebagai fallback dan font wordmark |
| Bentuk | Radius dasar 8px, dengan kontrol dan kartu memakai token yang sama |
| Jarak | Skala dasar 4, 8, 12, 16, 24, 32, 48, 64px; margin shell 16px per sisi pada mobile |
| Kontrol | Tombol utama minimal 48px; target kontrol lain minimal 44px; fokus keyboard terlihat |
| Status | Selalu memiliki teks selain warna. Error `#8f2f22`/`#ff9585`, sukses `#166534`/`#86efac`, peringatan `#854d0e`/`#fcd34d`, informasi `#1d4ed8`/`#93c5fd` untuk light/dark |
| Gerak | Transisi kontrol ringan; aturan `prefers-reduced-motion` global tetap berlaku |

Baseline visual awal tersimpan sebagai `baseline-*.png` di `docs/phase20/screenshots/`. Runner merekam viewport 1440×900 dan 390×844, respons API fiktif sesuai format kontrak sekarang, serta commit sumber. Style baseline dipertahankan saat pengambilan meski pembungkus publik dan aset font sudah tersedia. Foto dari URL Picsum yang telah ada di halaman memakai fixtures lokal di `docs/phase20/fixtures/` agar screenshot konsisten dan tidak bergantung pada jaringan.

## Matriks halaman ke kontrak API

Rujukan operationId adalah `docs/openapi.yaml`. Panggilan API mengikuti fungsi di `frontend/src/lib/api.ts`; pelacakan konversi melalui `recordConversion` adalah perilaku tambahan yang sudah tersedia.

| Halaman atau perilaku | Fungsi client | Path dan operationId |
|---|---|---|
| Beranda dan katalog | `getEvents` | `GET /api/v1/events` · `listEvents` |
| Detail acara dan data checkout | `getEvent` | `GET /api/v1/events/{eventID}` · `getEvent` |
| Pemulihan reservasi checkout | `getReservationEvent`, `getReservation` | `GET /api/v1/reservations/{reservationID}/event` · `getReservedEvent`; `GET /api/v1/reservations/{reservationID}` · `getReservation` |
| Menahan stok | `createReservation` | `POST /api/v1/reservations` · `createReservation` |
| Membuat order | `createOrder` | `POST /api/v1/reservations/{reservationID}/checkout` · `createOrder` |
| Status dan rincian pesanan | `getOrder` | `GET /api/v1/orders/{orderID}` · `getOrder` |
| Sesi pembayaran | `createSnapPayment` | `POST /api/v1/orders/{orderID}/payments` · `createSnapPayment` |
| Pembayaran simulasi yang sudah ada | `simulatePayment` | `POST /api/v1/orders/{orderID}/simulate-payment` · `simulatePayment` |
| Tiket dari order / Tiket Saya | `listOrderTickets` | `GET /api/v1/orders/{orderID}/tickets` · `listOrderTickets` |
| E-ticket | `getTicket` | `GET /api/v1/tickets/{ticketID}` · `getTicket` |
| Meminta dan memakai tautan pemulihan | `requestTicketRecovery`, `verifyTicketRecovery` | `POST /api/v1/ticket-recovery` · `requestTicketRecovery`; `POST /api/v1/ticket-recovery/verify` · `verifyTicketRecovery` |
| Kirim ulang email order | `resendOrderEmail` | `POST /api/v1/orders/{orderID}/resend-email` · `resendOrderEmail` |
| Mengajukan refund perubahan acara | `requestEventRefund` | `POST /api/v1/orders/{orderID}/refund-request` · `requestEventRefund` |

Endpoint pembayaran simulasi tidak berarti kontrak Snap dilewati. Pemakaian simulasi ditentukan konfigurasi pengembangan yang sudah ada.

## Baseline dan penerimaan

Jalankan server dari `frontend/` dengan `bun run dev --host 127.0.0.1`, lalu `python3 scripts/public-ui-check.py --current` untuk membandingkan UI baru. Script yang sama tanpa `--current` mengulang baseline awal. Script memakai Google Chrome dan Playwright Python yang sudah tersedia; API diintersep lokal dan tidak memerlukan backend atau data pelanggan.

Halaman standar yang diambil: beranda, katalog, detail konser, checkout, pesanan, Tiket Saya, e-ticket, pemulihan, panduan, dan tidak ditemukan. Keadaan tambahan meliputi loading, hasil kosong, error, tiga langkah checkout, perubahan jadwal, e-ticket nonaktif, tautan pemulihan siap, serta status order menunggu, dibatalkan, kedaluwarsa, refund tertunda, dan refund selesai. Baseline saat ini berisi 52 screenshot; indeks juga merekam lebar halaman pada 320px. Nomor pesanan panjang membungkus pada judul agar tidak meluap dari layar. Bandingkan screenshot terhadap baseline awal setelah mengubah UI.

Verifikasi perubahan: `bun test`, `bun run check`, dan `bun run build` dari `frontend/`. Pastikan `dist/_redirects` ada. Periksa kontras, navigasi keyboard, reduced motion, dark mode, tampilan cetak, dan bahwa admin tidak memakai font/token publik.
