<script lang="ts">
  import { onMount, tick } from "svelte";
  import { getEvents, ApiError } from "../../lib/api.ts";
  import { formatRupiah, type Concert, type Genre } from "../../lib/concerts.ts";
  import ConcertDateRange from "../../components/ConcertDateRange.svelte";
  import ConcertCard from "../../components/ConcertCard.svelte";
  import { dateRangeError, filterConcerts, sortConcerts, type ConcertFilters, type ConcertSort } from "../../lib/filters.ts";
  import { readCatalogSession, saveCatalogSession } from "../../lib/catalog-session.ts";

  let query = $state("");
  let genres = $state<Genre[]>([]);
  let city = $state("");
  let dateFrom = $state("");
  let dateTo = $state("");
  let maxPrice = $state<number | null>(null);
  let priceEnabled = $state(false);
  let sort = $state<ConcertSort>("date");
  let applied = $state<ConcertFilters>({ query: "", genres: [], city: "", maxPrice: null, dateFrom: "", dateTo: "" });
  let appliedSort = $state<ConcertSort>("date");
  let searched = $state(false);
  let filtersOpen = $state(false);
  let ready = $state(false);
  let restoreScrollY = 0;
  let input: HTMLInputElement;
  let concerts = $state<Concert[]>([]);
  let loading = $state(true);
  let error = $state("");
  const cities = $derived([...new Set(concerts.map(({ city }) => city))].sort((a, b) => a.localeCompare(b, "id-ID")));
  const catalogGenres = $derived([...new Set(concerts.map(({ genre }) => genre))].sort((a, b) => a.localeCompare(b, "id-ID")));
  const highestPrice = $derived(Math.max(0, ...concerts.map(({ price }) => price)));
  const priceCeiling = $derived(Math.ceil(Math.max(highestPrice, applied.maxPrice ?? 0) / 50000) * 50000);
  const matches = $derived(sortConcerts(filterConcerts(applied, concerts), appliedSort));
  const activeCount = $derived(Number(Boolean(applied.query.trim())) + applied.genres.length + Number(Boolean(applied.city)) + Number(applied.maxPrice !== null) + Number(Boolean(applied.dateFrom)) + Number(Boolean(applied.dateTo)));
  const filterSummary = $derived([applied.genres.length ? applied.genres.join(", ") : "Semua genre", applied.city || "Semua kota", applied.maxPrice === null ? "Semua harga" : `sampai ${formatRupiah.format(applied.maxPrice)}`, ...(applied.dateFrom || applied.dateTo ? [`${applied.dateFrom || "Awal"} sampai ${applied.dateTo || "Akhir"} WIB`] : [])].join(" · "));

  function toggleGenre(genre: Genre, checked: boolean) { genres = checked ? [...genres, genre] : genres.filter((value) => value !== genre); }
  function togglePrice(enabled: boolean) {
    priceEnabled = enabled;
    maxPrice = enabled ? priceCeiling : null;
  }
  function updatePrice(value: number) {
    if (Number.isSafeInteger(value) && value >= 0 && value <= priceCeiling && value % 50000 === 0) maxPrice = value;
  }
  function search(event: SubmitEvent) {
    event.preventDefault();
    if (dateRangeError(dateFrom, dateTo)) return;
    applied = { query, genres: [...genres], city, maxPrice, dateFrom, dateTo }; appliedSort = sort; searched = true;
  }
  function reset() { query = ""; genres = []; city = ""; dateFrom = ""; dateTo = ""; maxPrice = null; priceEnabled = false; sort = "date"; applied = { query, genres, city, maxPrice, dateFrom, dateTo }; appliedSort = sort; searched = false; input?.focus(); }
  function save(scrollY = window.scrollY) { saveCatalogSession({ ...applied, dateFrom: applied.dateFrom ?? "", dateTo: applied.dateTo ?? "", sort: appliedSort, filtersOpen, scrollY }); }
  async function load(signal?: AbortSignal, restore = false) {
    loading = true; error = "";
    try {
      concerts = await getEvents(signal);
    } catch (value) { if (!(value instanceof DOMException && value.name === "AbortError")) error = value instanceof ApiError ? value.message : "Katalog belum dapat dimuat."; }
    finally {
      if (!signal?.aborted) {
        loading = false;
        if (restore && !error) { await tick(); window.scrollTo(0, restoreScrollY); }
      }
    }
  }

  $effect(() => { if (ready) save(); });

  onMount(() => {
    const controller = new AbortController();
    const desktop = window.matchMedia("(min-width: 768px)");
    const previousScrollRestoration = history.scrollRestoration;
    const syncFilters = () => { if (!saved) filtersOpen = desktop.matches; };
    history.scrollRestoration = "manual";
    const saved = readCatalogSession();
    if (saved) {
      query = saved.query; genres = saved.genres; city = saved.city; dateFrom = saved.dateFrom; dateTo = saved.dateTo; maxPrice = saved.maxPrice;
      priceEnabled = saved.maxPrice !== null; sort = saved.sort; filtersOpen = saved.filtersOpen; restoreScrollY = saved.scrollY;
      applied = { query, genres: [...genres], city, maxPrice, dateFrom, dateTo }; appliedSort = sort; searched = Boolean(query.trim() || genres.length || city || maxPrice !== null || dateFrom || dateTo || sort !== "date");
      if (maxPrice !== null) maxPrice = Math.ceil(maxPrice / 50000) * 50000;
    } else filtersOpen = desktop.matches;
    ready = true;
    const persistScroll = () => save();
    const persistBeforeLeave = () => { restoreScrollY = window.scrollY; save(restoreScrollY); };
    const restoreFromCache = (event: PageTransitionEvent) => { if (event.persisted) requestAnimationFrame(() => requestAnimationFrame(() => window.scrollTo(0, restoreScrollY))); };
    window.addEventListener("pagehide", persistBeforeLeave);
    window.addEventListener("scroll", persistScroll, { passive: true });
    window.addEventListener("pageshow", restoreFromCache);
    desktop.addEventListener("change", syncFilters);
    void load(controller.signal, Boolean(saved));
    return () => { controller.abort(); window.removeEventListener("pagehide", persistBeforeLeave); window.removeEventListener("scroll", persistScroll); window.removeEventListener("pageshow", restoreFromCache); desktop.removeEventListener("change", syncFilters); history.scrollRestoration = previousScrollRestoration; };
  });
</script>

<svelte:head><title>Konser | Tiket Online</title><meta name="description" content="Jelajahi konser berdasarkan genre, kota, dan anggaran di Tiket Online." /></svelte:head>
<div class="catalog">
  <section class="catalog-intro shell" aria-labelledby="catalog-title"><p class="eyebrow">KATALOG KONSER</p><h1 id="catalog-title">Temukan konser pilihanmu</h1><p>Temukan konser dengan cepat, lalu gunakan filter bila perlu.</p></section>
  <section class="catalog-content shell" id="filter" aria-label="Cari dan saring konser">
    <form class="catalog-form" role="search" onsubmit={search}>
    <div class="results">
      <div class="catalog-search"><label for="concert-query">Cari konser</label><div class="catalog-search-row"><input bind:this={input} bind:value={query} id="concert-query" name="query" type="search" autocomplete="off" placeholder="Artis, venue, atau kota" />
      <div class="search-actions"><button class="button search-submit" type="submit" aria-label="Cari" title="Cari"><svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><circle cx="10.5" cy="10.5" r="6.5" /><path d="m16 16 4.5 4.5" /></svg></button>{#if searched}<button class="button reset-filters" type="button" onclick={reset}>Reset filter</button>{/if}</div>
      </div></div>
      {#if loading}<p class="results-count" role="status" aria-live="polite">Memuat katalog konser...</p>{:else if error}<div class="empty-state" role="alert"><p>{error}</p><button class="text-button" type="button" onclick={() => load()}>Coba lagi</button></div>{:else}<div class="results-topline"><p class="sample-note">Katalog terbaru.</p><p class="results-count" aria-live="polite">{matches.length ? `${matches.length} konser ditemukan` : "Tidak ada konser ditemukan"}</p></div>{/if}
    </div>
    {#if activeCount}
      <div class="active-filters" aria-label="Filter aktif">
        {#if applied.query.trim()}<button class="filter-chip" type="button" aria-label="Hapus pencarian {applied.query}" onclick={() => { query = ""; applied = { ...applied, query }; }}>Pencarian: {applied.query} <span aria-hidden="true">×</span></button>{/if}
        {#each applied.genres as genre (genre)}<button class="filter-chip" type="button" aria-label="Hapus genre {genre}" onclick={() => { genres = genres.filter((value) => value !== genre); applied = { ...applied, genres: applied.genres.filter((value) => value !== genre) }; }}>Genre: {genre} <span aria-hidden="true">×</span></button>{/each}
        {#if applied.city}<button class="filter-chip" type="button" aria-label="Hapus kota {applied.city}" onclick={() => { city = ""; applied = { ...applied, city }; }}>Kota: {applied.city} <span aria-hidden="true">×</span></button>{/if}
        {#if applied.dateFrom}<button class="filter-chip" type="button" onclick={() => { dateFrom = ""; applied = { ...applied, dateFrom }; }} aria-label="Hapus tanggal mulai">Mulai: {applied.dateFrom} <span aria-hidden="true">×</span></button>{/if}
        {#if applied.dateTo}<button class="filter-chip" type="button" onclick={() => { dateTo = ""; applied = { ...applied, dateTo }; }} aria-label="Hapus tanggal akhir">Akhir: {applied.dateTo} <span aria-hidden="true">×</span></button>{/if}
        {#if applied.maxPrice !== null}<button class="filter-chip" type="button" aria-label="Hapus batas harga {formatRupiah.format(applied.maxPrice)}" onclick={() => { togglePrice(false); applied = { ...applied, maxPrice: null }; }}>Maks. {formatRupiah.format(applied.maxPrice)} <span aria-hidden="true">×</span></button>{/if}
      </div>
    {/if}
    <div class="filter-tools">
      <details bind:open={filtersOpen} class="filters">
        <summary><strong>Filter ({activeCount} aktif)</strong><span>{filterSummary}</span></summary>
        <div class="filter-fields">
          <label class="filter-sort" for="concert-sort">Urutkan<select bind:value={sort} id="concert-sort" name="sort"><option value="date">Tanggal terdekat</option><option value="price">Harga terendah</option></select></label>
          <fieldset class="genre-fieldset"><legend>Genre</legend><div class="genre-chips">{#each catalogGenres as genre (genre)}<label><input type="checkbox" name="genre" value={genre} checked={genres.includes(genre)} onchange={(event) => toggleGenre(genre, event.currentTarget.checked)} /><span>{genre}</span></label>{/each}</div></fieldset>
          <div class="filter-city"><label for="concert-city">Kota</label><select bind:value={city} id="concert-city" name="city"><option value="">Semua kota</option>{#each cities as option (option)}<option value={option}>{option}</option>{/each}</select></div>
          <div class="filter-dates"><ConcertDateRange bind:dateFrom bind:dateTo /></div>
          <div class="filter-price"><label class="price-toggle"><input type="checkbox" checked={priceEnabled} disabled={loading || !concerts.length} onchange={(event) => togglePrice(event.currentTarget.checked)} />Batasi harga</label><div><input id="concert-price" name="price" type="range" min="0" max={priceCeiling} step="50000" value={Math.min(maxPrice ?? priceCeiling, priceCeiling)} oninput={(event) => updatePrice(event.currentTarget.valueAsNumber)} aria-label="Anggaran maksimum" aria-valuetext={maxPrice === null ? "Semua harga" : formatRupiah.format(maxPrice)} aria-describedby="concert-price-value" disabled={!priceEnabled || loading || !concerts.length || priceCeiling === 0} /><output id="concert-price-value" for="concert-price">{maxPrice === null ? "Semua harga" : `Sampai ${formatRupiah.format(maxPrice)}`}</output></div></div>
        </div>
      </details>
    </div>
    </form>
    {#if !loading && !error}
      <div class="concert-grid" aria-live="polite">{#each matches as concert, index (concert.id)}<ConcertCard {concert} featuredPoster={index === 0} />{/each}</div>
      {#if !concerts.length}<p class="catalog-empty">Belum ada konser di katalog.</p>{:else if dateRangeError(applied.dateFrom, applied.dateTo)}<p class="catalog-empty" role="status">Perbaiki rentang tanggal untuk melihat konser.</p>{:else if !matches.length}<div class="catalog-empty"><p>Tidak ada konser yang cocok dengan filtermu.</p><button class="button reset-filters" type="button" onclick={reset}>Reset filter</button></div>{/if}
    {/if}
  </section>
</div>
