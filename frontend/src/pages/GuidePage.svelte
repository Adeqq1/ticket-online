<script lang="ts">
  import Accordion from "../components/Accordion.svelte";

  const purchaseSteps = [
    { question: "Bagaimana simulasi pembelian tiket berjalan?", answer: "Pilih konser dari katalog, buka detail acara, lalu pilih jenis tiket yang tersedia. Isi data pemesan, pilih metode pembayaran, dan tinjau kembali detail pesanan sebelum menekan tombol Buat pesanan." },
    { question: "Apakah pembayaran di website ini benar-benar diproses?", answer: "Belum. Tiket Online saat ini adalah frontend simulasi. Pilihan QRIS, virtual account, dan GoPay hanya menggambarkan alur pembayaran yang akan tersedia pada versi produksi." },
    { question: "Di mana saya menemukan e-ticket setelah memesan?", answer: "Setelah pesanan simulasi selesai, e-ticket demo tersimpan di halaman Tiket saya pada browser yang sama. QR dan reference di halaman ini hanya visual demo dan tidak dapat digunakan untuk masuk event." }
  ];

  const paymentRules = [
    { question: "Metode pembayaran apa saja yang tersedia?", answer: "Simulasi checkout menampilkan QRIS, virtual account, dan GoPay. Ketersediaan metode pembayaran dapat berbeda pada setiap acara di versi produksi." },
    { question: "Kapan pesanan dianggap selesai?", answer: "Pesanan dianggap selesai setelah kamu meninjau data pemesan dan metode pembayaran, lalu mengonfirmasi pesanan. Pada simulasi ini, konfirmasi langsung membuat e-ticket tanpa transaksi uang." }
  ];

  const refundRules = [
    { question: "Apakah tiket bisa dibatalkan atau di-refund?", answer: "Untuk versi produksi, refund dan pembatalan mengikuti kebijakan masing-masing acara. Sebelum membayar, periksa ketentuan refund pada detail konser. Pada versi simulasi ini belum ada transaksi atau proses refund nyata." },
    { question: "Bagaimana jika acara dibatalkan penyelenggara?", answer: "Jika acara dibatalkan atau dijadwalkan ulang, penyelenggara akan menentukan opsi pengembalian dana atau penggunaan tiket. Informasi resmi akan dikirim melalui kontak pemesan yang terdaftar." },
    { question: "Apakah tiket yang sudah dipindai masih bisa digunakan lagi?", answer: "Pada versi produksi, QR valid akan digunakan sekali untuk satu kali check-in. Demo scanner di /admin/scan juga mengubah status tiket lokal menjadi sudah digunakan, tetapi tidak terhubung ke venue atau database produksi." }
  ];

  const wristbandSteps = [
    { question: "Apa yang perlu disiapkan sebelum menuju meja penukaran?", answer: "Pada versi produksi, buka e-ticket valid dan siapkan QR di layar ponsel sebelum menuju meja penukaran wristband. Untuk mencoba alur petugas, buka demo scanner di /admin/scan." },
    { question: "Bagaimana proses penukaran wristband di venue?", answer: "Pada versi produksi, tunjukkan QR valid kepada petugas untuk diverifikasi sebelum menerima wristband sesuai kategori tiket. Pada simulasi ini tidak ada pemindaian, verifikasi, atau check-in venue yang nyata." },
    { question: "Bolehkah menggunakan screenshot QR e-ticket?", answer: "Ikuti aturan penyelenggara pada versi produksi karena screenshot mungkin tidak diterima. QR pada aplikasi ini adalah visual demo, tidak memiliki status check-in, dan tidak dapat digunakan di venue." }
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
