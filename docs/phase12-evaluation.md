# Phase 12 — Pilih berdasarkan hasil

Status: **bukti belum cukup; fitur belum dipilih**. Lingkungan evaluasi saat ini lokal/staging. Persiapan pengukuran tidak berarti sesi pengguna sudah dijalankan atau hasil produksi sudah tersedia.

Contoh latihan tersedia pada [laporan simulasi Phase 12-15](phase12-15-simulation.md). Seluruh datanya sintetis dan tidak memenuhi checkpoint bukti nyata di dokumen ini.

Pelaksanaan lima sesi berikutnya memakai [checkpoint Phase 13](phase13-user-test.md), dengan catatan per tugas dan rekap frekuensi/dampak hambatan. Hasil sesi belum tersedia; ringkas bukti yang sudah tercatat di sana ke lembar evaluasi ini setelah pelaksanaan.

## Pengumpulan bukti

1. Tentukan lingkungan, event yang diamati, dan periode. Bawaan: 30 hari penuh terakhir dalam WIB, awal inklusif dan akhir eksklusif. Pisahkan sandbox dan produksi; pembayaran simulasi bukan bukti pembelian berulang sungguhan.
2. Jalankan [query agregat](../scripts/phase12-evaluation.sql) memakai akun database dengan hak `SELECT` pada `payments`, `order_buyers`, dan `payment_environment`. Jalankan dari root repo dengan konfigurasi klien yang tersimpan aman, tanpa password pada command line:

   ```sh
   mysql --defaults-extra-file=/path/to/read-only.cnf --database=ticket_online --batch < scripts/phase12-evaluation.sql
   ```

   Untuk periode tertentu, awali sesi klien yang sama dengan contoh berikut, lalu jalankan file memakai `source`:

   ```sql
   SET @phase12_date_from = '2026-09-01';
   SET @phase12_date_to_exclusive = '2026-10-01';
   source scripts/phase12-evaluation.sql
   ```

   Periode harus berformat `YYYY-MM-DD`, memiliki 1–366 hari, dan berakhir paling lambat awal hari ini WIB. Hasil `INVALID_PERIOD` memiliki metrik `NULL`; tanggal kalender yang tidak valid juga dapat ditolak MySQL. Koreksi input dan ulangi, jangan menafsirkan hasil tersebut sebagai nol. Reset kedua variabel ke `NULL` untuk kembali ke periode bawaan.
3. Buka `/admin/reports/conversion` sebagai ADMIN. Untuk rentang SQL `[2026-09-01, 2026-10-01)`, gunakan tanggal dashboard `2026-09-01` sampai `2026-09-30` (inklusif). Catat perjalanan matang, konversi tiap tahap, kehilangan terbesar, perangkat, event, perjalanan belum matang, transaksi tanpa atribusi, dan waktu snapshot. Cocokkan pembayaran dengan `/admin/reports`; snapshot berbeda dapat menghasilkan angka berbeda.
4. Jalankan lima sesi pengguna staging: tiga mobile dan dua desktop, dengan data uji dan pembayaran sandbox. Ikuti tugas pada tabel di bawah tanpa memberi petunjuk sampai peserta meminta bantuan. Pakai alias sesi, bukan nama, email, atau reference order. Catat hasil observasi saja; jangan menyalin token, tautan privat, QR, atau screenshot yang berisi akses tiket.
5. Jika tim memiliki kanal dukungan, rangkum kasus pada periode yang sama. Satu percakapan mengenai masalah pesanan yang sama dihitung sebagai satu kasus meskipun ada beberapa pesan. Beri satu kategori utama: pembayaran, email/pemulihan tiket, akses tiket, jadwal/refund, check-in, atau lainnya. Jika kanal/rekap belum ada, tulis **belum tersedia**, bukan nol kasus.

## Definisi dan lembar hasil

- **Order dibayar:** satu order dengan `payments.status='SUCCEEDED'` dan `paid_at` di dalam periode. Pembayaran gagal/pending tidak dihitung; percobaan, item, dan jumlah tiket tidak menambah hitungan order.
- **Pembeli unik:** kelompok `LOWER(TRIM(email))` dengan setidaknya satu order dibayar dalam periode. Perbandingan memakai byte hasil normalisasi, sehingga karakter beraksen tidak disamakan dengan karakter tanpa aksen. Email hanya dipakai di dalam query; keluaran tidak memuat email atau hash email.
- **Pembeli berulang:** kelompok tersebut memiliki minimal dua order dibayar sepanjang riwayat yang tersedia sebelum akhir periode. Pembayaran kedua boleh terjadi dalam periode; order pertama saja tidak cukup. Pembayaran setelah akhir periode tidak memengaruhi hasil.
- **Proporsi berulang:** pembeli berulang dibagi pembeli unik. Tanpa pembeli, nilainya `NULL`, bukan 0%. `orders_without_buyer` menunjukkan pembayaran yang tidak dapat dikelompokkan karena data pembeli tidak tersedia.
- Riwayat pembayaran tetap dihitung setelah refund, termasuk settlement terlambat. Metrik ini mengukur riwayat pembayaran, bukan pendapatan bersih atau pembelian ulang yang berhasil dipakai. Periksa konteks pembatalan/refund pada laporan penjualan sebelum mengambil keputusan. Email bersama/berbeda dan riwayat yang terpotong membatasi ketepatan proksi pembeli.
- **Konversi:** gunakan perjalanan matang yang dideduplikasi per perjalanan/event dengan jendela observasi 24 jam sesuai dashboard. Rasio antartahap memakai tahap sebelumnya; rasio akhir memakai pembayaran berhasil dibagi detail konser. Perjalanan per tab bukan pengunjung unik. Cohort konversi berdasarkan awal perjalanan, sedangkan SQL pembayaran berdasarkan `paid_at`; kedua angka tidak harus sama.

| Metadata/meter | Hasil |
|---|---|
| Lingkungan dan jenis data (uji/produksi) | Belum dikumpulkan |
| Event, periode WIB, dan cakupan riwayat pembayaran | Belum ditetapkan |
| Snapshot SQL / dashboard (sertakan zona waktu) | Belum dikumpulkan |
| Order dibayar / tanpa data pembeli | Belum dikumpulkan |
| Pembeli unik / berulang / proporsi berulang | Belum dikumpulkan |
| Perjalanan matang / belum matang / tanpa atribusi | Belum dikumpulkan |
| Konversi akhir dan antartahap, per event/perangkat | Belum dikumpulkan |
| Kehilangan terbesar dan hambatan tercatat | Belum dikumpulkan |
| Refund / perubahan acara yang memengaruhi interpretasi | Belum dikumpulkan |
| Kasus dukungan total dan per kategori | Belum tersedia |
| Sasaran konversi dan jangkauan, sumber pengukuran | Belum ditetapkan |
| Jangkauan aktual pada periode yang sama | Belum tersedia |

Tetapkan sasaran konversi dan jangkauan sebelum menilai hasil produksi. Jangkauan membutuhkan sumber pengukuran yang disebutkan tim; jumlah perjalanan per tab tidak membuktikan jumlah orang yang dijangkau. Belum ada atribusi kampanye pada laporan ini.

| Sesi | Perangkat | Checkout tamu | Temukan tiket lama di Tiket Saya | Pulihkan akses di browser baru | Pahami pembayaran dan refund | Bantuan/hambatan | Waktu per tugas (detik) |
|---|---|---|---|---|---|---|---|
| U1 | Mobile | | | | | | |
| U2 | Mobile | | | | | | |
| U3 | Mobile | | | | | | |
| U4 | Desktop | | | | | | |
| U5 | Desktop | | | | | | |

Untuk tiket lama, gunakan order sandbox yang sudah lunas. Untuk pemulihan, gunakan browser tanpa akses order tersebut dan mailbox uji; ikuti `/pulihkan-tiket`. Tanyakan kapan pembayaran dianggap lunas, bagaimana tiket diperoleh, serta hak refund pada pembatalan/jadwal baru; bandingkan jawaban dengan status order dan [kebijakan perubahan acara](event-change-policy.md). Jika SMTP/provider belum tersedia, tandai tugas **tidak dapat diuji**, bukan gagal oleh pengguna. Isi setiap tugas dengan berhasil/gagal/tidak dapat diuji, waktu, dan bantuan yang dibutuhkan.

Lima sesi memberi bukti awal tentang hambatan dan kebutuhan akses/bantuan. Sesi ini tidak membuktikan retensi, jangkauan, atau kenaikan konversi produksi. Evaluasi ini juga tidak menggantikan perbandingan baseline/sesudah perubahan pada [uji konversi Phase 10](conversion-user-test.md).

## Pemilihan satu fitur

Keputusan dilanjutkan pada [Phase 15](phase15-development-decision.md) setelah putaran uji, perbaikan, dan uji ulang Phase 13-14 selesai. Jika hasilnya baik dan tidak ada kebutuhan baru yang kuat, cakupan saat ini dapat ditutup dengan alasan berbasis bukti.

| Kandidat | Bukti yang diperlukan | Kriteria hasil fitur berikutnya |
|---|---|---|
| Akun pembeli, checkout tamu tetap tersedia | Pembelian berulang pada data nyata dan kesulitan mengakses tiket/riwayat saat kembali | Pembeli lama lebih mudah memperoleh tiket/riwayat; checkout tamu tetap dapat diselesaikan |
| Voucher terkelola | Promosi menjadi kebutuhan rutin dan perubahan kode promo dalam kode aplikasi menghambat operasi | Pengelola mengatur promo tanpa perubahan kode, dengan waktu pengerjaan dan kesalahan aturan yang berkurang |
| Pusat bantuan terkait pesanan | Kategori pertanyaan pesanan yang berulang pada rekap dukungan; observasi pengguna mendukung kebutuhan tersebut | Pembeli menemukan jawaban yang sesuai status pesanannya dan kebutuhan bantuan berkurang |
| Pemilihan kursi | Penyelenggara mengonfirmasi kebutuhan tiket bernomor kursi pada acara nyata beserta denah dan aturan alokasinya | Pembeli memilih kursi sesuai denah, petugas mengenali penempatan, dan tidak ada alokasi ganda |

Jika beberapa kandidat didukung, bandingkan jumlah pengguna/kasus terdampak, masalah yang berulang, kualitas bukti, dan usaha implementasinya. Hindari menjumlahkan ukuran yang berbeda (perjalanan, kelompok email, dan kasus dukungan). Pilih satu dengan alasan tertulis; jika belum ada pembeda yang kuat, tetap **bukti belum cukup**. Hambatan checkout yang belum terselesaikan perlu ditangani sebelum menganggap masalah utama adalah jangkauan.

| Catatan keputusan | Isi |
|---|---|
| Status | Bukti belum cukup |
| Kandidat terpilih | Belum dipilih |
| Bukti dan periode yang mendukung | Belum dikumpulkan |
| Pengguna/kasus terdampak dan batas interpretasi | Belum dikumpulkan |
| Alasan mendahulukan kandidat ini | Belum ditetapkan |
| Ukuran keberhasilan dan baseline | Belum ditetapkan |
| Penanggung jawab dan tanggal tinjauan berikutnya | Belum ditetapkan |

## Verifikasi persiapan

Pemeriksaan integrasi query berada pada paket `backend/internal/adminreports`. Jalankan terhadap database MySQL sementara yang sudah disediakan khusus untuk tes:

```sh
cd backend
GOCACHE=/tmp/ticket-online-go-cache go test ./internal/adminreports -run '^TestPhase12EvaluationSQL$' -count=1 -v
```

`MYSQL_TEST_DSN` harus menunjuk database tes yang dapat dimigrasikan dan diisi fixture; jangan gunakan staging/produksi. Pemeriksaan mencakup periode kosong, normalisasi email, pembelian pertama/berulang, pembayaran gagal/pending, refund, batas WIB, periode tidak valid, serta transaksi baca-saja. Tanpa DSN, tes dilewati dan bukan bukti query sudah terverifikasi.

Verifikasi lokal pada 9 Oktober 2026: tes `TestPhase12EvaluationSQL` dan seluruh paket `adminreports` lulus pada MySQL 8.4 sementara. Ini memverifikasi query dan kompatibilitas laporan, bukan hasil sesi pengguna atau perilaku produksi.

Persiapan selesai ketika panduan, query terverifikasi, dan lembar hasil tersedia. Sesi pengguna, data produksi, keputusan fitur, serta verifikasi staging Phase 11 tetap menunggu bukti masing-masing.
