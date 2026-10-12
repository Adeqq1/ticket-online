# Phase 51–55: efisiensi alur kerja admin

Verifikasi lokal: 11 Oktober 2026. Implementasi fase 51–54 dan pemeriksaan otomatis fase 55 selesai. Penerimaan akhir masih menunggu UAT dua admin; peserta belum tersedia, sehingga checkpoint manusia fase 50 juga tetap terbuka.

## Penyebab endpoint pricing gagal

Request langsung ke `http://127.0.0.1:8080/api/v1/admin/pricing` dan melalui proxy Vite `http://127.0.0.1:5173/api/v1/admin/pricing` sama-sama mengembalikan 404 `NOT_FOUND`, “Route not found”. Source sudah mendaftarkan route pricing melalui `backend/internal/pricing/pricing.go` dan `backend/internal/platform/server.go`.

Image API yang berjalan dibuat 10 Oktober 2026 pukul 19:56 WIB, sebelum commit pricing `f3feabf` pada 11 Oktober 2026 pukul 13:56 WIB. Menjalankan `docker compose up -d --build api` memperbarui backend lokal dari source yang sudah tersedia. Kedua alamat kemudian mengembalikan 401 `UNAUTHORIZED` tanpa token, bukan 404. GET pricing dengan autentikasi ADMIN serta save/reload diverifikasi terpisah pada backend nyata. Tidak ada perubahan source backend, endpoint, atau proxy default dalam diff ini.

## Perubahan produk

- Pricing memuat biaya/voucher/riwayat sebagai satu kelompok dan katalog konser secara terpisah. Kegagalan awal menggantikan formulir; kegagalan katalog hanya menahan voucher. Muat ulang mempertahankan draft dan data terakhir. Pesan keberhasilan tidak hilang bila refresh setelah save gagal.
- Hasil simpan dibedakan menjadi berhasil, ditolak, dan belum pasti. Hasil belum pasti menahan pengulangan sampai GET terbaru membuktikan nilai yang dikirim. Bila belum terbukti, tindakan tetap ditahan. Kunci berada dalam state halaman, tidak bertahan setelah reload penuh. Input pratinjau menerima rupiah bulat, memformat `100.000` saat blur, dan menolak input tidak valid; request tetap angka.
- Konser dimulai pada daftar penuh, dengan mode Tambah dan Edit terpisah. Nama mendahului ID, saran ID berhenti setelah edit manual, dan ID tersimpan tidak dapat diubah. Label Draf/Terbit/Diarsipkan memakai enum API yang sama. Kembali mempertahankan filter/posisi daftar dan mengembalikan fokus. Create membuka editor untuk melanjutkan zona/kategori sebelum publikasi.
- Pesanan memakai satu Cari, label Nomor pesanan, tabel desktop tujuh kolom, dan daftar berlabel di bawah 768px. Detail menggantikan daftar dan filter; Kembali ke daftar memulihkan hasil, filter, scroll, dan fokus. Cursor digabung menurut ID tanpa duplikasi, dan jumlah hanya menyebut pesanan yang sudah dimuat. Badge Kedaluwarsa memakai token peringatan; aturan refund tetap berlaku.
- Petugas memakai tampilan Daftar, Tambah, dan Edit yang saling menggantikan. Toolbar horizontal hanya muncul pada daftar. Mobile mempertahankan pencarian dan disclosure Filter lainnya beserta jumlah filter aktif. Checkbox berbunyi Hanya akun tanpa penugasan. Filter disimpan selama pengeditan dan dipulihkan saat kembali.

## Koreksi pemisahan tampilan

Atas klarifikasi pengguna, pola fase 52 diterapkan konsisten pada konser, petugas, dan pesanan. Konser sebelumnya sudah mengganti daftar dengan editor; judul halaman sekarang mengikuti mode dan tombol Tambah hanya tersedia pada daftar. Petugas sebelumnya berdampingan dan detail pesanan sebelumnya berada di bawah tabel; keduanya sekarang menggantikan area daftar sepenuhnya. Pesanan tetap hanya memiliki Daftar/Detail beserta tindakan yang sudah didukung.

Kembali ke daftar mempertahankan filter, hasil cursor, posisi scroll, dan fokus pemicu. Draft petugas/konser serta isian refund meminta konfirmasi sebelum dibuang. Kunci hasil refund yang belum pasti disimpan per pesanan selama halaman terbuka, sehingga membuka kembali detail tidak membolehkan mutasi ulang. Status pada baris daftar diperbarui dari detail terbaru tanpa membuang halaman cursor lain. Kunci ini tidak bertahan setelah reload penuh.

Pemeriksaan browser diperbarui untuk memastikan daftar/filter/tombol Tambah tidak ada dalam editor, fokus kembali, scroll kembali, serta penolakan pengulangan refund sesudah berpindah tampilan. Alur konser/pesanan dan petugas lulus pada 320/390/768/1440px. Detail pesanan juga lulus audit teks 200%; header membungkus tombol kembali pada 320px. Regresi admin khusus pesanan/petugas dan dua metode poster lulus. `bun test` (121 tes), `bun run check`, dan `bun run build` lulus.

Screenshot/audit koreksi berada di `/tmp/admin-separate-views/workflows/`, `/tmp/admin-separate-views/regression/{orders,staff}/`, serta artefak petugas di `/tmp/phase51-55/staff/` dan `/tmp/admin-separate-views/staff/`. Bukti awal di bagian berikut adalah catatan fase sebelumnya; koreksi ini memakai fixture browser dan tidak mengulang integrasi backend nyata. UAT manusia tetap belum dilakukan.

Screenshot pengguna kemudian menunjukkan UI lama pada `localhost:5173`. Alamat tersebut dilayani container `ticket-online-frontend-1` dengan salinan source dalam image, tanpa bind mount workspace; pemeriksaan sebelumnya memakai Vite terpisah pada `5175`. Container frontend diperbarui dengan `docker compose up -d --build --no-deps frontend`. Source map respons modul konser/petugas/pesanan pada `localhost:5173` kemudian dibandingkan dan sama persis dengan source workspace. Perubahan source berikutnya juga memerlukan rebuild container selama konfigurasi ini dipakai.

Pemeriksaan ulang browser pada `http://localhost:5173` lulus untuk alur konser/pesanan pada empat lebar dan petugas pada 1440px. Screenshot konser/pesanan tersimpan di `/tmp/admin-separate-views/live5173/`; skrip petugas memperbarui artefak `/tmp/phase51-55/staff/` pada lebar tersebut. Request pengujian ini memakai fixture, bukan mutasi data pengguna.

## Bukti otomatis

Perintah frontend dijalankan dari `frontend/`, dengan Vite uji pada `http://127.0.0.1:5175`:

```bash
bun test
bun run check
bun run build
python3 scripts/pricing-ui-check.py http://127.0.0.1:5175
python3 scripts/admin-ui-check.py --base-url http://127.0.0.1:5175 --workflows --phase 55 --output-dir /tmp/phase51-55/admin-workflows
for page in events orders staff; do
  python3 scripts/admin-ui-check.py --base-url http://127.0.0.1:5175 --only "$page" --phase 55 --phase35 --output-dir "/tmp/phase51-55/admin-regression/$page"
done
for width in 320 390 768 1440; do
  python3 scripts/staff-review-check.py http://127.0.0.1:5175 "$width"
done
python3 scripts/poster-ui-check.py --base-url http://127.0.0.1:5175
python3 scripts/public-ui-check.py --current --base-url http://127.0.0.1:5175 --output-dir /tmp/phase51-55/public-regression
```

| Pemeriksaan | Hasil / cakupan |
| --- | --- |
| Unit | 121 tes, 545 assertion, 29 berkas; semuanya lulus. Tiga tes baru mencakup parsing rupiah, keputusan hasil simpan, dan rekonsiliasi voucher. |
| Svelte/TypeScript dan build | 0 error, 0 warning; production build lulus. |
| Pricing browser | Lulus pada 320/390/768/1440px: kegagalan awal/parsial/refresh, format dan input invalid, konflik revisi, sukses diikuti refresh gagal, respons terputus, pemeriksaan hasil fee/voucher, dan submit ganda ditahan. CRUD/penggunaan voucher, penolakan STAFF, serta pemulihan checkout tetap diperiksa. |
| Konser/pesanan browser | Lulus pada empat lebar: fixture 20 konser, create → zona/kategori → terbit, draft batal/konfirmasi, saran ID/manual, fokus kembali, 22 pesanan unik dari dua halaman cursor yang sengaja tumpang tindih, tabel/daftar mobile, detail, dan pencarian kosong. |
| Petugas browser | Fixture 20 akun mencakup ADMIN, STAFF aktif/nonaktif/tanpa penugasan dan teks panjang. Create/profil/penugasan/status/reset password, reload fixture, kegagalan, perlindungan draft, filter AND, editor tersaring, dan kunci mutasi belum pasti lulus di empat lebar. Akun STAFF beserta Kelola terlihat tanpa scroll pada 1440×900. |
| Regresi admin | Masing-masing konser/pesanan/petugas lulus 12 skenario desktop/mobile, termasuk loading, kosong, gagal, sesi berakhir, dan akses ditolak; ditambah tablet, 320px, landscape, viewport pendek, dan pembesaran teks. Query `orderId` tetap membuka detail. |
| Poster dan scanner | Dua metode poster dan regresi publik terkait poster lulus di 1440/390/320px. Pemeriksaan scanner dalam skrip petugas lulus di empat lebar, termasuk fokus hasil, scan berikutnya, submit ganda, dan hasil belum pasti. |
| Kontras ter-render | Audit konser/pesanan: teks minimum 5,35:1, batas kontrol 4,49:1, fokus 10,52:1. Audit petugas: teks 6,81:1, batas 4,49:1, fokus 10,52:1. Pricing pada pembesaran teks 200%: teks 8,48:1 dan batas 4,49:1. Tanpa kegagalan audit/overflow pada skenario yang dicatat. |
| Regresi publik | 69 screenshot kondisi/alur publik berhasil direkam; pemeriksaan layout, font, tema, dan isolasi admin lulus. |

Pembesaran teks 200% dan keyboard virtual disimulasikan lewat ukuran teks serta viewport pendek di Chrome desktop. Keyboard OS pada perangkat fisik dan pengumuman screen reader oleh manusia belum diuji. Live region, label, urutan keyboard, fokus panel/detail dan pengembalian fokus diperiksa secara otomatis. Pemeriksaan ini tidak menggantikan UAT.

## Integrasi backend nyata

Lingkungan terpisah memakai database lokal `ticket_online_phase55_check`, akun ADMIN uji, API `127.0.0.1:8081`, dan Vite `127.0.0.1:5175` dengan `VITE_API_PROXY_TARGET=http://127.0.0.1:8081`. Skema memakai migrasi yang sudah tersedia. Data dummy hasil pengujian tetap berada di database uji.

```bash
# Kredensial akun uji diberikan melalui environment, bukan artefak hasil.
PHASE55_ISOLATED_TEST=true python3 scripts/admin-backend-check.py http://127.0.0.1:5175
```

Skrip juga memerlukan `PHASE55_EMAIL` dan `PHASE55_PASSWORD`. Browser tidak mengintersepsi API. Hasil lulus untuk login ADMIN, biaya save/reload, voucher create/reload, konser draf → zona/kategori → terbit/reload, serta petugas create dengan gate → edit profil → hapus penugasan → nonaktifkan/reload. Semua respons skenario berhasil, tanpa error JavaScript; nilai biaya sebelum skenario dikembalikan pada akhir pengujian yang lulus. Reset password dan fault injection browser memakai fixture, bukan integrasi ini.

Tes MySQL pricing juga lulus:

```bash
# Dari backend/, dengan MYSQL_TEST_DSN menunjuk database uji lokal.
go test -count=1 -run '^TestPricing' -v ./internal/platform
```

Tes tersebut mencakup autentikasi/role/sesi, save/reload pricing, konflik revisi, snapshot, kuota voucher, expiry, dan refund. Hasil backend nyata dicatat terpisah dari fixture browser.

## Artefak screenshot dan catatan

Artefak berada lokal di `/tmp/phase51-55/`; lokasi ini sementara dan perlu diarsipkan saat serah terima.

| Direktori | Bukti |
| --- | --- |
| `pricing/` | `initial-error-{width}.png`, `catalog-error-{width}.png`, `saved-{width}.png`, `text200-{width}.png`, `checks-{width}.json` |
| `admin-workflows/` | `workflow-events-list/create/edit-{width}.png`, `workflow-orders-list/detail/empty-{width}.png`, screenshot 200%, dan `workflow-{width}.json` berisi audit/mutasi |
| `staff/` | `roster20-list/create-open/event-gate-selection/manage/empty-search/filtered-editor-{width}.png`, fixture dan catatan audit/alur |
| `admin-regression/{events,orders,staff}/` | Screenshot kondisi serta viewport tambahan, `manifest.json` |
| `backend/` | `integration.json` dan screenshot pricing/konser/petugas setelah reload nyata |
| `public-regression/` | Screenshot dan manifest pemeriksaan halaman publik |

Screenshot daftar/editor desktop dan mobile ditinjau terpisah dari hasil tes. Screenshot tidak dipakai sebagai bukti bahwa mutasi tersimpan.

## Audit preservasi

Palet utama, font, logo, navigasi, URL/query, field API, enum publikasi/status, role dan izin ADMIN/STAFF dipertahankan. Penambahan warna hanya token semantik peringatan pada layout admin. Header ringkas hanya untuk empat halaman terkait. Badge pesanan menggunakan kelas khusus admin agar tidak berbenturan dengan `.order-status` publik; panel refund memakai permukaan admin. Tidak ada perubahan komponen publik, scanner, route, source backend, skema, dependency atau aturan bisnis. Regresi publik/scanner dan poster diperiksa; tidak dilakukan perbandingan piksel otomatis dengan baseline sebelum perubahan.

## UAT manusia — belum selesai

Dua peserta admin belum tersedia. Masing-masing perlu menjalankan tugas berikut secara terpisah, tanpa arahan urutan klik. Berikan hanya tujuan bisnis dan akun/data uji:

1. Ubah biaya admin ke nominal yang ditentukan, lalu periksa hasil setelah reload.
2. Buat voucher untuk konser tertentu, lalu jelaskan cakupan, satuan diskon, dan hasil penyimpanannya.
3. Buat konser baru, lengkapi zona/kategori, terbitkan; kemudian edit konser lain dan batalkan draft perubahan.
4. Temukan pesanan yang ditentukan, jelaskan statusnya, buka detail, lalu kembali ke hasil pencarian.
5. Temukan petugas, ubah penugasan gate, simpan dan periksa setelah reload; nonaktifkan akun uji yang ditentukan.

| Peserta | Tanggal/perangkat | Tugas selesai mandiri | Salah pengertian mode/status/simpan | Hambatan | Hasil |
| --- | --- | --- | --- | --- | --- |
| Admin 1 | Belum diisi | Belum diisi | Belum diisi | Belum diisi | Belum diuji |
| Admin 2 | Belum diisi | Belum diisi | Belum diisi | Belum diisi | Belum diuji |

Penerimaan akhir menunggu kedua admin menyelesaikan tugas tanpa salah memahami mode, status, atau hasil simpan. Kegagalan kritis pemuatan, penyimpanan, atau aksesibilitas menghalangi penerimaan.
