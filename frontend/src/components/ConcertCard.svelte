<script lang="ts">
  import { eventDate, formatRupiah, type Concert } from "../lib/concerts.ts";
  let { concert, variant = "catalog" }: { concert: Concert; variant?: "featured" | "catalog" } = $props();
</script>

{#if variant === "featured"}
  <article class="event-card">
    <a class="event-card-link" href={`/konser/${concert.id}`}>
      <img src={concert.image} alt={`Poster contoh konser ${concert.artist}`} width="900" height="1100" loading="lazy" />
      <div class="event-card-copy"><p>{eventDate(concert.startsAt)}</p><h3>{concert.artist}</h3><span>{concert.venue}, {concert.city}</span></div>
    </a>
  </article>
{:else}
  <article class:is-sold-out={concert.status === "Sold Out"} class="concert-card">
    <a class="concert-card-link" href={`/konser/${concert.id}`}>
      <img src={concert.image} alt={`Poster contoh konser ${concert.artist}`} width="900" height="1100" loading="lazy" />
      <span class="concert-status">{concert.status}</span>
      <div class="concert-card-body"><p class="concert-meta">{eventDate(concert.startsAt)}, {concert.genre}</p><h2>{concert.artist}</h2><p class="concert-venue">{concert.venue}, {concert.city}</p><p class="concert-price">Mulai {formatRupiah.format(concert.price)}</p></div>
    </a>
  </article>
{/if}
