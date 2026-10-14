<script lang="ts">
  import { onMount, tick } from "svelte";
  import { getEvents, ApiError } from "../../lib/api.ts";
  import type { Concert } from "../../lib/concerts.ts";
  import ConcertCard from "../../components/ConcertCard.svelte";
  import ConcertDateRange from "../../components/ConcertDateRange.svelte";
  import { dateRangeError, filterConcerts, type ConcertFilters } from "../../lib/filters.ts";
  let query = $state("");
  let city = $state("");
  let dateFrom = $state("");
  let dateTo = $state("");
  let applied = $state<ConcertFilters>({ query: "", city: "", dateFrom: "", dateTo: "", genres: [], maxPrice: null });
  let searched = $state(false);
  let limit = $state(4);
  let filtersOpen = $state(false);
  let concerts = $state<Concert[]>([]);
  let loading = $state(true);
  let error = $state("");
  let input: HTMLInputElement;
  const cities = $derived([...new Set(concerts.map(({ city }) => city))].sort((a, b) => a.localeCompare(b, "id-ID")));
  const filteredConcerts = $derived(filterConcerts(applied, concerts));
  const matches = $derived(filteredConcerts.slice(0, limit));
  const activeCount = $derived(Number(Boolean(city)) + Number(Boolean(dateFrom)) + Number(Boolean(dateTo)));
  function search(event: SubmitEvent) {
    event.preventDefault();
    if (dateRangeError(dateFrom, dateTo)) return;
    applied = { query, city, dateFrom, dateTo, genres: [], maxPrice: null }; searched = true; limit = 4;
  }
  function reset() { query = ""; city = ""; dateFrom = ""; dateTo = ""; applied = { query, city, dateFrom, dateTo, genres: [], maxPrice: null }; searched = false; limit = 4; input?.focus(); }
  async function more() {
    const nextId = filteredConcerts[limit]?.id;
    limit += 4;
    await tick();
    document.getElementById(`home-${nextId}`)?.querySelector<HTMLAnchorElement>("a")?.focus({ preventScroll: true });
  }
  async function load(signal?: AbortSignal) { loading = true; error = ""; try { concerts = await getEvents(signal); } catch (value) { if (!(value instanceof DOMException && value.name === "AbortError")) error = value instanceof ApiError ? value.message : "Konser belum dapat dimuat."; } finally { if (!signal?.aborted) loading = false; } }
  onMount(() => {
    filtersOpen = window.matchMedia("(min-width: 768px)").matches;
    const controller = new AbortController(); load(controller.signal); return () => controller.abort();
  });
</script>

<svelte:head><title>Tiket Online | Masuk untuk momen yang kamu tunggu</title><meta name="description" content="Cari konser berdasarkan artis, kota, venue, dan tanggal; pilih tiket lalu akses e-ticket setelah pembayaran dikonfirmasi." /></svelte:head>
<section class="hero shell" aria-labelledby="hero-title">
  <div class="hero-copy"><h1 id="hero-title">Konser berikutnya mulai di sini.</h1><p class="hero-summary">Temukan konser di kota dan tanggal pilihanmu, lalu pilih tiketnya.</p></div>
  <form class="event-search" role="search" onsubmit={search}>
    <div class="home-query"><label for="event-query">Cari berdasarkan artis, kota, atau venue</label>
    <div class="search-control"><input bind:this={input} bind:value={query} id="event-query" name="q" type="search" autocomplete="off" placeholder="Misalnya: Jakarta atau Nusa" /></div></div>
    <div class="filter-tools">
      <details class="filters" bind:open={filtersOpen}>
        <summary><strong>Filter</strong><span>{activeCount} filter aktif</span></summary>
        <div class="home-filter-fields">
          <div class="filter-city"><label for="event-city">Kota</label><select bind:value={city} id="event-city" name="city"><option value="">Semua kota</option>{#each cities as option (option)}<option value={option}>{option}</option>{/each}</select></div>
          <ConcertDateRange bind:dateFrom bind:dateTo />
        </div>
      </details>
    </div>
    <div class="search-actions"><button class="button search-submit" type="submit"><svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><circle cx="10.5" cy="10.5" r="6.5" /><path d="m16 16 4.5 4.5" /></svg>Cari</button>{#if searched}<button class="button reset-filters" type="button" onclick={reset}>Reset filter</button>{/if}</div>
  </form>
</section>
<section class="event-section shell" id="konser" aria-labelledby="event-title">
  <div class="section-intro"><h2 id="event-title">Temukan konser pilihanmu</h2></div>
  {#if loading}<p class="sample-note" role="status">Memuat konser...</p>
  {:else if error}<div class="empty-state" role="alert"><p>{error}</p><button class="text-button" type="button" onclick={() => load()}>Coba lagi</button></div>
  {:else if !concerts.length}<p class="empty-state">Belum ada konser di katalog.</p>
  {:else}
    <p class="results-count" aria-live="polite">Menampilkan {matches.length} dari {filteredConcerts.length} konser</p>
    <div class="event-list">{#each matches as concert, index (concert.id)}<div id={`home-${concert.id}`}><ConcertCard {concert} featuredPoster={index === 0} /></div>{/each}</div>
    {#if !matches.length}<div class="empty-state"><p>Tidak ada konser yang cocok dengan filtermu.</p><button class="button reset-filters" type="button" onclick={reset}>Reset filter</button></div>{/if}
    {#if matches.length < filteredConcerts.length}<button class="button load-more" type="button" onclick={more}>Muat lebih banyak</button>{/if}
  {/if}
  <a class="catalog-link" href="/konser">Buka katalog lengkap</a>
</section>
<section class="journey shell" id="cara-kerja" aria-labelledby="journey-title">
  <div class="journey-lead"><h2 id="journey-title">Cara membeli tiket</h2></div>
  <ol class="journey-list"><li><span>01</span><h3>Pilih konser</h3><p>Cari berdasarkan kota, tanggal, atau artis.</p></li><li><span>02</span><h3>Pilih tiket dan bayar</h3><p>Periksa harga dan gunakan metode pembayaran yang tersedia.</p></li><li><span>03</span><h3>Akses e-ticket</h3><p>E-ticket tersedia setelah pembayaran dikonfirmasi.</p></li></ol>
</section>
<section class="payment shell" id="pembayaran" aria-labelledby="payment-title">
  <div class="payment-copy"><h2 id="payment-title">Pilih metode pembayaran</h2><p>Pilihan pembayaran tersedia di checkout. Periksa rincian harga dan biaya sebelum membuat pesanan; e-ticket tersedia setelah pembayaran dikonfirmasi.</p><ul><li>QRIS</li><li>Virtual account</li><li>GoPay</li></ul></div>
</section>
<section class="ticket-section shell" aria-labelledby="ticket-title"><div class="ticket-copy"><h2 id="ticket-title">Sudah membeli tiket?</h2><p>Buka e-ticket dan periksa status pesananmu.</p><div class="ticket-access"><a class="button" href="/tiket-saya">Tiket saya</a><a class="text-button" href="/pulihkan-tiket">Pulihkan akses tiket</a></div></div></section>
