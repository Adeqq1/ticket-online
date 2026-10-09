# Phase 13 - Uji pengguna

Status: **persiapan tersedia; sesi belum dijalankan; checkpoint belum terpenuhi**.

Contoh pencatatan dan rekap tersedia pada [laporan simulasi Phase 12-15](phase12-15-simulation.md); sesi sintetis tersebut tidak menggantikan sesi pengguna nyata.

Tujuan: menjalankan lima sesi dengan lima peserta berbeda (tiga mobile, dua desktop) dan menghasilkan daftar hambatan nyata beserta frekuensi, dampak, dan bukti observasinya. Lanjutkan skenario [Phase 12](phase12-evaluation.md); hasil di sini menjadi rujukan bukti sesi pada evaluasi tersebut.

## Persiapan sesi

- Tetapkan fasilitator, tanggal, versi aplikasi/commit, lingkungan staging, dan event uji sebelum sesi. Gunakan versi dan skenario yang sama untuk kelima peserta; catat setiap penyimpangan.
- Siapkan stok, data pembeli uji, pembayaran sandbox, order lama yang sudah lunas, mailbox uji, dan browser/profil baru tanpa akses order. Siapkan contoh order untuk pembatalan, penundaan, dan jadwal pengganti sesuai [kebijakan perubahan acara](event-change-policy.md).
- Periksa pembayaran dan email/pemulihan sebelum sesi. Gangguan provider/SMTP dicatat sebagai kendala lingkungan; tugas yang terhalang diberi hasil **tidak dapat diuji**, bukan kegagalan pengguna.
- Gunakan tiga perangkat mobile dengan viewport di bawah 768 piksel dan dua desktop dengan viewport minimal 768 piksel. Catat browser, ukuran viewport, dan cara input; emulasi oleh pengembang tidak menggantikan peserta.
- Jelaskan bahwa yang diuji adalah aplikasi, minta persetujuan peserta untuk pencatatan observasi, dan izinkan berhenti kapan saja. Simpan alias U1-U5 saja. Jangan memasukkan nama, email, reference order, token, QR, tautan privat, atau rekaman yang membuka akses tiket ke repository.

## Tugas dan panduan fasilitator

Bacakan tujuan tugas tanpa menyebut tombol atau rute yang harus dipakai. Minta peserta mengutarakan apa yang mereka pikirkan. Jangan memberi petunjuk sebelum peserta meminta bantuan; catat bantuan dan tindakan sesudahnya. Ukur waktu dari tugas selesai dibacakan sampai selesai atau dihentikan, beserta alasan penghentian. Pisahkan waktu menunggu layanan bila terjadi gangguan.

| ID | Instruksi untuk peserta | Bukti penyelesaian yang diamati fasilitator |
|---|---|---|
| T1 | Cari acara uji yang ditentukan dan beli satu tiket dengan data serta pembayaran uji yang disediakan. | Peserta menyelesaikan checkout tamu, pembayaran sandbox terkonfirmasi lunas, dan dapat membuka tiket. Catat tahap berhenti bila tidak selesai. |
| T2 | Temukan kembali tiket dari pembelian lama yang sudah lunas. | Peserta menemukan tiket yang sesuai melalui Tiket Saya pada browser yang sudah memiliki akses order uji. |
| T3 | Anda berpindah ke browser baru. Dapatkan kembali akses tiket yang sudah dibeli. | Peserta menemukan alur pemulihan, menggunakan mailbox uji, dan membuka tiket di browser/profil tanpa akses awal. Rute `/pulihkan-tiket` hanya menjadi acuan fasilitator. |
| T4 | Dari contoh yang ditampilkan, jelaskan kapan pembayaran lunas, cara memperoleh tiket, dan pilihan Anda jika acara dibatalkan, ditunda, atau mendapat jadwal baru. | Catat jawaban sebelum menjelaskan kebijakan; cocokkan dengan status order dan kebijakan perubahan acara. Catat bagian yang disalahpahami, termasuk hak/tenggat refund. |

Untuk T4, jawaban harus membedakan pembayaran pending dari lunas; pembatalan memberi refund penuh termasuk biaya admin, penundaan memberi hak meminta refund tanpa tenggat, dan jadwal pengganti mengikuti kelayakan serta tenggat yang ditampilkan. Pengajuan refund tidak berarti dana sudah kembali. Ini menguji pemahaman, bukan membuktikan kemampuan refund merchant.

## Register sesi

Ganti **belum dijalankan** hanya setelah sesi berlangsung. Isi hasil tugas dalam catatan sesi berikut, bukan berdasarkan perkiraan atau pengujian otomatis.

| Sesi | Perangkat | Tanggal/jam WIB | Versi / staging / event uji | Browser / viewport / input | Fasilitator | Status |
|---|---|---|---|---|---|---|
| U1 | Mobile | Belum ditetapkan | Belum ditetapkan | Belum dicatat | Belum ditetapkan | Belum dijalankan |
| U2 | Mobile | Belum ditetapkan | Belum ditetapkan | Belum dicatat | Belum ditetapkan | Belum dijalankan |
| U3 | Mobile | Belum ditetapkan | Belum ditetapkan | Belum dicatat | Belum ditetapkan | Belum dijalankan |
| U4 | Desktop | Belum ditetapkan | Belum ditetapkan | Belum dicatat | Belum ditetapkan | Belum dijalankan |
| U5 | Desktop | Belum ditetapkan | Belum ditetapkan | Belum dicatat | Belum ditetapkan | Belum dijalankan |

Salin blok berikut di dokumen ini untuk setiap sesi yang telah berlangsung, dengan judul `### U1` hingga `### U5`. Gunakan hasil **berhasil**, **gagal**, atau **tidak dapat diuji**; tandai bantuan secara terpisah agar keberhasilan dengan bantuan tidak dianggap mandiri. Tugas belum dicoba tetap **belum diuji**.

| Tugas | Hasil | Waktu (detik) | Bantuan: tidak/ya + bentuk | Observasi / jawaban peserta yang dianonimkan | ID hambatan / alasan tidak dapat diuji |
|---|---|---|---|---|---|
| T1 | Belum diuji | Belum dicatat | Belum dicatat | Belum dicatat | Belum dicatat |
| T2 | Belum diuji | Belum dicatat | Belum dicatat | Belum dicatat | Belum dicatat |
| T3 | Belum diuji | Belum dicatat | Belum dicatat | Belum dicatat | Belum dicatat |
| T4 | Belum diuji | Belum dicatat | Belum dicatat | Belum dicatat | Belum dicatat |

Catat penyimpangan skenario, gangguan layanan, dan alasan penghentian di bawah tabel sesi. Bedakan tindakan/jawaban yang diamati dari dugaan penyebab.

## Rekap hambatan

**Belum ada hasil observasi.** Ini bukan berarti tidak ada hambatan.

Gabungkan observasi tentang masalah yang sama menjadi satu ID H1, H2, dan seterusnya. Frekuensi adalah jumlah peserta unik yang mengalami hambatan dibagi peserta yang benar-benar menguji bagian terkait (`n/N`), disertai alias sesi dan rincian mobile/desktop. Satu peserta yang mengulang kesalahan tetap dihitung sekali per hambatan. Tugas tidak dapat diuji tidak masuk penyebut; bila belum ada peserta yang dapat menguji, tulis **belum tersedia**, bukan 0%.

| ID | Hambatan dan tugas | Bukti: sesi/tugas + observasi | Frekuensi n/N (total; mobile; desktop) | Dampak yang diamati | Usulan tindak lanjut / penanggung jawab |
|---|---|---|---|---|---|
| Belum tersedia | Belum diamati | Belum tersedia | Belum tersedia | Belum dinilai | Belum ditetapkan |

Nilai dampak dari akibat yang diamati: **tinggi** bila tugas tidak selesai atau pengguna salah memahami status pembayaran/hak refund; **sedang** bila perlu bantuan atau mengulang langkah untuk selesai; **rendah** bila terjadi keraguan tetapi tugas selesai mandiri tanpa salah pemahaman. Catat kendala lingkungan terpisah dari hambatan penggunaan. Dahulukan dampak tinggi, lalu frekuensi pada konteks yang sama; jangan menganggap lima peserta mewakili prevalensi populasi.

## Checkpoint

- [ ] Lima sesi nyata tercatat dengan komposisi tiga mobile dan dua desktop serta metadata versi/lingkungan.
- [ ] Setiap sesi memiliki hasil T1-T4, waktu, bantuan, dan bukti observasi; tugas terhalang layanan sudah diulang setelah layanan siap, atau dicatat sebagai celah yang membuat checkpoint tetap terbuka.
- [ ] Daftar hambatan memiliki bukti sesi/tugas, frekuensi dengan penyebut yang jelas, dan dampak. Jika suatu tugas tidak memperlihatkan hambatan, nyatakan hanya untuk peserta yang telah mengujinya.
- [ ] 1-3 hambatan prioritas dipilih berdasarkan bukti untuk [perbaikan Phase 14](phase14-targeted-improvements.md), dengan alasan, penanggung jawab, dan kriteria uji ulang; bila tidak ditemukan hambatan, catat alasan tidak memilih perbaikan.
- [ ] Ringkasan sesi dirujuk pada Phase 12, tanpa mengubah status bukti produksi/keputusan fitur yang belum didukung.

Lima sesi ini memberi bukti awal kemudahan penggunaan. Perbandingan sebelum/sesudah perbaikan tetap mengikuti [uji konversi Phase 10](conversion-user-test.md) dengan lima sesi baru; retensi, jangkauan, dan kenaikan konversi produksi membutuhkan bukti tersendiri.
