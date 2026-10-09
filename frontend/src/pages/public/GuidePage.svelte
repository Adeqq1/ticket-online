<script lang="ts">
  import Accordion from "../../components/Accordion.svelte";

  const purchaseSteps = [
    { question: "Bagaimana simulasi pembelian tiket berjalan?", answer: "Pilih konser dari katalog, buka detail acara, lalu pilih jenis tiket yang tersedia. Isi data pemesan, pilih metode pembayaran, dan tinjau kembali detail pesanan sebelum menekan tombol Buat pesanan." },
    { question: "Apakah pembayaran di website ini benar-benar diproses?", answer: "Tidak ada uang yang diproses. Metode QRIS, virtual account, dan GoPay hanya simulasi development, serta dinonaktifkan pada lingkungan lain." },
    { question: "Di mana saya menemukan e-ticket setelah memesan?", answer: "Setelah pembayaran dikonfirmasi server, ringkasan pesanan dan tautan e-ticket setiap peserta dikirim ke email pemesan. Kamu juga dapat membuka tiket tersimpan pada halaman Tiket Saya. Tunjukkan QR atau kode ET dan gate yang tercetak di tiket kepada petugas." }
  ];

  const paymentRules = [
    { question: "Metode pembayaran apa saja yang tersedia?", answer: "QRIS, virtual account, dan GoPay tersedia sebagai simulasi hanya pada lingkungan development yang mengaktifkannya. Pembayaran belum tersedia pada lingkungan lain." },
    { question: "Kapan pesanan dianggap selesai?", answer: "Order menjadi lunas setelah server mengonfirmasi pembayaran. Server menerbitkan satu e-ticket untuk setiap peserta. Pembayaran simulasi tidak memproses transaksi uang." }
  ];

  const refundRules = [
    { question: "Apakah tiket bisa dibatalkan atau di-refund?", answer: "Pembatalan acara memberi refund penuh untuk semua order lunas, termasuk biaya admin. Jika acara ditunda atau dijadwalkan ulang, pembeli lama dapat meminta refund penuh dari halaman pesanan sampai tenggat yang diumumkan. Kanal yang tidak mendukung refund otomatis ditangani manual oleh tim." },
    { question: "Bagaimana jika acara dibatalkan penyelenggara?", answer: "Jika acara dibatalkan, tiket tidak berlaku dan refund semua order lunas diproses. Jika jadwal berubah, QR tetap sama; jadwal terbaru dan hak refund tampil pada acara, order, dan tiket serta dikirim ke email pembeli." },
    { question: "Apakah tiket yang sudah dipindai masih bisa digunakan lagi?", answer: "Tiket yang sudah check-in tetap terpakai, termasuk setelah penjadwalan ulang. Riwayat tetap tersimpan dan QR tidak dapat digunakan untuk masuk lagi." }
  ];

  const wristbandSteps = [
    { question: "Apa yang perlu disiapkan sebelum menuju meja penukaran?", answer: "Pada versi produksi, buka e-ticket valid dan siapkan QR di layar ponsel sebelum menuju meja penukaran wristband. Untuk mencoba alur petugas, buka demo scanner di /admin/scan." },
    { question: "Bagaimana proses penukaran wristband di venue?", answer: "Pada versi produksi, tunjukkan QR valid kepada petugas untuk diverifikasi sebelum menerima wristband sesuai kategori tiket. Pada simulasi ini tidak ada pemindaian, verifikasi, atau check-in venue yang nyata." },
    { question: "Bolehkah menggunakan screenshot e-ticket?", answer: "Kode ET dapat disimpan atau dicetak sebagai PDF. Kode belum mendukung pemindaian QR; tunjukkan e-ticket pada browser yang menyimpan akses order." }
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
  <p class="guide-intro">Ikuti alur pembelian, pahami ketentuannya, dan bedakan fitur simulasi dari proses venue versi produksi.</p>
</section>

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
