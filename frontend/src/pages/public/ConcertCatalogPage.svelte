<script lang="ts">
  import { onMount } from "svelte";
  import { getEvents, ApiError } from "../../lib/api.ts";
  import { formatRupiah, type Concert, type Genre } from "../../lib/concerts.ts";
  import ConcertCard from "../../components/ConcertCard.svelte";
  import { filterConcerts } from "../../lib/filters.ts";

  let query = $state("");
  let genres = $state<Genre[]>([]);
  let city = $state("");
  let maxPrice = $state(1_500_000);
  let input: HTMLInputElement;
  let filtersDetails: HTMLDetailsElement;
  let concerts = $state<Concert[]>([]);
  let loading = $state(true);
  let error = $state("");
  const matches = $derived(filterConcerts({ query, genres, city, maxPrice }, concerts));
  const filterSummary = $derived([genres.length ? genres.join(", ") : "Semua genre", city || "Semua kota", `sampai ${formatRupiah.format(maxPrice)}`].join(" · "));
  function toggleGenre(genre: Genre, checked: boolean) { genres = checked ? [...genres, genre] : genres.filter((value) => value !== genre); }
  function reset() { query = ""; genres = []; city = ""; maxPrice = 1_500_000; input?.focus(); }
  async function load(signal?: AbortSignal) {
    loading = true; error = "";
    try { concerts = await getEvents(signal); }
    catch (value) { if (!(value instanceof DOMException && value.name === "AbortError")) error = value instanceof ApiError ? value.message : "Katalog belum dapat dimuat."; }
    finally { if (!signal?.aborted) loading = false; }
  }
  onMount(() => {
    const controller = new AbortController();
    const desktop = window.matchMedia("(min-width: 768px)");
    const syncFilters = () => { filtersDetails.open = desktop.matches; };
    syncFilters();
    desktop.addEventListener("change", syncFilters);
    load(controller.signal);
    return () => { controller.abort(); desktop.removeEventListener("change", syncFilters); };
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
    <div class="filter-tools">
      <details bind:this={filtersDetails} class="filters">
        <summary><strong>Saring konser</strong><span>{filterSummary}</span></summary>
        <div class="filter-fields">
          <fieldset class="genre-fieldset"><legend>Genre</legend><div class="genre-chips">{#each ["Rock", "Pop", "Indie"] as genre}<label><input type="checkbox" name="genre" value={genre} checked={genres.includes(genre as Genre)} onchange={(event) => toggleGenre(genre as Genre, event.currentTarget.checked)} /><span>{genre}</span></label>{/each}</div></fieldset>
          <div class="filter-city"><label for="concert-city">Kota</label><select bind:value={city} id="concert-city" name="city"><option value="">Semua kota</option><option>Jakarta</option><option>Bandung</option><option>Yogyakarta</option><option>Surabaya</option></select></div>
          <div class="filter-price"><label for="concert-price">Anggaran maksimum</label><div><output for="concert-price">Sampai {formatRupiah.format(maxPrice)}</output><input bind:value={maxPrice} id="concert-price" name="price" type="range" min="100000" max="1500000" step="50000" /></div></div>
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
