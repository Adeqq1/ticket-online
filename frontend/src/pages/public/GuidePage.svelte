<script lang="ts">
  import Accordion from "../../components/Accordion.svelte";

  const purchaseSteps = [
    { question: "Bagaimana cara memesan tiket?", answer: "Pilih konser dari katalog, buka detail acara, lalu pilih jenis tiket yang tersedia. Isi data pemesan, pilih metode pembayaran, dan tinjau kembali detail pesanan sebelum menekan tombol Buat pesanan." },
    { question: "Di mana saya menemukan e-ticket setelah memesan?", answer: "Setelah server mengonfirmasi pembayaran, e-ticket diterbitkan untuk setiap peserta. Buka tautan di email atau lihat Tiket Saya. Tunjukkan QR dan gate yang tercetak di e-ticket kepada petugas." },
    { question: "Bagaimana jika tiket tidak muncul di browser ini?", answer: "Pulihkan akses memakai email pembeli dan kode pesanan TO- dari email berjudul E-ticket pesanan. Kode pesanan berbeda dari kode e-ticket ET-." }
  ];

  const paymentRules = [
    { question: "Metode pembayaran apa saja yang tersedia?", answer: "Pilihan yang tersedia mengikuti metode yang ditampilkan saat checkout. Selesaikan pembayaran melalui kanal tersebut dan gunakan detail pembayaran dari halaman pesanan." },
    { question: "Kapan pesanan dianggap selesai?", answer: "Pesanan menjadi lunas setelah server mengonfirmasi pembayaran. E-ticket diterbitkan untuk setiap peserta setelah konfirmasi tersebut." }
  ];

  const refundRules = [
    { question: "Apakah tiket bisa dibatalkan atau di-refund?", answer: "Pembatalan acara memberi refund penuh untuk semua order lunas, termasuk biaya admin. Jika acara ditunda atau dijadwalkan ulang, pembeli lama dapat meminta refund penuh dari halaman pesanan sampai tenggat yang diumumkan. Kanal yang tidak mendukung refund otomatis ditangani manual oleh tim." },
    { question: "Bagaimana jika acara dibatalkan penyelenggara?", answer: "Jika acara dibatalkan, tiket tidak berlaku dan refund semua order lunas diproses. Jika jadwal berubah, QR tetap sama; jadwal terbaru dan hak refund tampil pada acara, order, dan tiket serta dikirim ke email pembeli." },
    { question: "Apakah tiket yang sudah dipindai masih bisa digunakan lagi?", answer: "Tiket yang sudah check-in tetap terpakai, termasuk setelah penjadwalan ulang. Riwayat tetap tersimpan dan QR tidak dapat digunakan untuk masuk lagi." }
  ];

  const wristbandSteps = [
    { question: "Apa yang perlu disiapkan sebelum menuju gate?", answer: "Buka e-ticket sebelum tiba di gate. Pastikan status tiket aktif, informasi acara terbaru sudah diperiksa, dan QR terlihat jelas di layar." },
    { question: "Bagaimana tiket diperiksa di venue?", answer: "Tunjukkan QR e-ticket kepada petugas di gate yang tercetak pada tiket. Jangan bagikan QR atau kode tiket kepada orang lain." },
    { question: "Bolehkah menyimpan atau mencetak e-ticket?", answer: "Gunakan tombol Cetak / Simpan PDF. QR, nama peserta, gate, jadwal, dan rincian tiket disertakan pada hasil cetak." }
  ];
</script>

<svelte:head>
  <title>Panduan Pembelian dan Penukaran | Tiket Online</title>
  <meta name="description" content="Pelajari alur pembelian tiket, metode pembayaran, ketentuan refund, dan cara menukar wristband di venue." />
</svelte:head>

<section class="guide-hero shell" aria-labelledby="guide-title">
  <div>
    <p class="eyebrow">PANDUAN TIKET</p>
    <h1 id="guide-title">Semua yang perlu kamu tahu sebelum datang.</h1>
  </div>
  <p class="guide-intro">Temukan cara mengakses tiket, menyelesaikan pembayaran, dan menyiapkan QR sebelum tiba di gate.</p>
</section>

<p class="guide-recovery shell"><a class="text-button" href="/pulihkan-tiket">Sudah membeli, tetapi tiket tidak muncul? Pulihkan tiket</a></p>

<nav class="guide-index shell" aria-label="Daftar isi panduan">
  <a href="#cara-kerja"><span>01</span> Cara kerja</a>
  <a href="#pembayaran"><span>02</span> Pembayaran</a>
  <a href="#refund"><span>03</span> Refund & pembatalan</a>
  <a href="#penukaran"><span>04</span> Penukaran wristband</a>
</nav>

<div class="guide-content shell">
  <section class="guide-section" id="cara-kerja" aria-labelledby="cara-kerja-title">
    <div class="guide-section-heading"><span>01</span><h2 id="cara-kerja-title">Cara kerja pembelian tiket.</h2></div>
    <Accordion name="Cara kerja" items={purchaseSteps} />
  </section>

  <section class="guide-section" id="pembayaran" aria-labelledby="pembayaran-title">
    <div class="guide-section-heading"><span>02</span><h2 id="pembayaran-title">Pembayaran yang jelas sejak awal.</h2></div>
    <Accordion name="Pembayaran" items={paymentRules} />
  </section>

  <section class="guide-section" id="refund" aria-labelledby="refund-title">
    <div class="guide-section-heading"><span>03</span><h2 id="refund-title">Ketentuan refund dan pembatalan.</h2></div>
    <Accordion name="Refund dan pembatalan" items={refundRules} />
  </section>

  <section class="guide-section" id="penukaran" aria-labelledby="penukaran-title">
    <div class="guide-section-heading"><span>04</span><h2 id="penukaran-title">Rencana penukaran QR menjadi wristband.</h2></div>
    <Accordion name="Penukaran wristband" items={wristbandSteps} />
  </section>
</div>
