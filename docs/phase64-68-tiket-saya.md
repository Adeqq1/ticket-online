# Phase 64-68: Tiket Saya yang Mudah Dikenali

Implementasi Phase 64-67 selesai. Phase 68 memiliki bukti otomatis dan screenshot terpisah; checkpoint akhir masih terbuka untuk transaksi Midtrans sandbox nyata dan UAT dua pembeli. Pengguna menyatakan staging sandbox belum tersedia dan UAT akan dicoba nanti.

## Perubahan

- Detail pesanan menambahkan `eventId`, `eventArtist`, `eventVenue`, dan `eventCity` melalui relasi pesanan → reservasi → konser. Endpoint, migrasi, autentikasi, dan filter katalog publik tidak ditambahkan atau diubah.
- Satu kartu per pesanan, dengan konser, `currentEvent.startsAt`, venue/kota, total `items.quantity`, kelayakan tiket, dan nomor pesanan. Respons lama tidak ditebak dari katalog atau e-ticket.
- Default selalu Tiket aktif. Menunggu pembayaran dan Riwayat memiliki jumlah pesanan sendiri. Kegagalan akses/pemeriksaan tiket lunas berada di Perlu diperiksa di luar kategori. Jam mulai yang lewat tidak menentukan kelayakan.
- Pemilih peserta hanya memuat e-ticket dengan `usable: true`. Pembayaran diperiksa ulang sebelum membuka bagian pembayaran dan sebelum POST. POST hanya memakai endpoint pembayaran pesanan yang sama, termasuk akses tanpa metadata checkout. Sesi pembayaran PENDING dapat dibuka kembali memakai metode yang sama; pembayaran SUCCEEDED yang menunggu konfirmasi hanya diperiksa statusnya.
- Backend menerima URL kembali `/pesanan/{id}` hanya untuk pesanan yang sedang dibayar pada origin yang dikonfigurasi, tanpa query atau fragment. URL checkout yang sudah ada tetap didukung. Respons pengalihan yang tidak valid menjaga hasil pembayaran sebagai belum pasti.
- Pesan publik, tautan pemulihan, perbedaan nomor pesanan/kode e-ticket, dan pemberitahuan penyimpanan gagal diperjelas. Pesan demo dihapus dari halaman. Format akses tersimpan serta aturan refund/check-in tetap sama.
- Panel kartu memakai label status untuk pesanan kedaluwarsa, dibatalkan, dan pending; label E-ticket hanya muncul jika tiket telah dimuat dari server. Panel selain tiket aktif memakai warna netral yang sudah tersedia.
- Input pemulihan sejajar, tinggi 48px, dengan petunjuk di bawah setiap kolom dan tanpa ruang pesan kesalahan kosong. Penjelasan pemulihan akses tetap terlihat pada formulir, konfirmasi, dan tautan pemulihan: pemulihan tidak mengaktifkan kembali pesanan kedaluwarsa.

## Tes otomatis

| Pemeriksaan | Hasil |
| --- | --- |
| `bun test` | 143 lulus, 0 gagal, 691 assertion; 32 berkas. |
| `bun run check` | 0 error, 0 warning. |
| `bun run build` | Lulus; output `frontend/dist/`. |
| Backend checkout, platform, payment, orderaccess, recovery | Lulus dengan `MYSQL_TEST_DSN` pada database pengujian lokal. |
| Kontrak identitas dan akses pesanan | PENDING, EXPIRED, CANCELLED, PAID; konser diarsipkan; jadwal terbaru; penundaan tanpa jadwal; akses tanpa token/salah token tetap ditolak. |
| Pemulihan email backend | Lulus dengan SMTP uji lokal, tautan sekali pakai, pengiriman ulang, pembatasan permintaan, dan sesi akses hasil pemulihan. |
| Validasi URL kembali pembayaran | Lulus untuk checkout lama dan pesanan yang sama; origin lain, ID pesanan lain, credential, query/fragment pesanan, serta path tambahan ditolak. |
| Integrasi eventchange | 7 tes induk lulus, termasuk penundaan/jadwal baru, refund, check-in, dan race settlement; menggunakan `MYSQL_MIGRATION_TEST_DSN` dengan database sementara. |
| Browser Chrome, build produksi dan Docker lokal | Matriks 320, 390, 768, 1440px; [automated.json](phase64-68/automated.json) mencatat hasil dan URL pemeriksaan terakhir. |

Tes browser memakai respons API eksplisit, bukan transaksi Midtrans nyata. Cakupannya mencakup pesanan campuran, sebagian tiket digunakan, jumlah tiket tidak lengkap, kelayakan tidak diketahui, kegagalan satu pesanan, token berakhir, respons lama, koneksi terputus, penyimpanan ditolak, perubahan kategori, keyboard kategori, mode gelap, pemilihan peserta, klik ganda, hasil pembayaran belum pasti, tenggat lewat dengan status PENDING, serta pemulihan pada sesi terpisah tanpa sinkronisasi otomatis.

Pemeriksaan tambahan memverifikasi label panel sesuai status, kesamaan tinggi input, posisi input desktop, petunjuk di bawah input, jarak 12px sebelum tombol ketika tidak ada kesalahan, serta mode gelap formulir. Fixture pemulihan pesanan EXPIRED menggunakan akses yang masih berlaku setelah perubahan acara (`changedEvent: true`), sesuai kontrak pemulihan existing untuk pesanan tanpa e-ticket. Pada sesi terpisah, hasilnya harus tetap menampilkan Pesanan kedaluwarsa tanpa aksi membuka e-ticket atau melanjutkan pembayaran. Kelayakan pemulihan tidak diperluas.

Jalankan dari `frontend/`:

```sh
bun test
bun run check
bun run build
bun run preview -- --host 127.0.0.1 --port 5176 --strictPort
# Pada terminal lain, memakai Python Playwright dan Chrome yang sudah tersedia:
python3 scripts/buyer-tickets-ui-check.py http://127.0.0.1:5176
```

Tes Go dijalankan dari `backend/` menggunakan database pengujian, bukan database produksi. Paket eventchange membutuhkan `MYSQL_MIGRATION_TEST_DSN`; memberi `MYSQL_TEST_DSN` saja akan melewati tes integrasinya.

## Screenshot

Semua screenshot menggunakan data uji. Screenshot merupakan bukti tampilan, bukan bukti UAT manusia atau pembayaran nyata. Setiap skenario tersedia pada keempat viewport dalam [phase64-68/](phase64-68/).

| Tampilan | Contoh |
| --- | --- |
| Tiket aktif | [1440px](phase64-68/active-1440.png), [320px](phase64-68/active-320.png) |
| Mode gelap | [320px](phase64-68/active-dark-320.png) |
| Menunggu pembayaran | [390px](phase64-68/pending-390.png) |
| Riwayat | [390px](phase64-68/history-390.png) |
| Pembayaran belum pasti | [390px](phase64-68/payment-uncertain-390.png) |
| Daftar kosong | [320px](phase64-68/empty-320.png) |
| Penyimpanan ditolak | [320px](phase64-68/storage-denied-320.png) |
| Koneksi terputus | [320px](phase64-68/offline-320.png) |
| Pemulihan dan akses belum tersimpan | [form 390px](phase64-68/recovery-390.png), [hasil 390px](phase64-68/recovery-unsaved-390.png) |
| Pemulihan mode gelap | [390px](phase64-68/recovery-dark-390.png), [1440px](phase64-68/recovery-dark-1440.png) |

## Bantuan tanpa nomor pesanan yang masih terbuka

Email dukungan yang aktif dan dipantau tim belum tersedia. Sesuai pilihan pengguna, tautan “Tidak menemukan nomor pesanan?” menunggu kontak aktif; jangan mengarahkan ke alamat fiktif atau panduan umum sebagai pengganti bantuan manual. Setelah kontak tersedia, tautan harus membuka email dukungan tanpa mewajibkan nomor pesanan. Pembeli dapat menyertakan email pembelian, konser, dan waktu transaksi. Kebutuhan bantuan nyata ini belum selesai dan perlu ditutup sebelum rilis publik alur tersebut.

## Verifikasi sandbox nyata yang masih terbuka

Staging sandbox belum tersedia. Setelah dikonfigurasi, gunakan pesanan PENDING yang sama dan akses hasil email/pemulihan tanpa `basketKey`/`idempotencyKey`. Pilih metode, selesaikan pembayaran sandbox, dan catat bahwa ID pesanan tidak berubah, notifikasi valid mengubahnya menjadi PAID, serta Tiket aktif muncul setelah konfirmasi. Ulangi dengan tenggat lewat, hasil terputus, tiket yang telah digunakan, dan sesi lain yang hanya memperoleh akses melalui email. Catat waktu, ID pesanan yang sudah disamarkan, respons status, serta bukti provider/worker; jangan merekam token atau secret.

## UAT manusia yang masih terbuka

Minta dua pembeli yang belum mengetahui antarmuka. Bacakan tujuan berikut tanpa memberi label tombol, lokasi klik, atau petunjuk navigasi. Untuk setiap tugas, catat waktu, keberhasilan tanpa bantuan, kesalahan pemahaman, dan bantuan yang diberikan.

1. Temukan pesanan untuk konser yang ditentukan fasilitator. Jelaskan jadwal, venue/kota, jumlah tiket yang dipesan, dan berapa tiket yang dapat digunakan.
2. Buka tiket peserta tertentu dari pesanan dengan beberapa peserta. Jelaskan nama peserta dan kategori pada tiket yang dibuka.
3. Temukan pesanan yang kedaluwarsa. Jelaskan mengapa tiket belum tersedia, bedakan dengan akses pesanan yang berakhir, dan jelaskan bahwa pemulihan tidak mengaktifkan kembali pesanan kedaluwarsa.
4. Buka tiket di sesi/perangkat lain memakai email konfirmasi atau pemulihan. Jelaskan bahwa daftar tidak tersinkron otomatis.

| Peserta | Perangkat | Tugas | Berhasil tanpa bantuan | Waktu | Kesalahan pemahaman / bantuan |
| --- | --- | --- | --- | --- | --- |
| P1 | Belum tersedia | 1-4 belum dijalankan | Belum dinilai | Belum dicatat | Belum dicatat |
| P2 | Belum tersedia | 1-4 belum dijalankan | Belum dinilai | Belum dicatat | Belum dicatat |

Checkpoint akhir hanya ditutup setelah kedua pembeli dapat menjelaskan konser, kelayakan tiket, dan tindakan berikutnya dengan benar. Hasil agen dan screenshot tidak mengisi tabel peserta.

## Urutan rilis

Rilis backend terlebih dahulu. Verifikasi empat field identitas pada pesanan berizin, akses konser diarsipkan, serta URL kembali pembayaran untuk pesanan yang sama. Setelah backend terverifikasi, rilis `frontend/dist/` beserta `_redirects` dan SPA fallback. Docker lokal telah diperbarui berurutan: API pada port 8080, lalu frontend pada port 5173; pemeriksaan kesiapan API melalui kedua port berhasil. Belum ada deployment staging/produksi yang dilakukan dalam pekerjaan ini.
