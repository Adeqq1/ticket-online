# Phase 59–63: pencarian dan pembelian tiket

## Perubahan

Audit awal: Manrope lokal, aksen `#1358d8`, logo TO, radius 8px, URL dan anchor dipertahankan. Poster pada baseline Chrome 1440×900 berukuran 305×1100px akibat atribut tinggi HTML; nama/harga berada di bawah viewport. Perbaikan mempertahankan bingkai 4:5 dengan gambar absolut `contain`, maksimum lebar 260px, dan fallback teks berukuran sama. Mobile memakai poster 104×130px di kiri. Satu tautan meliputi kartu, termasuk penanda “Lihat tiket”; keadaan habis, perubahan acara, dan penjualan dihentikan tetap terbaca.

Pencarian berada dalam hero. Beranda dan katalog memakai `filterConcerts`, menggabungkan pencarian/kota/tanggal dengan AND. Rentang inklusif memakai tanggal kalender WIB dari `currentEvent.startsAt` bila ada, termasuk `null` untuk jadwal yang belum diumumkan. Tanpa tanggal, acara tersebut tetap terlihat; dengan tanggal, acara tersebut dikeluarkan. Rentang terbalik menampilkan pesan di input dan meminta perbaikan. Beranda menampilkan empat hasil, bertambah empat setiap kali, mengembalikan batas ke empat setelah pencarian dikirim, serta memindahkan fokus ke hasil baru.

Pencarian beranda dan katalog diterapkan setelah tombol Cari (ikon dengan teks “Cari” yang terlihat, tinggi minimum 48px) atau Enter digunakan. Input sementara tidak mengubah hasil. Reset muncul di area aksi yang sama setelah submit, termasuk pencarian kosong; tombol Reset pada hasil kosong memakai perilaku yang sama. Reset mengosongkan input/kriteria, validasi, serta pengurutan katalog, menyembunyikan Reset, dan memulihkan fokus pencarian. Tanggal/harga tidak valid mempertahankan hasil sebelumnya. Chip katalog menghapus kriteria yang bersangkutan langsung.

Sesi `ticket-online:catalog:v1` menyimpan kriteria yang sudah diterapkan, bukan edit sementara, dan mempertahankan nama key, filter, dan scroll. `dateFrom`/`dateTo` ditambahkan; sesi lama dibaca dengan tanggal kosong. Pengurutan tanggal juga menghormati jadwal terbaru yang kosong. Genre, batas harga, dan pengurutan tetap tersedia pada katalog.

Cara membeli menjadi tiga langkah. Metode pembayaran ditulis sebagai QRIS, virtual account, dan GoPay sesuai pilihan checkout, tanpa logo tiruan. E-ticket dijelaskan tersedia setelah pembayaran dikonfirmasi. Pembeli lama diarahkan ke `/tiket-saya` dan `/pulihkan-tiket`. Footer dan konfirmasi pesanan menyediakan Bantuan, Ketentuan pembelian, serta Kebijakan refund. `/panduan#ketentuan` dan dialog checkout membaca `concertTerms` yang sama; isi kebijakan tidak diubah. Tautan kontak `.example` dihapus sambil menunggu kanal operasional.

## Bukti otomatis dan visual

Dijalankan dari `frontend/`:

```sh
bun test
bun run check
bun run build
python3 scripts/discovery-ui-check.py
python3 scripts/public-ui-check.py --current --phase25 --output-dir /tmp/phase59-63-public
```

- Hasil lokal: 137 unit test lulus (661 assertion), Svelte/TypeScript tanpa error/warning, build Vite lulus dan `dist/_redirects` tersedia. Suite discovery lulus 53 pemeriksaan tanpa page error; suite publik menghasilkan 69 capture setelah seluruh assertion lulus.
- Unit test mencakup batas tengah malam WIB, tanggal awal/akhir inklusif, batas satu sisi, jadwal terbaru/null/tidak valid, AND semua filter, tahun kabisat, rentang terbalik, sesi baru/lama/rusak, serta filter/scroll.
- Suite discovery menyimpan fixture delapan konser dan hasil assertion dalam [browser-checks.json](phase59-63/browser-checks.json). Gambar diagnostik bertuliskan POSTER UJI memiliki rasio berbeda; fixture lain memiliki poster rusak, nama panjang, habis, ditunda tanpa jadwal, jadwal berubah, dibatalkan, serta penjualan dihentikan. Ini respons API yang diintersep, bukan katalog operasional atau aset resmi.
- Matrix Chrome: 320, 390, 768, 1024, 1366, 1440px; light/dark; normal dan pembesaran font/line-height 200%; disclosure/fokus keyboard dan reduced motion. Pembesaran font adalah simulasi teks, bukan pengujian zoom browser atau perangkat fisik. Nama/harga pertama dan pencarian diperiksa pada 1440×900. Pemulihan tanggal/kota/scroll diperiksa setelah kembali dari detail tanpa BFCache; suite publik juga memeriksa BFCache.
- Screenshot `phase59-63/home-*.png` merupakan bukti visual; assertion dan JSON merupakan bukti otomatis. Screenshot publik lengkap terbaru berada di `/tmp/search-reset-public-check`, terpisah dari artefak fase sebelumnya. Keduanya tidak membuktikan transaksi staging atau keberhasilan UAT manusia.

## Checkpoint yang masih terbuka

| Checkpoint | Status / kebutuhan |
| --- | --- |
| Poster resmi/berizin | Belum diberikan. Penggantian gambar generik harus memakai pengelolaan poster admin yang ada, disertai sumber/izin per aset; belum ada data operasional yang diubah. Fallback tidak memakai foto pengganti. |
| Kontak operasional | Belum diberikan. Tidak ada klaim kontak sudah dapat dihubungi. Setelah kanal diberikan, pasang di footer/panduan dan catat pengiriman serta respons nyata. |
| Domain, sertifikat, redirect dan renewal | Belum ada domain/akses deployment. File Caddy yang ada mengaktifkan automatic HTTPS dan Compose menyimpan `/data` serta `/config`; konfigurasi bukan bukti sertifikat/renewal aktual. |
| Mixed content dan SPA HTTPS | Audit kode: API memakai URL relatif; origin email/media/kembali pembayaran berasal dari `FRONTEND_URL`; production memvalidasi HTTPS. URL poster eksternal lama masih dapat berisi HTTP dan wajib diperiksa/diganti melalui admin. Reload HTTPS aktual belum diuji. |
| Transaksi sandbox staging | Belum ada URL/kredensial sandbox. Alur browser dengan fixture tidak menutup checkpoint ini. |
| UAT dua calon pembeli | Belum ada peserta/hasil. Gunakan lembar di bawah; hasil tidak boleh diisi dari agen atau screenshot. |
| Production | Tetap gunakan `TRANSACTIONS_ENABLED=false` sampai verifikasi deployment, pembayaran dan kontak selesai. Default production dan konfigurasi yang ada tidak diaktifkan oleh perubahan ini. |

## Prosedur verifikasi deployment dan staging

Gunakan `.env.staging.example`, `compose.staging.yaml`, dan `deploy/Caddyfile.staging` yang ada. Isi domain dan secret melalui environment operator; staging memakai Midtrans sandbox, database/SMTP terpisah. Pastikan DNS domain menuju host, port 80/443 terbuka, serta volume Caddy persisten. Jangan menyalin secret ke bukti.

1. Periksa `http://DOMAIN/konser` mengalihkan ke `https://DOMAIN/konser`. Periksa sertifikat dengan hostname/SNI, rantai valid dan tanggal berlaku; jangan memakai opsi yang menonaktifkan validasi TLS. Catat tanggal, issuer, serial/fingerprint dan expiry.
2. Buka langsung lalu reload `/`, `/konser`, satu URL detail, `/panduan#refund`, `/tiket-saya`, dan `/pulihkan-tiket` melalui HTTPS. Periksa respons dokumen, pemuatan aset, API, Console/Network, serta tidak adanya permintaan HTTP. Pastikan `frontend/dist/_redirects` tersedia untuk hosting statis; backend static handler juga menyediakan SPA fallback.
3. Di admin yang sudah tersedia, unggah/simpan poster resmi melalui origin HTTPS atau URL berizin HTTPS. Periksa setiap URL katalog, respons media, dan fallback rusak. Jangan menanam pemetaan aset ke frontend.
4. Jalankan pencarian kota/tanggal → detail → pilih tiket → checkout → Snap sandbox. Catat harga/subtotal/biaya dari server, ID pesanan, metode, URL provider HTTPS dan URL kembali HTTPS. Selesaikan pembayaran sandbox; verifikasi notifikasi valid diterima, order menjadi PAID, dan e-ticket muncul setelah konfirmasi. Periksa QR, nama, jadwal terbaru, email, `/tiket-saya`, serta pemulihan pada browser lain. Redaksi token/email peserta pada bukti.
5. Periksa tautan email e-ticket, pemulihan dan perubahan acara menggunakan HTTPS. Uji Bantuan, Ketentuan pembelian dan refund sebelum membuat pesanan tanpa kehilangan isi form.
6. Uji kanal dukungan dengan pesan percobaan yang disetujui pemilik, lalu catat bukti diterima dan respons. Jangan mengirim pesan ke pihak lain tanpa izin.
7. Catat log Caddy penerbitan/pembaruan sertifikat dan jadwal pemantauan expiry. Verifikasi pembaruan aktual saat terjadi atau lakukan latihan renewal terkontrol memakai CA staging dan penyimpanan terpisah; jangan menghapus sertifikat aktif atau memicu rate limit production. Simpan hasil sebagai bukti terpisah dari sekadar konfigurasi automatic HTTPS.
8. Simpan bukti HTTP/API/provider/worker dan tiket dengan timestamp. Beri status lulus hanya setelah rangkaian selesai. Production tetap nonaktif sampai pemeriksaan tersebut ditutup.

## Lembar UAT manusia

Minta dua calon pembeli yang belum melihat implementasi. Mulai masing-masing dari beranda; baca tujuan tugas saja, tanpa menyebut label tombol atau lokasi klik. Gunakan staging berisi minimal delapan konser dan daftar harga/jadwal yang diketahui fasilitator. Untuk fixture otomatis, tanggal uji adalah 24 Agustus 2027 WIB; jangan memakai fixture sebagai bukti peserta sungguhan.

| Tugas | Instruksi kepada peserta | Catatan fasilitator |
| --- | --- | --- |
| Kota/tanggal | Temukan konser di Jakarta pada 24 Agustus 2027. | Fixture 1 dan 5 cocok; fixture 5 memakai jadwal terbaru. |
| Bandingkan harga | Bandingkan konser di Bandung dan pilih harga mulai terendah. Jelaskan apakah tiketnya dapat dibeli. | Bedakan harga, status stok, dan penjualan dihentikan. |
| Seluruh hasil | Temukan Konser Uji 8 dari beranda dan jelaskan status penjualannya. | Berada di luar empat pertama; penjualan dihentikan. |
| Refund | Cari ketentuan refund jika konser dibatalkan atau jadwal berubah. | Peserta menemukan kebijakan dan dapat menjelaskan hak/tenggat sesuai isi. |
| Tiket lama | Anda sudah membeli tiket. Tunjukkan bagaimana membuka tiket dan memulihkan akses di browser lain. | Catat apakah jalur pembelian baru tertukar dengan tiket lama. |
| Penerbitan tiket | Jelaskan kapan e-ticket tersedia setelah melakukan pesanan. | Jawaban harus menyebut pembayaran dikonfirmasi. |

Isi satu baris per tugas per peserta. Mulai waktu saat instruksi selesai dibaca; akhiri saat tujuan tercapai atau peserta berhenti. Catat bantuan sekecil apa pun dan kutip salah pengertian secara apa adanya.

| Peserta | Perangkat/viewport | Tugas | Berhasil tanpa bantuan | Waktu (detik) | Salah pengertian | Bantuan yang diminta/diberikan |
| --- | --- | --- | --- | --- | --- | --- |
| P1 | Belum tersedia | Belum dijalankan | — | — | — | — |
| P2 | Belum tersedia | Belum dijalankan | — | — | — | — |

Checkpoint akhir tetap terbuka sampai kedua peserta menyelesaikan pencarian dan seluruh hasil tanpa bantuan, tidak keliru mengenai harga/ketersediaan/penerbitan tiket, tidak menemukan aksi tertutup atau overflow, dan rangkaian transaksi terbukti pada backend staging nyata.
