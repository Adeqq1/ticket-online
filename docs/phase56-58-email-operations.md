# Phase 56–58 — Email dan Operasional

Tanggal verifikasi: 11 Oktober 2026. Phase 56–57 terimplementasi; Phase 58 terverifikasi otomatis dan melalui backend/SMTP lokal. **UAT manusia belum selesai**. Peserta dua admin belum tersedia; checkpoint UAT Phase 50/55 juga tetap terbuka.

| Cakupan | Aturan yang berlaku |
| --- | --- |
| Masalah | Semua job `FAILED` dengan `supersededBy = null`; penerima di daftar tetap memakai masking backend. |
| Operasional | Job gagal yang belum digantikan: `TICKETS` langsung dihitung; jenis lain minimal tiga percobaan, kecuali `tidak ada pesanan yang memenuhi syarat pemulihan` dan `permintaan pemulihan kedaluwarsa`. |
| Waktu daftar | Waktu respons halaman pertama diterima browser; halaman cursor berikutnya mempunyai waktu pemuatan tersendiri. Refresh gagal mempertahankan data dan waktu terakhir. |
| Waktu snapshot | `collectedAt` dari backend Operasional. Bukan waktu pemuatan daftar ataupun `updatedAt` job. Seluruh waktu ditampilkan dalam WIB. |
| Dijadwalkan ulang | Mutasi/antrian terbukti; belum membuktikan email terkirim. `SENT` menunjukkan layanan email menerima pesan; penerimaan oleh penampung dibuktikan terpisah. |

Daftar dan detail menjadi tampilan terpisah pada URL yang sama. Kembali mempertahankan hasil, scroll, dan fokus pemicu; jika job sudah keluar dari daftar, fokus ke judul daftar. Alasan yang tidak dikenali tetap netral dengan detail teknis di detail. Tombol retry mengikuti `canRetry`; hasil belum pasti tetap terkunci sampai status dan riwayat `RETRY` terbaru membuktikan penjadwalan.

| Jenis bukti | Hasil dan lokasi |
| --- | --- |
| Tes frontend | `bun test`: 125 lulus, 574 assertion. `bun run check`: tanpa error/peringatan. `bun run build`: lulus. [Rekaman pemeriksaan](phase56-58/checks.json). |
| Browser dengan fixture | 40 skenario pada 320, 390, 768, 1440px. Daftar/detail, fallback penerima, retry ditolak/429/401, klik ganda, respons terputus tanpa/dengan bukti, refresh gagal, detail gagal, sukses diikuti refresh gagal. [Hasil](phase56-58/browser-fixtures.json). |
| Aksesibilitas otomatis | Keyboard membuka detail dan memulihkan fokus/scroll; pesan hasil memakai live region. Audit warna yang dirender: teks minimal 4,5:1/14px, batas kontrol/fokus minimal 3:1. Daftar dan detail email diuji dengan pembesaran font 200%, tanpa overflow halaman. Pemeriksaan admin juga mencakup tablet, landscape, menu keyboard, dan simulasi viewport pendek. Ini bukan UAT pembaca layar atau keyboard virtual perangkat fisik. |
| Backend nyata | Paket Go `email`, `operations`, `adminissues`, `platform` lulus menggunakan MySQL `ticket_online_phase58_check`, termasuk tes integrasi yang memerlukan DSN. Data pesanan uji dibuat/dibayar melalui handler yang ada. |
| Browser → API → worker → SMTP | Browser pada `localhost:5173`, relay HTTP Playwright ke server Go terisolasi, tanpa fixture respons API. Kedua respons retry diputus setelah transaksi berhasil; browser memverifikasi status/riwayat tanpa POST ulang. ID lama untuk e-ticket dan job pengganti untuk pemulihan kemudian diterima SMTP lokal, disimpan sebagai `SENT`, dan keluar dari daftar setelah reload. [Rekaman HTTP](phase56-58/backend-browser.json). Status `0` menandai koneksi yang sengaja diputus. |
| Konsistensi metrik | Sebelum retry: tiga kegagalan dalam daftar, satu masuk pemantauan. Sesudah worker: satu kegagalan pemulihan yang dikecualikan tetap di daftar, metrik nol. Retry yang dilarang ditolak 409, pengulangan saat antrean ditolak 409, dan kegagalan baru dalam cooldown ditolak 429. |
| Runtime pengguna | Container frontend direbuild. Sumber Masalah, Operasional, dan helper retry yang disajikan `localhost:5173` cocok dengan workspace. [Pemeriksaan runtime](phase56-58/runtime.json). |
| UAT manusia | Belum dilakukan; tidak dinyatakan lulus berdasarkan screenshot atau tes otomatis. |

Bukti visual dengan fixture: [daftar desktop](phase56-58/fixture-list-1440.png), [daftar 320px](phase56-58/fixture-list-320.png), [detail mobile](phase56-58/fixture-detail-390.png), [detail 320px pada teks 200%](phase56-58/fixture-detail-320-text200.png).

Bukti visual backend nyata: [antrean ID lama](phase56-58/backend-same-id-queued.png), [antrean job pengganti](phase56-58/backend-replacement-queued.png), [snapshot sebelum](phase56-58/backend-operations-before.png), [snapshot sesudah](phase56-58/backend-operations-after.png), [satu kegagalan tersisa](phase56-58/backend-list-after.png). Screenshot ini mendukung bukti HTTP, database, dan penampung SMTP; bukan pengganti pengujian fungsional.

Audit preservasi: palet/token, font, logo, navigasi, route/query, field dan endpoint API, aturan retry/alert, skema database, serta izin ADMIN/STAFF tidak berubah. CSS global yang diubah hanya selector `.issue-workspace`; gaya tambahan berada dalam kedua halaman admin. Perubahan backend hanya pada tes. Tidak ada pengiriman ke pelanggan: worker pengujian memakai SMTP loopback dan database uji.

Untuk mengulang dari `frontend/`:

```sh
bun test
bun run check
bun run build
python3 scripts/email-ui-check.py
python3 scripts/admin-ui-check.py --only issues --phase 58 --phase35 --output-dir /tmp/phase56-58/issues
python3 scripts/admin-ui-check.py --only operations --phase 58 --phase35 --output-dir /tmp/phase56-58/operations
```

Dari `backend/`, dengan `MYSQL_TEST_DSN` menunjuk database MySQL kosong dan dapat dibuang:

```sh
go test -p 1 -count=1 ./internal/email ./internal/operations ./internal/adminissues ./internal/platform
PHASE58_BROWSER_URL=http://localhost:5173 go test -count=1 -run '^TestEmailFailureWebhookReplayAndCrossDeviceRecovery$' -v ./internal/platform
```

Tes browser backend memakai Chrome dan Playwright yang sudah tersedia. Harness mempercepat snapshot untuk pengujian; interval/aturan pemantauan aplikasi tidak diubah.

Lembar UAT berikut belum diisi. Berikan tujuan tugas kepada peserta tanpa arahan urutan klik. Catat penyelesaian mandiri, penjelasan mereka, hambatan, dan kesalahan pemahaman secara verbatim.

| Tugas tanpa petunjuk klik | Admin 1 | Admin 2 |
| --- | --- | --- |
| Temukan email bermasalah dan jelaskan alasan kegagalannya, termasuk penerima kosong. | Belum tersedia | Belum tersedia |
| Tentukan tindakan yang tersedia pada dua email dengan kewenangan retry berbeda. | Belum tersedia | Belum tersedia |
| Lakukan tindakan yang tersedia dan jelaskan apakah email sudah terkirim. | Belum tersedia | Belum tersedia |
| Pastikan hasil setelah worker memproses pengiriman dan halaman dimuat ulang. | Belum tersedia | Belum tersedia |
| Jelaskan mengapa daftar berisi satu email gagal sementara Operasional menunjukkan nol; bedakan waktu daftar, job, dan snapshot. | Belum tersedia | Belum tersedia |

Penerimaan manusia tetap terbuka sampai kedua admin menyelesaikan tugas tanpa salah memahami penjadwalan/pengiriman dan cakupan angka nol. Kegagalan kritis pemuatan, mutasi, atau aksesibilitas menghalangi penerimaan.

## Perapian teks setelah Phase 58

Penjelasan utama Masalah diringkas menjadi “Email yang gagal dikirim. Jumlahnya dapat berbeda dari Operasional.” Rincian job, kriteria pemantauan, serta perbedaan waktu daftar dan snapshot berada dalam “Apa yang dihitung?” yang tertutup saat awal. Label waktu menjadi “Terakhir berhasil dimuat” dan “Data Operasional per”; sumber waktunya tetap sama. Detail memakai istilah pengiriman, dan pesan penjadwalan tetap menjelaskan bahwa email belum tentu terkirim.

Verifikasi ulang: 125 tes frontend/574 assertion lulus, Svelte/TypeScript tanpa error atau peringatan, build lulus. Browser email lulus 40 skenario pada 320, 390, 768, dan 1440px, termasuk buka/tutup disclosure dengan Enter, kondisi awal tertutup, teks 200%, kontras fokus lime, serta pemulihan fokus/scroll. Pemeriksaan admin Masalah (16 capture) dan Operasional (14 capture) juga lulus, termasuk data lama saat refresh gagal. [Rekaman perapian](phase56-58/copy-checks.json); screenshot terbaru berada di `/tmp/phase56-58-copy/`.

Container frontend direbuild dan sumber kedua halaman di `localhost:5173` cocok dengan workspace. Tes backend/SMTP di atas merupakan bukti fase sebelumnya dan tidak dijalankan ulang untuk perubahan teks ini. UAT manusia tetap belum selesai.

## Urutan detail email

Detail email kini menampilkan masalah, langkah berikutnya, dan tindakan sebelum rincian pendukung. Pasangan label–nilai memakai grid dua kolom tanpa indentasi; label lebih redup dan nilai lebih terang. “Muat ulang detail” dijelaskan sebagai pemeriksaan status, dan arti status pengiriman disimpan dalam disclosure tertutup. Referensi pemulihan tanpa pesanan terkait diberi label “Referensi permintaan”, dengan keterangan bahwa nilainya adalah kode yang dimasukkan pengguna, bukan ID unik permintaan.

Permintaan kedaluwarsa dengan `canRetry = false` tidak menampilkan tombol pengiriman dan mengarahkan admin meminta pengguna membuat permintaan baru melalui alur Pulihkan tiket yang tersedia. Jika `canRetry = true`, endpoint retry existing membuat permintaan dan pengiriman pengganti; tombol diberi nama “Buat permintaan pemulihan baru”. Alasan backend yang menggabungkan penyebab tidak pasti ditampilkan secara netral, kecuali alasan spesifik tersedia. Tidak ada perubahan API, backend, izin, atau aturan retry.

Verifikasi frontend: 127 tes/586 assertion, check tanpa error/peringatan, build lulus. Browser menguji 44 skenario pada 320, 390, 768, 1440px, termasuk permintaan kedaluwarsa yang tidak dapat diproses ulang dan permintaan pengganti yang diizinkan, keyboard, fokus kembali, kontras, teks 200%, serta penguncian hasil belum pasti. Bukti perbaikan detail berada dalam [rekaman pengujian](phase56-58/email-detail-checks.json); screenshot fixture berada di `/tmp/email-detail-review/`. Backend/SMTP dan UAT manusia tidak dijalankan ulang; UAT manusia tetap terbuka.

## Penghapusan teks dan pemakaian badge existing

Sesuai pilihan terbaru pengguna, teks yang diblok di daftar dihapus: pengantar, waktu pemuatan, disclosure cakupan, dan jumlah hasil. Blok ringkasan detail beserta “Kembali ke daftar”, “Muat ulang detail”, dan keterangannya juga dihapus. Rincian pengiriman tetap ditampilkan; daftar dapat dibuka melalui navigasi Masalah. Tombol retry yang diizinkan backend tetap tersedia. Logika ringkasan yang tidak lagi dipakai dibuang.

Status email memakai gaya badge existing dari Pesanan: token peringatan untuk gagal, aktif untuk terkirim, dan netral untuk antrean/pemrosesan. Tidak ada penambahan palet atau CSS badge baru. Verifikasi: 126 tes/577 assertion, check tanpa error/peringatan, build lulus, serta 44 skenario browser di empat lebar layar dengan audit kontras dan teks 200%. [Bukti terbaru](phase56-58/email-copy-removal-checks.json); screenshot berada di `/tmp/email-copy-removal/`. Runtime `localhost:5173` diperbarui dan sumbernya cocok dengan workspace. UAT manusia tetap terbuka.

## Penghapusan bagian Operasional

Teks yang dipilih pengguna di Operasional dihapus: judul Penanganan yang tersedia, disclosure cakupan, pesan email nol, dan tautan daftar email gagal. Pada kondisi screenshot, seluruh kartu tersebut hilang. Tindak lanjut kasus pembayaran, refund, dan error API tetap muncul jika datanya memerlukan penanganan. Ringkasan metrik, kondisi pemantauan, waktu data, dan perlindungan data lama saat refresh gagal tetap tersedia. Verifikasi: 126 tes/577 assertion, check dan build lulus, serta 14 capture browser termasuk kondisi kosong, stale, 320px dan teks 200%. [Bukti terbaru](phase56-58/operations-copy-removal-checks.json). UAT manusia tetap terbuka.
