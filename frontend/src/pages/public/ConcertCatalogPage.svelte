<script lang="ts">
  import { onMount, tick } from "svelte";
  import { getEvents, ApiError } from "../../lib/api.ts";
  import { formatRupiah, type Concert, type Genre } from "../../lib/concerts.ts";
  import ConcertCard from "../../components/ConcertCard.svelte";
  import { filterConcerts, sortConcerts, type ConcertSort } from "../../lib/filters.ts";
  import { readCatalogSession, saveCatalogSession } from "../../lib/catalog-session.ts";

  let query = $state("");
  let genres = $state<Genre[]>([]);
  let city = $state("");
  let maxPrice = $state<number | null>(null);
  let priceEnabled = $state(false);
  let priceInput = $state("");
  let priceError = $state(false);
  let sort = $state<ConcertSort>("date");
  let filtersOpen = $state(false);
  let ready = $state(false);
  let restoreScrollY = 0;
  let input: HTMLInputElement;
  let concerts = $state<Concert[]>([]);
  let loading = $state(true);
  let error = $state("");
  const cities = $derived([...new Set(concerts.map(({ city }) => city))].sort((a, b) => a.localeCompare(b, "id-ID")));
  const catalogGenres = $derived([...new Set(concerts.map(({ genre }) => genre))].sort((a, b) => a.localeCompare(b, "id-ID")));
  const matches = $derived(sortConcerts(filterConcerts({ query, genres, city, maxPrice }, concerts), sort));
  const activeCount = $derived(Number(Boolean(query.trim())) + genres.length + Number(Boolean(city)) + Number(maxPrice !== null));
  const filterSummary = $derived([genres.length ? genres.join(", ") : "Semua genre", city || "Semua kota", maxPrice === null ? "Semua harga" : `sampai ${formatRupiah.format(maxPrice)}`].join(" · "));

  function toggleGenre(genre: Genre, checked: boolean) { genres = checked ? [...genres, genre] : genres.filter((value) => value !== genre); }
  function togglePrice(enabled: boolean) {
    priceEnabled = enabled;
    if (!enabled) { maxPrice = null; priceInput = ""; priceError = false; return; }
    const highest = Math.max(0, ...concerts.map(({ price }) => price));
    maxPrice = concerts.length ? highest : null; priceInput = concerts.length ? String(highest) : ""; priceError = !concerts.length;
  }
  function updatePrice(value: string) {
    priceInput = value;
    const parsed = Number(value);
    if (value !== "" && Number.isSafeInteger(parsed) && parsed >= 0) { maxPrice = parsed; priceError = false; }
    else priceError = true;
  }
  function reset() { query = ""; genres = []; city = ""; maxPrice = null; priceEnabled = false; priceInput = ""; priceError = false; sort = "date"; input?.focus(); }
  function save(scrollY = window.scrollY) { saveCatalogSession({ query, genres, city, maxPrice, sort, filtersOpen, scrollY }); }
  async function load(signal?: AbortSignal, restore = false) {
    loading = true; error = "";
    try {
      concerts = await getEvents(signal);
      if (priceEnabled && maxPrice === null && concerts.length) { maxPrice = Math.max(0, ...concerts.map(({ price }) => price)); priceInput = String(maxPrice); priceError = false; }
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
      query = saved.query; genres = saved.genres; city = saved.city; maxPrice = saved.maxPrice;
      priceInput = saved.maxPrice === null ? "" : String(saved.maxPrice); priceEnabled = saved.maxPrice !== null; sort = saved.sort; filtersOpen = saved.filtersOpen; restoreScrollY = saved.scrollY;
    } else filtersOpen = desktop.matches;
    ready = true;
    const persistScroll = () => save();
    const restoreFromCache = (event: PageTransitionEvent) => { if (event.persisted) requestAnimationFrame(() => requestAnimationFrame(() => window.scrollTo(0, restoreScrollY))); };
    window.addEventListener("pagehide", persistScroll);
    window.addEventListener("scroll", persistScroll, { passive: true });
    window.addEventListener("pageshow", restoreFromCache);
    desktop.addEventListener("change", syncFilters);
    void load(controller.signal, Boolean(saved));
    return () => { controller.abort(); window.removeEventListener("pagehide", persistScroll); window.removeEventListener("scroll", persistScroll); window.removeEventListener("pageshow", restoreFromCache); desktop.removeEventListener("change", syncFilters); history.scrollRestoration = previousScrollRestoration; };
  });
</script>

<svelte:head><title>Konser | Tiket Online</title><meta name="description" content="Jelajahi konser berdasarkan genre, kota, dan anggaran di Tiket Online." /></svelte:head>
<div class="catalog">
  <section class="catalog-intro shell" aria-labelledby="catalog-title"><p class="eyebrow">KATALOG KONSER</p><h1 id="catalog-title">Cari panggung yang ingin kamu datangi.</h1><p>Temukan konser dengan cepat, lalu gunakan filter bila perlu.</p></section>
  <section class="catalog-content shell" id="filter" aria-label="Cari dan saring konser">
    <div class="results">
      <form class="catalog-search" role="search" onsubmit={(event) => event.preventDefault()}><label for="concert-query">Cari konser</label><input bind:this={input} bind:value={query} id="concert-query" name="query" type="search" autocomplete="off" placeholder="Artis, venue, atau kota" /></form>
      {#if loading}<p class="results-count" role="status" aria-live="polite">Memuat katalog konser...</p>{:else if error}<div class="empty-state" role="alert"><p>{error}</p><button class="text-button" type="button" onclick={() => load()}>Coba lagi</button></div>{:else}<div class="results-topline"><p class="sample-note">Katalog terbaru.</p><p class="results-count" aria-live="polite">{matches.length ? `${matches.length} konser ditemukan` : "Tidak ada konser ditemukan"}</p></div>{/if}
    </div>
    {#if activeCount}
      <div class="active-filters" aria-label="Filter aktif">
        {#if query.trim()}<button class="filter-chip" type="button" aria-label="Hapus pencarian {query}" onclick={() => query = ""}>Pencarian: {query} <span aria-hidden="true">×</span></button>{/if}
        {#each genres as genre (genre)}<button class="filter-chip" type="button" aria-label="Hapus genre {genre}" onclick={() => genres = genres.filter((value) => value !== genre)}>Genre: {genre} <span aria-hidden="true">×</span></button>{/each}
        {#if city}<button class="filter-chip" type="button" aria-label="Hapus kota {city}" onclick={() => city = ""}>Kota: {city} <span aria-hidden="true">×</span></button>{/if}
        {#if maxPrice !== null}<button class="filter-chip" type="button" aria-label="Hapus batas harga {formatRupiah.format(maxPrice)}" onclick={() => togglePrice(false)}>Maks. {formatRupiah.format(maxPrice)} <span aria-hidden="true">×</span></button>{/if}
      </div>
    {/if}
    <div class="filter-tools">
      <details bind:open={filtersOpen} class="filters">
        <summary><strong>Saring konser</strong><span>{filterSummary}</span></summary>
        <div class="filter-fields">
          <label class="filter-sort" for="concert-sort">Urutkan<select bind:value={sort} id="concert-sort" name="sort"><option value="date">Tanggal terdekat</option><option value="price">Harga terendah</option></select></label>
          <fieldset class="genre-fieldset"><legend>Genre</legend><div class="genre-chips">{#each catalogGenres as genre (genre)}<label><input type="checkbox" name="genre" value={genre} checked={genres.includes(genre)} onchange={(event) => toggleGenre(genre, event.currentTarget.checked)} /><span>{genre}</span></label>{/each}</div></fieldset>
          <div class="filter-city"><label for="concert-city">Kota</label><select bind:value={city} id="concert-city" name="city"><option value="">Semua kota</option>{#each cities as option (option)}<option value={option}>{option}</option>{/each}</select></div>
          <div class="filter-price"><label class="price-toggle"><input type="checkbox" checked={priceEnabled} onchange={(event) => togglePrice(event.currentTarget.checked)} />Batasi harga</label>{#if priceEnabled}<label for="concert-price">Anggaran maksimum</label><div><input bind:value={priceInput} oninput={(event) => updatePrice(event.currentTarget.value)} id="concert-price" name="price" type="text" inputmode="numeric" autocomplete="off" placeholder="Jumlah rupiah" aria-invalid={priceError} aria-describedby={priceError ? "concert-price-error" : undefined} />{#if maxPrice !== null}<output for="concert-price">Sampai {formatRupiah.format(maxPrice)}</output>{/if}{#if priceError}<p id="concert-price-error" class="field-error">Masukkan jumlah rupiah bulat nol atau lebih.</p>{/if}</div>{/if}</div>
        </div>
      </details>
      <button class="text-button reset-filters" type="button" onclick={reset}>Reset filter</button>
    </div>
    {#if !loading && !error}
      <div class="concert-grid" aria-live="polite">{#each matches as concert (concert.id)}<ConcertCard {concert} />{/each}</div>
      {#if !matches.length}<div class="catalog-empty"><p>Tidak ada konser yang cocok dengan filtermu.</p><button class="text-button" type="button" onclick={reset}>Reset filter</button></div>{/if}
    {/if}
  </section>
</div>
