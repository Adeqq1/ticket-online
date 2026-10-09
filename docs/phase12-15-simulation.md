# Laporan simulasi Phase 12-15

**SIMULASI SINTETIS - bukan observasi pengguna, hasil staging, atau data produksi.**

Seluruh angka, peserta, kasus dukungan, dan perilaku sebelum/sesudah di laporan ini dibuat untuk latihan evaluasi. Tidak ada pembayaran, email, sesi browser, atau perubahan aplikasi yang dijalankan. Perbaikan Phase 14 adalah varian hipotetis; hasilnya ditentukan dalam fixture, bukan prediksi maupun bukti efektivitas. Checkpoint nyata pada dokumen Phase 12-15 tetap terbuka.

## Ringkasan keputusan simulasi

Dalam skenario ini, tiga hambatan diprioritaskan: H1 pemahaman status pembayaran, H2 pemulihan tiket, dan H3 pemahaman refund. Keberhasilan mandiri naik dari 11/20 (55.0%) menjadi 18/20 (90.0%) tugas; kebutuhan bantuan turun dari 9/20 (45.0%) menjadi 2/20 (10.0%). Angka ini mencakup 20 tugas per kelompok, bukan 20 peserta independen.

Pilihan pengembangan dalam simulasi: **pusat bantuan terkait order**, karena pertanyaan pembayaran, refund, dan akses mendominasi kasus sintetis dan masih ada kebutuhan bantuan pada uji ulang. Keputusan produk nyata tetap menunggu validasi lapangan.

## Skema, asal data, dan reproduksi

- Generator dan seluruh fixture: [scripts/simulate-phase12-15.py](../scripts/simulate-phase12-15.py). Stdlib Python, deterministik, tanpa dependensi tambahan, seed acak, jaringan, atau database.
- Skema sesi: alias, perangkat, tugas T1-T4, hasil, detik, bantuan boolean, ID hambatan, observasi sintetis. T1 checkout; T2 tiket lama; T3 pemulihan browser baru; T4 pemahaman status dan hak refund.
- Skema funnel: hitungan perjalanan matang per perangkat untuk detail, reservasi, order, sesi pembayaran, lunas. Pembayaran dan dukungan berupa agregat yang dirancang, bukan keluaran query SQL atau ekspor dashboard.
- Referensi kode saat penyusunan: `29e9c900f5c66574c4773b136672b7d75308865e`. Label varian `SIM-A` dan `SIM-B` adalah kondisi hipotetis, bukan commit implementasi.
- Kalender fiktif: agregat 1-30 September 2026 WIB; snapshot 2 Oktober 2026 09:00 WIB; baseline 2 Oktober dan uji ulang 6 Oktober. Waktu ini label skenario, bukan log eksekusi. Laporan disusun 9 Oktober 2026.
- Event fiktif `SIM-E1`; peserta U1-U5 dan V1-V5 diasumsikan berbeda dengan pengalaman sebanding. Fasilitator simulasi `SIM-F`. Mobile: viewport 390x844, sentuh; desktop: 1440x900, mouse/keyboard. Browser dimodelkan sebagai Chromium dengan versi sama pada kedua kelompok, tanpa klaim pengujian perangkat nyata.
- Asumsi layanan: stok cukup, sandbox/SMTP siap, order lama lunas tersedia, profil pemulihan kosong. Tidak ada gangguan layanan, waktu tunggu terpisah, atau penyimpangan skenario dalam fixture. Semua tugas dapat diuji.

Jalankan dari root repository:

```sh
python3 scripts/simulate-phase12-15.py > docs/phase12-15-simulation.md
python3 scripts/simulate-phase12-15.py --check
```

`--check` memeriksa fixture, rekonsiliasi agregat, penghitungan bantuan pada tugas gagal, pengecualian tugas tidak dapat diuji, median, serta kesamaan laporan tersimpan dengan keluaran generator. Pemeriksaan tersebut memvalidasi simulasi, bukan aplikasi.

## Phase 12 - Gabungkan bukti awal sintetis

Periode SQL dimodelkan sebagai `[2026-09-01, 2026-10-01)` WIB, setara filter dashboard 1-30 September. Semua pembayaran cohort dalam fixture terjadi di periode itu dan dalam 24 jam awal perjalanan; tidak ada pembayaran lintas batas cohort/periode. Asumsi ini sengaja memungkinkan rekonsiliasi sederhana; data nyata tidak selalu demikian.

| Ukuran | Nilai sintetis / interpretasi |
|---|---|
| Order lunas / tanpa data pembeli | 400 / 0 |
| Pembeli unik | 340: 280 membeli sekali, 60 membeli dua kali dalam periode |
| Pembeli berulang | 60/340 (17.6%); tidak ada riwayat sebelum periode dalam model |
| Perjalanan matang / belum matang | 1.000 / 0; snapshot lebih dari 24 jam setelah akhir cohort |
| Pembayaran teratribusi / tanpa atribusi | 370 / 30; total 400, pembayaran tanpa atribusi tidak masuk rasio funnel |
| Refund selesai | 12 dari 400 order lunas; tetap dihitung dalam riwayat pembayaran |
| Konteks perubahan acara | 12 refund setelah jadwal pengganti SIM-E1; bukan pembatalan, sehingga pembayaran baru masih mungkin |
| Sasaran akhir funnel untuk latihan | Minimal 35%, ditetapkan sebagai asumsi sebelum evaluasi; tercapai 37% |
| Jangkauan / sasaran jangkauan | Belum tersedia / belum ditetapkan; perjalanan bukan orang unik |

| Perangkat | Detail | Reservasi | Order | Sesi pembayaran | Lunas | Konversi akhir |
|---|---|---|---|---|---|---|
| Mobile | 600 | 360 | 240 | 216 | 180 | 180/600 (30.0%) |
| Desktop | 400 | 280 | 224 | 212 | 190 | 190/400 (47.5%) |
| Total | 1000 | 640 | 464 | 428 | 370 | 370/1000 (37.0%) |

| Transisi | Konversi tahap | Kehilangan perjalanan |
|---|---|---|
| Detail -> Reservasi | 640/1000 (64.0%) | 360 |
| Reservasi -> Order | 464/640 (72.5%) | 176 |
| Order -> Sesi pembayaran | 428/464 (92.2%) | 36 |
| Sesi pembayaran -> Lunas | 370/428 (86.4%) | 58 |

Kehilangan terbesar adalah detail ke reservasi: 360 perjalanan. Ini lokasi kehilangan, bukan bukti penyebab. Sesi sintetis tidak menemukan hambatan di pemilihan event/tiket; penyebab kehilangan awal tetap belum diketahui. H1 terjadi lebih jauh di alur dan tidak boleh dianggap menjelaskan seluruh kehilangan tersebut. Perangkat mobile memiliki konversi akhir 30%, desktop 47,5%; komposisi perangkat harus dipertahankan saat membandingkan.

| Kategori dukungan | Kasus sintetis |
|---|---|
| Pembayaran | 22 |
| Email/pemulihan | 14 |
| Akses tiket | 8 |
| Jadwal/refund | 10 |
| Check-in | 4 |
| Lainnya | 2 |
| Total | 60 |

Model dukungan memiliki satu percakapan per order yang berbeda, seluruhnya dari 400 order tersebut, dan satu kategori utama per percakapan. Kategori pembayaran/email/akses/refund berjumlah 54/60 (90.0%) kasus dan 54/400 (13.5%) order lunas. Kasus pembayaran diasumsikan menanyakan status pending sebelum akhirnya lunas. Karena tidak mencakup order yang tidak pernah lunas, rasio ini tidak mewakili semua permintaan dukungan checkout.

Kebutuhan tambahan fiktif: dua permintaan promo pada bulan itu, tidak ada jadwal promosi rutin atau hambatan operasi yang terkonfirmasi; dua penyelenggara fiktif hanya memerlukan tiket tanpa nomor kursi. Data ini hanya alasan penundaan dalam skenario, bukan klaim kondisi bisnis sebenarnya.

Keputusan awal Phase 12 simulasi: lanjutkan observasi dan perbaikan; jangan langsung memilih akun hanya dari 17,6% pembeli berulang. Status Phase 12 nyata tetap **bukti belum cukup**.

## Phase 13 - Lima sesi baseline sintetis

Semua sesi berikut memakai SIM-A, SIM-E1, dan SIM-F. Instruksi, aturan bantuan, dan definisi keberhasilan mengikuti [Phase 13](phase13-user-test.md). Durasi gagal adalah waktu sampai penghentian dan tidak dipakai sebagai waktu keberhasilan. Seluruh catatan berikut merupakan perilaku yang dikarang, bukan kutipan peserta.

| Sesi | Perangkat | Tugas | Hasil | Detik | Bantuan | Hambatan | Observasi sintetis |
|---|---|---|---|---|---|---|---|
| U1 | Mobile | T1 | gagal | 420 | Ya | H1 | Menganggap pending sudah lunas; berhenti meski status dijelaskan. |
| U1 | Mobile | T2 | berhasil | 65 | Tidak | - | Menemukan tiket lama yang sesuai. |
| U1 | Mobile | T3 | gagal | 300 | Ya | H2 | Tidak menemukan reference uji meski diarahkan ke email; menyerah. |
| U1 | Mobile | T4 | gagal | 180 | Ya | H3 | Menganggap refund langsung cair setelah diminta; jawaban tetap keliru setelah bantuan. |
| U2 | Mobile | T1 | berhasil | 360 | Ya | H1 | Meminta penjelasan pending; melanjutkan hingga tiket terbuka. |
| U2 | Mobile | T2 | berhasil | 80 | Tidak | - | Memilih tiket lama dari daftar. |
| U2 | Mobile | T3 | berhasil | 240 | Ya | H2 | Menemukan reference setelah ditunjukkan letaknya di email uji. |
| U2 | Mobile | T4 | gagal | 170 | Ya | H3 | Tidak dapat membedakan tenggat jadwal baru dan penundaan setelah bantuan. |
| U3 | Mobile | T1 | berhasil | 280 | Tidak | - | Menunggu konfirmasi dan membuka tiket. |
| U3 | Mobile | T2 | berhasil | 70 | Tidak | - | Menemukan tiket lama. |
| U3 | Mobile | T3 | berhasil | 210 | Ya | H2 | Meminta petunjuk masuk alur pemulihan, lalu berhasil. |
| U3 | Mobile | T4 | berhasil | 160 | Ya | H3 | Menjawab hak refund dengan benar setelah aturan dijelaskan. |
| U4 | Desktop | T1 | berhasil | 240 | Tidak | - | Checkout dan tiket selesai mandiri. |
| U4 | Desktop | T2 | berhasil | 50 | Tidak | - | Menemukan tiket lama. |
| U4 | Desktop | T3 | berhasil | 120 | Tidak | - | Memulihkan akses melalui mailbox uji. |
| U4 | Desktop | T4 | berhasil | 150 | Ya | H3 | Memerlukan penjelasan perbedaan pengajuan dan refund selesai. |
| U5 | Desktop | T1 | berhasil | 250 | Tidak | - | Checkout dan tiket selesai mandiri. |
| U5 | Desktop | T2 | berhasil | 55 | Tidak | - | Menemukan tiket lama. |
| U5 | Desktop | T3 | berhasil | 130 | Tidak | - | Memulihkan akses mandiri. |
| U5 | Desktop | T4 | berhasil | 100 | Tidak | - | Menjelaskan status dan seluruh hak refund dengan tepat. |

H1 didukung U1/T1 dan U2/T1; H2 oleh U1/T3, U2/T3, U3/T3; H3 oleh U1-U4/T4. Dampak awal ketiganya tinggi karena kegagalan tugas atau salah pemahaman status/hak. T2 selesai mandiri pada seluruh peserta, sehingga kebutuhan akun belum didukung oleh tugas ini. Tidak ada tugas terhalang layanan.

## Phase 14 - Tiga perbaikan hipotetis dan lima sesi baru

Urutan prioritas: H3 (4/5), H2 (3/5), H1 (2/5), seluruhnya berdampak tinggi pada baseline. Pemilik peran yang diusulkan: produk menetapkan salinan dan kriteria; frontend menerapkan; QA memeriksa perilaku. Belum ada penugasan orang atau perubahan kode.

| ID | Varian SIM-B yang dimodelkan | Kriteria yang diasumsikan ditetapkan sebelum uji ulang |
|---|---|---|
| H3 | Perjelas pemisahan hak, tenggat, dan progres refund pada konteks order | T4 minimal 4/5 mandiri, maksimal 1/5 memerlukan bantuan, tidak ada jawaban salah pada akhir tugas |
| H2 | Perjelas jalur pemulihan dan contoh letak reference pada email uji | T3 minimal 4/5 mandiri, maksimal 1/5 memerlukan bantuan, semua dapat membuka tiket |
| H1 | Perjelas pending vs lunas dan langkah berikutnya saat menunggu konfirmasi | T1 5/5 mandiri, tanpa bantuan atau gagal; status tetap mengikuti backend |

Aplikasi sudah memiliki informasi status dan tautan pemulihan; hipotesisnya adalah kemudahan menemukan/memahami informasi tersebut. Ini bukan temuan bahwa fitur tersebut tidak ada. Varian tidak menghapus verifikasi akses, validasi checkout/tiket, aturan refund, fokus, live region, atau dukungan perangkat. Verifikasi teknis varian **belum dijalankan** karena varian belum diimplementasikan. Saat benar-benar diterapkan, jalankan tes terkait, `bun run check`, dan `bun run build` dari frontend, serta uji layanan yang terdampak.

| Sesi | Perangkat | Tugas | Hasil | Detik | Bantuan | Hambatan | Observasi sintetis |
|---|---|---|---|---|---|---|---|
| V1 | Mobile | T1 | berhasil | 270 | Tidak | - | Membedakan pending/lunas dan membuka tiket. |
| V1 | Mobile | T2 | berhasil | 62 | Tidak | - | Menemukan tiket lama. |
| V1 | Mobile | T3 | berhasil | 150 | Tidak | - | Menemukan alur, reference, dan tiket. |
| V1 | Mobile | T4 | berhasil | 110 | Tidak | - | Menjelaskan status, hak, dan tenggat dengan tepat. |
| V2 | Mobile | T1 | berhasil | 260 | Tidak | - | Menunggu konfirmasi sebelum membuka tiket. |
| V2 | Mobile | T2 | berhasil | 75 | Tidak | - | Menemukan tiket lama. |
| V2 | Mobile | T3 | berhasil | 190 | Ya | H2 | Masih memerlukan petunjuk menemukan reference di email. |
| V2 | Mobile | T4 | berhasil | 115 | Tidak | - | Membedakan pengajuan dan refund selesai. |
| V3 | Mobile | T1 | berhasil | 255 | Tidak | - | Checkout selesai mandiri. |
| V3 | Mobile | T2 | berhasil | 68 | Tidak | - | Menemukan tiket lama. |
| V3 | Mobile | T3 | berhasil | 145 | Tidak | - | Memulihkan akses mandiri. |
| V3 | Mobile | T4 | berhasil | 140 | Ya | H3 | Meminta penjelasan tenggat jadwal baru; kemudian menjawab benar. |
| V4 | Desktop | T1 | berhasil | 230 | Tidak | - | Checkout selesai mandiri. |
| V4 | Desktop | T2 | berhasil | 48 | Tidak | - | Menemukan tiket lama. |
| V4 | Desktop | T3 | berhasil | 110 | Tidak | - | Memulihkan akses mandiri. |
| V4 | Desktop | T4 | berhasil | 90 | Tidak | - | Menjelaskan seluruh contoh status dan refund dengan tepat. |
| V5 | Desktop | T1 | berhasil | 235 | Tidak | - | Checkout selesai mandiri. |
| V5 | Desktop | T2 | berhasil | 52 | Tidak | - | Menemukan tiket lama. |
| V5 | Desktop | T3 | berhasil | 115 | Tidak | - | Memulihkan akses mandiri. |
| V5 | Desktop | T4 | berhasil | 95 | Tidak | - | Menjelaskan seluruh contoh status dan refund dengan tepat. |

### Perbandingan per tugas dan perangkat

Semua sel menampilkan sebelum -> sesudah. Penyebut adalah peserta yang dapat menguji tugas terkait; tidak ada pengecualian dalam fixture. Membutuhkan bantuan termasuk peserta yang akhirnya gagal. Median hanya dari keberhasilan mandiri; sampel sebelum/sesudah berbeda, jadi ini deskripsi kelompok, bukan perubahan waktu orang yang sama.

| Tugas / perangkat | Mandiri | Berhasil dibantu | Memerlukan bantuan | Gagal | Median mandiri detik (n) |
|---|---|---|---|---|---|
| T1 / Mobile | 1/3 (33.3%) -> 3/3 (100.0%) | 1/3 (33.3%) -> 0/3 (0.0%) | 2/3 (66.7%) -> 0/3 (0.0%) | 1/3 (33.3%) -> 0/3 (0.0%) | 280 (n=1) -> 260 (n=3) |
| T1 / Desktop | 2/2 (100.0%) -> 2/2 (100.0%) | 0/2 (0.0%) -> 0/2 (0.0%) | 0/2 (0.0%) -> 0/2 (0.0%) | 0/2 (0.0%) -> 0/2 (0.0%) | 245.0 (n=2) -> 232.5 (n=2) |
| T1 / Total | 3/5 (60.0%) -> 5/5 (100.0%) | 1/5 (20.0%) -> 0/5 (0.0%) | 2/5 (40.0%) -> 0/5 (0.0%) | 1/5 (20.0%) -> 0/5 (0.0%) | 250 (n=3) -> 255 (n=5) |
| T2 / Mobile | 3/3 (100.0%) -> 3/3 (100.0%) | 0/3 (0.0%) -> 0/3 (0.0%) | 0/3 (0.0%) -> 0/3 (0.0%) | 0/3 (0.0%) -> 0/3 (0.0%) | 70 (n=3) -> 68 (n=3) |
| T2 / Desktop | 2/2 (100.0%) -> 2/2 (100.0%) | 0/2 (0.0%) -> 0/2 (0.0%) | 0/2 (0.0%) -> 0/2 (0.0%) | 0/2 (0.0%) -> 0/2 (0.0%) | 52.5 (n=2) -> 50.0 (n=2) |
| T2 / Total | 5/5 (100.0%) -> 5/5 (100.0%) | 0/5 (0.0%) -> 0/5 (0.0%) | 0/5 (0.0%) -> 0/5 (0.0%) | 0/5 (0.0%) -> 0/5 (0.0%) | 65 (n=5) -> 62 (n=5) |
| T3 / Mobile | 0/3 (0.0%) -> 2/3 (66.7%) | 2/3 (66.7%) -> 1/3 (33.3%) | 3/3 (100.0%) -> 1/3 (33.3%) | 1/3 (33.3%) -> 0/3 (0.0%) | belum tersedia -> 147.5 (n=2) |
| T3 / Desktop | 2/2 (100.0%) -> 2/2 (100.0%) | 0/2 (0.0%) -> 0/2 (0.0%) | 0/2 (0.0%) -> 0/2 (0.0%) | 0/2 (0.0%) -> 0/2 (0.0%) | 125.0 (n=2) -> 112.5 (n=2) |
| T3 / Total | 2/5 (40.0%) -> 4/5 (80.0%) | 2/5 (40.0%) -> 1/5 (20.0%) | 3/5 (60.0%) -> 1/5 (20.0%) | 1/5 (20.0%) -> 0/5 (0.0%) | 125.0 (n=2) -> 130.0 (n=4) |
| T4 / Mobile | 0/3 (0.0%) -> 2/3 (66.7%) | 1/3 (33.3%) -> 1/3 (33.3%) | 3/3 (100.0%) -> 1/3 (33.3%) | 2/3 (66.7%) -> 0/3 (0.0%) | belum tersedia -> 112.5 (n=2) |
| T4 / Desktop | 1/2 (50.0%) -> 2/2 (100.0%) | 1/2 (50.0%) -> 0/2 (0.0%) | 1/2 (50.0%) -> 0/2 (0.0%) | 0/2 (0.0%) -> 0/2 (0.0%) | 100 (n=1) -> 92.5 (n=2) |
| T4 / Total | 1/5 (20.0%) -> 4/5 (80.0%) | 2/5 (40.0%) -> 1/5 (20.0%) | 4/5 (80.0%) -> 1/5 (20.0%) | 2/5 (40.0%) -> 0/5 (0.0%) | 100 (n=1) -> 102.5 (n=4) |

### Frekuensi hambatan

| ID | Tugas | Dampak baseline | Total | Mobile | Desktop |
|---|---|---|---|---|---|
| H1 | T1 | Tinggi | 2/5 (40.0%) -> 0/5 (0.0%) | 2/3 (66.7%) -> 0/3 (0.0%) | 0/2 (0.0%) -> 0/2 (0.0%) |
| H2 | T3 | Tinggi | 3/5 (60.0%) -> 1/5 (20.0%) | 3/3 (100.0%) -> 1/3 (33.3%) | 0/2 (0.0%) -> 0/2 (0.0%) |
| H3 | T4 | Tinggi | 4/5 (80.0%) -> 1/5 (20.0%) | 3/3 (100.0%) -> 1/3 (33.3%) | 1/2 (50.0%) -> 0/2 (0.0%) |

H1 tidak muncul pada sesi baru; H2 tersisa di V2/T3 dan H3 di V3/T4. Keduanya berdampak sedang pada uji ulang karena peserta akhirnya berhasil dengan bantuan. Tidak ada hambatan baru dalam fixture. Total berhasil (mandiri atau dibantu) meningkat dari 16/20 menjadi 20/20 tugas; T2 tetap 5/5 mandiri. Target deskriptif simulasi terpenuhi tanpa penurunan pada rincian perangkat, tetapi pelaksanaan dan checkpoint Phase 14 nyata tetap terbuka.

Ketiga varian dimodelkan bersama sehingga kontribusi individual tidak dapat dipisahkan. Tidak ada funnel atau rekap dukungan setelah perbaikan yang dimodelkan; jangan menyimpulkan konversi produksi naik atau kasus pelanggan sudah turun. Simulasi ini bukan eksperimen satu perubahan pada [Phase 10](conversion-user-test.md).

## Phase 15 - Pilih satu pengembangan dalam skenario

| Kandidat | Bukti sintetis | Keputusan simulasi |
|---|---|---|
| Akun pembeli | 60/340 pembeli berulang, tetapi T2 mandiri 5/5 pada kedua kelompok; pemulihan membaik tanpa akun | Tunda; perlu kesulitan riwayat lintas kunjungan yang benar-benar diamati |
| Voucher terkelola | Dua permintaan promo, kebutuhan rutin belum terbukti | Tunda sampai ada frekuensi dan biaya operasi; kode promo hardcoded saja belum cukup |
| Pusat bantuan terkait order | 54/60 kasus terkait pembayaran/akses/refund; bantuan masih diperlukan oleh V2 dan V3 | Pilih satu ini sebagai hipotesis pengembangan berikutnya |
| Pemilihan kursi | Dua penyelenggara fiktif tidak memerlukan kursi bernomor | Tunda sampai ada acara dan aturan penempatan terkonfirmasi |

Cakupan minimum yang diusulkan: bantuan kontekstual pada order untuk status pembayaran, akses tiket, hak/tenggat dan progres refund, dengan jawaban berasal dari status backend serta jalur pemulihan yang sudah ada. Perkiraan usaha relatif untuk diskusi: bantuan kontekstual lebih kecil daripada akun atau inventori kursi, tetapi belum diestimasi melalui desain teknis. Hindari membuka data order tanpa otorisasi. Akun, voucher, kursi, chatbot, dan sistem dukungan baru berada di luar cakupan usulan ini.

| Ukuran | Baseline sintetis | Target usulan untuk validasi berikutnya | Metode / periode |
|---|---|---|---|
| Jawaban status/refund mandiri T4 | 4/5 pada SIM-B | 5/5 pada lima peserta baru, 3 mobile dan 2 desktop; nol jawaban salah akhir | Tugas dan kondisi setara; setelah prototipe/implementasi bantuan siap |
| Pemulihan mandiri T3 | 4/5 pada SIM-B | 5/5 pada kelompok baru yang sama | Tanpa petunjuk fasilitator; tetap verifikasi akses |
| Kasus pembayaran/akses/refund per order lunas | 54/400 = 13,5% selama 30 hari sintetis | Penurunan relatif minimal 25%; dari contoh baseline menjadi maksimal 10,125% | Kumpulkan baseline nyata 30 hari sebelum rollout dan bandingkan 30 hari setelah; kasus unik yang masuk dalam periode dan terkait order lunas dalam periode yang sama |
| Penjagaan alur checkout dan tiket lama | T1 dan T2 masing-masing 5/5 mandiri | Tetap 5/5 mandiri; tanpa hambatan tinggi baru | Kelompok uji baru yang sama dan verifikasi teknis |

Target dukungan adalah contoh rencana, bukan hasil yang sudah dicapai. Gunakan jeda snapshot, kategori, dan metode deduplikasi yang sama; bandingkan konteks acara, perubahan jadwal/refund, dan perangkat. Kasus yang masuk setelah akhir periode tidak masuk rasio ini; rasio bukan ukuran dukungan sepanjang umur order. Ukur juga pertanyaan order belum lunas secara terpisah agar dampak pada pengguna yang gagal membeli tidak tersembunyi. Sesuaikan target setelah baseline nyata tersedia dan sebelum rollout, bukan setelah melihat hasil.

Penanggung jawab yang diusulkan: produk untuk keputusan/cakupan, dukungan untuk kategorisasi, engineering untuk implementasi, QA/peneliti untuk sesi. Nama dan tanggal tinjauan nyata belum ditetapkan; tinjauan dilakukan setelah putaran nyata Phase 13-14 dan sebelum persetujuan implementasi fitur baru. Tidak ada pesan atau penugasan eksternal yang dibuat.

Alternatif keputusan yang sah: bila bukti nyata menunjukkan tugas mandiri, hambatan penting selesai, dan tidak ada kebutuhan baru kuat, tutup cakupan saat ini. Dalam fixture ini opsi itu ditunda karena masih ada dua kebutuhan bantuan dan pola pertanyaan yang berulang.

## Laporan penutup dan langkah pengembangan nyata

| Phase | Hasil latihan | Yang masih diperlukan di dunia nyata |
|---|---|---|
| 12 | Agregat sintetis direkonsiliasi, kebutuhan awal ditimbang | Snapshot SQL/dashboard nyata, dukungan, periode, dan konteks refund |
| 13 | Lima sesi fiktif menghasilkan tiga hambatan dengan frekuensi/dampak | Lima peserta nyata, 3 mobile dan 2 desktop, catatan observasi dan bantuan |
| 14 | Tiga varian hipotetis dan lima sesi fiktif memenuhi target deskriptif | Perbaikan berdasarkan bukti, pemeriksaan teknis, lima peserta baru dan perbandingan nyata |
| 15 | Satu hipotesis fitur beserta cakupan minimum, baseline dan target dipilih | Validasi kebutuhan, pemilik keputusan dan jadwal, lalu pilih fitur atau tutup cakupan |

Prioritas berikutnya adalah mengumpulkan bukti nyata memakai [Phase 12](phase12-evaluation.md), [Phase 13](phase13-user-test.md), [Phase 14](phase14-targeted-improvements.md), dan [Phase 15](phase15-development-decision.md). Gunakan fixture ini sebagai contoh pencatatan dan pemeriksaan aritmetika; jangan menyalin angka atau perilakunya ke hasil pengguna. Skenario sengaja memberi hasil perbaikan positif sehingga tidak menguji ketidakpastian efek; jika hasil nyata campuran, ikuti keputusan ulang Phase 14. Kesiapan rollout tetap mengikuti verifikasi staging yang terpisah.
