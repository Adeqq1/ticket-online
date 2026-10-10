<script lang="ts">
  import { eventDate, formatRupiah, type Concert } from "../lib/concerts.ts";
  import { eventStatus } from "../lib/event-changes.ts";
  let { concert, variant = "catalog", featuredPoster = false }: { concert: Concert; variant?: "featured" | "catalog"; featuredPoster?: boolean } = $props();
  const changedStatus = $derived(eventStatus(concert.currentEvent));
  const available = $derived(concert.ticketTiers.length > 0 && concert.status !== "Sold Out" && !concert.currentEvent?.salesPaused);
</script>

<article class:is-sold-out={!available} class="concert-card">
  <a class="concert-card-link" href={`/konser/${concert.id}`}>
    <img class="concert-poster" src={concert.image} alt={`Poster konser ${concert.artist}`} width="900" height="1100" loading={featuredPoster ? "eager" : "lazy"} fetchpriority={featuredPoster ? "high" : "auto"} onerror={(event) => event.currentTarget.classList.add("poster-unavailable")} />
    <div class="concert-card-body">
      {#if changedStatus}<p class="concert-change">{changedStatus}</p>{/if}
      <p class="concert-meta">{eventDate(concert.currentEvent ? concert.currentEvent.startsAt : concert.startsAt)}{variant === "catalog" ? ` · ${concert.genre}` : ""}</p>
      <h3>{concert.artist}</h3>
      <p class="concert-venue">{concert.venue}, {concert.city}</p>
      <p class="concert-price">{concert.ticketTiers.length ? `Mulai ${formatRupiah.format(Math.min(...concert.ticketTiers.map((tier) => tier.price)))}` : "Tiket belum tersedia"}</p>
      <span class="concert-status">{available ? concert.status : concert.status === "Sold Out" ? "Habis" : "Tiket belum tersedia"}</span>
    </div>
  </a>
</article>
