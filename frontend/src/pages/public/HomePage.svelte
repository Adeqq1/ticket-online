<script lang="ts">
  import { onMount } from "svelte";
  import { getEvents, ApiError } from "../../lib/api.ts";
  import type { Concert } from "../../lib/concerts.ts";
  import ConcertCard from "../../components/ConcertCard.svelte";
  import { filterEvents } from "../../lib/filters.ts";
  let query = $state("");
  let submittedQuery = $state("");
  let concerts = $state<Concert[]>([]);
  let loading = $state(true);
  let error = $state("");
  let input: HTMLInputElement;
  const filteredConcerts = $derived(filterEvents(submittedQuery, concerts));
  const matches = $derived(submittedQuery.trim() ? filteredConcerts : filteredConcerts.slice(0, 4));
  function reset() { query = ""; submittedQuery = ""; input.focus(); }
  async function load(signal?: AbortSignal) { loading = true; error = ""; try { concerts = await getEvents(signal); } catch (value) { if (!(value instanceof DOMException && value.name === "AbortError")) error = value instanceof ApiError ? value.message : "Konser belum dapat dimuat."; } finally { if (!signal?.aborted) loading = false; } }
  onMount(() => { const controller = new AbortController(); load(controller.signal); return () => controller.abort(); });
</script>

<svelte:head><title>Tiket Online | Masuk untuk momen yang kamu tunggu</title><meta name="description" content="Tiket Online membantu kamu menemukan konser, memilih pembayaran, dan masuk dengan e-ticket." /></svelte:head>
<section class="hero shell" aria-labelledby="hero-title"><div class="hero-copy"><p class="eyebrow">MUSIK, LANGSUNG</p><h1 id="hero-title">Masuk untuk momen yang kamu tunggu.</h1><p class="hero-summary">Temukan konser, bayar sesuai pilihanmu, lalu masuk cukup dengan e-ticket.</p><a class="button" href="#konser">Cari konser</a></div></section>
<section class="event-section shell" id="konser" aria-labelledby="event-title"><div class="section-intro"><h2 id="event-title">Temukan malam berikutnya.</h2><p>Konser yang tersedia dari katalog terbaru.</p></div><form class="event-search" role="search" onsubmit={(event) => { event.preventDefault(); submittedQuery = query; }}><label for="event-query">Cari berdasarkan artis, kota, atau venue</label><div class="search-control"><input bind:this={input} bind:value={query} id="event-query" name="q" type="search" autocomplete="off" placeholder="Misalnya: Jakarta atau Nusa" /><button class="button button-small" type="submit">Cari</button></div></form>{#if loading}<p class="sample-note" role="status" aria-live="polite">Memuat konser...</p>{:else if error}<div class="empty-state" role="alert"><p>{error}</p><button class="text-button" type="button" onclick={() => load()}>Coba lagi</button></div>{:else}<p class="results-count" aria-live="polite">{filteredConcerts.length} konser ditemukan{submittedQuery.trim() ? " untuk pencarian ini" : filteredConcerts.length > matches.length ? ", menampilkan 4 pertama" : ""}</p><div class="event-list">{#each matches as concert (concert.id)}<ConcertCard {concert} variant="featured" />{/each}</div>{#if !matches.length}<div class="empty-state"><p>Tidak ada konser yang cocok dengan pencarianmu.</p><button class="text-button" type="button" onclick={reset}>Tampilkan semua konser</button></div>{/if}{#if !submittedQuery.trim()}<a class="catalog-link" href="/konser">Lihat semua konser</a>{/if}{/if}</section>
<section class="journey shell" id="cara-kerja" aria-labelledby="journey-title"><div class="journey-lead"><h2 id="journey-title">Dari antusias sampai pintu masuk.</h2><p>Satu alur yang jelas untuk tiket yang siap dipakai saat kamu tiba.</p></div><ol class="journey-list"><li><span>01</span><h3>Pilih konser</h3><p>Cari acara yang pas dengan kota dan waktumu.</p></li><li><span>02</span><h3>Pesan tempat</h3><p>Periksa detail dan lanjutkan ke pembayaran.</p></li><li><span>03</span><h3>Bayar pilihanmu</h3><p>Pilih metode yang tersedia untuk acara tersebut.</p></li><li><span>04</span><h3>Tunjukkan e-ticket</h3><p>Scan tiketmu saat check-in di lokasi konser.</p></li></ol></section>
<section class="payment shell" id="pembayaran" aria-labelledby="payment-title"><div class="payment-copy"><h2 id="payment-title">Bayar dengan caramu.</h2><p>Pilih QRIS, virtual account, atau e-wallet yang tersedia sebelum pesanan dikonfirmasi.</p><ul><li>QRIS untuk pembayaran sekali scan</li><li>Virtual account untuk transfer yang terarah</li><li>E-wallet untuk checkout yang ringkas</li></ul><p class="fine-print">Rincian harga dan biaya layanan ditampilkan sebelum kamu membayar.</p></div></section>
<section class="ticket-section shell" aria-labelledby="ticket-title"><div class="ticket-copy"><h2 id="ticket-title">Tiketmu siap saat kamu siap.</h2><p>Simpan e-ticket setelah pembayaran berhasil, lalu tunjukkan kodenya saat check-in.</p><a class="button" href="/konser">Cari konser</a></div></section>
