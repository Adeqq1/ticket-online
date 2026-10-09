# Phase 15 - Pilih satu pengembangan

Status: **menunggu putaran uji, perbaikan, dan uji ulang; fitur belum dipilih**.

Contoh keputusan berbasis data sintetis tersedia pada [laporan simulasi Phase 12-15](phase12-15-simulation.md). Pilihan dalam latihan itu bukan keputusan pengembangan produk nyata.

Selesaikan [uji Phase 13](phase13-user-test.md) dan [perbaikan serta uji ulang Phase 14](phase14-targeted-improvements.md) terlebih dahulu. Setelah itu, gabungkan hasil sesi, laporan konversi, dan pertanyaan pelanggan untuk memilih satu pengembangan dengan alasan dan ukuran keberhasilan yang jelas. Jika hasilnya baik dan belum ada kebutuhan baru yang kuat, versi saat ini dapat dinyatakan selesai untuk cakupannya.

## Bukti yang dipakai

Ikuti definisi, periode WIB, dan batas interpretasi pada [evaluasi Phase 12](phase12-evaluation.md). Catat sumber, periode, lingkungan, versi, dan celah bukti; pisahkan sandbox dari produksi. Data yang belum tersedia tidak berarti nol masalah.

| Sumber | Ringkasan yang diperlukan | Hasil / rujukan |
|---|---|---|
| Sesi Phase 13 dan Phase 14 | Hambatan tersisa, frekuensi/dampak, keberhasilan mandiri, bantuan, dan perbandingan perangkat | Belum tersedia |
| Laporan konversi | Perjalanan matang, kehilangan antartahap, cakupan tanpa atribusi, event/perangkat, periode dan waktu snapshot | Belum tersedia |
| Pertanyaan pelanggan | Kasus unik pembayaran, refund, akses tiket, dan kategori lain pada periode yang disebutkan; pertanyaan berulang serta dampaknya | Belum tersedia |
| Pembelian berulang | Agregat produksi dari query Phase 12 dan bukti kesulitan mengelola riwayat/tiket | Belum tersedia |
| Kebutuhan promosi / penyelenggara | Frekuensi operasi promosi atau permintaan acara bernomor kursi yang terkonfirmasi, beserta konteks dan dampaknya | Belum tersedia |

Gunakan ringkasan anonim; jangan menyalin identitas pelanggan, percakapan mentah, token, QR, atau tautan akses order. Hitung satu masalah pesanan dalam satu percakapan sebagai satu kasus meskipun pesannya banyak. Jangan menjumlahkan peserta, perjalanan, pembeli, dan kasus dukungan sebagai satu ukuran.

## Kandidat dan ukuran keberhasilan

| Kandidat | Pilih jika bukti menunjukkan | Ukuran keberhasilan yang ditetapkan sebelum implementasi |
|---|---|---|
| Akun pembeli | Pembeli berulang pada data nyata kesulitan mengelola riwayat dan tiket | Proporsi pembeli lama yang menemukan riwayat/tiket tanpa bantuan meningkat; checkout tamu tetap dapat diselesaikan |
| Voucher terkelola | Promosi menjadi kebutuhan rutin dan perubahan kode promo dalam kode aplikasi menghambat operasi | Pengelola membuat, mengubah, dan menonaktifkan promo tanpa perubahan kode; waktu pengerjaan dan kesalahan aturan promo berkurang |
| Pusat bantuan terkait order | Pertanyaan pembayaran, refund, dan akses tiket berulang; observasi mendukung kesulitan menemukan jawaban sesuai status order | Proporsi tugas menemukan jawaban yang benar tanpa bantuan meningkat; rasio kasus dukungan kategori sasaran terhadap order terkait menurun pada periode sebanding |
| Pemilihan kursi | Penyelenggara mengonfirmasi kebutuhan tiket bernomor kursi untuk acara nyata, dengan denah, aturan alokasi, dan dampak operasional yang jelas | Pembeli memilih kursi dan petugas mengenali penempatan sesuai denah; tidak ada alokasi kursi ganda pada pengujian transaksi bersamaan |

Untuk setiap ukuran yang dipilih, isi baseline, target konkret, penyebut/populasi, cara ukur, dan periode evaluasi. Ukuran di tabel adalah arah evaluasi, bukan target yang sudah tercapai. Kode promo yang masih ditentukan dalam kode merupakan batas implementasi saat ini; kebutuhan rutin tetap harus dibuktikan. Pelacakan kampanye hanya masuk cakupan jika ada bukti kebutuhannya sendiri.

Jika beberapa kandidat didukung, bandingkan dampak, frekuensi dalam unit yang sama, kualitas bukti, serta usaha implementasi. Pilih satu; jelaskan alasan menunda kandidat lainnya. Dahulukan hambatan inti yang masih tersisa sebelum memperluas cakupan. Lima sesi tidak membuktikan retensi atau kenaikan konversi produksi.

## Catatan keputusan

| Butir | Isi |
|---|---|
| Status keputusan | Belum dapat dinilai |
| Hasil putaran Phase 13-14 dan rujukan bukti | Belum tersedia |
| Fitur terpilih atau alasan menutup cakupan | Belum ditetapkan |
| Masalah, pengguna terdampak, sumber dan periode bukti | Belum tersedia |
| Alasan prioritas dan kandidat yang ditunda | Belum ditetapkan |
| Cakupan minimum fitur | Belum ditetapkan |
| Baseline / target / populasi atau penyebut | Belum ditetapkan |
| Cara ukur / periode evaluasi | Belum ditetapkan |
| Penanggung jawab / tanggal tinjauan | Belum ditetapkan |

Checkpoint pengembangan terpenuhi setelah satu fitur dipilih dengan bukti, alasan, cakupan minimum, dan ukuran keberhasilan lengkap. Sinkronkan ringkasan keputusan ke Phase 12 dengan merujuk dokumen ini.

Keputusan **selesai untuk cakupan saat ini** juga sah jika hasil putaran uji baik, hambatan penting sudah terselesaikan, dan peninjauan kebutuhan tidak menemukan alasan kuat untuk fitur baru. Catat alasan dan kondisi yang memicu peninjauan kembali, misalnya promosi rutin atau permintaan acara bernomor kursi. Jangan menyamakan bukti yang belum dikumpulkan dengan tidak adanya kebutuhan. Penutupan cakupan produk tetap terpisah dari verifikasi staging dan kesiapan rollout pada [kebijakan perubahan acara](event-change-policy.md).
