<script lang="ts">
  let { onHome }: { onHome: () => void } = $props();
</script>

<svelte:head>
  <title>Halaman tidak ditemukan | Tiket Online</title>
  <meta name="description" content="Halaman yang kamu cari tidak tersedia di Tiket Online." />
  <meta name="robots" content="noindex" />
</svelte:head>

<section class="not-found-page shell" aria-labelledby="not-found-title">
  <div class="not-found-copy">
    <p class="not-found-kicker">ERROR 404</p>
    <h1 id="not-found-title">Halaman ini tidak ditemukan.</h1>
    <p>Alamat yang kamu buka mungkin salah atau halamannya sudah dipindahkan. Kita bisa mengantarmu kembali ke awal.</p>
    <button class="button" type="button" onclick={onHome}>Kembali ke Beranda</button>
  </div>
  <div class="not-found-number" aria-hidden="true">404</div>
</section>
