<script lang="ts">
  let { page = "" }: { page?: string } = $props();
</script>

<a class="skip-link" href="#konten">Lewati ke konten</a>
<header class="site-header">
  <nav class="nav shell" aria-label="Navigasi utama">
    <a class="wordmark" href="/" aria-label="Tiket Online, beranda"><span class="wordmark-mark" aria-hidden="true">TO</span><span>Tiket Online</span></a>
    <div class="nav-links"><a href="/konser" aria-current={page === "concerts" ? "page" : undefined}>Konser</a><a href="/tiket-saya" aria-current={page === "my-tickets" ? "page" : undefined}>Tiket saya</a><a href="/panduan#cara-kerja" aria-current={page === "guide" ? "page" : undefined}>Cara kerja</a><a href="/panduan#pembayaran">Pembayaran</a></div>
    {#if page === "home"}<a class="button button-small" href="/konser">Cari konser</a>
    {:else if page === "concerts"}<a class="button button-small" href="#filter">Filter konser</a>
    {:else}<a class="button button-small" href="/konser">Katalog konser</a>{/if}
  </nav>
</header>
