"""Deterministic synthetic study; no app, database, provider, or human sessions.

Run from the repository root:
  python3 scripts/simulate-phase12-15.py > docs/phase12-15-simulation.md
  python3 scripts/simulate-phase12-15.py --check
"""

import argparse
from pathlib import Path
from statistics import median


# ponytail: fixed fictional cohorts; replace fixtures with reviewed observations
# for a real study, keeping synthetic results separate from production evidence.
TASKS = {"T1": "Checkout tamu", "T2": "Tiket lama", "T3": "Pemulihan", "T4": "Pembayaran/refund"}
DEVICES = ["Mobile", "Mobile", "Mobile", "Desktop", "Desktop"]
FUNNEL = {"Mobile": [600, 360, 240, 216, 180], "Desktop": [400, 280, 224, 212, 190]}
SUPPORT = {"Pembayaran": 22, "Email/pemulihan": 14, "Akses tiket": 8, "Jadwal/refund": 10, "Check-in": 4, "Lainnya": 2}
# Each row: outcome, elapsed seconds, assistance, obstacle, fictional observation.
# Rows follow T1-T4; assisted failures remain failures and count as needing help.
BASELINE = [
    [("gagal", 420, True, "H1", "Menganggap pending sudah lunas; berhenti meski status dijelaskan."),
     ("berhasil", 65, False, "-", "Menemukan tiket lama yang sesuai."),
     ("gagal", 300, True, "H2", "Tidak menemukan reference uji meski diarahkan ke email; menyerah."),
     ("gagal", 180, True, "H3", "Menganggap refund langsung cair setelah diminta; jawaban tetap keliru setelah bantuan.")],
    [("berhasil", 360, True, "H1", "Meminta penjelasan pending; melanjutkan hingga tiket terbuka."),
     ("berhasil", 80, False, "-", "Memilih tiket lama dari daftar."),
     ("berhasil", 240, True, "H2", "Menemukan reference setelah ditunjukkan letaknya di email uji."),
     ("gagal", 170, True, "H3", "Tidak dapat membedakan tenggat jadwal baru dan penundaan setelah bantuan.")],
    [("berhasil", 280, False, "-", "Menunggu konfirmasi dan membuka tiket."),
     ("berhasil", 70, False, "-", "Menemukan tiket lama."),
     ("berhasil", 210, True, "H2", "Meminta petunjuk masuk alur pemulihan, lalu berhasil."),
     ("berhasil", 160, True, "H3", "Menjawab hak refund dengan benar setelah aturan dijelaskan.")],
    [("berhasil", 240, False, "-", "Checkout dan tiket selesai mandiri."),
     ("berhasil", 50, False, "-", "Menemukan tiket lama."),
     ("berhasil", 120, False, "-", "Memulihkan akses melalui mailbox uji."),
     ("berhasil", 150, True, "H3", "Memerlukan penjelasan perbedaan pengajuan dan refund selesai.")],
    [("berhasil", 250, False, "-", "Checkout dan tiket selesai mandiri."),
     ("berhasil", 55, False, "-", "Menemukan tiket lama."),
     ("berhasil", 130, False, "-", "Memulihkan akses mandiri."),
     ("berhasil", 100, False, "-", "Menjelaskan status dan seluruh hak refund dengan tepat.")],
]
FOLLOWUP = [
    [("berhasil", 270, False, "-", "Membedakan pending/lunas dan membuka tiket."),
     ("berhasil", 62, False, "-", "Menemukan tiket lama."),
     ("berhasil", 150, False, "-", "Menemukan alur, reference, dan tiket."),
     ("berhasil", 110, False, "-", "Menjelaskan status, hak, dan tenggat dengan tepat.")],
    [("berhasil", 260, False, "-", "Menunggu konfirmasi sebelum membuka tiket."),
     ("berhasil", 75, False, "-", "Menemukan tiket lama."),
     ("berhasil", 190, True, "H2", "Masih memerlukan petunjuk menemukan reference di email."),
     ("berhasil", 115, False, "-", "Membedakan pengajuan dan refund selesai.")],
    [("berhasil", 255, False, "-", "Checkout selesai mandiri."),
     ("berhasil", 68, False, "-", "Menemukan tiket lama."),
     ("berhasil", 145, False, "-", "Memulihkan akses mandiri."),
     ("berhasil", 140, True, "H3", "Meminta penjelasan tenggat jadwal baru; kemudian menjawab benar.")],
    [("berhasil", 230, False, "-", "Checkout selesai mandiri."),
     ("berhasil", 48, False, "-", "Menemukan tiket lama."),
     ("berhasil", 110, False, "-", "Memulihkan akses mandiri."),
     ("berhasil", 90, False, "-", "Menjelaskan seluruh contoh status dan refund dengan tepat.")],
    [("berhasil", 235, False, "-", "Checkout selesai mandiri."),
     ("berhasil", 52, False, "-", "Menemukan tiket lama."),
     ("berhasil", 115, False, "-", "Memulihkan akses mandiri."),
     ("berhasil", 95, False, "-", "Menjelaskan seluruh contoh status dan refund dengan tepat.")],
]


def stats(rows):
    tested = [row for row in rows if row[0] != "tidak dapat diuji"]
    independent = [row for row in tested if row[0] == "berhasil" and not row[2]]
    return {
        "n": len(tested),
        "independent": len(independent),
        "assisted": sum(row[0] == "berhasil" and row[2] for row in tested),
        "help": sum(row[2] for row in tested),
        "failed": sum(row[0] == "gagal" for row in tested),
        "median": median(row[1] for row in independent) if independent else None,
    }


def ratio(n, total):
    return f"{n}/{total} ({n / total:.1%})" if total else "belum tersedia"


def table(headers, rows):
    return "\n".join([
        "| " + " | ".join(headers) + " |",
        "|" + "|".join("---" for _ in headers) + "|",
        *("| " + " | ".join(str(value) for value in row) + " |" for row in rows),
    ])


def session_table(cohort, prefix):
    return table(
        ["Sesi", "Perangkat", "Tugas", "Hasil", "Detik", "Bantuan", "Hambatan", "Observasi sintetis"],
        [(f"{prefix}{i + 1}", DEVICES[i], task, outcome, seconds, "Ya" if help_ else "Tidak", obstacle, note)
         for i, session in enumerate(cohort)
         for task, (outcome, seconds, help_, obstacle, note) in zip(TASKS, session)],
    )


def validate():
    for cohort in (BASELINE, FOLLOWUP):
        assert len(cohort) == 5
        for session in cohort:
            assert len(session) == 4
            for outcome, seconds, help_, obstacle, note in session:
                assert outcome in {"berhasil", "gagal", "tidak dapat diuji"}
                assert seconds > 0 and isinstance(help_, bool) and note
                assert obstacle in {"-", "H1", "H2", "H3"}
    assert DEVICES.count("Mobile") == 3 and DEVICES.count("Desktop") == 2
    for stages in FUNNEL.values():
        assert all(a >= b >= 0 for a, b in zip(stages, stages[1:]))
    assert sum(stages[-1] for stages in FUNNEL.values()) + 30 == 400
    assert 280 + 60 == 340 and 280 + 2 * 60 == 400
    assert sum(SUPPORT.values()) == 60
    before = stats([row for session in BASELINE for row in session])
    after = stats([row for session in FOLLOWUP for row in session])
    assert (before["independent"], before["help"], before["failed"]) == (11, 9, 4)
    assert (after["independent"], after["help"], after["failed"]) == (18, 2, 0)
    for task_index, target in ((0, 5), (2, 4), (3, 4)):
        result = stats([session[task_index] for session in FOLLOWUP])
        assert result["independent"] >= target and result["help"] <= 5 - target
        assert result["failed"] == 0
    for task_index in range(4):
        for device in set(DEVICES):
            old, new = [stats([session[task_index] for i, session in enumerate(cohort)
                               if DEVICES[i] == device]) for cohort in (BASELINE, FOLLOWUP)]
            assert new["independent"] >= old["independent"] and new["failed"] <= old["failed"]
    # Regression check: assisted failures count as help; unavailable tasks do not
    # enter denominators, and only independent successes enter this median.
    edge = stats([("gagal", 90, True), ("tidak dapat diuji", 20, False),
                  ("berhasil", 40, False), ("berhasil", 60, False), ("berhasil", 80, True)])
    assert edge == {"n": 4, "independent": 2, "assisted": 1, "help": 2, "failed": 1, "median": 50}
    assert stats([])["median"] is None and ratio(0, 0) == "belum tersedia"


def report():
    total = [sum(stages[i] for stages in FUNNEL.values()) for i in range(5)]
    funnel_rows = []
    for device, stages in {**FUNNEL, "Total": total}.items():
        funnel_rows.append([device, *stages, ratio(stages[-1], stages[0])])
    conversion_rows = [[f"{a} -> {b}", ratio(total[i + 1], total[i]), total[i] - total[i + 1]]
                       for i, (a, b) in enumerate(zip(
                           ["Detail", "Reservasi", "Order", "Sesi pembayaran"],
                           ["Reservasi", "Order", "Sesi pembayaran", "Lunas"]))]
    comparison = []
    for task_index, task in enumerate(TASKS):
        for device in ("Mobile", "Desktop", "Total"):
            both = [stats([session[task_index] for i, session in enumerate(cohort)
                           if device == "Total" or DEVICES[i] == device])
                    for cohort in (BASELINE, FOLLOWUP)]
            cells = [f"{task} / {device}"]
            for metric in ("independent", "assisted", "help", "failed"):
                cells.append(" -> ".join(ratio(s[metric], s["n"]) for s in both))
            cells.append(" -> ".join(f"{s['median']} (n={s['independent']})" if s["median"] is not None
                                     else "belum tersedia" for s in both))
            comparison.append(cells)
    obstacle_rows = []
    for obstacle, task, impact in [("H1", "T1", "Tinggi"), ("H2", "T3", "Tinggi"), ("H3", "T4", "Tinggi")]:
        cells = [obstacle, task, impact]
        for device in ("Total", "Mobile", "Desktop"):
            values = []
            for cohort in (BASELINE, FOLLOWUP):
                rows = [session[list(TASKS).index(task)] for i, session in enumerate(cohort)
                        if device == "Total" or DEVICES[i] == device]
                values.append(ratio(sum(row[3] == obstacle for row in rows), stats(rows)["n"]))
            cells.append(" -> ".join(values))
        obstacle_rows.append(cells)
    before, after = [stats([row for session in cohort for row in session]) for cohort in (BASELINE, FOLLOWUP)]
    target_support = sum(SUPPORT[key] for key in ("Pembayaran", "Email/pemulihan", "Akses tiket", "Jadwal/refund"))
    return f"""# Laporan simulasi Phase 12-15

**SIMULASI SINTETIS - bukan observasi pengguna, hasil staging, atau data produksi.**

Seluruh angka, peserta, kasus dukungan, dan perilaku sebelum/sesudah di laporan ini dibuat untuk latihan evaluasi. Tidak ada pembayaran, email, sesi browser, atau perubahan aplikasi yang dijalankan. Perbaikan Phase 14 adalah varian hipotetis; hasilnya ditentukan dalam fixture, bukan prediksi maupun bukti efektivitas. Checkpoint nyata pada dokumen Phase 12-15 tetap terbuka.

## Ringkasan keputusan simulasi

Dalam skenario ini, tiga hambatan diprioritaskan: H1 pemahaman status pembayaran, H2 pemulihan tiket, dan H3 pemahaman refund. Keberhasilan mandiri naik dari {ratio(before['independent'], before['n'])} menjadi {ratio(after['independent'], after['n'])} tugas; kebutuhan bantuan turun dari {ratio(before['help'], before['n'])} menjadi {ratio(after['help'], after['n'])}. Angka ini mencakup 20 tugas per kelompok, bukan 20 peserta independen.

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
| Pembeli berulang | {ratio(60, 340)}; tidak ada riwayat sebelum periode dalam model |
| Perjalanan matang / belum matang | 1.000 / 0; snapshot lebih dari 24 jam setelah akhir cohort |
| Pembayaran teratribusi / tanpa atribusi | 370 / 30; total 400, pembayaran tanpa atribusi tidak masuk rasio funnel |
| Refund selesai | 12 dari 400 order lunas; tetap dihitung dalam riwayat pembayaran |
| Konteks perubahan acara | 12 refund setelah jadwal pengganti SIM-E1; bukan pembatalan, sehingga pembayaran baru masih mungkin |
| Sasaran akhir funnel untuk latihan | Minimal 35%, ditetapkan sebagai asumsi sebelum evaluasi; tercapai 37% |
| Jangkauan / sasaran jangkauan | Belum tersedia / belum ditetapkan; perjalanan bukan orang unik |

{table(['Perangkat', 'Detail', 'Reservasi', 'Order', 'Sesi pembayaran', 'Lunas', 'Konversi akhir'], funnel_rows)}

{table(['Transisi', 'Konversi tahap', 'Kehilangan perjalanan'], conversion_rows)}

Kehilangan terbesar adalah detail ke reservasi: 360 perjalanan. Ini lokasi kehilangan, bukan bukti penyebab. Sesi sintetis tidak menemukan hambatan di pemilihan event/tiket; penyebab kehilangan awal tetap belum diketahui. H1 terjadi lebih jauh di alur dan tidak boleh dianggap menjelaskan seluruh kehilangan tersebut. Perangkat mobile memiliki konversi akhir 30%, desktop 47,5%; komposisi perangkat harus dipertahankan saat membandingkan.

{table(['Kategori dukungan', 'Kasus sintetis'], list(SUPPORT.items()) + [('Total', sum(SUPPORT.values()))])}

Model dukungan memiliki satu percakapan per order yang berbeda, seluruhnya dari 400 order tersebut, dan satu kategori utama per percakapan. Kategori pembayaran/email/akses/refund berjumlah {ratio(target_support, sum(SUPPORT.values()))} kasus dan {ratio(target_support, 400)} order lunas. Kasus pembayaran diasumsikan menanyakan status pending sebelum akhirnya lunas. Karena tidak mencakup order yang tidak pernah lunas, rasio ini tidak mewakili semua permintaan dukungan checkout.

Kebutuhan tambahan fiktif: dua permintaan promo pada bulan itu, tidak ada jadwal promosi rutin atau hambatan operasi yang terkonfirmasi; dua penyelenggara fiktif hanya memerlukan tiket tanpa nomor kursi. Data ini hanya alasan penundaan dalam skenario, bukan klaim kondisi bisnis sebenarnya.

Keputusan awal Phase 12 simulasi: lanjutkan observasi dan perbaikan; jangan langsung memilih akun hanya dari 17,6% pembeli berulang. Status Phase 12 nyata tetap **bukti belum cukup**.

## Phase 13 - Lima sesi baseline sintetis

Semua sesi berikut memakai SIM-A, SIM-E1, dan SIM-F. Instruksi, aturan bantuan, dan definisi keberhasilan mengikuti [Phase 13](phase13-user-test.md). Durasi gagal adalah waktu sampai penghentian dan tidak dipakai sebagai waktu keberhasilan. Seluruh catatan berikut merupakan perilaku yang dikarang, bukan kutipan peserta.

{session_table(BASELINE, 'U')}

H1 didukung U1/T1 dan U2/T1; H2 oleh U1/T3, U2/T3, U3/T3; H3 oleh U1-U4/T4. Dampak awal ketiganya tinggi karena kegagalan tugas atau salah pemahaman status/hak. T2 selesai mandiri pada seluruh peserta, sehingga kebutuhan akun belum didukung oleh tugas ini. Tidak ada tugas terhalang layanan.

## Phase 14 - Tiga perbaikan hipotetis dan lima sesi baru

Urutan prioritas: H3 (4/5), H2 (3/5), H1 (2/5), seluruhnya berdampak tinggi pada baseline. Pemilik peran yang diusulkan: produk menetapkan salinan dan kriteria; frontend menerapkan; QA memeriksa perilaku. Belum ada penugasan orang atau perubahan kode.

| ID | Varian SIM-B yang dimodelkan | Kriteria yang diasumsikan ditetapkan sebelum uji ulang |
|---|---|---|
| H3 | Perjelas pemisahan hak, tenggat, dan progres refund pada konteks order | T4 minimal 4/5 mandiri, maksimal 1/5 memerlukan bantuan, tidak ada jawaban salah pada akhir tugas |
| H2 | Perjelas jalur pemulihan dan contoh letak reference pada email uji | T3 minimal 4/5 mandiri, maksimal 1/5 memerlukan bantuan, semua dapat membuka tiket |
| H1 | Perjelas pending vs lunas dan langkah berikutnya saat menunggu konfirmasi | T1 5/5 mandiri, tanpa bantuan atau gagal; status tetap mengikuti backend |

Aplikasi sudah memiliki informasi status dan tautan pemulihan; hipotesisnya adalah kemudahan menemukan/memahami informasi tersebut. Ini bukan temuan bahwa fitur tersebut tidak ada. Varian tidak menghapus verifikasi akses, validasi checkout/tiket, aturan refund, fokus, live region, atau dukungan perangkat. Verifikasi teknis varian **belum dijalankan** karena varian belum diimplementasikan. Saat benar-benar diterapkan, jalankan tes terkait, `bun run check`, dan `bun run build` dari frontend, serta uji layanan yang terdampak.

{session_table(FOLLOWUP, 'V')}

### Perbandingan per tugas dan perangkat

Semua sel menampilkan sebelum -> sesudah. Penyebut adalah peserta yang dapat menguji tugas terkait; tidak ada pengecualian dalam fixture. Membutuhkan bantuan termasuk peserta yang akhirnya gagal. Median hanya dari keberhasilan mandiri; sampel sebelum/sesudah berbeda, jadi ini deskripsi kelompok, bukan perubahan waktu orang yang sama.

{table(['Tugas / perangkat', 'Mandiri', 'Berhasil dibantu', 'Memerlukan bantuan', 'Gagal', 'Median mandiri detik (n)'], comparison)}

### Frekuensi hambatan

{table(['ID', 'Tugas', 'Dampak baseline', 'Total', 'Mobile', 'Desktop'], obstacle_rows)}

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
"""


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="Validate fixtures and the saved report")
    args = parser.parse_args()
    validate()
    rendered = report()
    if args.check:
        path = Path(__file__).resolve().parents[1] / "docs/phase12-15-simulation.md"
        assert path.read_text() == rendered, "Saved report differs; regenerate it with the documented command"
        print("OK: fixtures, reconciliation, edge cases, and saved report")
    else:
        print(rendered, end="")
