<script lang="ts">
  import { eventDate, formatRupiah, type Concert } from "../lib/concerts.ts";
  import { canBuy, eventStatus } from "../lib/event-changes.ts";
  let { concert, featuredPoster = false }: { concert: Concert; featuredPoster?: boolean } = $props();
  const changedStatus = $derived(eventStatus(concert.currentEvent));
  let failedImage = $state("");
  const available = $derived(concert.ticketTiers.some((tier) => tier.stock > 0) && concert.status !== "Sold Out" && canBuy(concert.currentEvent));
</script>

<article class:is-sold-out={!available} class="concert-card">
  <a class="concert-card-link" href={`/konser/${concert.id}`}>
    <div class="concert-poster-frame">
      {#if concert.image && failedImage !== concert.image}
        <img class="concert-poster" src={concert.image} alt={`Poster konser ${concert.artist}`} width="400" height="500" loading={featuredPoster ? "eager" : "lazy"} fetchpriority={featuredPoster ? "high" : "auto"} onerror={() => failedImage = concert.image} />
      {:else}<span class="poster-fallback">Poster belum tersedia</span>{/if}
    </div>
    <div class="concert-card-body">
      <h3>{concert.artist}</h3>
      <p class="concert-meta">{eventDate(concert.currentEvent ? concert.currentEvent.startsAt : concert.startsAt)}</p>
      <p class="concert-venue">{concert.venue}, {concert.city}</p>
      <p class="concert-price">{concert.ticketTiers.length ? `Mulai ${formatRupiah.format(Math.min(...concert.ticketTiers.map((tier) => tier.price)))}` : "Tiket belum tersedia"}</p>
      {#if changedStatus}<p class="concert-change">{changedStatus}</p>{/if}
      {#if concert.currentEvent?.salesPaused}<p class="concert-change">Penjualan dihentikan</p>{/if}
      <span class="concert-status">{available ? concert.status : concert.status === "Sold Out" || concert.ticketTiers.length && !concert.ticketTiers.some((tier) => tier.stock > 0) ? "Habis" : canBuy(concert.currentEvent) ? "Tiket belum tersedia" : "Penjualan ditutup"}</span>
      <span class="concert-action">Lihat tiket<span class="sr-only"> {concert.artist}</span></span>
    </div>
  </a>
</article>
