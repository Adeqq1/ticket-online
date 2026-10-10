# Phase 25 — Validasi UI Publik

## Cakupan dan bukti lokal

Runner `frontend/scripts/public-ui-check.py --current --phase25` merekam halaman publik dengan respons API fixture, bukan data pembeli. Manifest dan screenshot lokal disimpan di `docs/phase25/screenshots/` (diabaikan Git seperti baseline fase sebelumnya).

| Pemeriksaan | Status |
|---|---|
| Chrome desktop, lebar 320, 360, 390, 768, 1024, dan 1440 px | Lulus; tidak ada overflow horizontal |
| Landscape 844 × 390 dan simulasi teks 200% | Lulus; semua halaman publik tetap dalam viewport |
| Target sentuh minimal 44px dan input 16px | Lulus pada halaman publik yang diperiksa |
| Dimensi poster dan kontras token light/dark | Lulus; tidak ada dimensi poster hilang, rasio token melewati ambang pemeriksaan |
| Keyboard/fokus, status live, dark mode, reduced motion, dan cetak | Lulus melalui alur browser; fokus layar kecil, status live, durasi gerak tereduksi, dan pemulihan detail tiket saat dicetak |
| Isolasi gaya admin | Lulus; enam screenshot desktop/mobile identik sebelum dan sesudah |
| `bun test`, `bun run check`, `bun run build`, dan `_redirects` | Lulus; 101 test, 0 error/warning, build sukses, `_redirects` tersalin identik |
| Chrome Android dan Safari iOS pada perangkat nyata | Belum dijalankan; perlu perangkat |
| Alur staging | Belum dijalankan; akses staging tidak tersedia |

## Checklist manual perangkat

- [ ] Chrome Android: menu, filter katalog, keyboard checkout, bilah aksi detail, pemulihan, dan pemindaian QR.
- [ ] Safari iOS: menu, filter katalog, keyboard checkout, safe area bilah aksi, pemulihan, dan tampilan QR.
- [ ] Orientasi landscape dan pembesaran teks pada perangkat.
- [ ] Alur lengkap di backend staging, termasuk pembayaran, batas waktu, tiket, perubahan acara, dan refund.

Jangan menandai pemeriksaan perangkat atau staging selesai tanpa hasil langsung dari lingkungan tersebut.
