# Phase 14 - Perbaikan terarah

Status: **menunggu hasil Phase 13; hambatan belum dipilih; checkpoint belum terpenuhi**.

Contoh perbaikan hipotetis dan perbandingan sintetis tersedia pada [laporan simulasi Phase 12-15](phase12-15-simulation.md); implementasi dan uji ulang nyata tetap menunggu bukti.

Tujuan: pilih 1-3 hambatan terbesar dari [sesi Phase 13](phase13-user-test.md), perbaiki, lalu uji dengan lima sesi baru. Checkpoint: tugas lebih mudah diselesaikan dan kebutuhan bantuan berkurang berdasarkan perbandingan observasi sebelum/sesudah.

## Pilih dan perbaiki

Gunakan ID hambatan serta bukti sesi/tugas Phase 13. Dahulukan dampak tinggi, lalu frekuensi pada konteks yang sama; gunakan usaha implementasi sebagai pembeda bila bukti setara. Pilih maksimal tiga hambatan, dengan alasan tertulis. Tanpa hasil observasi, pemilihan tetap terbuka.

| ID hambatan / tugas | Bukti baseline: sesi, frekuensi, dampak | Alasan prioritas | Perbaikan dan hasil yang diharapkan | Penanggung jawab / commit | Verifikasi teknis |
|---|---|---|---|---|---|
| Belum dipilih | Belum tersedia | Belum ditetapkan | Belum ditetapkan | Belum ditetapkan | Belum dijalankan |

Tetapkan ukuran keberhasilan tiap tugas sebelum mengubah aplikasi. Batasi perubahan pada hambatan terpilih, pertahankan validasi checkout/tiket dan aksesibilitas, lalu jalankan pemeriksaan teknis yang sesuai sebelum sesi. Catat versi baseline dan versi perbaikan agar hasil dapat ditelusuri. Jika beberapa perbaikan diuji bersama, hasil menunjukkan dampak gabungan; jangan mengklaim kontribusi masing-masing tanpa pengujian terpisah.

## Lima sesi baru

Gunakan lima peserta baru dengan profil pengalaman yang sebanding, tiga mobile dan dua desktop, untuk mengurangi pengaruh belajar dari sesi sebelumnya. Pertahankan tugas T1-T4, data/skenario uji, aturan bantuan, dan pengukuran waktu dari Phase 13. Gunakan staging, pembayaran sandbox, dan mailbox uji; periksa kesiapan layanan sebelum sesi.

| Sesi | Perangkat | Tanggal/jam WIB | Versi / staging / event uji | Browser / viewport / input | Fasilitator | Status |
|---|---|---|---|---|---|---|
| V1 | Mobile | Belum ditetapkan | Belum ditetapkan | Belum dicatat | Belum ditetapkan | Belum dijalankan |
| V2 | Mobile | Belum ditetapkan | Belum ditetapkan | Belum dicatat | Belum ditetapkan | Belum dijalankan |
| V3 | Mobile | Belum ditetapkan | Belum ditetapkan | Belum dicatat | Belum ditetapkan | Belum dijalankan |
| V4 | Desktop | Belum ditetapkan | Belum ditetapkan | Belum dicatat | Belum ditetapkan | Belum dijalankan |
| V5 | Desktop | Belum ditetapkan | Belum ditetapkan | Belum dicatat | Belum ditetapkan | Belum dijalankan |

Salin lembar hasil per tugas Phase 13 ke dokumen ini untuk setiap sesi yang berlangsung, dengan judul V1-V5. Catat hasil, waktu, bantuan, observasi anonim, ID hambatan lama maupun baru, dan penyimpangan kondisi. Jangan menyimpan identitas peserta atau data yang membuka akses tiket. Tugas yang terhalang layanan diberi hasil **tidak dapat diuji** dan diulang setelah layanan siap; pengujian otomatis tidak menggantikan sesi pengguna.

## Perbandingan hasil

Isi satu baris per tugas dan kelompok perangkat (mobile, desktop, total), dengan tautan ke catatan baseline dan sesi baru. Gunakan format **sebelum -> sesudah** pada setiap ukuran.

| Tugas / perangkat | Bukti sesi sebelum / sesudah | Berhasil mandiri n/N | Berhasil dengan bantuan n/N | Memerlukan bantuan n/N | Gagal n/N | Waktu median tugas berhasil (detik; n) | Hambatan lama / baru dan dampak |
|---|---|---|---|---|---|---|---|
| Belum dibandingkan | Belum tersedia | Belum tersedia | Belum tersedia | Belum tersedia | Belum tersedia | Belum tersedia | Belum diamati |

`N` adalah peserta yang benar-benar dapat menguji tugas pada kelompok tersebut; keluarkan tugas **tidak dapat diuji**, tetapi laporkan jumlah dan alasannya di bawah tabel. Berhasil mandiri, berhasil dengan bantuan, dan gagal harus berjumlah N. **Memerlukan bantuan** mencakup peserta yang dibantu meskipun akhirnya gagal. Satu peserta dihitung sekali per ukuran per tugas. Tanpa observasi, tulis **belum tersedia**.

Bandingkan waktu hanya untuk hasil dan kondisi bantuan yang sebanding, sertakan jumlah pengamatan, dan pisahkan waktu menunggu layanan. Waktu lebih singkat akibat menyerah bukan perbaikan. Laporkan frekuensi hambatan sebagai peserta unik terdampak per peserta yang menguji bagian terkait, mengikuti Phase 13.

## Checkpoint dan keputusan

- [ ] 1-3 hambatan dipilih dari bukti Phase 13, dengan alasan, baseline, dan ukuran keberhasilan sebelum implementasi.
- [ ] Perbaikan diterapkan dan verifikasi teknis tercatat pada versi yang diuji.
- [ ] Lima sesi baru selesai (tiga mobile, dua desktop), dengan catatan T1-T4 dan celah pengujian terselesaikan.
- [ ] Pada tugas sasaran, proporsi keberhasilan mandiri meningkat dan proporsi peserta yang memerlukan bantuan menurun; tidak ada penurunan penyelesaian atau hambatan berdampak tinggi baru pada tugas lain. Periksa rincian perangkat agar hasil total tidak menyembunyikan regresi.
- [ ] Keputusan mempertahankan, merevisi, atau membatalkan perbaikan memiliki rujukan bukti serta penanggung jawab tindak lanjut.

Keputusan saat ini: **belum dapat dinilai**. Jika hasil campuran, ukuran tidak membaik, atau kondisi tidak sebanding, checkpoint tetap terbuka; catat penyebab dan tentukan uji berikutnya. Lima sesi memberi indikasi awal kemudahan penggunaan, bukan bukti statistik kenaikan konversi produksi. Untuk evaluasi funnel pembelian, tetap ikuti [uji konversi Phase 10](conversion-user-test.md); bila beberapa perubahan digabung, jangan menganggapnya eksperimen satu perubahan.

Setelah putaran ini selesai, gunakan [Phase 15](phase15-development-decision.md) untuk memilih satu pengembangan berdasarkan bukti atau menutup cakupan saat ini jika hasilnya baik dan belum ada kebutuhan baru yang kuat.
